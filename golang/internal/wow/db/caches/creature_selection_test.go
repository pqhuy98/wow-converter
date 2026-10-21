package caches

import (
	"context"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
)

func TestNativeSelectionRadiusUsesClientBoundsAndNonlinearScale(t *testing.T) {
	display := db.DB2Row{"CreatureModelScale": float32(2)}
	model := db.DB2Row{"ModelScale": float32(.5), "GeoBoxMin": []float64{-1, -2, 0}, "GeoBoxMax": []float64{1, 2, 8}}
	options := casc.NpcSelectionOptions{ObjectScale: 1, Level: 10}
	radius, err := selectionRadiusFromRows(display, model, nil, 1, options)
	if err != nil {
		t.Fatal(err)
	}
	want := math.Sqrt(math.Sqrt(20) * .5)
	if math.Abs(radius-want) > 1e-7 {
		t.Fatalf("native radius=%g want %g", radius, want)
	}
	// Native object scale occurs inside sqrt, not as a second linear multiplier.
	options.ObjectScale = 4
	larger, err := selectionRadiusFromRows(display, model, nil, 1, options)
	if err != nil {
		t.Fatal(err)
	}
	scaled := math.Sqrt(20) * 2
	want = math.Sqrt(scaled) + math.Pow(scaled-5, 2)*.059999999
	if math.Abs(larger-want) > 1e-7 {
		t.Fatalf("large client branch=%g want %g", larger, want)
	}
	options.ObjectScale = 100
	radius, err = selectionRadiusFromRows(display, model, nil, 1, options)
	if err != nil || radius != 10 {
		t.Fatalf("native radius cap: %g %v", radius, err)
	}
}

func TestNativeSelectionFamilyExtraScaleAndEmptyBounds(t *testing.T) {
	display := db.DB2Row{"CreatureModelScale": float64(1)}
	model := db.DB2Row{"ModelScale": float64(1), "GeoBox": []any{float64(0), float64(0), float64(0), float64(2), float64(0), float64(2)}}
	family := db.DB2Row{"MinScale": float64(2), "MaxScale": float64(4), "MinScaleLevel": int64(10), "MaxScaleLevel": int64(20)}
	for _, tc := range []struct {
		level int
		want  float64
	}{{1, math.Sqrt(2)}, {15, math.Sqrt(3)}, {99, 2}} {
		got, err := selectionRadiusFromRows(display, model, family, 1, casc.NpcSelectionOptions{Level: tc.level, ObjectScale: 1})
		if err != nil || math.Abs(got-tc.want) > 1e-7 {
			t.Fatalf("family level %d: %g %v", tc.level, got, err)
		}
	}
	got, err := selectionRadiusFromRows(display, model, family, 5, casc.NpcSelectionOptions{Level: 20, ObjectScale: 1})
	if err != nil || math.Abs(got-math.Sqrt(5)) > 1e-7 {
		t.Fatalf("racial/display scale should exceed family scale: %g %v", got, err)
	}
	zero := db.DB2Row{"GeoBoxMinX": float64(0), "GeoBoxMaxX": float64(0)}
	got, err = selectionRadiusFromRows(display, zero, nil, 1, casc.NpcSelectionOptions{ObjectScale: 100})
	if err != nil || got != 1.2 {
		t.Fatalf("empty-box native fallback: %g %v", got, err)
	}
	if _, err := selectionRadiusFromRows(display, db.DB2Row{}, nil, 1, casc.NpcSelectionOptions{}); err == nil {
		t.Fatal("missing DB fields silently guessed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := CreatureSelectionRadius(ctx, 1, casc.NpcSelectionOptions{}); err == nil {
		t.Fatal("selection lookup ignored cancellation")
	}
}
