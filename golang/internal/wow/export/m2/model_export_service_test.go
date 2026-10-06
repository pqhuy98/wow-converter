package m2export

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
)

func TestGetSkinForDisplayPreservesTextureVariationPositions(t *testing.T) {
	display := caches.ModelDisplay{Textures: []uint32{0, 0, 303, 404}}
	skin := getSkinForDisplay(display)
	if want := []int{0, 0, 303, 404}; !reflect.DeepEqual(skin.Textures, want) {
		t.Fatalf("skin textures = %v, want positional texture IDs %v", skin.Textures, want)
	}

	name, _ := archivecasc.GetByID(303)
	wantID := "unknown_303"
	if name != "" {
		wantID = strings.TrimSuffix(filepath.Base(name), ".blp")
	}
	if skin.ID != wantID {
		t.Fatalf("skin ID = %q, want primary populated texture ID %q", skin.ID, wantID)
	}
}

func TestSkinVariantKeyIncludesFourthTextureSlot(t *testing.T) {
	base := caches.ModelDisplay{Textures: []uint32{101, 202, 0, 0}, ExtraGeosets: []uint32{301}}
	withFourth := caches.ModelDisplay{Textures: []uint32{101, 202, 0, 404}, ExtraGeosets: []uint32{301}}
	if skinVariantKey(base) == skinVariantKey(withFourth) {
		t.Fatal("displays with different fourth texture slots shared a skin variant key")
	}
}

func TestFirstPositiveTextureFindsSparseDisplayTexture(t *testing.T) {
	if got := firstPositiveTexture([]uint32{0, 0, 0, 505}); got != 505 {
		t.Fatalf("first positive texture = %d, want 505", got)
	}
	if got := firstPositiveTexture([]uint32{0, 0, 0, 0}); got != 0 {
		t.Fatalf("empty texture variation = %d, want 0", got)
	}
}

func TestDisambiguateSkinVariantsOnlyChangesDuplicateLabels(t *testing.T) {
	skins := []casc.ModelSkin{
		{ID: "skin", Label: "skin", DisplayID: 10},
		{ID: "skin", Label: "skin", DisplayID: 20},
		{ID: "other", Label: "other", DisplayID: 30},
	}
	disambiguateSkinVariants(skins)
	if skins[0].ID != "skin_display10" || skins[0].Label != "skin [display 10]" ||
		skins[1].ID != "skin_display20" || skins[1].Label != "skin [display 20]" {
		t.Fatalf("duplicate variants not uniquely labeled: %+v", skins[:2])
	}
	if skins[2].ID != "other" || skins[2].Label != "other" {
		t.Fatalf("non-duplicate variant ID changed: %+v", skins[2])
	}
}
