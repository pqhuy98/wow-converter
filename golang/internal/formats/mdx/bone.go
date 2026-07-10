package mdx

import "strconv"

// Bone is a skeletal bone node.
type Bone struct {
	*GenericObject
	GeosetID          int32
	GeosetAnimationID int32
}

func NewBone() *Bone {
	return &Bone{
		GenericObject:     NewGenericObject(0x100),
		GeosetID:          -1,
		GeosetAnimationID: -1,
	}
}

func (b *Bone) ReadMdx(stream *BinaryStream, _ int) error {
	if err := b.GenericObject.ReadMdx(stream); err != nil {
		return err
	}
	gid, err := stream.ReadInt32()
	if err != nil {
		return err
	}
	b.GeosetID = gid
	gaid, err := stream.ReadInt32()
	if err != nil {
		return err
	}
	b.GeosetAnimationID = gaid
	return nil
}

func (b *Bone) WriteMdx(stream *BinaryStream, _ int) {
	b.GenericObject.WriteMdx(stream)
	stream.WriteInt32(b.GeosetID)
	stream.WriteInt32(b.GeosetAnimationID)
}

func (b *Bone) ReadMdl(stream *TokenStream) error {
	return b.ReadGenericBlock(stream, func(token string) error {
		switch token {
		case "GeosetId":
			val, err := stream.Read()
			if err != nil {
				return err
			}
			if val == "Multiple" {
				b.GeosetID = -1
			} else {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				b.GeosetID = int32(n)
			}
		case "GeosetAnimId":
			val, err := stream.Read()
			if err != nil {
				return err
			}
			if val == "None" {
				b.GeosetAnimationID = -1
			} else {
				n, err := strconv.Atoi(val)
				if err != nil {
					return err
				}
				b.GeosetAnimationID = int32(n)
			}
		default:
			return unknownToken("Bone "+b.Name, token)
		}
		return nil
	})
}

func (b *Bone) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("Bone", b.Name)
	b.WriteGenericHeader(stream)
	if b.GeosetID == -1 {
		stream.WriteFlagAttrib("GeosetId", "Multiple")
	} else {
		stream.WriteNumberAttrib("GeosetId", float64(b.GeosetID))
	}
	if b.GeosetAnimationID == -1 {
		stream.WriteFlagAttrib("GeosetAnimId", "None")
	} else {
		stream.WriteNumberAttrib("GeosetAnimId", float64(b.GeosetAnimationID))
	}
	b.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (b *Bone) GetByteLength(_ int) int {
	return 8 + b.GenericObject.GetByteLength()
}
