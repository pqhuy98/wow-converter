package common

import (
	"testing"

	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
)

func TestTileRegistryMovesTerrainPNGToTextureSource(t *testing.T) {
	const rel = "maps/kalimdor/tex_0_0.png"
	texturesource.Unregister(rel)

	reg := NewTileRegistry()
	defer reg.Release()
	png := []byte{137, 80, 78, 71, 1, 2, 3}
	snap := &exportadt.ConversionOutput{
		ObjectPath: "maps/kalimdor/adt_0_0",
		ObjText:    "v 0 0 0",
		MtlText:    "newmtl m",
		Textures:   []exportadt.BakedTexture{{RelPath: rel, PNG: png}},
	}
	reg.Register(snap)
	reg.RegisterTerrainTexturesFor(snap.ObjectPath)

	if len(snap.Textures[0].PNG) != 0 {
		t.Fatal("snapshot still holds terrain PNG bytes after register")
	}
	got, ok := texturesource.Get(rel)
	if !ok || got.Kind != texturesource.KindPNG || len(got.PNG) != len(png) {
		t.Fatalf("terrain PNG not in texturesource: ok=%v len=%d", ok, len(got.PNG))
	}
}

func TestTileRegistryTrimAfterParseKeepsPlacements(t *testing.T) {
	reg := NewTileRegistry()
	defer reg.Release()
	snap := &exportadt.ConversionOutput{
		ObjectPath: "maps/kalimdor/adt_0_0",
		ObjText:    "v 0 0 0",
		Placements: []exportadt.PlacementRow{{ModelFile: "foo.m2", Type: "m2", FileDataID: "1"}},
		WmoPlacements: map[string][]exportadt.PlacementRow{
			"maps/kalimdor/building": {{ModelFile: "bar.m2", Type: "m2", FileDataID: "2"}},
		},
	}
	reg.Register(snap)
	reg.TrimAfterParse()

	if snap.ObjText != "" {
		t.Fatal("expected snapshot obj text cleared")
	}
	if len(reg.Placements("maps/kalimdor/adt_0_0")) != 1 {
		t.Fatal("adt placements missing after trim")
	}
	if len(reg.Placements("maps/kalimdor/building")) != 1 {
		t.Fatal("wmo placements missing after trim")
	}
	if reg.Snapshot("maps/kalimdor/adt_0_0") != nil {
		t.Fatal("snapshot lookup should be empty after trim")
	}
}
