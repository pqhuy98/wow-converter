package m2

import (
	"fmt"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

// TrackDataType is an M2 animation track value type.
type TrackDataType string

const (
	TrackUint8    TrackDataType = "uint8"
	TrackUint16   TrackDataType = "uint16"
	TrackInt16    TrackDataType = "int16"
	TrackUint32   TrackDataType = "uint32"
	TrackFloat    TrackDataType = "float"
	TrackFloat2   TrackDataType = "float2"
	TrackFloat3   TrackDataType = "float3"
	TrackFloat4   TrackDataType = "float4"
	TrackCompQuat TrackDataType = "compquat"
)

// Track is an M2 animation track.
type Track struct {
	GlobalSeq     uint16
	Interpolation uint16
	Timestamps    [][]uint32
	Values        [][][]float64
}

// CAABox is an axis-aligned bounding box.
type CAABox struct {
	Min, Max [3]float32
}

func readTrackValue(data *buffer.Buffer, dt TrackDataType) []float64 {
	switch dt {
	case TrackUint32:
		return []float64{float64(readU32(data))}
	case TrackUint16:
		return []float64{float64(readU16(data))}
	case TrackInt16:
		v := data.ReadInt16LE().(int64)
		return []float64{float64(int16(v))}
	case TrackFloat:
		return []float64{float64(data.ReadFloatLE().(float32))}
	case TrackFloat2:
		raw := data.ReadFloatLE(2).([]float32)
		return []float64{float64(raw[0]), float64(raw[1])}
	case TrackFloat3:
		raw := data.ReadFloatLE(3).([]float32)
		return []float64{float64(raw[0]), float64(raw[1]), float64(raw[2])}
	case TrackFloat4:
		raw := data.ReadFloatLE(4).([]float32)
		return []float64{float64(raw[0]), float64(raw[1]), float64(raw[2]), float64(raw[3])}
	case TrackCompQuat:
		raw := data.ReadUInt16LE(4).([]int64)
		out := make([]float64, 4)
		for i, e := range raw {
			v := int(e)
			if v < 0 {
				v += 32768
			} else {
				v -= 32767
			}
			out[i] = float64(v) / 32767
		}
		return out
	case TrackUint8:
		return []float64{float64(data.ReadUInt8().(int64))}
	default:
		panic(fmt.Sprintf("unknown track data type: %s", dt))
	}
}

func trackValueSize(dt TrackDataType) int {
	switch dt {
	case TrackUint8:
		return 1
	case TrackUint16, TrackInt16:
		return 2
	case TrackCompQuat:
		return 8
	case TrackUint32, TrackFloat:
		return 4
	case TrackFloat2:
		return 8
	case TrackFloat3:
		return 12
	case TrackFloat4:
		return 16
	default:
		panic(fmt.Sprintf("unknown track data type: %s", dt))
	}
}

func readM2ArrayArray(data *buffer.Buffer, ofs int, dt TrackDataType, animFiles map[int]*buffer.Buffer) [][][]float64 {
	arrCount := int(readU32(data))
	arrOfs := int(readU32(data))
	base := data.Offset()
	if arrCount == 0 || arrOfs == 0 {
		data.Seek(base)
		return nil
	}
	absOfs := ofs + arrOfs
	if absOfs < 0 || absOfs > data.ByteLength() {
		data.Seek(base)
		return nil
	}
	data.Seek(absOfs)

	arr := make([][][]float64, arrCount)
	for i := 0; i < arrCount; i++ {
		subCount := int(readU32(data))
		subOfs := int(readU32(data))
		subBase := data.Offset()
		data.Seek(ofs + subOfs)
		arr[i] = make([][]float64, subCount)
		animFile := animFiles[i]
		for j := 0; j < subCount; j++ {
			if animFile != nil {
				saved := animFile.Offset()
				animFile.Seek(subOfs + j*trackValueSize(dt))
				arr[i][j] = readTrackValue(animFile, dt)
				animFile.Seek(saved)
			} else {
				arr[i][j] = readTrackValue(data, dt)
			}
		}
		data.Seek(subBase)
	}
	data.Seek(base)
	return arr
}

func readM2Timestamps(data *buffer.Buffer, ofs int, animFiles map[int]*buffer.Buffer) [][]uint32 {
	raw := readM2ArrayArray(data, ofs, TrackUint32, animFiles)
	out := make([][]uint32, len(raw))
	for i, sub := range raw {
		out[i] = make([]uint32, len(sub))
		for j, v := range sub {
			if len(v) > 0 {
				out[i][j] = uint32(v[0])
			}
		}
	}
	return out
}

// ReadM2Track reads a standard M2 animation block.
func ReadM2Track(data *buffer.Buffer, ofs int, dt TrackDataType, animFiles map[int]*buffer.Buffer) Track {
	interp := readU16(data)
	globalSeq := readU16(data)
	return Track{
		GlobalSeq:     globalSeq,
		Interpolation: interp,
		Timestamps:    readM2Timestamps(data, ofs, animFiles),
		Values:        readM2ArrayArray(data, ofs, dt, animFiles),
	}
}

func readCAABBox(data *buffer.Buffer) CAABox {
	minRaw := data.ReadFloatLE(3).([]float32)
	maxRaw := data.ReadFloatLE(3).([]float32)
	return CAABox{
		Min: [3]float32{minRaw[0], minRaw[1], minRaw[2]},
		Max: [3]float32{maxRaw[0], maxRaw[1], maxRaw[2]},
	}
}

func readM2ArrayU16(data *buffer.Buffer, ofs int) []uint16 {
	arrCount := int(readU32(data))
	arrOfs := int(readU32(data))
	base := data.Offset()
	out := make([]uint16, arrCount)
	if arrCount > 0 && arrOfs > 0 {
		data.Seek(ofs + arrOfs)
		for i := 0; i < arrCount; i++ {
			out[i] = readU16(data)
		}
	}
	data.Seek(base)
	return out
}

func readM2ArrayU8(data *buffer.Buffer, ofs int) []uint8 {
	arrCount := int(readU32(data))
	arrOfs := int(readU32(data))
	base := data.Offset()
	out := make([]uint8, arrCount)
	if arrCount > 0 && arrOfs > 0 {
		data.Seek(ofs + arrOfs)
		for i := 0; i < arrCount; i++ {
			out[i] = uint8(data.ReadUInt8().(int64))
		}
	}
	data.Seek(base)
	return out
}

func readM2ArrayFloat(data *buffer.Buffer, ofs int) []float32 {
	arrCount := int(readU32(data))
	arrOfs := int(readU32(data))
	base := data.Offset()
	out := make([]float32, arrCount)
	if arrCount > 0 && arrOfs > 0 {
		data.Seek(ofs + arrOfs)
		for i := 0; i < arrCount; i++ {
			out[i] = data.ReadFloatLE().(float32)
		}
	}
	data.Seek(base)
	return out
}

func readM2ArrayU32(data *buffer.Buffer, ofs int) []uint32 {
	arrCount := int(readU32(data))
	arrOfs := int(readU32(data))
	base := data.Offset()
	out := make([]uint32, arrCount)
	if arrCount > 0 && arrOfs > 0 {
		data.Seek(ofs + arrOfs)
		for i := 0; i < arrCount; i++ {
			out[i] = readU32(data)
		}
	}
	data.Seek(base)
	return out
}

func trackValueSizeFor(dt TrackDataType) int { return trackValueSize(dt) }

func readM2ArrayValues(data *buffer.Buffer, ofs int, dt TrackDataType) [][]float64 {
	arrCount := int(readU32(data))
	arrOfs := int(readU32(data))
	base := data.Offset()
	out := make([][]float64, arrCount)
	if arrCount > 0 && arrOfs > 0 {
		data.Seek(ofs + arrOfs)
		for i := 0; i < arrCount; i++ {
			out[i] = readTrackValue(data, dt)
		}
	}
	data.Seek(base)
	return out
}

// ReadM2PartTrack reads an age-based M2 particle track.
func ReadM2PartTrack(data *buffer.Buffer, ofs int, dt TrackDataType) PartTrack {
	tsRaw := readM2ArrayU16(data, ofs)
	ts := make([]uint16, len(tsRaw))
	copy(ts, tsRaw)
	return PartTrack{Timestamps: ts, Values: readM2ArrayValues(data, ofs, dt)}
}

func readSplineKey(data *buffer.Buffer, dt TrackDataType) []float64 {
	switch dt {
	case TrackFloat:
		return []float64{
			float64(data.ReadFloatLE().(float32)),
			float64(data.ReadFloatLE().(float32)),
			float64(data.ReadFloatLE().(float32)),
		}
	case TrackFloat3:
		v := data.ReadFloatLE(3).([]float32)
		it := data.ReadFloatLE(3).([]float32)
		ot := data.ReadFloatLE(3).([]float32)
		return []float64{
			float64(v[0]), float64(v[1]), float64(v[2]),
			float64(it[0]), float64(it[1]), float64(it[2]),
			float64(ot[0]), float64(ot[1]), float64(ot[2]),
		}
	default:
		panic(fmt.Sprintf("unsupported spline key type: %s", dt))
	}
}

func readM2SplineArrayArray(data *buffer.Buffer, ofs int, dt TrackDataType) [][][]float64 {
	arrCount := int(readU32(data))
	arrOfs := int(readU32(data))
	base := data.Offset()
	data.Seek(ofs + arrOfs)
	arr := make([][][]float64, arrCount)
	for i := 0; i < arrCount; i++ {
		subCount := int(readU32(data))
		subOfs := int(readU32(data))
		subBase := data.Offset()
		data.Seek(ofs + subOfs)
		arr[i] = make([][]float64, subCount)
		for j := 0; j < subCount; j++ {
			arr[i][j] = readSplineKey(data, dt)
		}
		data.Seek(subBase)
	}
	data.Seek(base)
	return arr
}

// ReadM2SplineTrack reads an M2 spline animation block (e.g. camera tracks).
func ReadM2SplineTrack(data *buffer.Buffer, ofs int, dt TrackDataType) Track {
	interp := readU16(data)
	globalSeq := readU16(data)
	return Track{
		GlobalSeq:     globalSeq,
		Interpolation: interp,
		Timestamps:    readM2Timestamps(data, ofs, nil),
		Values:        readM2SplineArrayArray(data, ofs, dt),
	}
}
