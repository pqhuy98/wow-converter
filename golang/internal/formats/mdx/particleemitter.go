package mdx

// ParticleEmitterFlags for particle emitter flags.
type ParticleEmitterFlags uint32

const (
	ParticleEmitterFlagUsesMDL ParticleEmitterFlags = 0x8000
	ParticleEmitterFlagUsesTGA ParticleEmitterFlags = 0x10000
)

// ParticleEmitter is the MDX particle emitter block.
type ParticleEmitter struct {
	*GenericObject
	EmissionRate float32
	Gravity      float32
	Longitude    float32
	Latitude     float32
	Path         string
	LifeSpan     float32
	Speed        float32
}

func NewParticleEmitter() *ParticleEmitter {
	return &ParticleEmitter{GenericObject: NewGenericObject(0x1000)}
}

func (p *ParticleEmitter) ReadMdx(stream *BinaryStream, _ int) error {
	start := stream.Index()
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	if err := p.GenericObject.ReadMdx(stream); err != nil {
		return err
	}
	p.EmissionRate, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Gravity, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Longitude, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Latitude, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Path, err = stream.Read(260)
	if err != nil {
		return err
	}
	p.LifeSpan, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	p.Speed, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	return p.ReadAnimations(stream, int(size)-(stream.Index()-start))
}

func (p *ParticleEmitter) WriteMdx(stream *BinaryStream, _ int) {
	stream.WriteUint32(uint32(p.GetByteLength(0)))
	p.GenericObject.WriteMdx(stream)
	stream.WriteFloat32(p.EmissionRate)
	stream.WriteFloat32(p.Gravity)
	stream.WriteFloat32(p.Longitude)
	stream.WriteFloat32(p.Latitude)
	written := stream.Write(p.Path)
	stream.Skip(260 - written)
	stream.WriteFloat32(p.LifeSpan)
	stream.WriteFloat32(p.Speed)
	p.WriteNonGenericAnimationChunks(stream)
}

func (p *ParticleEmitter) ReadMdl(stream *TokenStream) error {
	return p.ReadGenericBlock(stream, func(token string) error {
		switch token {
		case "EmitterUsesMDL":
			p.Flags |= GenericObjectFlags(ParticleEmitterFlagUsesMDL)
		case "EmitterUsesTGA":
			p.Flags |= GenericObjectFlags(ParticleEmitterFlagUsesTGA)
		case "static EmissionRate":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.EmissionRate = float32(v)
		case "EmissionRate":
			return p.ReadAnimation(stream, "KPEE")
		case "static Gravity":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Gravity = float32(v)
		case "Gravity":
			return p.ReadAnimation(stream, "KPEG")
		case "static Longitude":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Longitude = float32(v)
		case "Longitude":
			return p.ReadAnimation(stream, "KPLN")
		case "static Latitude":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			p.Latitude = float32(v)
		case "Latitude":
			return p.ReadAnimation(stream, "KPLT")
		case "Visibility":
			return p.ReadAnimation(stream, "KPEV")
		case "Particle":
			return p.ReadAnimatedBlock(stream, func(t string) error {
				switch t {
				case "static LifeSpan":
					v, err := stream.ReadFloat()
					if err != nil {
						return err
					}
					p.LifeSpan = float32(v)
				case "LifeSpan":
					return p.ReadAnimation(stream, "KPEL")
				case "static InitVelocity":
					v, err := stream.ReadFloat()
					if err != nil {
						return err
					}
					p.Speed = float32(v)
				case "InitVelocity":
					return p.ReadAnimation(stream, "KPES")
				case "Path":
					s, err := stream.Read()
					if err != nil {
						return err
					}
					p.Path = s
				}
				return nil
			})
		default:
			return unknownToken("ParticleEmitter", token)
		}
		return nil
	})
}

func (p *ParticleEmitter) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("ParticleEmitter", p.Name)
	p.WriteGenericHeader(stream)
	if p.Flags&GenericObjectFlags(ParticleEmitterFlagUsesMDL) != 0 {
		stream.WriteFlag("EmitterUsesMDL")
	}
	if p.Flags&GenericObjectFlags(ParticleEmitterFlagUsesTGA) != 0 {
		stream.WriteFlag("EmitterUsesTGA")
	}
	if !p.WriteAnimation(stream, "KPEE") {
		stream.WriteNumberAttrib("static EmissionRate", float64(p.EmissionRate))
	}
	if !p.WriteAnimation(stream, "KPEG") {
		stream.WriteNumberAttrib("static Gravity", float64(p.Gravity))
	}
	if !p.WriteAnimation(stream, "KPLN") {
		stream.WriteNumberAttrib("static Longitude", float64(p.Longitude))
	}
	if !p.WriteAnimation(stream, "KPLT") {
		stream.WriteNumberAttrib("static Latitude", float64(p.Latitude))
	}
	p.WriteAnimation(stream, "KPEV")
	stream.StartBlock("Particle")
	if !p.WriteAnimation(stream, "KPEL") {
		stream.WriteNumberAttrib("static LifeSpan", float64(p.LifeSpan))
	}
	if !p.WriteAnimation(stream, "KPES") {
		stream.WriteNumberAttrib("static InitVelocity", float64(p.Speed))
	}
	if p.Flags&GenericObjectFlags(ParticleEmitterFlagUsesMDL|ParticleEmitterFlagUsesTGA) != 0 {
		stream.WriteStringAttrib("Path", p.Path)
	}
	stream.EndBlock()
	p.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (p *ParticleEmitter) GetByteLength(_ int) int {
	return 288 + p.GenericObject.GetByteLength()
}
