package character

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestCollectionsReuseBakedHairInsteadOfRawHighlights(t *testing.T) {
	base := &mdl.MDL{Textures: []*components.Texture{
		{Image: "baked-skin.blp", WowData: components.TextureWowData{Type: 1}},
		{Image: "baked-brown-hair-with-white-highlights.blp", WowData: components.TextureWowData{Type: 6}},
		{WowData: components.TextureWowData{Type: 8}},
	}}
	raw := map[string]int{"1": 100, "6": 6856619, "8": 300}
	images := reuseBakedCustomizationTextures(base, raw)
	if images[6] != "baked-brown-hair-with-white-highlights.blp" || raw["6"] != 0 {
		t.Fatal("hair collections must reuse the blended base texture rather than the white highlight layer")
	}
	if images[1] != "baked-skin.blp" || raw["1"] != 0 {
		t.Fatal("skin collections must continue reusing the baked body texture")
	}
	if raw["8"] != 300 {
		t.Fatal("a missing base image must retain its raw texture fallback")
	}
}
