package directm2

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestBuildGeosetMaskForSkinDefaultSuffixWhenNoExtraGeosets(t *testing.T) {
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: 0},
			{SubmeshID: 101},
			{SubmeshID: 110},
			{SubmeshID: 201},
			{SubmeshID: 901},
		},
	}

	selected := &ModelSkin{ID: "arcanegolem2voidmount", Textures: []int{1, 2, 3}}
	mask := BuildGeosetMaskForSkin(skin, selected)

	want := map[int]bool{
		0:   true, // ends with 0
		101: true, // ends with 01
		110: true, // ends with 0
		201: true, // ends with 01
		901: true, // ends with 01
	}
	for i, entry := range mask {
		if entry.ID != int(skin.SubMeshes[i].SubmeshID) {
			t.Fatalf("mask[%d].ID = %d, want %d", i, entry.ID, skin.SubMeshes[i].SubmeshID)
		}
		if entry.Checked != want[entry.ID] {
			t.Fatalf("geoset %d checked=%v, want %v", entry.ID, entry.Checked, want[entry.ID])
		}
	}
}

func TestBuildGeosetMaskForSkinExtraGeosetsOverride(t *testing.T) {
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: 0},
			{SubmeshID: 101},
			{SubmeshID: 201},
			{SubmeshID: 901},
		},
	}

	selected := &ModelSkin{
		ID:           "variant",
		ExtraGeosets: []int{201},
	}
	mask := BuildGeosetMaskForSkin(skin, selected)

	want := map[int]bool{
		0:   true,  // base geometry
		101: false, // disabled in 0..900 range
		201: true,  // explicitly enabled
		901: true,  // outside 0..900 range
	}
	for i, entry := range mask {
		if entry.ID != int(skin.SubMeshes[i].SubmeshID) {
			t.Fatalf("mask[%d].ID = %d, want %d", i, entry.ID, skin.SubMeshes[i].SubmeshID)
		}
		if entry.Checked != want[entry.ID] {
			t.Fatalf("geoset %d checked=%v, want %v", entry.ID, entry.Checked, want[entry.ID])
		}
	}
}
