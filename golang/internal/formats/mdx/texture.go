package mdx

import "strings"

// TextureWrapMode for texture wrap flags.
type TextureWrapMode uint32

const (
	TextureWrapRepeatBoth TextureWrapMode = 0
	TextureWrapWidth      TextureWrapMode = 1
	TextureWrapHeight     TextureWrapMode = 2
	TextureWrapBoth       TextureWrapMode = 3
)

// Texture is a model texture reference.
type Texture struct {
	ReplaceableID uint32
	Path          string
	WrapMode      TextureWrapMode
}

func NewTexture() *Texture {
	return &Texture{}
}

func (t *Texture) ReadMdx(stream *BinaryStream) error {
	rid, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	t.ReplaceableID = rid
	path, err := stream.Read(260)
	if err != nil {
		return err
	}
	t.Path = path
	wm, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	t.WrapMode = TextureWrapMode(wm)
	return nil
}

func (t *Texture) WriteMdx(stream *BinaryStream) {
	stream.WriteUint32(t.ReplaceableID)
	written := stream.Write(strings.ReplaceAll(t.Path, "/", "\\"))
	stream.Skip(260 - written)
	stream.WriteUint32(uint32(t.WrapMode))
}

func (t *Texture) ReadMdl(stream *TokenStream) error {
	return stream.ReadBlockIter(func(token string) error {
		switch token {
		case "Image":
			s, err := stream.Read()
			if err != nil {
				return err
			}
			t.Path = s
		case "ReplaceableId":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			t.ReplaceableID = uint32(v)
		case "WrapWidth":
			t.WrapMode |= TextureWrapWidth
		case "WrapHeight":
			t.WrapMode |= TextureWrapHeight
		default:
			return unknownToken("Texture", token)
		}
		return nil
	})
}

func (t *Texture) WriteMdl(stream *TokenStream, _ int) {
	stream.StartBlock("Bitmap")
	if len(t.Path) > 0 {
		stream.WriteStringAttrib("Image", t.Path)
	}
	if t.ReplaceableID != 0 {
		stream.WriteNumberAttrib("ReplaceableId", float64(t.ReplaceableID))
	}
	if t.WrapMode&TextureWrapWidth != 0 {
		stream.WriteFlag("WrapWidth")
	}
	if t.WrapMode&TextureWrapHeight != 0 {
		stream.WriteFlag("WrapHeight")
	}
	stream.EndBlock()
}

func (t *Texture) GetByteLength(_ int) int { return 268 }
