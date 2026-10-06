package caches

import (
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

func TestCreatureTextureVariationsPreservesEmptySlots(t *testing.T) {
	got := creatureTextureVariations(db.DB2Row{
		"TextureVariationFileDataID": []int64{101, 0, 303, 0},
	})
	want := []uint32{101, 0, 303, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("texture variations = %v, want positional slots %v", got, want)
	}
}
