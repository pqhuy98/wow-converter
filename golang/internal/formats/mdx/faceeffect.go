package mdx

// FaceEffect is a Reforged face effect reference.
type FaceEffect struct {
	Type string
	Path string
}

func NewFaceEffect() *FaceEffect {
	return &FaceEffect{}
}

func (f *FaceEffect) ReadMdx(stream *BinaryStream) error {
	typ, err := stream.Read(80)
	if err != nil {
		return err
	}
	f.Type = typ
	f.Path, err = stream.Read(260)
	return err
}

func (f *FaceEffect) WriteMdx(stream *BinaryStream) {
	written := stream.Write(f.Type)
	stream.Skip(80 - written)
	written = stream.Write(f.Path)
	stream.Skip(260 - written)
}

func (f *FaceEffect) ReadMdl(stream *TokenStream) error {
	typ, err := stream.Read()
	if err != nil {
		return err
	}
	f.Type = typ
	return stream.ReadBlockIter(func(token string) error {
		switch token {
		case "Path":
			s, err := stream.Read()
			if err != nil {
				return err
			}
			f.Path = s
		default:
			return unknownToken("FaceEffect", token)
		}
		return nil
	})
}

func (f *FaceEffect) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("FaceFX", f.Type)
	stream.WriteStringAttrib("Path", f.Path)
	stream.EndBlock()
}

func (f *FaceEffect) GetByteLength(_ int) int { return 340 }
