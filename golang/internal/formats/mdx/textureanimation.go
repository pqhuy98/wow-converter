package mdx

// TextureAnimation animates texture coordinates.
type TextureAnimation struct {
	AnimatedObject
}

func NewTextureAnimation() *TextureAnimation {
	return &TextureAnimation{}
}

func (t *TextureAnimation) ReadMdx(stream *BinaryStream) error {
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	return t.ReadAnimations(stream, int(size)-4)
}

func (t *TextureAnimation) WriteMdx(stream *BinaryStream, _ int) {
	stream.WriteUint32(uint32(t.GetByteLength(0)))
	t.WriteAnimations(stream)
}

func (t *TextureAnimation) ReadMdl(stream *TokenStream) error {
	return stream.ReadBlockIter(func(token string) error {
		switch token {
		case "Translation":
			return t.ReadAnimation(stream, "KTAT")
		case "Rotation":
			return t.ReadAnimation(stream, "KTAR")
		case "Scaling":
			return t.ReadAnimation(stream, "KTAS")
		default:
			return unknownToken("TextureAnimation", token)
		}
	})
}

func (t *TextureAnimation) WriteMdl(stream *TokenStream, _ int) {
	stream.StartBlock("TVertexAnim ")
	t.WriteAnimation(stream, "KTAT")
	t.WriteAnimation(stream, "KTAR")
	t.WriteAnimation(stream, "KTAS")
	stream.EndBlock()
}

func (t *TextureAnimation) GetByteLength(_ int) int {
	return 4 + t.AnimatedObject.GetByteLength(0)
}
