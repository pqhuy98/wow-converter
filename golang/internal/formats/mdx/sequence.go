package mdx

// Sequence is an animation sequence.
type Sequence struct {
	Name       string
	Interval   []uint32
	MoveSpeed  float32
	NonLooping uint32
	Rarity     float32
	SyncPoint  uint32
	Extent     *Extent
}

func NewSequence() *Sequence {
	return &Sequence{
		Interval: make([]uint32, 2),
		Extent:   NewExtent(),
	}
}

func (s *Sequence) ReadMdx(stream *BinaryStream) error {
	name, err := stream.Read(80)
	if err != nil {
		return err
	}
	s.Name = name
	s.Interval, err = stream.ReadUint32Array(2)
	if err != nil {
		return err
	}
	s.MoveSpeed, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	s.NonLooping, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	s.Rarity, err = stream.ReadFloat32()
	if err != nil {
		return err
	}
	s.SyncPoint, err = stream.ReadUint32()
	if err != nil {
		return err
	}
	return s.Extent.ReadMdx(stream)
}

func (s *Sequence) WriteMdx(stream *BinaryStream) {
	written := stream.Write(s.Name)
	stream.Skip(80 - written)
	stream.WriteUint32Array(s.Interval)
	stream.WriteFloat32(s.MoveSpeed)
	stream.WriteUint32(s.NonLooping)
	stream.WriteFloat32(s.Rarity)
	stream.WriteUint32(s.SyncPoint)
	s.Extent.WriteMdx(stream)
}

func (s *Sequence) ReadMdl(stream *TokenStream) error {
	name, err := stream.Read()
	if err != nil {
		return err
	}
	s.Name = name
	return stream.ReadBlockIter(func(token string) error {
		switch token {
		case "Interval":
			view := make([]float32, 2)
			if _, err := stream.ReadVector(view); err != nil {
				return err
			}
			s.Interval[0] = uint32(view[0])
			s.Interval[1] = uint32(view[1])
		case "NonLooping":
			s.NonLooping = 1
		case "MoveSpeed":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			s.MoveSpeed = float32(v)
		case "Rarity":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			s.Rarity = float32(v)
		case "MinimumExtent":
			_, err := stream.ReadVector(s.Extent.Min)
			return err
		case "MaximumExtent":
			_, err := stream.ReadVector(s.Extent.Max)
			return err
		case "BoundsRadius":
			v, err := stream.ReadFloat()
			if err != nil {
				return err
			}
			s.Extent.BoundsRadius = float32(v)
		default:
			return unknownToken("Sequence", token)
		}
		return nil
	})
}

func (s *Sequence) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("Anim", s.Name)
	stream.WriteVectorAttrib("Interval", s.Interval)
	if s.NonLooping == 1 {
		stream.WriteFlag("NonLooping")
	}
	if s.MoveSpeed != 0 {
		stream.WriteNumberAttrib("MoveSpeed", float64(s.MoveSpeed))
	}
	if s.Rarity != 0 {
		stream.WriteNumberAttrib("Rarity", float64(s.Rarity))
	}
	s.Extent.WriteMdl(stream)
	stream.EndBlock()
}

func (s *Sequence) GetByteLength(_ int) int { return 132 }
