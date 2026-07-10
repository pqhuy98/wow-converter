package m2

import (
	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func readAnimChunk(data *buffer.Buffer, size int) []byte {
	return data.ReadBuffer(buffer.ReadBufferOptions{Length: size, Wrap: false}).([]byte)
}

const (
	chunkAFM2 = 0x324D4641
	chunkAFSA = 0x41534641
	chunkAFSB = 0x42534641
)

// AnimLoader parses WoW .anim files.
type AnimLoader struct {
	Data *buffer.Buffer

	IsLoaded               bool
	AnimData               []byte
	SkeletonAttachmentData []byte
	SkeletonBoneData       []byte
}

// NewAnimLoader creates an .anim loader.
func NewAnimLoader(data *buffer.Buffer) *AnimLoader {
	return &AnimLoader{Data: data}
}

// Load parses chunked or raw .anim data.
func (l *AnimLoader) Load(isChunked bool) {
	if l.IsLoaded {
		return
	}
	if !isChunked {
		l.AnimData = readAnimChunk(l.Data, l.Data.RemainingBytes())
		l.IsLoaded = true
		return
	}
	for l.Data.RemainingBytes() > 0 {
		chunkID := readU32(l.Data)
		chunkSize := int(readU32(l.Data))
		nextChunkPos := l.Data.Offset() + chunkSize
		switch chunkID {
		case chunkAFM2:
			l.AnimData = readAnimChunk(l.Data, chunkSize)
		case chunkAFSA:
			l.SkeletonAttachmentData = readAnimChunk(l.Data, chunkSize)
		case chunkAFSB:
			l.SkeletonBoneData = readAnimChunk(l.Data, chunkSize)
		}
		l.Data.Seek(nextChunkPos)
	}
	l.IsLoaded = true
}
