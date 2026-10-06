package character

import (
	"context"
	"errors"
	"testing"

	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

func TestGeneratedTextureDoesNotMatchReplaceableComponent(t *testing.T) {
	texture := &components.Texture{
		Image:   "wow/baked/uv2/shoulder_a1b2c3d4e5f6.blp",
		WowData: components.TextureWowData{Type: -1, PngPath: "baked/uv2/shoulder.png"},
	}
	model := mdl.New(mdl.NewMDLOptions{Name: "shoulder"})
	model.Textures = []*components.Texture{texture}

	if err := ApplyReplaceableTextures(&ExportContext{}, model, map[string]int{"0": 123}); err != nil {
		t.Fatalf("generated atlas should bypass replaceable export: %v", err)
	}
	if texture.Image != "wow/baked/uv2/shoulder_a1b2c3d4e5f6.blp" {
		t.Fatalf("generated atlas image was replaced with %q", texture.Image)
	}

	replaceable := map[string]int{"-1": 123}
	if images := reuseBakedCustomizationTextures(model, replaceable); len(images) != 0 {
		t.Fatalf("generated texture was reused as a customization component: %#v", images)
	}
	if replaceable["-1"] != 123 {
		t.Fatal("generated texture consumed replaceable type -1")
	}
}

func TestReplaceableTextureTypeMapPreservesTypeIDs(t *testing.T) {
	got := replaceableTextureTypeMap(map[string]int{"3": 903, "4": 904, "invalid": 905, "5": 0})
	if len(got) != 2 || got[3] != 903 || got[4] != 904 {
		t.Fatalf("typed replaceable map = %#v, want exact positive component mappings", got)
	}
}

func TestSkinMatchScoreIgnoresEmptyTextureVariationSlots(t *testing.T) {
	if got := skinMatchScore(nil, []int{101, 0, 0, 404}, nil, []int{202, 0, 0, 404}); got != 1 {
		t.Fatalf("skin score = %d, want one matching positive texture", got)
	}
	if got := skinMatchScore(nil, []int{0, 0}, nil, []int{0, 0}); got != 0 {
		t.Fatalf("empty variation slots scored %d", got)
	}
}

func TestCollectionGeosetMaskMatchesPostConversionSelection(t *testing.T) {
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: 101, TriangleCount: 3},
			{SubmeshID: 202, TriangleStart: 3, TriangleCount: 3},
			{SubmeshID: 203, TriangleStart: 3, TriangleCount: 3},
		},
		Triangles: []uint16{0, 1, 2, 0, 1, 2},
		Indices:   []uint16{0, 1, 2},
	}
	selected := map[int]struct{}{202: {}}
	mask := collectionGeosetMask(skin, selected)

	want := []m2export.GeosetMaskEntry{
		{ID: 101, Checked: false},
		{ID: 202, Checked: true},
		{ID: 203, Checked: false},
	}
	if len(mask) != len(want) {
		t.Fatalf("mask length = %d, want %d", len(mask), len(want))
	}
	for i := range want {
		if mask[i] != want[i] {
			t.Fatalf("mask[%d] = %+v, want %+v", i, mask[i], want[i])
		}
	}

	meshes := directm2.BuildMeshes(&m2.Loader{}, skin, mask, nil, nil)
	if len(meshes) != 1 || meshes[0].Name != "FacialB2" {
		t.Fatalf("selected collection meshes = %+v, want only geoset 202", meshes)
	}
}

func TestEquipmentCollectionMaskSelectsExactRenderableGeoset(t *testing.T) {
	shoulderOffset := 1
	selectedID := ComputeZamMeshId(26, &shoulderOffset)
	otherID := selectedID + 1
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: uint16(otherID), TriangleCount: 3},
			{SubmeshID: uint16(selectedID), TriangleCount: 3},
			{SubmeshID: 2699, TriangleCount: 0},
		},
		TextureUnits: []m2.SkinTextureUnit{
			{SkinSectionIndex: 1},
			{SkinSectionIndex: 0},
			{SkinSectionIndex: 2},
			{SkinSectionIndex: 9},
		},
	}
	proxy := collectionSelectionProxyModel(skin)
	if got, want := geosetSubmeshIDs(proxy), []int{otherID, selectedID}; !equalIntSlices(got, want) {
		t.Fatalf("selection proxy ids = %v, want renderable submesh order %v", got, want)
	}

	slots := []EquipmentSlotData{{
		SlotID: wowhead.SlotShoulder,
		Data: ItemMetadata{
			ModelFiles:     []FileWithComponent{{FileDataID: 77}},
			ZamGeosetGroup: []int{shoulderOffset},
		},
	}}
	builder := equipmentCollectionGeosetMaskBuilder(&ExportContext{
		WowClient: collectionMaskFileClient{fileName: "Item\\ObjectComponents\\Collections\\shoulder.m2"},
	}, slots, 77)
	if builder == nil {
		t.Fatal("known collection model did not get an early geoset mask")
	}
	mask := builder(skin)
	want := []m2export.GeosetMaskEntry{
		{ID: otherID, Checked: false},
		{ID: selectedID, Checked: true},
		{ID: 2699, Checked: false},
	}
	if !equalGeosetMasks(mask, want) {
		t.Fatalf("early equipment mask = %+v, want %+v", mask, want)
	}
}

func TestEquipmentCollectionNoneVariantDoesNotRestoreSiblings(t *testing.T) {
	noneOffset := -1
	const fileID = 77
	slots := []EquipmentSlotData{{
		SlotID: wowhead.SlotShoulder,
		Data: ItemMetadata{
			ModelFiles:     []FileWithComponent{{FileDataID: fileID}},
			ZamGeosetGroup: []int{noneOffset},
		},
	}}
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{
		{SubmeshID: 0, TriangleCount: 3},
		{SubmeshID: 2601, TriangleCount: 3},
		{SubmeshID: 2602, TriangleCount: 3},
	}}
	builder := equipmentCollectionGeosetMaskBuilder(&ExportContext{
		WowClient: collectionMaskFileClient{fileName: "item/objectcomponents/collections/shoulder.m2"},
	}, slots, fileID)
	if builder == nil {
		t.Fatal("known collection model did not get an early geoset mask")
	}
	mask := builder(skin)
	want := []m2export.GeosetMaskEntry{
		{ID: 0, Checked: true},
		{ID: 2601, Checked: false},
		{ID: 2602, Checked: false},
	}
	if !equalGeosetMasks(mask, want) {
		t.Fatalf("None-only equipment mask = %+v, want base only %+v", mask, want)
	}
}

func TestCollectionGeosetMaskSkipsEmptyPlaceholderWithSameID(t *testing.T) {
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: 2601, TriangleCount: 0},
			{SubmeshID: 2601, TriangleCount: 3},
			{SubmeshID: 2602, TriangleCount: 3},
		},
	}
	mask := collectionGeosetMask(skin, map[int]struct{}{2601: {}})
	want := []m2export.GeosetMaskEntry{
		{ID: 2601, Checked: false},
		{ID: 2601, Checked: true},
		{ID: 2602, Checked: false},
	}
	if !equalGeosetMasks(mask, want) {
		t.Fatalf("mask = %+v, want empty placeholder off %+v", mask, want)
	}
}

func TestCollectionVariantKeepsRenderableBaseSubmesh(t *testing.T) {
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: 0, TriangleCount: 3},
			{SubmeshID: 2601, TriangleCount: 3},
			{SubmeshID: 2602, TriangleCount: 3},
			{SubmeshID: 0, TriangleCount: 0},
		},
	}
	mask := collectionGeosetMask(skin, map[int]struct{}{2602: {}})
	want := []m2export.GeosetMaskEntry{
		{ID: 0, Checked: true},
		{ID: 2601, Checked: false},
		{ID: 2602, Checked: true},
		{ID: 0, Checked: false},
	}
	if !equalGeosetMasks(mask, want) {
		t.Fatalf("collection variant mask = %+v, want base plus selected extra %+v", mask, want)
	}
	missMask := collectionGeosetMask(skin, map[int]struct{}{2603: {}})
	missWant := []m2export.GeosetMaskEntry{
		{ID: 0, Checked: true},
		{ID: 2601, Checked: true},
		{ID: 2602, Checked: true},
		{ID: 0, Checked: false},
	}
	if !equalGeosetMasks(missMask, missWant) {
		t.Fatalf("unmatched variant mask = %+v, want all renderable geosets as fallback %+v", missMask, missWant)
	}

	base := &components.Geoset{
		Name: "opaque-base", WowData: components.GeosetWowData{SubmeshID: 0}, Faces: []components.Face{{}},
		Material: &components.Material{Layers: []components.Layer{{FilterMode: components.BlendNone}}},
	}
	selected := &components.Geoset{
		Name: "selected-effect", WowData: components.GeosetWowData{SubmeshID: 2602}, Faces: []components.Face{{}},
		Material: &components.Material{Layers: []components.Layer{{FilterMode: components.BlendAdditive}}},
	}
	unselected := &components.Geoset{
		Name: "other-effect-variant", WowData: components.GeosetWowData{SubmeshID: 2601}, Faces: []components.Face{{}},
		Material: &components.Material{Layers: []components.Layer{{FilterMode: components.BlendAdditive}}},
	}
	emptyBase := &components.Geoset{Name: "empty-base-placeholder", WowData: components.GeosetWowData{SubmeshID: 0}}
	model := mdl.New(mdl.NewMDLOptions{Name: "collection"})
	model.Geosets = []*components.Geoset{base, unselected, selected, emptyBase}
	got := geosetsMatchingSubmeshIDs(model, []int{2602})
	if len(got) != 2 || got[0] != base || got[1] != selected {
		t.Fatalf("post-filtered collection meshes = %+v, want base and selected variant only", got)
	}
}

func TestGeosetsMatchingSubmeshIDsDoesNotFallbackForNoneVariant(t *testing.T) {
	g2401 := &components.Geoset{Name: "horns", WowData: components.GeosetWowData{SubmeshID: 2401}}
	g2402 := &components.Geoset{Name: "horns-b", WowData: components.GeosetWowData{SubmeshID: 2402}}
	model := mdl.New(mdl.NewMDLOptions{Name: "collection"})
	model.Geosets = []*components.Geoset{g2401, g2402}
	got := geosetsMatchingSubmeshIDs(model, []int{2400})
	if len(got) != 0 {
		t.Fatalf("None variant kept %v, want no arbitrary sibling", got)
	}
}

func TestGeosetsMatchingSubmeshIDsKeepsRequestedVariantAndBaseForNone(t *testing.T) {
	base := &components.Geoset{Name: "base", WowData: components.GeosetWowData{SubmeshID: 0}, Faces: []components.Face{{}}}
	wanted := &components.Geoset{Name: "horns", WowData: components.GeosetWowData{SubmeshID: 2404}, Faces: []components.Face{{}}}
	other := &components.Geoset{Name: "blindfold", WowData: components.GeosetWowData{SubmeshID: 2503}, Faces: []components.Face{{}}}
	model := mdl.New(mdl.NewMDLOptions{Name: "collection"})
	model.Geosets = []*components.Geoset{base, wanted, other}
	got := geosetsMatchingSubmeshIDs(model, []int{2404, 2500})
	if len(got) != 2 || got[0] != base || got[1] != wanted {
		t.Fatalf("None and selected variant kept %v, want base and exact 2404", got)
	}
}

func TestGeosetsMatchingSubmeshIDsKeepsExactZeroVariant(t *testing.T) {
	zero := &components.Geoset{Name: "explicit-none", WowData: components.GeosetWowData{SubmeshID: 2400}, Vertices: make([]*components.GeosetVertex, 8)}
	sibling := &components.Geoset{Name: "horns", WowData: components.GeosetWowData{SubmeshID: 2401}, Vertices: make([]*components.GeosetVertex, 400)}
	model := mdl.New(mdl.NewMDLOptions{Name: "collection"})
	model.Geosets = []*components.Geoset{zero, sibling}
	got := geosetsMatchingSubmeshIDs(model, []int{2400})
	if len(got) != 1 || got[0] != zero {
		t.Fatalf("exact None variant kept %v, want explicit 2400 mesh", got)
	}
}

func TestGeosetsMatchingSubmeshIDsIgnoresEmptyZeroPlaceholder(t *testing.T) {
	zeroPlaceholder := &components.Geoset{Name: "empty-none", WowData: components.GeosetWowData{SubmeshID: 2400}}
	sibling := &components.Geoset{Name: "horns", WowData: components.GeosetWowData{SubmeshID: 2401}, Vertices: make([]*components.GeosetVertex, 400)}
	model := mdl.New(mdl.NewMDLOptions{Name: "collection"})
	model.Geosets = []*components.Geoset{zeroPlaceholder, sibling}
	if got := geosetsMatchingSubmeshIDs(model, []int{2400}); len(got) != 0 {
		t.Fatalf("empty exact None mesh selected %v, want no geometry and no sibling fallback", got)
	}
}

func TestCollectionGeosetMaskDoesNotFallbackForNoneOnlySelection(t *testing.T) {
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{
		{SubmeshID: 0, TriangleCount: 3},
		{SubmeshID: 2501, TriangleCount: 3},
		{SubmeshID: 2503, TriangleCount: 3},
	}}
	mask := collectionGeosetMask(skin, map[int]struct{}{}, []int{2500})
	want := []m2export.GeosetMaskEntry{
		{ID: 0, Checked: true},
		{ID: 2501, Checked: false},
		{ID: 2503, Checked: false},
	}
	if !equalGeosetMasks(mask, want) {
		t.Fatalf("None-only mask = %+v, want base only %+v", mask, want)
	}
	missingVariant := collectionGeosetMask(skin, map[int]struct{}{}, []int{2504})
	if !missingVariant[1].Checked || !missingVariant[2].Checked {
		t.Fatalf("missing nonzero variant should retain fallback behavior, got %+v", missingVariant)
	}
}

func TestCollectionGeosetMaskMixedFallbackPreservesNoneGroup(t *testing.T) {
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{
		{SubmeshID: 0, TriangleCount: 3},
		{SubmeshID: 2401, TriangleCount: 3},
		{SubmeshID: 2402, TriangleCount: 3},
		{SubmeshID: 2503, TriangleCount: 3},
	}}
	mask := collectionGeosetMask(skin, map[int]struct{}{}, []int{2404, 2500})
	want := []m2export.GeosetMaskEntry{
		{ID: 0, Checked: true},
		{ID: 2401, Checked: true},
		{ID: 2402, Checked: true},
		{ID: 2503, Checked: false},
	}
	if !equalGeosetMasks(mask, want) {
		t.Fatalf("mixed missing-variant mask = %+v, want fallback only in requested nonzero groups %+v", mask, want)
	}
}

func TestGeosetsMatchingSubmeshIDsPrefersLargestInGroup(t *testing.T) {
	stub := &components.Geoset{Name: "stub", WowData: components.GeosetWowData{SubmeshID: 2600}, Vertices: make([]*components.GeosetVertex, 12)}
	real := &components.Geoset{Name: "shoulders", WowData: components.GeosetWowData{SubmeshID: 2602}, Vertices: make([]*components.GeosetVertex, 400)}
	model := mdl.New(mdl.NewMDLOptions{Name: "collection"})
	model.Geosets = []*components.Geoset{stub, real}
	got := geosetsMatchingSubmeshIDs(model, []int{2601})
	if len(got) != 1 || got[0] != real {
		t.Fatalf("missing 2601 kept %v, want the largest geoset in group 26", got)
	}
}

func TestFilterCollectionGeosetsUsesChosenSeparateShoulderMetadata(t *testing.T) {
	rightSide, leftSide := 1, 0
	slots := []EquipmentSlotData{
		{
			SlotID:       wowhead.SlotShoulder,
			ShoulderSide: &rightSide,
			Data:         ItemMetadata{ZamGeosetGroup: []int{1}},
		},
		{
			SlotID:       wowhead.SlotShoulder,
			ShoulderSide: &leftSide,
			Data:         ItemMetadata{ZamGeosetGroup: []int{2}},
		},
	}
	rightVariant := &components.Geoset{Name: "right-gold", WowData: components.GeosetWowData{SubmeshID: 2602}, Vertices: make([]*components.GeosetVertex, 3), Faces: []components.Face{{}}}
	leftVariant := &components.Geoset{Name: "left-mushroom", WowData: components.GeosetWowData{SubmeshID: 2603}, Vertices: make([]*components.GeosetVertex, 3), Faces: []components.Face{{}}}
	otherVariant := &components.Geoset{Name: "unselected", WowData: components.GeosetWowData{SubmeshID: 2604}, Vertices: make([]*components.GeosetVertex, 3), Faces: []components.Face{{}}}
	model := mdl.New(mdl.NewMDLOptions{Name: "separate-shoulders"})
	model.Geosets = []*components.Geoset{rightVariant, leftVariant, otherVariant}

	for _, tc := range []struct {
		name string
		slot EquipmentSlotData
		want *components.Geoset
	}{
		{name: "right", slot: slots[0], want: rightVariant},
		{name: "left", slot: slots[1], want: leftVariant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := FilterCollectionGeosets(slots, tc.slot, model)
			if len(got) != 1 || got[0] != tc.want {
				t.Fatalf("selected shoulder geosets = %v, want %s", got, tc.want.Name)
			}
		})
	}
}

func TestGeosetsMatchingSubmeshIDsPreservesSmallExactVariant(t *testing.T) {
	exact := &components.Geoset{Name: "small-valid-variant", WowData: components.GeosetWowData{SubmeshID: 2601}, Vertices: make([]*components.GeosetVertex, 12)}
	sibling := &components.Geoset{Name: "larger-unrequested-sibling", WowData: components.GeosetWowData{SubmeshID: 2602}, Vertices: make([]*components.GeosetVertex, 400)}
	model := mdl.New(mdl.NewMDLOptions{Name: "collection"})
	model.Geosets = []*components.Geoset{exact, sibling}
	got := geosetsMatchingSubmeshIDs(model, []int{2601})
	if len(got) != 1 || got[0] != exact {
		t.Fatalf("small exact variant 2601 kept %v, want requested mesh", got)
	}
}

func TestCollectionGeosetMaskKeepsRenderableWhenSelectionMisses(t *testing.T) {
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: 1, TriangleCount: 3},
			{SubmeshID: 2, TriangleCount: 0},
			{SubmeshID: 3, TriangleCount: 3},
		},
	}
	mask := collectionGeosetMask(skin, map[int]struct{}{2601: {}})
	if !mask[0].Checked || mask[1].Checked || !mask[2].Checked {
		t.Fatalf("missing selection should keep renderable geosets, got %+v", mask)
	}
}

func TestCollectionGeosetMaskPreservesModelWhenRequestedGroupIsAbsent(t *testing.T) {
	// This shoulder M2 has its own legacy section IDs (201/2000), not the
	// equipment Zam group 2601. The selected model itself identifies the item,
	// so the unmatched equipment group must not mask every section out.
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{
		{SubmeshID: 201, TriangleCount: 36},
		{SubmeshID: 2000, TriangleCount: 48},
		{SubmeshID: 2000, TriangleCount: 4152},
	}}
	mask := collectionGeosetMask(skin, map[int]struct{}{}, []int{2601})
	want := []m2export.GeosetMaskEntry{
		{ID: 201, Checked: true},
		{ID: 2000, Checked: true},
		{ID: 2000, Checked: true},
	}
	if !equalGeosetMasks(mask, want) {
		t.Fatalf("mask for collection with unrelated source IDs = %+v, want all source sections %+v", mask, want)
	}
}

func TestCollectionGeosetMaskKeepsNoneIntentDuringMissingGroupFallback(t *testing.T) {
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{
		{SubmeshID: 2401, TriangleCount: 36},
		{SubmeshID: 2501, TriangleCount: 48},
		{SubmeshID: 2000, TriangleCount: 4152},
	}}
	// Group 24 is requested but missing, while group 25 explicitly requests
	// None. The absent group must not cause the present None group to return.
	mask := collectionGeosetMask(skin, map[int]struct{}{}, []int{2404, 2500})
	if !mask[0].Checked || mask[1].Checked || mask[2].Checked {
		t.Fatalf("missing-group fallback should keep only the requested-group fallback: %+v", mask)
	}
}

func TestCollectionProxyIncludesRenderableSubmeshesWithoutTextureUnits(t *testing.T) {
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: 2601, TriangleCount: 3},
			{SubmeshID: 1801, TriangleCount: 3},
		},
	}
	if got, want := geosetSubmeshIDs(collectionSelectionProxyModel(skin)), []int{2601, 1801}; !equalIntSlices(got, want) {
		t.Fatalf("proxy ids = %v, want all renderable submeshes %v", got, want)
	}
}

func TestEquipmentCollectionMaskFallsBackOnLookupErrorAndMissingVariant(t *testing.T) {
	shoulderOffset := 3
	selectedFallbackID := ComputeZamMeshId(26, nil)
	otherID := selectedFallbackID + 1
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: uint16(selectedFallbackID), TriangleCount: 3},
			{SubmeshID: uint16(otherID), TriangleCount: 3},
		},
		TextureUnits: []m2.SkinTextureUnit{{SkinSectionIndex: 0}, {SkinSectionIndex: 1}},
	}
	slots := []EquipmentSlotData{{
		SlotID: wowhead.SlotShoulder,
		Data: ItemMetadata{
			ModelFiles:     []FileWithComponent{{FileDataID: 88}},
			ZamGeosetGroup: []int{shoulderOffset},
		},
	}}

	if builder := equipmentCollectionGeosetMaskBuilder(&ExportContext{
		WowClient: collectionMaskFileClient{err: errors.New("lookup failed")},
	}, slots, 88); builder != nil {
		t.Fatal("lookup failure should preserve unmasked conversion fallback")
	}
	if builder := equipmentCollectionGeosetMaskBuilder(&ExportContext{
		WowClient: collectionMaskFileClient{fileName: "Item\\Weapons\\sword.m2"},
	}, slots, 88); builder != nil {
		t.Fatal("non-collection model should preserve unmasked conversion")
	}

	builder := equipmentCollectionGeosetMaskBuilder(&ExportContext{
		WowClient: collectionMaskFileClient{fileName: "item/objectcomponents/collections/shoulder.m2"},
	}, slots, 88)
	if builder == nil {
		t.Fatal("collection path should install a mask builder")
	}
	mask := builder(skin)
	if len(mask) != 2 || !mask[0].Checked || mask[1].Checked {
		t.Fatalf("missing variant fallback mask = %+v, want first geoset in the requested group", mask)
	}
}

func TestEquipmentCollectionMaskUnionsSlotsSharingModelFile(t *testing.T) {
	shoulderOffset := 1
	shoulderID := ComputeZamMeshId(26, &shoulderOffset)
	chestOffset := 2
	chestID := ComputeZamMeshId(10, &chestOffset)
	otherChestID := chestID + 1
	const sharedFileID = 99
	slots := []EquipmentSlotData{
		{
			SlotID: wowhead.SlotShoulder,
			Data: ItemMetadata{
				ModelFiles:     []FileWithComponent{{FileDataID: sharedFileID}},
				ZamGeosetGroup: []int{shoulderOffset},
			},
		},
		{
			SlotID: wowhead.SlotChest,
			Data: ItemMetadata{
				ModelFiles:     []FileWithComponent{{FileDataID: sharedFileID}},
				ZamGeosetGroup: []int{0, chestOffset},
			},
		},
	}
	skin := &m2.Skin{
		SubMeshes: []m2.SkinSubMesh{
			{SubmeshID: uint16(shoulderID), TriangleCount: 3},
			{SubmeshID: uint16(chestID), TriangleCount: 3},
			{SubmeshID: uint16(otherChestID), TriangleCount: 3},
		},
		TextureUnits: []m2.SkinTextureUnit{{SkinSectionIndex: 0}, {SkinSectionIndex: 1}, {SkinSectionIndex: 2}},
	}
	builder := equipmentCollectionGeosetMaskBuilder(&ExportContext{
		WowClient: collectionMaskFileClient{fileName: "item/objectcomponents/collections/shared.m2"},
	}, slots, sharedFileID)
	mask := builder(skin)
	want := []m2export.GeosetMaskEntry{
		{ID: shoulderID, Checked: true},
		{ID: chestID, Checked: true},
		{ID: otherChestID, Checked: false},
	}
	if !equalGeosetMasks(mask, want) {
		t.Fatalf("shared-file equipment mask = %+v, want union %+v", mask, want)
	}
}

type collectionMaskFileClient struct {
	client.Client
	fileName string
	err      error
}

func (c collectionMaskFileClient) GetFileByID(_ context.Context, fileDataID int) (casc.ListfileEntry, error) {
	return casc.ListfileEntry{FileDataID: fileDataID, FileName: c.fileName}, c.err
}

func geosetSubmeshIDs(model *mdl.MDL) []int {
	ids := make([]int, 0, len(model.Geosets))
	for _, geoset := range model.Geosets {
		if geoset != nil {
			ids = append(ids, geoset.WowData.SubmeshID)
		}
	}
	return ids
}

func equalIntSlices(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func equalGeosetMasks(left, right []m2export.GeosetMaskEntry) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
