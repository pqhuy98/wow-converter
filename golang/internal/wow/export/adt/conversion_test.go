package adt

import (
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

func TestNormalizeConversionMapDir(t *testing.T) {
	if got := normalizeConversionMapDir("Stormwind City", true); got != "StormwindCity" {
		t.Fatalf("unexpected normalized directory: %q", got)
	}
	if got := normalizeConversionMapDir("Stormwind City", false); got != "Stormwind City" {
		t.Fatalf("directory should remain unchanged: %q", got)
	}
}

func TestGameObjectPositionAcceptsDBNumericSlices(t *testing.T) {
	tests := []struct {
		name string
		pos  any
	}{
		{name: "float32", pos: []float32{1, 2, 3}},
		{name: "float64", pos: []float64{1, 2, 3}},
		{name: "mixed DB values", pos: []any{float32(1), int32(2), uint32(3)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GameObjectPosition(db.DB2Row{"Pos": tt.pos})
			if !reflect.DeepEqual(got, []float64{1, 2, 3}) {
				t.Fatalf("GameObjectPosition() = %v", got)
			}
		})
	}
}

func TestGameObjectScalarIDsAcceptSingleValueDBArrays(t *testing.T) {
	if got := toUint32([]int64{571}); got != 571 {
		t.Fatalf("toUint32(single-value DB array) = %d", got)
	}
}
