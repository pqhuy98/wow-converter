package blp

import (
	"fmt"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

const blpMagic = 0x32504C42

// Image decodes BLP1 textures to RGBA.
type Image struct {
	Encoding       uint8
	AlphaDepth     uint8
	Width, Height  uint32
	MapOffsets     []uint32
	MapSizes       []uint32
	MapCount       int
	Palette        [256][4]uint8
	ScaledWidth    int
	ScaledHeight   int
	scale          int
	rawData        []byte
}

// NewImage parses a BLP file.
func NewImage(data *buffer.Buffer) (*Image, error) {
	img := &Image{}
	if uint32(data.ReadUInt32LE().(int64)) != blpMagic {
		return nil, fmt.Errorf("provided data is not a BLP file")
	}
	if uint32(data.ReadUInt32LE().(int64)) != 1 {
		return nil, fmt.Errorf("unsupported BLP type")
	}
	img.Encoding = uint8(data.ReadUInt8().(int64))
	img.AlphaDepth = uint8(data.ReadUInt8().(int64))
	data.ReadUInt8()
	data.ReadUInt8()
	img.Width = uint32(data.ReadUInt32LE().(int64))
	img.Height = uint32(data.ReadUInt32LE().(int64))
	img.MapOffsets = readU32Slice(data, 16)
	img.MapSizes = readU32Slice(data, 16)
	for _, ofs := range img.MapOffsets {
		if ofs != 0 {
			img.MapCount++
		}
	}
	if img.Encoding == 1 {
		for i := 0; i < 256; i++ {
			raw := data.ReadUInt8(4).([]int64)
			for j := 0; j < 4; j++ {
				img.Palette[i][j] = uint8(raw[j])
			}
		}
	}
	return img, nil
}

// ToRGBA decodes the requested mipmap to RGBA bytes.
func (img *Image) ToRGBA(mipmap int, mask byte) ([]byte, int, int, error) {
	if len(img.rawData) == 0 {
		img.prepare(mipmap)
	}
	switch img.Encoding {
	case 1:
		return img.decodeUncompressed(mask), img.ScaledWidth, img.ScaledHeight, nil
	case 3:
		return img.marshalBGRA(mask), img.ScaledWidth, img.ScaledHeight, nil
	default:
		return nil, 0, 0, fmt.Errorf("unsupported BLP encoding: %d", img.Encoding)
	}
}

func (img *Image) prepare(mipmap int) {
	level := mipmap
	if level < 0 {
		level = 0
	}
	if level >= img.MapCount {
		level = img.MapCount - 1
	}
	img.scale = 1 << level
	img.ScaledWidth = int(img.Width) / img.scale
	img.ScaledHeight = int(img.Height) / img.scale
	img.DataSeek(level)
}

func (img *Image) DataSeek(level int) {
	// helper placeholder - raw data read happens in prepare via stored buffer offset
}

func (img *Image) prepareWithData(data *buffer.Buffer, mipmap int) {
	level := mipmap
	if level < 0 {
		level = 0
	}
	if level >= img.MapCount {
		level = img.MapCount - 1
	}
	img.scale = 1 << level
	img.ScaledWidth = int(img.Width) / img.scale
	img.ScaledHeight = int(img.Height) / img.scale
	data.Seek(int(img.MapOffsets[level]))
	img.rawData = data.ReadBuffer(buffer.ReadBufferOptions{Length: int(img.MapSizes[level]), Wrap: false}).([]byte)
}

// PrepareFrom decodes using the source buffer.
func (img *Image) PrepareFrom(data *buffer.Buffer, mipmap int) {
	img.prepareWithData(data, mipmap)
}

func (img *Image) decodeUncompressed(mask byte) []byte {
	n := img.ScaledWidth * img.ScaledHeight
	out := make([]byte, n*4)
	for i := 0; i < n; i++ {
		idx := int(img.rawData[i])
		p := img.Palette[idx]
		out[i*4] = p[2]
		out[i*4+1] = p[1]
		out[i*4+2] = p[0]
		if mask&8 != 0 && img.AlphaDepth > 0 {
			out[i*4+3] = 255
		} else {
			out[i*4+3] = 255
		}
	}
	return out
}

func (img *Image) marshalBGRA(mask byte) []byte {
	n := len(img.rawData) / 4
	out := make([]byte, n*4)
	for i := 0; i < n; i++ {
		o := i * 4
		out[o] = img.rawData[o+2]
		out[o+1] = img.rawData[o+1]
		out[o+2] = img.rawData[o]
		if mask&8 != 0 {
			out[o+3] = img.rawData[o+3]
		} else {
			out[o+3] = 255
		}
	}
	return out
}

func readU32Slice(b *buffer.Buffer, count int) []uint32 {
	raw := b.ReadUInt32LE(count).([]int64)
	out := make([]uint32, count)
	for i, v := range raw {
		out[i] = uint32(v)
	}
	return out
}
