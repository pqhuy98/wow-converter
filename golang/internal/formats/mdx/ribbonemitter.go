package mdx

// RibbonEmitter is a ribbon emitter object.
type RibbonEmitter struct {
	*GenericObject
	HeightAbove  float32
	HeightBelow  float32
	Alpha        float32
	Color        []float32
	LifeSpan     float32
	TextureSlot  uint32
	EmissionRate uint32
	Rows         uint32
	Columns      uint32
	MaterialID   int32
	Gravity      float32
}

func NewRibbonEmitter() *RibbonEmitter {
	return &RibbonEmitter{
		GenericObject: NewGenericObject(0x4000),
		Color:         make([]float32, 3),
	}
}

func (r *RibbonEmitter) ReadMdx(stream *BinaryStream, _ int) error {
	start := stream.Index()
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	if err := r.GenericObject.ReadMdx(stream); err != nil {
		return err
	}
	r.HeightAbove, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	r.HeightBelow, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	r.Alpha, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	r.Color, err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	r.LifeSpan, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	r.TextureSlot, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	r.EmissionRate, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	r.Rows, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	r.Columns, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	r.MaterialID, err = stream.ReadInt32()
	if err != nil {
		return err
	}
	r.Gravity, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	return r.ReadAnimations(stream, int(size)-(stream.Index()-start))
}

func (r *RibbonEmitter) WriteMdx(stream *BinaryStream, _ int) {
	stream.WriteUint32(uint32(r.GetByteLength(0)))
	r.GenericObject.WriteMdx(stream)
	stream.WriteFloat32(r.HeightAbove)
	stream.WriteFloat32(r.HeightBelow)
	stream.WriteFloat32(r.Alpha)
	stream.WriteFloat32Array(r.Color)
	stream.WriteFloat32(r.LifeSpan)
	stream.WriteUint32(r.TextureSlot)
	stream.WriteUint32(r.EmissionRate)
	stream.WriteUint32(r.Rows)
	stream.WriteUint32(r.Columns)
	stream.WriteInt32(r.MaterialID)
	stream.WriteFloat32(r.Gravity)
	r.WriteNonGenericAnimationChunks(stream)
}

func (r *RibbonEmitter) ReadMdl(stream *TokenStream) error {
	return r.ReadGenericBlock(stream, func(token string) error {
		switch token {
		case "static HeightAbove":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			r.HeightAbove = float32(v)
		case "HeightAbove":
			return r.ReadAnimation(stream, "KRHA")
		case "static HeightBelow":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			r.HeightBelow = float32(v)
		case "HeightBelow":
			return r.ReadAnimation(stream, "KRHB")
		case "static Alpha":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			r.Alpha = float32(v)
		case "Alpha":
			return r.ReadAnimation(stream, "KRAL")
		case "static Color":
			_, err := stream.ReadColor(r.Color)
			return err
		case "Color":
			return r.ReadAnimation(stream, "KRCO")
		case "static TextureSlot":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			r.TextureSlot = uint32(v)
		case "TextureSlot":
			return r.ReadAnimation(stream, "KRTX")
		case "Visibility":
			return r.ReadAnimation(stream, "KRVS")
		case "EmissionRate":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			r.EmissionRate = uint32(v)
		case "LifeSpan":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			r.LifeSpan = float32(v)
		case "Gravity":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			r.Gravity = float32(v)
		case "Rows":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			r.Rows = uint32(v)
		case "Columns":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			r.Columns = uint32(v)
		case "MaterialID":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			r.MaterialID = int32(v)
		default:
			return unknownToken("RibbonEmitter", token)
		}
		return nil
	})
}

func (r *RibbonEmitter) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("RibbonEmitter", r.Name)
	r.WriteGenericHeader(stream)
	if !r.WriteAnimation(stream, "KRHA") {
		stream.WriteNumberAttrib("static HeightAbove", float64(r.HeightAbove))
	}
	if !r.WriteAnimation(stream, "KRHB") {
		stream.WriteNumberAttrib("static HeightBelow", float64(r.HeightBelow))
	}
	if !r.WriteAnimation(stream, "KRAL") {
		stream.WriteNumberAttrib("static Alpha", float64(r.Alpha))
	}
	if !r.WriteAnimation(stream, "KRCO") {
		stream.WriteColor("static Color", r.Color)
	}
	if !r.WriteAnimation(stream, "KRTX") {
		stream.WriteNumberAttrib("static TextureSlot", float64(r.TextureSlot))
	}
	r.WriteAnimation(stream, "KRVS")
	stream.WriteNumberAttrib("EmissionRate", float64(r.EmissionRate))
	stream.WriteNumberAttrib("LifeSpan", float64(r.LifeSpan))
	if r.Gravity != 0 {
		stream.WriteNumberAttrib("Gravity", float64(r.Gravity))
	}
	stream.WriteNumberAttrib("Rows", float64(r.Rows))
	stream.WriteNumberAttrib("Columns", float64(r.Columns))
	stream.WriteNumberAttrib("MaterialID", float64(r.MaterialID))
	r.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (r *RibbonEmitter) GetByteLength(_ int) int {
	return 56 + r.GenericObject.GetByteLength()
}
