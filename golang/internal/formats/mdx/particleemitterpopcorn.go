package mdx

// ParticleEmitterPopcornFlags for popcorn emitter flags.
type ParticleEmitterPopcornFlags uint32

const (
	PopcornFlagUnshaded      ParticleEmitterPopcornFlags = 0x8000
	PopcornFlagSortPrimsFarZ ParticleEmitterPopcornFlags = 0x10000
	PopcornFlagUnfogged      ParticleEmitterPopcornFlags = 0x40000
)

// ParticleEmitterPopcorn is a PopcornFX particle emitter.
type ParticleEmitterPopcorn struct {
	*GenericObject
	LifeSpan                 float32
	EmissionRate             float32
	Speed                    float32
	Color                    []float32
	Alpha                    float32
	ReplaceableID            uint32
	Path                     string
	AnimationVisibilityGuide string
}

func NewParticleEmitterPopcorn() *ParticleEmitterPopcorn {
	return &ParticleEmitterPopcorn{
		GenericObject: NewGenericObject(0),
		Color:         []float32{0, 0, 0},
		Alpha:         1,
	}
}

func (p *ParticleEmitterPopcorn) ReadMdx(stream *BinaryStream, _ int) error {
	start := stream.Index()
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	if err := p.GenericObject.ReadMdx(stream); err != nil {
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
	p.Speed, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Color, err = stream.ReadFloat32Array(3)
	if err != nil {
		return err
	}
	p.Alpha, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.ReplaceableID, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	p.Path, err = stream.Read(260)
	if err != nil {
		return err
	}
	p.AnimationVisibilityGuide, err = stream.Read(260)
	if err != nil {
		return err
	}
	return p.ReadAnimations(stream, int(size)-(stream.Index()-start))
}

func (p *ParticleEmitterPopcorn) WriteMdx(stream *BinaryStream, _ int) {
	stream.WriteUint32(uint32(p.GetByteLength(0)))
	p.GenericObject.WriteMdx(stream)
	stream.WriteFloat32(p.LifeSpan)
	stream.WriteFloat32(p.EmissionRate)
	stream.WriteFloat32(p.Speed)
	stream.WriteFloat32Array(p.Color)
	stream.WriteFloat32(p.Alpha)
	stream.WriteUint32(p.ReplaceableID)
	written := stream.Write(p.Path)
	stream.Skip(260 - written)
	written = stream.Write(p.AnimationVisibilityGuide)
	stream.Skip(260 - written)
	p.WriteNonGenericAnimationChunks(stream)
}

func (p *ParticleEmitterPopcorn) ReadMdl(stream *TokenStream) error {
	return p.ReadGenericBlock(stream, func(token string) error {
		switch token {
		case "SortPrimsFarZ":
			p.Flags |= GenericObjectFlags(PopcornFlagSortPrimsFarZ)
		case "Unshaded":
			p.Flags |= GenericObjectFlags(PopcornFlagUnshaded)
		case "Unfogged":
			p.Flags |= GenericObjectFlags(PopcornFlagUnfogged)
		case "static LifeSpan":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.LifeSpan = float32(v)
		case "LifeSpan":
			return p.ReadAnimation(stream, "KPPL")
		case "static EmissionRate":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.EmissionRate = float32(v)
		case "EmissionRate":
			return p.ReadAnimation(stream, "KPPE")
		case "static Speed":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Speed = float32(v)
		case "Speed":
			return p.ReadAnimation(stream, "KPPS")
		case "static Color":
			_, err := stream.ReadVector(p.Color)
			return err
		case "Color":
			return p.ReadAnimation(stream, "KPPC")
		case "static Alpha":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Alpha = float32(v)
		case "Alpha":
			return p.ReadAnimation(stream, "KPPA")
		case "Visibility":
			return p.ReadAnimation(stream, "KPPV")
		case "ReplaceableId":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			p.ReplaceableID = uint32(v)
		case "Path":
			s, err := stream.Read()
			if err != nil {
				return err
			}
			p.Path = s
		case "AnimVisibilityGuide":
			s, err := stream.Read()
			if err != nil {
				return err
			}
			p.AnimationVisibilityGuide = s
		default:
			return unknownToken("ParticleEmitterPopcorn", token)
		}
		return nil
	})
}

func (p *ParticleEmitterPopcorn) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("ParticleEmitterPopcorn", p.Name)
	p.WriteGenericHeader(stream)
	if p.Flags&GenericObjectFlags(PopcornFlagSortPrimsFarZ) != 0 {
		stream.WriteFlag("SortPrimsFarZ")
	}
	if p.Flags&GenericObjectFlags(PopcornFlagUnshaded) != 0 {
		stream.WriteFlag("Unshaded")
	}
	if p.Flags&GenericObjectFlags(PopcornFlagUnfogged) != 0 {
		stream.WriteFlag("Unfogged")
	}
	if !p.WriteAnimation(stream, "KPPL") {
		stream.WriteNumberAttrib("static LifeSpan", float64(p.LifeSpan))
	}
	if !p.WriteAnimation(stream, "KPPE") {
		stream.WriteNumberAttrib("static EmissionRate", float64(p.EmissionRate))
	}
	if !p.WriteAnimation(stream, "KPPS") {
		stream.WriteNumberAttrib("static Speed", float64(p.Speed))
	}
	if !p.WriteAnimation(stream, "KPPC") {
		stream.WriteVectorAttrib("static Color", p.Color)
	}
	if !p.WriteAnimation(stream, "KPPA") {
		stream.WriteNumberAttrib("static Alpha", float64(p.Alpha))
	}
	p.WriteAnimation(stream, "KPPV")
	if p.ReplaceableID != 0 {
		stream.WriteNumberAttrib("ReplaceableId", float64(p.ReplaceableID))
	}
	if len(p.Path) > 0 {
		stream.WriteStringAttrib("Path", p.Path)
	}
	if len(p.AnimationVisibilityGuide) > 0 {
		stream.WriteStringAttrib("AnimVisibilityGuide", p.AnimationVisibilityGuide)
	}
	p.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (p *ParticleEmitterPopcorn) GetByteLength(_ int) int {
	return 556 + p.GenericObject.GetByteLength()
}
