package mdx

// GeosetAnimation animates geoset alpha/color.
type GeosetAnimation struct {
	AnimatedObject
	Alpha     float32
	Flags     uint32
	Color     []float32
	GeosetID  int32
}

func NewGeosetAnimation() *GeosetAnimation {
	return &GeosetAnimation{
		Alpha:    1,
		Color:    []float32{1, 1, 1},
		GeosetID: -1,
	}
}

func (g *GeosetAnimation) ReadMdx(stream *BinaryStream) error {
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.Alpha, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	flags, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	g.Flags = flags
	g.Color, err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	g.GeosetID, err = stream.ReadInt32()
	if err != nil {
		return err
	}
	return g.ReadAnimations(stream, int(size)-28)
}

func (g *GeosetAnimation) WriteMdx(stream *BinaryStream, _ int) {
	stream.WriteUint32(uint32(g.GetByteLength(0)))
	stream.WriteFloat32(g.Alpha)
	stream.WriteUint32(g.Flags)
	stream.WriteFloat32Array(g.Color)
	stream.WriteInt32(g.GeosetID)
	g.WriteAnimations(stream)
}

func (g *GeosetAnimation) ReadMdl(stream *TokenStream) error {
	return g.ReadAnimatedBlock(stream, func(token string) error {
		switch token {
		case "DropShadow":
			g.Flags |= 0x1
		case "static Alpha":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			g.Alpha = float32(v)
		case "Alpha":
			return g.ReadAnimation(stream, "KGAO")
		case "static Color":
			g.Flags |= 0x2
			_, err := stream.ReadColor(g.Color)
			return err
		case "Color":
			g.Flags |= 0x2
			return g.ReadAnimation(stream, "KGAC")
		case "GeosetId":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			g.GeosetID = int32(v)
		default:
			return unknownToken("GeosetAnimation", token)
		}
		return nil
	})
}

func (g *GeosetAnimation) WriteMdl(stream *TokenStream, _ int) {
	stream.StartBlock("GeosetAnim")
	if g.Flags&0x1 != 0 {
		stream.WriteFlag("DropShadow")
	}
	if !g.WriteAnimation(stream, "KGAO") {
		stream.WriteNumberAttrib("static Alpha", float64(g.Alpha))
	}
	if g.Flags&0x2 != 0 {
		if !g.WriteAnimation(stream, "KGAC") && (g.Color[0] != 1 || g.Color[1] != 1 || g.Color[2] != 1) {
			stream.WriteColor("static Color ", g.Color)
		}
	}
	stream.WriteNumberAttrib("GeosetId", float64(g.GeosetID))
	stream.EndBlock()
}

func (g *GeosetAnimation) GetByteLength(_ int) int {
	return 28 + g.AnimatedObject.GetByteLength(0)
}
