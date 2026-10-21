package terrainfit

import (
	"context"
	"image"
	"math"
	"testing"
)

func TestNativeShaderAndAddressing(t *testing.T) {
	for _, p := range []struct {
		u, v float64
		bit  uint8
	}{{1, 1, 1}, {0, 1, 2}, {1, 0, 4}, {0, 0, 8}} {
		if got := maskAlpha(p.bit, p.u, p.v); got != 1 {
			t.Fatalf("corner bit %d = %g", p.bit, got)
		}
		if got := maskAlpha(15^p.bit, p.u, p.v); got != 0 {
			t.Fatalf("opposite mask = %g", got)
		}
	}
	// Actual recursive shader: top.a=.5; mid mix alpha=.625; final mix .625.
	w := shaderWeights([4]float64{1, .75, .5}, 3)
	if w != [4]float64{.375, .3125, .3125, 0} {
		t.Fatalf("recursive alpha coefficients: %v", w)
	}
	f := &fit{width: 1, height: 1, labels: []uint8{3, 2, 1, 0}}
	l := f.cellLayers(0, 0)
	if l.count != 4 || l.ids != [4]uint8{0, 1, 2, 3} || l.masks != [4]uint8{1, 2, 4, 8} {
		t.Fatalf("layer ordering/masks: %+v", l)
	}
}

func TestMetricsAndCoverage(t *testing.T) {
	ref := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	result := image.NewNRGBA(ref.Bounds())
	coverage := make([]bool, 128*128)
	for i := range coverage {
		coverage[i] = true
		for k := 0; k < 3; k++ {
			ref.Pix[i*4+k] = 100
			result.Pix[i*4+k] = 110
		}
	}
	identity := measure(ref, ref, coverage)
	if identity.MAE != 0 || identity.RMSE != 0 || math.Abs(identity.SSIM-1) > 1e-12 {
		t.Fatalf("identity: %+v", identity.metrics)
	}
	shifted := measure(ref, result, coverage)
	if shifted.MAE != 10 || shifted.RMSE != 10 || shifted.P95 != 10 || shifted.EdgeMAE != 0 {
		t.Fatalf("offset: %+v", shifted.metrics)
	}
	for i := range coverage {
		coverage[i] = false
	}
	coverage[0] = true
	result.Pix[4] = 255
	masked := measure(ref, result, coverage)
	if masked.Pixels != 1 || masked.MAE != 10 {
		t.Fatalf("hole coverage not respected: %+v", masked.metrics)
	}
}

func TestFitMonotonicAndDeterministic(t *testing.T) {
	ref := make([]byte, 4*3*sampleCount*3)
	valid := make([]bool, 12)
	for i := range valid {
		valid[i] = true
	}
	for i := range ref {
		ref[i] = byte((i*17+i/11)%180 + 30)
	}
	a, b := newFit(4, 3, ref, valid, 4), newFit(4, 3, ref, valid, 4)
	score := func(f *fit) float64 {
		s := 0.0
		for y := 0; y < f.height; y++ {
			for x := 0; x < f.width; x++ {
				s += f.cost(x, y, f.cellLayers(x, y), 0)
			}
		}
		return s
	}
	before := score(a)
	if e := a.optimize(context.Background(), 2); e != nil {
		t.Fatal(e)
	}
	if score(a) > before+1e-6 {
		t.Fatal("shared-corner optimization regressed")
	}
	if e := b.optimize(context.Background(), 2); e != nil {
		t.Fatal(e)
	}
	for i := range a.labels {
		if a.labels[i] != b.labels[i] {
			t.Fatal("non-deterministic labels")
		}
	}
}

func TestHeightMeshAndHoleCoverage(t *testing.T) {
	m := Mesh{Positions: [][3]float64{{0, 0, 10}, {1, 0, 20}, {0, 1, 30}, {1, 1, 40}}, Indices: []uint32{0, 1, 2, 2, 1, 3}}
	heights, ok := rasterHeights(m, 3, false)
	if !ok[4] || math.Abs(heights[4]-25) > 1e-8 {
		t.Fatalf("barycentric height %v %v", heights, ok)
	}
	_, g := nativeMesh(Region{X0: 0, Y0: 0, X1: 0, Y1: 0}, []Mesh{m})
	if g.Max > 1/(modelScale*4) || g.ClampedFraction != 0 {
		t.Fatalf("representable plane: %+v", g)
	}
	m.Indices = m.Indices[:3]
	_, ok = rasterHeights(m, 4, true)
	if !ok[0] || ok[15] {
		t.Fatal("missing triangles became ground")
	}
}
