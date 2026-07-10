package mdx

// LayerFilterMode for layer filter modes.
type LayerFilterMode uint32

const (
	LayerFilterNone       LayerFilterMode = 0
	LayerFilterTransparent LayerFilterMode = 1
	LayerFilterBlend      LayerFilterMode = 2
	LayerFilterAdditive   LayerFilterMode = 3
	LayerFilterAddAlpha   LayerFilterMode = 4
	LayerFilterModulate   LayerFilterMode = 5
	LayerFilterModulate2x LayerFilterMode = 6
)

// LayerFlags for layer flags.
type LayerFlags uint32

const (
	LayerFlagNone        LayerFlags = 0x0
	LayerFlagUnshaded    LayerFlags = 0x1
	LayerFlagSphereEnvMap LayerFlags = 0x2
	LayerFlagTwoSided    LayerFlags = 0x10
	LayerFlagUnfogged    LayerFlags = 0x20
	LayerFlagNoDepthTest LayerFlags = 0x40
	LayerFlagNoDepthSet  LayerFlags = 0x80
	LayerFlagUnlit       LayerFlags = 0x100
)

func stringToFilterMode(s string) LayerFilterMode {
	switch s {
	case "None":
		return LayerFilterNone
	case "Transparent":
		return LayerFilterTransparent
	case "Blend":
		return LayerFilterBlend
	case "Additive":
		return LayerFilterAdditive
	case "AddAlpha":
		return LayerFilterAddAlpha
	case "Modulate":
		return LayerFilterModulate
	case "Modulate2x":
		return LayerFilterModulate2x
	default:
		return LayerFilterNone
	}
}

func filterModeToString(m LayerFilterMode) string {
	switch m {
	case LayerFilterNone:
		return "None"
	case LayerFilterTransparent:
		return "Transparent"
	case LayerFilterBlend:
		return "Blend"
	case LayerFilterAdditive:
		return "Additive"
	case LayerFilterAddAlpha:
		return "AddAlpha"
	case LayerFilterModulate:
		return "Modulate"
	case LayerFilterModulate2x:
		return "Modulate2x"
	default:
		return "None"
	}
}

// Layer is a material layer.
type Layer struct {
	AnimatedObject
	FilterMode         LayerFilterMode
	Flags              LayerFlags
	TextureID          int32
	TextureAnimationID int32
	CoordID            uint32
	Alpha              float32
	EmissiveGain       float32
	FresnelColor       []float32
	FresnelOpacity     float32
	FresnelTeamColor   float32
}

func NewLayer() *Layer {
	return &Layer{
		TextureID:          -1,
		TextureAnimationID: -1,
		Alpha:              1,
		EmissiveGain:       1,
		FresnelColor:       []float32{1, 1, 1},
	}
}

func (l *Layer) ReadMdx(stream *BinaryStream, version int) error {
	start := stream.Index()
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	fm, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	l.FilterMode = LayerFilterMode(fm)
	flags, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	l.Flags = LayerFlags(flags)
	tid, err := stream.ReadInt32()
	if err != nil {
		return err
	}
	l.TextureID = tid
	taid, err := stream.ReadInt32()
	if err != nil {
		return err
	}
	l.TextureAnimationID = taid
	cid, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	l.CoordID = cid
	alpha, err := stream.ReadFloat32()
	if err != nil {
		return err
	}
	l.Alpha = alpha
	if version > 800 {
		eg, err := stream.ReadFloat32()
		if err != nil {
			return err
		}
		l.EmissiveGain = eg
		l.FresnelColor, err = stream.ReadFloat32Array(3)
		if err != nil {
			return err
		}
		l.FresnelOpacity, err = stream.ReadFloat32()
		if err != nil {
			return err
		}
		l.FresnelTeamColor, err = stream.ReadFloat32()
		if err != nil {
			return err
		}
	}
	return l.ReadAnimations(stream, int(size)-(stream.Index()-start))
}

func (l *Layer) WriteMdx(stream *BinaryStream, version int) {
	stream.WriteUint32(uint32(l.GetByteLength(version)))
	stream.WriteUint32(uint32(l.FilterMode))
	stream.WriteUint32(uint32(l.Flags))
	stream.WriteInt32(l.TextureID)
	stream.WriteInt32(l.TextureAnimationID)
	stream.WriteUint32(l.CoordID)
	stream.WriteFloat32(l.Alpha)
	if version > 800 {
		stream.WriteFloat32(l.EmissiveGain)
		stream.WriteFloat32Array(l.FresnelColor)
		stream.WriteFloat32(l.FresnelOpacity)
		stream.WriteFloat32(l.FresnelTeamColor)
	}
	l.WriteAnimations(stream)
}

func (l *Layer) ReadMdl(stream *TokenStream) error {
	return l.ReadAnimatedBlock(stream, func(token string) error {
		switch token {
		case "FilterMode":
			s, err := stream.Read()
			if err != nil {
				return err
			}
			l.FilterMode = stringToFilterMode(s)
		case "Unshaded":
			l.Flags |= LayerFlagUnshaded
		case "SphereEnvMap":
			l.Flags |= LayerFlagSphereEnvMap
		case "TwoSided":
			l.Flags |= LayerFlagTwoSided
		case "Unfogged":
			l.Flags |= LayerFlagUnfogged
		case "NoDepthTest":
			l.Flags |= LayerFlagNoDepthTest
		case "NoDepthSet":
			l.Flags |= LayerFlagNoDepthSet
		case "Unlit":
			l.Flags |= LayerFlagUnlit
		case "static TextureID":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			l.TextureID = int32(v)
		case "TextureID":
			return l.ReadAnimation(stream, "KMTF")
		case "TVertexAnimId":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			l.TextureAnimationID = int32(v)
		case "CoordId":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			l.CoordID = uint32(v)
		case "static Alpha":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			l.Alpha = float32(v)
		case "Alpha":
			return l.ReadAnimation(stream, "KMTA")
		case "static EmissiveGain":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			l.EmissiveGain = float32(v)
		case "EmissiveGain":
			return l.ReadAnimation(stream, "KMTE")
		case "static FresnelColor":
			_, err := stream.ReadVector(l.FresnelColor)
			return err
		case "FresnelColor":
			return l.ReadAnimation(stream, "KFC3")
		case "static FresnelOpacity":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			l.FresnelOpacity = float32(v)
		case "FresnelOpacity":
			return l.ReadAnimation(stream, "KFCA")
		case "static FresnelTeamColor":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			l.FresnelTeamColor = float32(v)
		case "FresnelTeamColor":
			return l.ReadAnimation(stream, "KFTC")
		default:
			return unknownToken("Layer", token)
		}
		return nil
	})
}

func (l *Layer) WriteMdl(stream *TokenStream, version int) {
	stream.StartBlock("Layer")
	stream.WriteFlagAttrib("FilterMode", filterModeToString(l.FilterMode))
	if l.Flags&LayerFlagUnshaded != 0 {
		stream.WriteFlag("Unshaded")
	}
	if l.Flags&LayerFlagSphereEnvMap != 0 {
		stream.WriteFlag("SphereEnvMap")
	}
	if l.Flags&LayerFlagTwoSided != 0 {
		stream.WriteFlag("TwoSided")
	}
	if l.Flags&LayerFlagUnfogged != 0 {
		stream.WriteFlag("Unfogged")
	}
	if l.Flags&LayerFlagNoDepthTest != 0 {
		stream.WriteFlag("NoDepthTest")
	}
	if l.Flags&LayerFlagNoDepthSet != 0 {
		stream.WriteFlag("NoDepthSet")
	}
	if version > 800 {
		if l.Flags&LayerFlagUnlit != 0 {
			stream.WriteFlag("Unlit")
		}
	}
	if !l.WriteAnimation(stream, "KMTF") {
		stream.WriteNumberAttrib("static TextureID", float64(l.TextureID))
	}
	if l.TextureAnimationID != -1 {
		stream.WriteNumberAttrib("TVertexAnimId", float64(l.TextureAnimationID))
	}
	if l.CoordID != 0 {
		stream.WriteNumberAttrib("CoordId", float64(l.CoordID))
	}
	if !l.WriteAnimation(stream, "KMTA") && l.Alpha != 1 {
		stream.WriteNumberAttrib("static Alpha", float64(l.Alpha))
	}
	if version > 800 {
		if !l.WriteAnimation(stream, "KMTE") && l.EmissiveGain != 1 {
			stream.WriteNumberAttrib("static EmissiveGain", float64(l.EmissiveGain))
		}
		if !l.WriteAnimation(stream, "KFC3") && (l.FresnelColor[0] != 1 || l.FresnelColor[1] != 1 || l.FresnelColor[2] != 1) {
			stream.WriteVectorAttrib("static FresnelColor", l.FresnelColor)
		}
		if !l.WriteAnimation(stream, "KFCA") && l.FresnelOpacity != 0 {
			stream.WriteNumberAttrib("static FresnelOpacity", float64(l.FresnelOpacity))
		}
		if !l.WriteAnimation(stream, "KFTC") && l.FresnelTeamColor != 0 {
			stream.WriteNumberAttrib("static FresnelTeamColor", float64(l.FresnelTeamColor))
		}
	}
	stream.EndBlock()
}

func (l *Layer) GetByteLength(version int) int {
	size := 28 + l.AnimatedObject.GetByteLength(0)
	if version > 800 {
		size += 24
	}
	return size
}
