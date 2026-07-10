package mdx

// Helper is a helper object node.
type Helper struct {
	*GenericObject
}

func NewHelper() *Helper {
	return &Helper{GenericObject: NewGenericObject(0)}
}

func (h *Helper) ReadMdx(stream *BinaryStream, _ int) error {
	return h.GenericObject.ReadMdx(stream)
}

func (h *Helper) WriteMdx(stream *BinaryStream, _ int) {
	h.GenericObject.WriteMdx(stream)
}

func (h *Helper) ReadMdl(stream *TokenStream) error {
	return h.ReadGenericBlock(stream, func(token string) error {
		return unknownToken("Helper", token)
	})
}

func (h *Helper) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("Helper", h.Name)
	h.WriteGenericHeader(stream)
	h.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (h *Helper) GetByteLength(_ int) int {
	return h.GenericObject.GetByteLength()
}
