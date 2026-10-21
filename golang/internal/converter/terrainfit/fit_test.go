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
	// Historical HiveWE shader: top.a=.5; mid mix alpha=.625; final mix .625.
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

func TestHeightSamplingBridgesSourceCoordinateJitter(t *testing.T) {
	const gap = 8e-7
	m := Mesh{
		Positions: [][3]float64{{0, 0, 10}, {.5 - gap, 0, 15}, {0, 1, 30}, {.5 - gap, 1, 35}, {.5 + gap, 0, 15}, {1, 0, 20}, {.5 + gap, 1, 35}, {1, 1, 40}},
		Indices:   []uint32{0, 1, 2, 2, 1, 3, 4, 5, 6, 6, 5, 7},
	}
	heights, valid := rasterHeights(m, 3, false)
	if !valid[4] || math.Abs(heights[4]-25) > 1e-8 {
		t.Fatalf("float32 chunk gap lost the shared height: %v, %v", heights, valid)
	}
	if m.Positions[1][0] != .5-gap {
		t.Fatal("sampling mutated the source mesh")
	}
}

func TestSeamMetricsSeparateContinuityFromAccuracy(t *testing.T) {
	a, b := [3]float64{50, 50, 50}, [3]float64{100, 100, 100}
	var exact, artificial seamSum
	exact.add(a, a)
	artificial.add(a, b)
	if score := exact.metrics(); score.MAE != 0 || score.P95 != 50 || score.ReferenceStep != 50 {
		t.Fatalf("authored edge penalized: %+v", score)
	}
	if score := artificial.metrics(); score.MAE != 50 || score.P95 != 100 {
		t.Fatalf("extra tile discontinuity missed: %+v", score)
	}
}

func TestBoundaryJumpRemovesSmoothSlopes(t *testing.T) {
	ref, result := image.NewNRGBA(image.Rect(0, 0, 1024, 1024)), image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	mask, coverage := make([]bool, cellsPerADT*cellsPerADT), make([]bool, cellsPerADT*cellsPerADT)
	coverage[63], coverage[64] = true, true
	for y := 0; y < 8; y++ {
		for x := 500; x < 530; x++ {
			for k := 0; k < 3; k++ {
				result.Pix[y*result.Stride+x*4+k] = byte(x - 500)
			}
		}
	}
	var smooth seamMeasurement
	smooth.add(ref, result, mask, coverage)
	if score := smooth.metrics().Native; score.ResultStep != 0 || score.Pixels != 8 {
		t.Fatalf("smooth slope treated as a seam: %+v", score)
	}
	for y := 0; y < 8; y++ {
		for x := 512; x < 530; x++ {
			for k := 0; k < 3; k++ {
				result.Pix[y*result.Stride+x*4+k] += 20
			}
		}
	}
	var stepped seamMeasurement
	stepped.add(ref, result, mask, coverage)
	if score := stepped.metrics().Native; score.ResultStep != 20 || score.P95 != 20 {
		t.Fatalf("tile color jump missed: %+v", score)
	}
}

func TestModelFallbackClipsOnlySelectedArea(t *testing.T) {
	m := Mesh{Positions: [][3]float64{{0, 0, 10}, {1, 0, 20}, {0, 1, 30}, {1, 1, 40}}, Indices: []uint32{0, 1, 2, 2, 1, 3}}
	mask := make([]bool, cellsPerADT*cellsPerADT)
	placements := []patchPlacement{}
	for y := 0; y < cellsPerADT/2; y++ {
		for x := 0; x < cellsPerADT/2; x++ {
			mask[y*cellsPerADT+x] = true
			placements = append(placements, patchPlacement{x: x, y: y})
		}
	}
	result, _ := clipPatches(m, mask, placements, patchAtlasSize, 2)
	area := 0.0
	for i := 0; i < len(result.Indices); i += 3 {
		a, b, c := result.Positions[result.Indices[i]], result.Positions[result.Indices[i+1]], result.Positions[result.Indices[i+2]]
		area += math.Abs((b[0]-a[0])*(c[1]-a[1])-(c[0]-a[0])*(b[1]-a[1])) / 2
	}
	if math.Abs(area-.25) > 1e-9 {
		t.Fatalf("patch area %g != .25", area)
	}
	for _, p := range result.Positions {
		if p[0] < -1e-10 || p[0] > .5+1e-10 || p[1] < -1e-10 || p[1] > .5+1e-10 || math.Abs(p[2]-(10+10*p[0]+20*p[1])) > 1e-8 {
			t.Fatalf("source geometry changed: %v", p)
		}
	}
}

func TestHeightClampModelsAreMandatory(t *testing.T) {
	width, height := cellsPerADT, cellsPerADT
	mask := make([]bool, width*height)
	m := Mesh{Positions: [][3]float64{{.5, .5, 250}, {.125, .25, 0}, {.75, .75, 50}}}
	markHeightFallback(m, 0, 0, width, height, [2]float64{10, 100}, mask)
	for _, p := range m.Positions[:2] {
		x, y := int(p[0]*cellsPerADT), int(p[1]*cellsPerADT)
		for dy := -1; dy <= 0; dy++ {
			for dx := -1; dx <= 0; dx++ {
				if !mask[(y+dy)*width+x+dx] {
					t.Fatal("out-of-range vertex did not preserve all adjoining cells as models")
				}
			}
		}
	}
	if mask[96*width+96] {
		t.Fatal("in-range point became mandatory model")
	}
	native := Mesh{Positions: make([][3]float64, (width+1)*(height+1))}
	for i := range native.Positions {
		native.Positions[i][2] = 25
	}
	patches := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := width / 2; x < width; x++ {
			patches[y*width+x] = true
		}
	}
	patch := Mesh{Positions: [][3]float64{{.5, .4, 30}, {.75, .4, 250}, {1, .4, 400}}}
	before, after := weldPatchBoundary(&patch, nil, native, width, height, 0, 0, patches)
	if before != 5 || after != 0 || patch.Positions[0][2] != 25 || patch.Positions[1][2] != 250 || patch.Positions[2][2] != 400 {
		t.Fatalf("welding changed source interior or left a seam: %v, %g %g", patch.Positions, before, after)
	}
}
