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
	if got, want := geosetSubmeshIDs(proxy), []int{selectedID, otherID}; !equalIntSlices(got, want) {
		t.Fatalf("selection proxy ids = %v, want renderable texture-unit order %v", got, want)
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
