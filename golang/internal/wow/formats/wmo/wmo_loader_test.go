package wmo

import (
	"bytes"
	"encoding/binary"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"testing"
)

func TestMOC2IsSeparateFromBothMOCVStreams(t *testing.T) {
	for _, nested := range []bool{false, true} {
		var chunks bytes.Buffer
		for _, c := range []struct{ id, word uint32 }{{0x4D4F4356, 0xff010203}, {0x4D4F4356, 0x40000000}, {0x4D4F4332, 0x00ff0000}} {
			for _, word := range []uint32{c.id, 4, c.word} {
				binary.Write(&chunks, binary.LittleEndian, word)
			}
		}
		raw := chunks.Bytes()
		if nested {
			var group bytes.Buffer
			binary.Write(&group, binary.LittleEndian, uint32(0x4D4F4750))
			binary.Write(&group, binary.LittleEndian, uint32(68+len(raw)))
			group.Write(make([]byte, 68))
			group.Write(raw)
			raw = group.Bytes()
		}
		l := NewLoader(buffer.From(raw), 0, "", false)
		if err := l.Load(); err != nil {
			t.Fatal(err)
		}
		if len(l.VertexColours) != 2 || l.VertexColours[0][0] != 0xff010203 || l.VertexColours[1][0] != 0x40000000 || len(l.BlendColours) != 1 || l.BlendColours[0] != 0x00ff0000 {
			t.Fatalf("nested %v: MOCV %x MOC2 %x", nested, l.VertexColours, l.BlendColours)
		}
	}
}

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
