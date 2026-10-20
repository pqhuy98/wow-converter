package wmo

import "testing"

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
