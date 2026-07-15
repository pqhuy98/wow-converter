package common

import (
	"testing"

	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
)

func TestTileRegistryRegisterAndLookup(t *testing.T) {
	texturesource.Unregister("maps/kalimdor/tex_0_0.png")

	reg := NewTileRegistry()
	defer reg.Release()
	snap := &exportadt.ConversionOutput{
		ObjectPath: "maps/kalimdor/adt_0_0",
		Placements: []exportadt.PlacementRow{{ModelFile: "foo.m2", Type: "m2", FileDataID: "1"}},
		WmoPlacements: map[string][]exportadt.PlacementRow{
			"maps/kalimdor/building": {{ModelFile: "bar.m2", Type: "m2", FileDataID: "2"}},
		},
		Textures: []exportadt.BakedTexture{{RelPath: "maps/kalimdor/tex_0_0.png", PNG: []byte{137, 80, 78, 71}}},
	}
	reg.Register(snap)
	if got := reg.Snapshot("maps/kalimdor/adt_0_0"); got != snap {
		t.Fatal("snapshot lookup failed")
	}
	if len(reg.Placements("maps/kalimdor/adt_0_0")) != 1 {
		t.Fatal("adt placements missing")
	}
	if len(reg.Placements("maps/kalimdor/building")) != 1 {
		t.Fatal("wmo placements missing")
	}
	reg.RegisterTerrainTextures()
	if _, ok := texturesource.Get("maps/kalimdor/tex_0_0.png"); !ok {
		t.Fatal("terrain texture not registered")
	}
	texturesource.Unregister("maps/kalimdor/tex_0_0.png")
}
