package directm2

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestResolveTexturesUsesTypedOverridesBeforePositionalVariants(t *testing.T) {
	loader := &m2.Loader{
		Textures: []m2.TextureEntry{
			{FileDataID: 100},
			{FileDataID: 200},
			{FileDataID: 300},
		},
		TextureTypes: []uint32{0, 3, 4},
	}
	root := t.TempDir()
	getRaw := func(_ context.Context, id int) ([]byte, error) { return []byte{byte(id)}, nil }
	getName := func(_ context.Context, id int) (string, error) {
		return filepath.ToSlash(filepath.Join("item", "texture", fmt.Sprintf("%d.blp", id))), nil
	}

	resolved, err := ResolveTextures(context.Background(), loader, []int{701, 702, 703}, map[int]int{3: 903, 4: 904}, nil, filepath.Join(root, "sources"), root, getRaw, getName)
	if err != nil {
		t.Fatal(err)
	}
	if loader.Textures[0].FileDataID != 100 {
		t.Fatalf("type 0 source texture unexpectedly changed to %d", loader.Textures[0].FileDataID)
	}
	if loader.Textures[1].FileDataID != 903 || loader.Textures[2].FileDataID != 904 {
		t.Fatalf("typed overrides were not applied by M2 type: %#v", loader.Textures)
	}
	for _, id := range []int{100, 903, 904} {
		if _, ok := resolved.ValidTextures[uint32(id)]; !ok {
			t.Fatalf("resolved texture manifest missing exact replacement FID %d: %#v", id, resolved.ValidTextures)
		}
	}
	for _, id := range []int{701, 702, 703} {
		if _, ok := resolved.ValidTextures[uint32(id)]; ok {
			t.Fatalf("positional variant FID %d overrode explicit component mapping", id)
		}
	}
}

func TestResolveTexturesPreservesCreatureVariationSlots(t *testing.T) {
	for _, test := range []struct {
		name       string
		variations []int
		overrides  map[int]int
		want       []uint32
	}{
		{"all four", []int{701, 702, 703, 704}, nil, []uint32{701, 702, 703, 704}},
		{"sparse fourth", []int{701, 0, 0, 704}, nil, []uint32{701, 0, 0, 704}},
		{"fourth absent", []int{701}, nil, []uint32{701, 0, 0, 0}},
		{"typed fourth override", []int{701, 702, 703, 704}, map[int]int{5: 905}, []uint32{701, 702, 703, 905}},
	} {
		t.Run(test.name, func(t *testing.T) {
			loader := &m2.Loader{Textures: make([]m2.TextureEntry, 4), TextureTypes: []uint32{11, 12, 13, 5}}
			root := t.TempDir()
			getRaw := func(_ context.Context, id int) ([]byte, error) { return []byte{byte(id)}, nil }
			getName := func(_ context.Context, id int) (string, error) {
				return fmt.Sprintf("creature/texture_%d.blp", id), nil
			}
			resolved, err := ResolveTextures(context.Background(), loader, test.variations, test.overrides, nil, root, root, getRaw, getName)
			if err != nil {
				t.Fatal(err)
			}
			for i, want := range test.want {
				if got := loader.Textures[i].FileDataID; got != want {
					t.Fatalf("type %d resolved %d, want %d", loader.TextureTypes[i], got, want)
				}
				if want > 0 {
					if _, ok := resolved.ValidTextures[want]; !ok {
						t.Fatalf("missing resolved FID %d", want)
					}
				}
			}
		})
	}
}
