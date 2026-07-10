package mdx

import "fmt"

const (
	InterpolationDontInterp = 0
	InterpolationLinear       = 1
	InterpolationHermite      = 2
	InterpolationBezier       = 3
)

// Animation holds keyframe animation data.
type Animation struct {
	Name              string
	InterpolationType int
	GlobalSequenceID  int32
	Frames            []int32
	Values            []interface{} // []float32 or []uint32
	InTans            []interface{}
	OutTans           []interface{}
	valueKind         int
}

const (
	animValueUint = iota
	animValueFloat
	animValueVec3
	animValueVec4
)

func newAnimation(kind int) *Animation {
	return &Animation{valueKind: kind, GlobalSequenceID: -1}
}

func (a *Animation) readMdxValue(stream *BinaryStream) (interface{}, error) {
	switch a.valueKind {
	case animValueUint:
		v, err := stream.ReadUint32Array(1)
		if err != nil {
			return nil, err
		}
		return v, nil
	case animValueFloat:
		v, err := stream.ReadFloat32Array(1)
		if err != nil {
			return nil, err
		}
		return v, nil
	case animValueVec3:
		return stream.ReadFloat32Array(3)
	case animValueVec4:
		return stream.ReadFloat32Array(4)
	default:
		return nil, fmt.Errorf("unknown animation value kind")
	}
}

func (a *Animation) writeMdxValue(stream *BinaryStream, value interface{}) {
	switch v := value.(type) {
	case []uint32:
		stream.WriteUint32(v[0])
	case []float32:
		stream.WriteFloat32Array(v)
	}
}

func (a *Animation) readMdlValue(stream *TokenStream) (interface{}, error) {
	switch a.valueKind {
	case animValueUint:
		n, err := stream.ReadInt()
		if err != nil {
			return nil, err
		}
		return []uint32{uint32(n)}, nil
	case animValueFloat:
		f, err := stream.ReadFloat()
		if err != nil {
			return nil, err
		}
		return []float32{float32(f)}, nil
	case animValueVec3:
		view := make([]float32, 3)
		return stream.ReadVector(view)
	case animValueVec4:
		view := make([]float32, 4)
		return stream.ReadVector(view)
	default:
		return nil, fmt.Errorf("unknown animation value kind")
	}
}

func (a *Animation) writeMdlValue(stream *TokenStream, name string, value interface{}) {
	switch v := value.(type) {
	case []uint32:
		stream.WriteNumberAttrib(name, float64(v[0]))
	case []float32:
		if len(v) == 1 {
			stream.WriteNumberAttrib(name, float64(v[0]))
		} else {
			stream.WriteVectorAttrib(name, v)
		}
	}
}

func (a *Animation) ReadMdx(stream *BinaryStream, name string) error {
	tracksCount, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	interpolationType, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	globalSeq, err := stream.ReadInt32()
	if err != nil {
		return err
	}

	a.Name = name
	a.InterpolationType = int(interpolationType)
	a.GlobalSequenceID = globalSeq
	a.Frames = make([]int32, tracksCount)
	a.Values = make([]interface{}, tracksCount)

	if interpolationType > 1 {
		a.InTans = make([]interface{}, tracksCount)
		a.OutTans = make([]interface{}, tracksCount)
	}

	for i := uint32(0); i < tracksCount; i++ {
		frame, err := stream.ReadInt32()
		if err != nil {
			return err
		}
		a.Frames[i] = frame
		val, err := a.readMdxValue(stream)
		if err != nil {
			return err
		}
		a.Values[i] = val
		if interpolationType > 1 {
			inTan, err := a.readMdxValue(stream)
			if err != nil {
				return err
			}
			outTan, err := a.readMdxValue(stream)
			if err != nil {
				return err
			}
			a.InTans[i] = inTan
			a.OutTans[i] = outTan
		}
	}
	return nil
}

func (a *Animation) WriteMdx(stream *BinaryStream) {
	tracksCount := len(a.Frames)
	stream.WriteBinary(a.Name)
	stream.WriteUint32(uint32(tracksCount))
	stream.WriteUint32(uint32(a.InterpolationType))
	stream.WriteInt32(a.GlobalSequenceID)

	for i := 0; i < tracksCount; i++ {
		stream.WriteInt32(a.Frames[i])
		a.writeMdxValue(stream, a.Values[i])
		if a.InterpolationType > InterpolationLinear {
			a.writeMdxValue(stream, a.InTans[i])
			a.writeMdxValue(stream, a.OutTans[i])
		}
	}
}

func (a *Animation) ReadMdl(stream *TokenStream, name string) error {
	tracksCount, err := stream.ReadInt()
	if err != nil {
		return err
	}
	if _, err := stream.Read(); err != nil { // {
		return err
	}

	token, err := stream.Read()
	if err != nil {
		return err
	}

	interpolationType := 0
	switch token {
	case "DontInterp":
		interpolationType = InterpolationDontInterp
	case "Linear":
		interpolationType = InterpolationLinear
	case "Hermite":
		interpolationType = InterpolationHermite
	case "Bezier":
		interpolationType = InterpolationBezier
	}

	a.Name = name
	a.InterpolationType = interpolationType
	a.GlobalSequenceID = -1
	a.Frames = make([]int32, tracksCount)
	a.Values = make([]interface{}, tracksCount)

	if interpolationType > InterpolationLinear {
		a.InTans = make([]interface{}, tracksCount)
		a.OutTans = make([]interface{}, tracksCount)
	}

	if peek, _ := stream.Peek(); peek == "GlobalSeqId" {
		if _, err := stream.Read(); err != nil {
			return err
		}
		gs, err := stream.ReadInt()
		if err != nil {
			return err
		}
		a.GlobalSequenceID = int32(gs)
	}

	for i := 0; i < tracksCount; i++ {
		frame, err := stream.ReadInt()
		if err != nil {
			return err
		}
		a.Frames[i] = int32(frame)
		val, err := a.readMdlValue(stream)
		if err != nil {
			return err
		}
		a.Values[i] = val
		if interpolationType > InterpolationLinear {
			if _, err := stream.Read(); err != nil { // InTan
				return err
			}
			inTan, err := a.readMdlValue(stream)
			if err != nil {
				return err
			}
			if _, err := stream.Read(); err != nil { // OutTan
				return err
			}
			outTan, err := a.readMdlValue(stream)
			if err != nil {
				return err
			}
			a.InTans[i] = inTan
			a.OutTans[i] = outTan
		}
	}
	_, err = stream.Read() // }
	return err
}

func (a *Animation) WriteMdl(stream *TokenStream, name string) {
	stream.StartBlock(name, len(a.Frames))

	token := ""
	switch a.InterpolationType {
	case InterpolationDontInterp:
		token = "DontInterp"
	case InterpolationLinear:
		token = "Linear"
	case InterpolationHermite:
		token = "Hermite"
	case InterpolationBezier:
		token = "Bezier"
	}
	stream.WriteFlag(token)

	if a.GlobalSequenceID != -1 {
		stream.WriteNumberAttrib("GlobalSeqId", float64(a.GlobalSequenceID))
	}

	for i := 0; i < len(a.Frames); i++ {
		a.writeMdlValue(stream, fmt.Sprintf("%d:", a.Frames[i]), a.Values[i])
		if a.InterpolationType > InterpolationLinear {
			stream.Indent()
			a.writeMdlValue(stream, "InTan", a.InTans[i])
			a.writeMdlValue(stream, "OutTan", a.OutTans[i])
			stream.Unindent()
		}
	}
	stream.EndBlock()
}

func (a *Animation) GetByteLength() int {
	tracksCount := len(a.Frames)
	size := 16
	if tracksCount > 0 {
		bytesPerValue := valueByteLength(a.Values[0])
		valuesPerTrack := 1
		if a.InterpolationType > InterpolationLinear {
			valuesPerTrack = 3
		}
		size += (4 + valuesPerTrack*bytesPerValue) * tracksCount
	}
	return size
}

func valueByteLength(v interface{}) int {
	switch val := v.(type) {
	case []uint32:
		return len(val) * 4
	case []float32:
		return len(val) * 4
	default:
		return 4
	}
}

func newAnimationForTag(tag string) (*Animation, string, error) {
	entry, ok := animationMap[tag]
	if !ok {
		return nil, "", fmt.Errorf("unknown animation tag: %s", tag)
	}
	return newAnimation(entry.kind), entry.mdlName, nil
}
