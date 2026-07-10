package mdx

// LightType for light types.
type LightType int32

const (
	LightTypeNone            LightType = -1
	LightTypeOmnidirectional LightType = 0
	LightTypeDirectional     LightType = 1
	LightTypeAmbient         LightType = 2
)

// Light is a model light.
type Light struct {
	*GenericObject
	Type             LightType
	Attenuation      []float32
	Color            []float32
	Intensity        float32
	AmbientColor     []float32
	AmbientIntensity float32
}

func NewLight() *Light {
	return &Light{
		GenericObject: NewGenericObject(0x200),
		Type:          LightTypeNone,
		Attenuation:   make([]float32, 2),
		Color:         make([]float32, 3),
		AmbientColor:  make([]float32, 3),
	}
}

func (l *Light) ReadMdx(stream *BinaryStream, _ int) error {
	start := stream.Index()
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	if err := l.GenericObject.ReadMdx(stream); err != nil {
		return err
	}
	lt, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	l.Type = LightType(lt)
	l.Attenuation, err = stream.ReadFloat32Array(2)
	if err != nil {
		return err
	}
	l.Color, err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	l.Intensity, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	l.AmbientColor, err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	l.AmbientIntensity, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	return l.ReadAnimations(stream, int(size)-(stream.Index()-start))
}

func (l *Light) WriteMdx(stream *BinaryStream, _ int) {
	stream.WriteUint32(uint32(l.GetByteLength(0)))
	l.GenericObject.WriteMdx(stream)
	stream.WriteUint32(uint32(l.Type))
	stream.WriteFloat32Array(l.Attenuation)
	stream.WriteFloat32Array(l.Color)
	stream.WriteFloat32(l.Intensity)
	stream.WriteFloat32Array(l.AmbientColor)
	stream.WriteFloat32(l.AmbientIntensity)
	l.WriteNonGenericAnimationChunks(stream)
}

func (l *Light) ReadMdl(stream *TokenStream) error {
	return l.ReadGenericBlock(stream, func(token string) error {
		switch token {
		case "Omnidirectional":
			l.Type = LightTypeOmnidirectional
		case "Directional":
			l.Type = LightTypeDirectional
		case "Ambient":
			l.Type = LightTypeAmbient
		case "static AttenuationStart":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			l.Attenuation[0] = float32(v)
		case "AttenuationStart":
			return l.ReadAnimation(stream, "KLAS")
		case "static AttenuationEnd":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			l.Attenuation[1] = float32(v)
		case "AttenuationEnd":
			return l.ReadAnimation(stream, "KLAE")
		case "static Intensity":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			l.Intensity = float32(v)
		case "Intensity":
			return l.ReadAnimation(stream, "KLAI")
		case "static Color":
			_, err := stream.ReadColor(l.Color)
			return err
		case "Color":
			return l.ReadAnimation(stream, "KLAC")
		case "static AmbIntensity":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			l.AmbientIntensity = float32(v)
		case "AmbIntensity":
			return l.ReadAnimation(stream, "KLBI")
		case "static AmbColor":
			_, err := stream.ReadColor(l.AmbientColor)
			return err
		case "AmbColor":
			return l.ReadAnimation(stream, "KLBC")
		case "Visibility":
			return l.ReadAnimation(stream, "KLAV")
		default:
			return unknownToken("Light", token)
		}
		return nil
	})
}

func (l *Light) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("Light", l.Name)
	l.WriteGenericHeader(stream)
	switch l.Type {
	case LightTypeOmnidirectional:
		stream.WriteFlag("Omnidirectional")
	case LightTypeDirectional:
		stream.WriteFlag("Directional")
	case LightTypeAmbient:
		stream.WriteFlag("Ambient")
	}
	if !l.WriteAnimation(stream, "KLAS") {
		stream.WriteNumberAttrib("static AttenuationStart", float64(l.Attenuation[0]))
	}
	if !l.WriteAnimation(stream, "KLAE") {
		stream.WriteNumberAttrib("static AttenuationEnd", float64(l.Attenuation[1]))
	}
	if !l.WriteAnimation(stream, "KLAI") {
		stream.WriteNumberAttrib("static Intensity", float64(l.Intensity))
	}
	if !l.WriteAnimation(stream, "KLAC") {
		stream.WriteColor("static Color", l.Color)
	}
	if !l.WriteAnimation(stream, "KLBI") {
		stream.WriteNumberAttrib("static AmbIntensity", float64(l.AmbientIntensity))
	}
	if !l.WriteAnimation(stream, "KLBC") {
		stream.WriteColor("static AmbColor", l.AmbientColor)
	}
	l.WriteAnimation(stream, "KLAV")
	l.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (l *Light) GetByteLength(_ int) int {
	return 48 + l.GenericObject.GetByteLength()
}
