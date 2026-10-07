package character

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

func TestCollectionMaterialRespectsSelectedColorVariation(t *testing.T) {
	meta := wowhead.CharacterCustomization{Options: []wowhead.CustomizationOption{{ID: 57, Choices: []wowhead.CustomizationChoice{{ID: 902, Elements: []wowhead.CustomizationElement{
		{SkinnedModel: &wowhead.ElementSkinnedModel{CollectionFileDataID: 7760204, GeosetType: 25, GeosetID: 1}},
		{VariationChoiceID: 7636, Material: &wowhead.ElementMaterial{TextureTarget: 40, MaterialResourcesID: 1104902}},
		{VariationChoiceID: 62818, Material: &wowhead.ElementMaterial{TextureTarget: 40, MaterialResourcesID: 1104901}},
	}}}}}}
	for _, color := range []int{7636, 62818} {
		selected := selectedCustomizationElements(meta, []wowhead.Customization{{OptionID: 57, ChoiceID: 902}, {OptionID: 683, ChoiceID: color}})
		if len(selected) != 2 || selected[0].SkinnedModel == nil || selected[1].Material == nil || selected[1].VariationChoiceID != color {
			t.Fatalf("color %d did not retain only its collection material: %+v", color, selected)
		}
	}
}

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
