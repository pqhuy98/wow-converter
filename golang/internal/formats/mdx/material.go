package mdx

// MaterialFlags for material flags.
type MaterialFlags uint32

const (
	MaterialFlagNone            MaterialFlags = 0x0
	MaterialFlagConstantColor   MaterialFlags = 0x1
	MaterialFlagTwoSided        MaterialFlags = 0x2
	MaterialFlagSortPrimsNearZ  MaterialFlags = 0x8
	MaterialFlagSortPrimsFarZ   MaterialFlags = 0x10
	MaterialFlagFullResolution  MaterialFlags = 0x20
)

// Material is a model material.
type Material struct {
	PriorityPlane int32
	Flags         MaterialFlags
	Shader        string
	Layers        []*Layer
}

func NewMaterial() *Material {
	return &Material{}
}

func (m *Material) ReadMdx(stream *BinaryStream, version int) error {
	if _, err := stream.ReadUint32(); err != nil {
		return err
	}
	pp, err := stream.ReadInt32()
	if err != nil {
		return err
	}
	m.PriorityPlane = pp
	flags, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	m.Flags = MaterialFlags(flags)
	if version > 800 {
		shader, err := stream.Read(80)
		if err != nil {
			return err
		}
		m.Shader = shader
	}
	if err := stream.Skip(4); err != nil { // LAYS
		return err
	}
	count, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		layer := NewLayer()
		if err := layer.ReadMdx(stream, version); err != nil {
			return err
		}
		m.Layers = append(m.Layers, layer)
	}
	return nil
}

func (m *Material) WriteMdx(stream *BinaryStream, version int) {
	stream.WriteUint32(uint32(m.GetByteLength(version)))
	stream.WriteInt32(m.PriorityPlane)
	stream.WriteUint32(uint32(m.Flags))
	if version > 800 {
		written := stream.Write(m.Shader)
		stream.Skip(80 - written)
	}
	stream.WriteBinary("LAYS")
	stream.WriteUint32(uint32(len(m.Layers)))
	for _, layer := range m.Layers {
		layer.WriteMdx(stream, version)
	}
}

func (m *Material) ReadMdl(stream *TokenStream) error {
	return stream.ReadBlockIter(func(token string) error {
		switch token {
		case "ConstantColor":
			m.Flags |= MaterialFlagConstantColor
		case "TwoSided":
			m.Flags |= MaterialFlagTwoSided
		case "SortPrimsNearZ":
			m.Flags |= MaterialFlagSortPrimsNearZ
		case "SortPrimsFarZ":
			m.Flags |= MaterialFlagSortPrimsFarZ
		case "FullResolution":
			m.Flags |= MaterialFlagFullResolution
		case "PriorityPlane":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			m.PriorityPlane = int32(v)
		case "Shader":
			s, err := stream.Read()
			if err != nil {
				return err
			}
			m.Shader = s
		case "Layer":
			layer := NewLayer()
			if err := layer.ReadMdl(stream); err != nil {
				return err
			}
			m.Layers = append(m.Layers, layer)
		default:
			return unknownToken("Material", token)
		}
		return nil
	})
}

func (m *Material) WriteMdl(stream *TokenStream, version int) {
	stream.StartBlock("Material")
	if m.Flags&MaterialFlagConstantColor != 0 {
		stream.WriteFlag("ConstantColor")
	}
	if version > 800 {
		if m.Flags&MaterialFlagTwoSided != 0 {
			stream.WriteFlag("TwoSided")
		}
	}
	if m.Flags&MaterialFlagSortPrimsNearZ != 0 {
		stream.WriteFlag("SortPrimsNearZ")
	}
	if m.Flags&MaterialFlagSortPrimsFarZ != 0 {
		stream.WriteFlag("SortPrimsFarZ")
	}
	if m.Flags&MaterialFlagFullResolution != 0 {
		stream.WriteFlag("FullResolution")
	}
	if m.PriorityPlane != 0 {
		stream.WriteNumberAttrib("PriorityPlane", float64(m.PriorityPlane))
	}
	if version > 800 {
		stream.WriteStringAttrib("Shader", m.Shader)
	}
	for _, layer := range m.Layers {
		layer.WriteMdl(stream, version)
	}
	stream.EndBlock()
}

func (m *Material) GetByteLength(version int) int {
	size := 20
	if version > 800 {
		size += 80
	}
	for _, layer := range m.Layers {
		size += layer.GetByteLength(version)
	}
	return size
}
