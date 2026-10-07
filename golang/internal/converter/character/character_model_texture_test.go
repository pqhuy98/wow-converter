package character

import (
	"reflect"
	"testing"

	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestItemAttachmentModelOptionsPreserveTextureVariants(t *testing.T) {
	item := ItemMetadata{ModelTextureFiles: [2][]FileWithComponent{
		{{FileDataID: 4532396, ComponentID: 2}, {FileDataID: 4532398, ComponentID: 4}},
		{{FileDataID: 4532396, ComponentID: 3}},
	}}
	maskBuilder := func(*m2.Skin) []m2export.GeosetMaskEntry {
		return []m2export.GeosetMaskEntry{{ID: 2600, Checked: true}}
	}

	opts := itemAttachmentModelOptions(item, maskBuilder)
	if !reflect.DeepEqual(opts.TextureIDs, []int{4532396, 4532398}) {
		t.Fatalf("attachment texture IDs = %v, want unique model variants", opts.TextureIDs)
	}
	if opts.ReplaceableTextures["2"] != 4532396 || opts.ReplaceableTextures["4"] != 4532398 || opts.ReplaceableTextures["3"] != 4532396 {
		t.Fatalf("attachment replaceable textures = %v", opts.ReplaceableTextures)
	}
	if opts.GeosetMaskBuilder == nil {
		t.Fatal("attachment geoset mask builder was dropped")
	}
	mask := opts.GeosetMaskBuilder(nil)
	if !reflect.DeepEqual(mask, []m2export.GeosetMaskEntry{{ID: 2600, Checked: true}}) {
		t.Fatalf("attachment mask builder result = %v", mask)
	}
}

func TestCollectionModelTextureIDsAreStableAndUnique(t *testing.T) {
	got := textureIDsFromReplaceableTextures(map[string]int{
		"5": 902, "3": 901, "4": 902, "8": 0, "9": -1,
	})
	want := []int{901, 902}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("collection texture IDs = %v, want %v", got, want)
	}
}
