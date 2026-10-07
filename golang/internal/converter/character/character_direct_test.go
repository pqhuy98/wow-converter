package character

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestCharacterCustomizationNoneClearsAbsentZeroSection(t *testing.T) {
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{
		{SubmeshID: 0}, {SubmeshID: 101}, {SubmeshID: 101}, {SubmeshID: 204},
	}}
	body := ExportCharacterParams{Customizations: map[string]int{"facialHair": 1107, "hair": 7412}, CustomizationOrder: []int{1107, 7412}}
	choices := map[int]parsedChoiceMeta{1107: {Geosets: []int{100}}, 7412: {Geosets: []int{204}}}
	mask := buildCharacterGeosetMask(skin, choices, body)
	for i, want := range []bool{true, false, false, true} {
		if mask[i].Checked != want {
			t.Fatalf("section %d checked=%v, want %v", mask[i].ID, mask[i].Checked, want)
		}
	}

	// An unavailable nonzero variant must not erase the default as if it were None.
	choices[1107] = parsedChoiceMeta{Geosets: []int{105}}
	mask = buildCharacterGeosetMask(skin, choices, body)
	if !mask[1].Checked || !mask[2].Checked || !mask[3].Checked {
		t.Fatalf("missing nonzero variant erased existing geometry: %+v", mask)
	}
}
