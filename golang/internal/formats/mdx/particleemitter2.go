package mdx

// ParticleEmitter2Flags for PE2 flags.
type ParticleEmitter2Flags uint32

const (
	PE2FlagUnshaded       ParticleEmitter2Flags = 0x8000
	PE2FlagSortPrimsFarZ  ParticleEmitter2Flags = 0x10000
	PE2FlagLineEmitter    ParticleEmitter2Flags = 0x20000
	PE2FlagUnfogged       ParticleEmitter2Flags = 0x40000
	PE2FlagModelSpace     ParticleEmitter2Flags = 0x80000
	PE2FlagXYQuad         ParticleEmitter2Flags = 0x100000
)

// PE2FilterMode for PE2 filter modes.
type PE2FilterMode uint32

const (
	PE2FilterBlend     PE2FilterMode = 0
	PE2FilterAdditive  PE2FilterMode = 1
	PE2FilterModulate  PE2FilterMode = 2
	PE2FilterModulate2 PE2FilterMode = 3
	PE2FilterAlphaKey  PE2FilterMode = 4
)

// PE2HeadOrTail for head/tail mode.
type PE2HeadOrTail uint32

const (
	PE2Head PE2HeadOrTail = 0
	PE2Tail PE2HeadOrTail = 1
	PE2Both PE2HeadOrTail = 2
)

// ParticleEmitter2 is a particle emitter type 2.
type ParticleEmitter2 struct {
	*GenericObject
	Speed           float32
	Variation       float32
	Latitude        float32
	Gravity         float32
	LifeSpan        float32
	EmissionRate    float32
	Width           float32
	Length          float32
	FilterMode      PE2FilterMode
	Rows            uint32
	Columns         uint32
	HeadOrTail      PE2HeadOrTail
	TailLength      float32
	TimeMiddle      float32
	SegmentColors   [3][]float32
	SegmentAlphas   []uint8
	SegmentScaling  []float32
	HeadIntervals   [2][]uint32
	TailIntervals   [2][]uint32
	TextureID       int32
	Squirt          uint32
	PriorityPlane   int32
	ReplaceableID   uint32
}

func NewParticleEmitter2() *ParticleEmitter2 {
	return &ParticleEmitter2{
		GenericObject:  NewGenericObject(0),
		TextureID:      -1,
		SegmentColors:  [3][]float32{make([]float32, 3), make([]float32, 3), make([]float32, 3)},
		SegmentAlphas:  make([]uint8, 3),
		SegmentScaling: make([]float32, 3),
		HeadIntervals:  [2][]uint32{make([]uint32, 3), make([]uint32, 3)},
		TailIntervals:  [2][]uint32{make([]uint32, 3), make([]uint32, 3)},
	}
}

func (p *ParticleEmitter2) ReadMdx(stream *BinaryStream, _ int) error {
	start := stream.Index()
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	if err := p.GenericObject.ReadMdx(stream); err != nil {
		return err
	}
	p.Speed, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Variation, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Latitude, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Gravity, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.LifeSpan, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.EmissionRate, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Width, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Length, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	fm, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	p.FilterMode = PE2FilterMode(fm)
	p.Rows, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	p.Columns, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	hot, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	p.HeadOrTail = PE2HeadOrTail(hot)
	p.TailLength, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.TimeMiddle, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.SegmentColors[0], err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	p.SegmentColors[1], err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	p.SegmentColors[2], err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	p.SegmentAlphas, err = stream.ReadUint8Array(3)
	if err != nil {
		return err
	}
	p.SegmentScaling, err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	p.HeadIntervals[0], err = stream.ReadUint32Array(3)
	if err != nil {
		return err
	}
	p.HeadIntervals[1], err = stream.ReadUint32Array(3)
	if err != nil {
		return err
	}
	p.TailIntervals[0], err = stream.ReadUint32Array(3)
	if err != nil {
		return err
	}
	p.TailIntervals[1], err = stream.ReadUint32Array(3)
	if err != nil {
		return err
	}
	p.TextureID, err = stream.ReadInt32()
	if err != nil {
		return err
	}
	p.Squirt, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	p.PriorityPlane, err = stream.ReadInt32()
	if err != nil {
		return err
	}
	p.ReplaceableID, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	return p.ReadAnimations(stream, int(size)-(stream.Index()-start))
}

func (p *ParticleEmitter2) WriteMdx(stream *BinaryStream, _ int) {
	stream.WriteUint32(uint32(p.GetByteLength(0)))
	p.GenericObject.WriteMdx(stream)
	stream.WriteFloat32(p.Speed)
	stream.WriteFloat32(p.Variation)
	stream.WriteFloat32(p.Latitude)
	stream.WriteFloat32(p.Gravity)
	stream.WriteFloat32(p.LifeSpan)
	stream.WriteFloat32(p.EmissionRate)
	stream.WriteFloat32(p.Width)
	stream.WriteFloat32(p.Length)
	stream.WriteUint32(uint32(p.FilterMode))
	stream.WriteUint32(p.Rows)
	stream.WriteUint32(p.Columns)
	stream.WriteUint32(uint32(p.HeadOrTail))
	stream.WriteFloat32(p.TailLength)
	stream.WriteFloat32(p.TimeMiddle)
	stream.WriteFloat32Array(p.SegmentColors[0])
	stream.WriteFloat32Array(p.SegmentColors[1])
	stream.WriteFloat32Array(p.SegmentColors[2])
	stream.WriteUint8Array(p.SegmentAlphas)
	stream.WriteFloat32Array(p.SegmentScaling)
	stream.WriteUint32Array(p.HeadIntervals[0])
	stream.WriteUint32Array(p.HeadIntervals[1])
	stream.WriteUint32Array(p.TailIntervals[0])
	stream.WriteUint32Array(p.TailIntervals[1])
	stream.WriteInt32(p.TextureID)
	stream.WriteUint32(p.Squirt)
	stream.WriteInt32(p.PriorityPlane)
	stream.WriteUint32(p.ReplaceableID)
	p.WriteNonGenericAnimationChunks(stream)
}

func (p *ParticleEmitter2) ReadMdl(stream *TokenStream) error {
	return p.ReadGenericBlock(stream, func(token string) error {
		switch token {
		case "SortPrimsFarZ":
			p.Flags |= GenericObjectFlags(PE2FlagSortPrimsFarZ)
		case "Unshaded":
			p.Flags |= GenericObjectFlags(PE2FlagUnshaded)
		case "LineEmitter":
			p.Flags |= GenericObjectFlags(PE2FlagLineEmitter)
		case "Unfogged":
			p.Flags |= GenericObjectFlags(PE2FlagUnfogged)
		case "ModelSpace":
			p.Flags |= GenericObjectFlags(PE2FlagModelSpace)
		case "XYQuad":
			p.Flags |= GenericObjectFlags(PE2FlagXYQuad)
		case "static Speed":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Speed = float32(v)
		case "Speed":
			return p.ReadAnimation(stream, "KP2S")
		case "static Variation":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Variation = float32(v)
		case "Variation":
			return p.ReadAnimation(stream, "KP2R")
		case "static Latitude":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Latitude = float32(v)
		case "Latitude":
			return p.ReadAnimation(stream, "KP2L")
		case "static Gravity":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Gravity = float32(v)
		case "Gravity":
			return p.ReadAnimation(stream, "KP2G")
		case "Visibility":
			return p.ReadAnimation(stream, "KP2V")
		case "Squirt":
			p.Squirt = 1
		case "LifeSpan":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.LifeSpan = float32(v)
		case "static EmissionRate":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.EmissionRate = float32(v)
		case "EmissionRate":
			return p.ReadAnimation(stream, "KP2E")
		case "static Width":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Width = float32(v)
		case "Width":
			return p.ReadAnimation(stream, "KP2N")
		case "static Length":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Length = float32(v)
		case "Length":
			return p.ReadAnimation(stream, "KP2W")
		case "Blend":
			p.FilterMode = PE2FilterBlend
		case "Additive":
			p.FilterMode = PE2FilterAdditive
		case "Modulate":
			p.FilterMode = PE2FilterModulate
		case "Modulate2x":
			p.FilterMode = PE2FilterModulate2
		case "AlphaKey":
			p.FilterMode = PE2FilterAlphaKey
		case "Rows":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			p.Rows = uint32(v)
		case "Columns":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			p.Columns = uint32(v)
		case "Head":
			p.HeadOrTail = PE2Head
		case "Tail":
			p.HeadOrTail = PE2Tail
		case "Both":
			p.HeadOrTail = PE2Both
		case "TailLength":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.TailLength = float32(v)
		case "Time":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.TimeMiddle = float32(v)
		case "SegmentColor":
			if _, err := stream.Read(); err != nil { // {
				return err
			}
			for i := 0; i < 3; i++ {
				if _, err := stream.Read(); err != nil { // Color
					return err
				}
				if _, err := stream.ReadColor(p.SegmentColors[i]); err != nil {
					return err
				}
			}
			if _, err := stream.Read(); err != nil { // }
				return err
			}
		case "Alpha":
			view := make([]float32, 3)
			if _, err := stream.ReadVector(view); err != nil {
				return err
			}
			for i, v := range view {
				p.SegmentAlphas[i] = uint8(v)
			}
		case "ParticleScaling":
			_, err := stream.ReadVector(p.SegmentScaling)
			return err
		case "LifeSpanUVAnim":
			view := make([]float32, 3)
			if _, err := stream.ReadVector(view); err != nil {
				return err
			}
			for i, v := range view {
				p.HeadIntervals[0][i] = uint32(v)
			}
		case "DecayUVAnim":
			view := make([]float32, 3)
			if _, err := stream.ReadVector(view); err != nil {
				return err
			}
			for i, v := range view {
				p.HeadIntervals[1][i] = uint32(v)
			}
		case "TailUVAnim":
			view := make([]float32, 3)
			if _, err := stream.ReadVector(view); err != nil {
				return err
			}
			for i, v := range view {
				p.TailIntervals[0][i] = uint32(v)
			}
		case "TailDecayUVAnim":
			view := make([]float32, 3)
			if _, err := stream.ReadVector(view); err != nil {
				return err
			}
			for i, v := range view {
				p.TailIntervals[1][i] = uint32(v)
			}
		case "TextureID":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			p.TextureID = int32(v)
		case "ReplaceableId":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			p.ReplaceableID = uint32(v)
		case "PriorityPlane":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			p.PriorityPlane = int32(v)
		default:
			return unknownToken("ParticleEmitter2", token)
		}
		return nil
	})
}

func (p *ParticleEmitter2) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("ParticleEmitter2", p.Name)
	p.WriteGenericHeader(stream)
	if p.Flags&GenericObjectFlags(PE2FlagSortPrimsFarZ) != 0 {
		stream.WriteFlag("SortPrimsFarZ")
	}
	if p.Flags&GenericObjectFlags(PE2FlagUnshaded) != 0 {
		stream.WriteFlag("Unshaded")
	}
	if p.Flags&GenericObjectFlags(PE2FlagLineEmitter) != 0 {
		stream.WriteFlag("LineEmitter")
	}
	if p.Flags&GenericObjectFlags(PE2FlagUnfogged) != 0 {
		stream.WriteFlag("Unfogged")
	}
	if p.Flags&GenericObjectFlags(PE2FlagModelSpace) != 0 {
		stream.WriteFlag("ModelSpace")
	}
	if p.Flags&GenericObjectFlags(PE2FlagXYQuad) != 0 {
		stream.WriteFlag("XYQuad")
	}
	if !p.WriteAnimation(stream, "KP2S") {
		stream.WriteNumberAttrib("static Speed", float64(p.Speed))
	}
	if !p.WriteAnimation(stream, "KP2R") {
		stream.WriteNumberAttrib("static Variation", float64(p.Variation))
	}
	if !p.WriteAnimation(stream, "KP2L") {
		stream.WriteNumberAttrib("static Latitude", float64(p.Latitude))
	}
	if !p.WriteAnimation(stream, "KP2G") {
		stream.WriteNumberAttrib("static Gravity", float64(p.Gravity))
	}
	p.WriteAnimation(stream, "KP2V")
	if p.Squirt != 0 {
		stream.WriteFlag("Squirt")
	}
	stream.WriteNumberAttrib("LifeSpan", float64(p.LifeSpan))
	if !p.WriteAnimation(stream, "KP2E") {
		stream.WriteNumberAttrib("static EmissionRate", float64(p.EmissionRate))
	}
	if !p.WriteAnimation(stream, "KP2N") {
		stream.WriteNumberAttrib("static Width", float64(p.Width))
	}
	if !p.WriteAnimation(stream, "KP2W") {
		stream.WriteNumberAttrib("static Length", float64(p.Length))
	}
	switch p.FilterMode {
	case PE2FilterBlend:
		stream.WriteFlag("Blend")
	case PE2FilterAdditive:
		stream.WriteFlag("Additive")
	case PE2FilterModulate:
		stream.WriteFlag("Modulate")
	case PE2FilterModulate2:
		stream.WriteFlag("Modulate2x")
	case PE2FilterAlphaKey:
		stream.WriteFlag("AlphaKey")
	}
	stream.WriteNumberAttrib("Rows", float64(p.Rows))
	stream.WriteNumberAttrib("Columns", float64(p.Columns))
	switch p.HeadOrTail {
	case PE2Head:
		stream.WriteFlag("Head")
	case PE2Tail:
		stream.WriteFlag("Tail")
	case PE2Both:
		stream.WriteFlag("Both")
	}
	stream.WriteNumberAttrib("TailLength", float64(p.TailLength))
	stream.WriteNumberAttrib("Time", float64(p.TimeMiddle))
	stream.StartBlock("SegmentColor")
	stream.WriteColor("Color", p.SegmentColors[0])
	stream.WriteColor("Color", p.SegmentColors[1])
	stream.WriteColor("Color", p.SegmentColors[2])
	stream.EndBlockComma()
	stream.WriteVectorAttrib("Alpha", p.SegmentAlphas)
	stream.WriteVectorAttrib("ParticleScaling", p.SegmentScaling)
	stream.WriteVectorAttrib("LifeSpanUVAnim", p.HeadIntervals[0])
	stream.WriteVectorAttrib("DecayUVAnim", p.HeadIntervals[1])
	stream.WriteVectorAttrib("TailUVAnim", p.TailIntervals[0])
	stream.WriteVectorAttrib("TailDecayUVAnim", p.TailIntervals[1])
	stream.WriteNumberAttrib("TextureID", float64(p.TextureID))
	if p.ReplaceableID != 0 {
		stream.WriteNumberAttrib("ReplaceableId", float64(p.ReplaceableID))
	}
	if p.PriorityPlane != 0 {
		stream.WriteNumberAttrib("PriorityPlane", float64(p.PriorityPlane))
	}
	p.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (p *ParticleEmitter2) GetByteLength(_ int) int {
	return 175 + p.GenericObject.GetByteLength()
}
