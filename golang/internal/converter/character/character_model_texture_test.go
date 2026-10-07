package character

import (
	"reflect"
	"testing"

	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestItemModelTexturesStayPairedWithModelComponent(t *testing.T) {
	item := ItemMetadata{
		ModelFiles: []FileWithComponent{{FileDataID: 4994107, ComponentID: 0}, {FileDataID: 4994072, ComponentID: 1}},
		ModelTextureFiles: [2][]FileWithComponent{
			{{FileDataID: 5153458, ComponentID: 2}, {FileDataID: 5154025, ComponentID: 3}},
			{{FileDataID: 5153434, ComponentID: 2}, {FileDataID: 5154043, ComponentID: 3}},
		},
	}
	maskBuilder := func(*m2.Skin) []m2export.GeosetMaskEntry {
		return []m2export.GeosetMaskEntry{{ID: 2600, Checked: true}}
	}

	first := itemAttachmentModelOptions(itemModelTextureFiles(item, 0), maskBuilder)
	if !reflect.DeepEqual(first.TextureIDs, []int{5153458, 5154025}) {
		t.Fatalf("first model texture IDs = %v, want its component-0 textures", first.TextureIDs)
	}
	if first.ReplaceableTextures["2"] != 5153458 || first.ReplaceableTextures["3"] != 5154025 {
		t.Fatalf("first model replaceable textures = %v", first.ReplaceableTextures)
	}

	second := itemAttachmentModelOptions(itemModelTextureFiles(item, 1), maskBuilder)
	if !reflect.DeepEqual(second.TextureIDs, []int{5153434, 5154043}) {
		t.Fatalf("second model texture IDs = %v, want its component-1 textures", second.TextureIDs)
	}
	if second.ReplaceableTextures["2"] != 5153434 || second.ReplaceableTextures["3"] != 5154043 {
		t.Fatalf("second model replaceable textures = %v", second.ReplaceableTextures)
	}
	if first.ReplaceableTextures["2"] == second.ReplaceableTextures["2"] {
		t.Fatal("models sharing a replaceable type unexpectedly share the same texture binding")
	}
	if first.GeosetMaskBuilder == nil {
		t.Fatal("attachment mask builder was dropped")
	}
	mask := first.GeosetMaskBuilder(nil)
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

func TestBareFootModelKeepsSkinAndAnkleArmor(t *testing.T) {
	files := []FileWithComponent{{FileDataID: 4535940, ComponentID: 6}, {FileDataID: 4535939, ComponentID: 7}}
	if got := characterBodyTextureFiles(files, 1); !reflect.DeepEqual(got, files[:1]) {
		t.Fatalf("bare-foot model must retain ankle overlay but preserve natural feet: %v", got)
	}
	if got := characterBodyTextureFiles(files, 0); !reflect.DeepEqual(got, files) {
		t.Fatalf("ordinary model lost its foot armor: %v", got)
	}
	if len(files) != 2 {
		t.Fatal("source equipment changed")
	}
}
