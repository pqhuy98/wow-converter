package mdx

// UnknownChunk holds unrecognized MDX chunk data.
type UnknownChunk struct {
	Tag   string
	Chunk []byte
}

func NewUnknownChunk(stream *BinaryStream, size int, tag string) (*UnknownChunk, error) {
	data, err := stream.ReadUint8Array(size)
	if err != nil {
		return nil, err
	}
	return &UnknownChunk{Tag: tag, Chunk: data}, nil
}

func (u *UnknownChunk) WriteMdx(stream *BinaryStream) {
	stream.WriteBinary(u.Tag)
	stream.WriteUint32(uint32(len(u.Chunk)))
	stream.WriteUint8Array(u.Chunk)
}

func (u *UnknownChunk) GetByteLength(_ int) int {
	return 8 + len(u.Chunk)
}
