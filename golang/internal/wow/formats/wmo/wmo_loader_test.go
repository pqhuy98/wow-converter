package wmo

import (
	"bytes"
	"encoding/binary"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"testing"
)

func TestMaterialUVSpeedChunkHasFourFloatsPerMaterial(t *testing.T) {
	var raw bytes.Buffer
	for _, v := range []uint32{0x4D4F5556, 32} {
		if err := binary.Write(&raw, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	want := [][4]float32{{100, -200, 300, -400}, {0, 20, 0, -30}}
	for _, v := range want {
		if err := binary.Write(&raw, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	l := NewLoader(buffer.From(raw.Bytes()), 0, "", false)
	if err := l.Load(); err != nil {
		t.Fatal(err)
	}
	if len(l.MaterialUVSpeed) != 2 || l.MaterialUVSpeed[0] != want[0] || l.MaterialUVSpeed[1] != want[1] {
		t.Fatalf("UV speeds: %v", l.MaterialUVSpeed)
	}
}
