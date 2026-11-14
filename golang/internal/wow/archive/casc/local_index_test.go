package casc

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func TestRealLocalIndexParity(t *testing.T) {
	dir := os.Getenv("CASC_LOCAL_INDEX_BENCH_DIR")
	if dir == "" {
		t.Skip("set CASC_LOCAL_INDEX_BENCH_DIR")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.idx"))
	if err != nil || len(files) == 0 {
		t.Fatal("no local journal indexes", err)
	}
	c := NewCASCLocal("")
	want := map[string]localIndexEntry{}
	for _, path := range files {
		if err := c.ParseIndex(path); err != nil {
			t.Fatal(err)
		}
		// Reference the previous reader, including first-entry-wins duplicates.
		data, err := buffer.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		header := int(data.ReadInt32LE().(int64))
		data.Seek((8 + header + 15) & ^15)
		length := int(data.ReadInt32LE().(int64))
		data.Move(4)
		for i := 0; i < length/18; i++ {
			key := data.ReadHexString(9)
			high := int(data.ReadUInt8().(int64))
			low := int(data.ReadInt32BE().(int64))
			size := int(data.ReadInt32LE().(int64))
			if _, exists := want[key]; !exists {
				want[key] = localIndexEntry{index: uint16(high<<2 | (low&0xC0000000)>>30), offset: uint32(low & 0x3FFFFFFF), size: uint32(size)}
			}
		}
	}
	if len(c.LocalIndexes) != len(want) {
		t.Fatal("local index count changed")
	}
	for key, entry := range c.LocalIndexes {
		if want[hex.EncodeToString(key[:])] != entry {
			t.Fatalf("local index entry changed: %x", key)
		}
	}
	t.Logf("matched %d local index entries in %d journals", len(want), len(files))
}
