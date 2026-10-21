package wmo

import (
	"bytes"
	"encoding/binary"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"testing"
)

func TestRenderingPreviewIncludesLiquidOnRequest(t *testing.T) {
	body := new(bytes.Buffer)
	for _, n := range []uint32{2, 2, 1, 1} {
		_ = binary.Write(body, binary.LittleEndian, n)
	}
	for _, n := range []float32{10, 20, 100} {
		_ = binary.Write(body, binary.LittleEndian, n)
	}
	_ = binary.Write(body, binary.LittleEndian, uint16(0))
	for i := 0; i < 4; i++ {
		_ = binary.Write(body, binary.LittleEndian, uint32(0))
		_ = binary.Write(body, binary.LittleEndian, float32(7))
	}
	body.WriteByte(0)
	file := new(bytes.Buffer)
	_ = binary.Write(file, binary.LittleEndian, uint32(0x4D4C4951))
	_ = binary.Write(file, binary.LittleEndian, uint32(body.Len()))
	file.Write(body.Bytes())
	for _, include := range []bool{false, true} {
		g := NewLoader(buffer.From(file.Bytes()), 0, "preview.wmo", true)
		g.IncludeLiquid = include
		if err := g.Load(); err != nil {
			t.Fatal(err)
		}
		if (g.Liquid != nil) != include {
			t.Fatalf("IncludeLiquid=%t did not select liquid parsing", include)
		}
		if include {
			g.LiquidType = 5
			surfaces, err := g.LiquidSurfaces(4)
			if err != nil || len(surfaces) != 1 || surfaces[0].Cells[0][0][2] != 7 {
				t.Fatal("lost local liquid height", surfaces, err)
			}
		}
	}
}

func TestLiquidLocalHeightsMaskAndLegacyTypes(t *testing.T) {
	g := &Loader{LiquidType: 5, Liquid: &Liquid{VertX: 3, VertY: 2, TileX: 2, TileY: 1, Corner: []float32{10, 20, 100}, Tiles: []byte{64, 15}, Vertices: []LiquidVertex{{Height: 7}, {Height: 7}, {Height: 7}, {Height: 7}, {Height: 7}, {Height: 7}}}}
	s, err := g.LiquidSurfaces(4)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != 1 || len(s[0].Cells) != 1 || s[0].TypeID != 5 || s[0].Cells[0][0][2] != 7 {
		t.Fatal("bad WMO mask or liquid corner Z was added twice", s)
	}
	g.LiquidType = 1
	s, err = g.LiquidSurfaces(0)
	if err != nil || s[0].TypeID != 14 {
		t.Fatal("legacy ocean type was not resolved", err)
	}
}
