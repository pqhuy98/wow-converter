package metadata

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestExtractWmoTexturesMaterialsUsesBlendMode(t *testing.T) {
	cfg := config.DefaultConfig()
	f := &File{
		Config:   cfg,
		FilePath: cfg.ExportAssetDir + "/world/test.wmo",
		fileType: "wmo",
		IsLoaded: true,
		textures: []textureMeta{
			{
				FileDataID:       100,
				FileNameExternal: "mm_street_03.png",
				MtlName:          "mat_mm_street_03",
			},
		},
		wmoMaterials: []wmoMaterialMeta{
			{Flags: 0x4, BlendMode: 0, Texture1: 100},
		},
	}

	result := f.extractWmoTexturesMaterials()
	mat := f.GetWmoMaterialByMtlName("mat_mm_street_03")
	if mat == nil {
		t.Fatal("expected WMO material lookup by mtl name")
	}
	if len(mat.Layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(mat.Layers))
	}
	if mat.Layers[0].FilterMode != components.BlendNone {
		t.Fatalf("opaque WMO blend mode 0 should be None, got %q", mat.Layers[0].FilterMode)
	}
	if !mat.TwoSided || !mat.Layers[0].TwoSided {
		t.Fatal("expected F_UNCULLED two-sided flag")
	}
	if len(result.Textures) != 1 {
		t.Fatalf("expected 1 texture, got %d", len(result.Textures))
	}
}

func TestExtractWmoTexturesMaterialsAlphaBlend(t *testing.T) {
	cfg := config.DefaultConfig()
	f := &File{
		Config:   cfg,
		FilePath: cfg.ExportAssetDir + "/world/test.wmo",
		fileType: "wmo",
		IsLoaded: true,
		textures: []textureMeta{
			{FileDataID: 200, FileNameExternal: "window.png", MtlName: "mat_window"},
		},
		wmoMaterials: []wmoMaterialMeta{
			{Flags: 0x2, BlendMode: 2, Texture1: 200},
		},
	}

	f.extractWmoTexturesMaterials()
	mat := f.GetWmoMaterialByMtlName("mat_window")
	if mat == nil {
		t.Fatal("expected material")
	}
	if mat.Layers[0].FilterMode != components.BlendBlend {
		t.Fatalf("alpha blend mode 2 should be Blend, got %q", mat.Layers[0].FilterMode)
	}
	if !mat.Layers[0].Unfogged {
		t.Fatal("expected F_UNFOGGED")
	}
	if !mat.Layers[0].NoDepthSet {
		t.Fatal("expected NoDepthSet for blendMode > 1")
	}
}
