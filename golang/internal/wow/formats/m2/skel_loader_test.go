package m2

import (
	"encoding/binary"
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func TestSkelLoaderParseChunkSKS1GlobalLoopDurations(t *testing.T) {
	want := []uint32{3000, 0x00012345, 0x12345678}
	const headerSize = 32
	chunk := make([]byte, headerSize+len(want)*4)
	putU32 := func(offset int, value uint32) {
		t.Helper()
		binary.LittleEndian.PutUint32(chunk[offset:offset+4], value)
	}

	// SKS1 stores global-loop count and a payload-relative offset, followed by
	// sequence and lookup headers. Each global-loop duration is a u32.
	putU32(0, uint32(len(want)))
	putU32(4, headerSize)
	for i, duration := range want {
		putU32(headerSize+i*4, duration)
	}

	loader := NewSkelLoader(buffer.From(chunk), nil)
	loader.parseChunkSKS1()

	if !reflect.DeepEqual(loader.GlobalLoops, want) {
		t.Fatalf("global loop durations = %#v, want %#v", loader.GlobalLoops, want)
	}
}
