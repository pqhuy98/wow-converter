package mdx

// Attachment is a model attachment point.
type Attachment struct {
	*GenericObject
	Path         string
	AttachmentID int32
}

func NewAttachment() *Attachment {
	return &Attachment{GenericObject: NewGenericObject(0x800)}
}

func (a *Attachment) ReadMdx(stream *BinaryStream, _ int) error {
	start := stream.Index()
	size, err := stream.ReadUint32()
	if err != nil {
		return err
	}
	if err := a.GenericObject.ReadMdx(stream); err != nil {
		return err
	}
	a.Path, err = stream.Read(260)
	if err != nil {
		return err
	}
	a.AttachmentID, err = stream.ReadInt32()
	if err != nil {
		return err
	}
	return a.ReadAnimations(stream, int(size)-(stream.Index()-start))
}

func (a *Attachment) WriteMdx(stream *BinaryStream, _ int) {
	stream.WriteUint32(uint32(a.GetByteLength(0)))
	a.GenericObject.WriteMdx(stream)
	written := stream.Write(a.Path)
	stream.Skip(260 - written)
	stream.WriteInt32(a.AttachmentID)
	a.WriteNonGenericAnimationChunks(stream)
}

func (a *Attachment) ReadMdl(stream *TokenStream) error {
	return a.ReadGenericBlock(stream, func(token string) error {
		switch token {
		case "AttachmentID":
			v, err := stream.ReadInt()
			if err != nil {
				return err
			}
			a.AttachmentID = int32(v)
		case "Path":
			s, err := stream.Read()
			if err != nil {
				return err
			}
			a.Path = s
		case "Visibility":
			return a.ReadAnimation(stream, "KATV")
		default:
			return unknownToken("Attachment "+a.Name, token)
		}
		return nil
	})
}

func (a *Attachment) WriteMdl(stream *TokenStream, _ int) {
	stream.StartObjectBlock("Attachment", a.Name)
	a.WriteGenericHeader(stream)
	stream.WriteNumberAttrib("AttachmentID", float64(a.AttachmentID))
	if len(a.Path) > 0 {
		stream.WriteStringAttrib("Path", a.Path)
	}
	a.WriteAnimation(stream, "KATV")
	a.WriteGenericAnimations(stream)
	stream.EndBlock()
}

func (a *Attachment) GetByteLength(_ int) int {
	return 268 + a.GenericObject.GetByteLength()
}
