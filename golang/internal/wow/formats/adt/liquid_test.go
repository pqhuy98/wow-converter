package adt

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func TestLiquidLegacyRectangleMaskAndAbsoluteHeights(t *testing.T) {
	// Native MH2O v0 height/depth rectangle at x2/y1, width1/height2.
	// Only the first cell is present. Final vertex block ends the chunk.
	b := new(bytes.Buffer)
	put := func(v uint32) { _ = binary.Write(b, binary.LittleEndian, v) }
	put(3072)
	put(1)
	put(0)
	b.Write(make([]byte, 255*12))
	_ = binary.Write(b, binary.LittleEndian, uint16(5))
	_ = binary.Write(b, binary.LittleEndian, uint16(0))
	_ = binary.Write(b, binary.LittleEndian, float32(100))
	_ = binary.Write(b, binary.LittleEndian, float32(105))
	b.Write([]byte{2, 1, 1, 2})
	put(3096)
	put(3097)
	b.WriteByte(1)
	for i := 0; i < 6; i++ {
		_ = binary.Write(b, binary.LittleEndian, float32(100+i))
	}
	b.Write(make([]byte, 6))
	a := NewADTLoader(nil)
	a.Chunks = make([]ADTChunk, 256)
	a.Chunks[0].Position = [3]float32{10, 20, 500}
	parseMH2O(a, buffer.From(b.Bytes()), b.Len())
	s, err := a.LiquidSurfaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 1 || len(s[0].Cells) != 1 || s[0].Cells[0][0][2] != 100 || s[0].Cells[0][2][2] != 103 {
		t.Fatalf("lost rectangle/mask or added chunk base height: %+v", s)
	}
	if len(a.LiquidChunks[0].Instances[0].VertexData.Depth) != 6 {
		t.Fatal("final vertex block was not decoded")
	}
	if x := s[0].Cells[0][0][0]; x < 5.8 || x > 5.9 {
		t.Fatal("rectangle offsets were discarded", x)
	}
}

func TestLegacyMCLQMaskedMagma(t *testing.T) {
	b := new(bytes.Buffer)
	for _, v := range []float32{42, 42} {
		_ = binary.Write(b, binary.LittleEndian, v)
	}
	for i := 0; i < 81; i++ {
		_ = binary.Write(b, binary.LittleEndian, uint32(0))
		_ = binary.Write(b, binary.LittleEndian, float32(42))
	}
	mask := bytes.Repeat([]byte{15}, 64)
	mask[3] = 0x80
	b.Write(mask)
	chunk := ADTChunk{Flags: 0x10, Position: [3]float32{0, 0, 900}}
	rootMCNKHandlers[0x4D434C51](&chunk, buffer.From(b.Bytes()), b.Len())
	a := NewADTLoader(nil)
	a.Chunks = []ADTChunk{chunk}
	s, err := a.LiquidSurfaces()
	if err != nil || len(s) != 1 || len(s[0].Cells) != 1 || s[0].TypeID != 3 || s[0].Cells[0][0][2] != 42 {
		t.Fatal("legacy liquid mask, material or absolute height was lost", s, err)
	}
}

func TestLiquidDepthOnlyOceanAndMalformedData(t *testing.T) {
	a := NewADTLoader(nil)
	a.Chunks = make([]ADTChunk, 1)
	a.LiquidChunks = []LiquidChunk{{Instances: []LiquidInstance{{LiquidType: 2, Width: 1, Height: 1, MinHeightLevel: 42, MaxHeightLevel: 42}}}}
	s, err := a.LiquidSurfaces()
	if err != nil || s[0].Cells[0][0][2] != 0 {
		t.Fatal("depth-only ocean must remain at sea level", err)
	}
	a.LiquidChunks[0].Instances[0].Width = 9
	if _, err = a.LiquidSurfaces(); err == nil {
		t.Fatal("accepted invalid liquid dimensions")
	}
}
