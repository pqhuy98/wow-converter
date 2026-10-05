package directm2

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

func TestEveryWMOShaderSelectsTheRendererTable(t *testing.T) {
	want := [24][3]int{{0, 0, 1}, {1, 3, 1}, {2, 3, 1}, {3, 1, 2}, {4, 0, 1}, {5, 1, 2}, {6, 4, 2}, {7, 0, 3}, {8, 6, 2}, {9, 4, 2}, {-1, -1, 2}, {10, 2, 3}, {11, 2, 3}, {12, 4, 2}, {-1, -1, 2}, {13, 4, 2}, {0, 0, 1}, {14, 2, 3}, {15, 7, 3}, {16, 4, 2}, {17, 7, 3}, {18, 0, 1}, {19, 8, 6}, {20, 0, 9}}
	if wmoEffects != want {
		t.Fatal("WMO pixel/vertex/sampler table diverged")
	}
	if _, err := WMOShaderSamplerCount(24); err == nil {
		t.Fatal("unknown shader silently accepted")
	}
}

func TestWMOHeightBlendUsesAllFourUVSetsAndSecondVertexColour(t *testing.T) {
	var images [9]*image.NRGBA
	for i := range images {
		images[i] = image.NewNRGBA(image.Rect(0, 0, 2, 1))
		images[i].SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
		images[i].SetNRGBA(1, 0, color.NRGBA{0, 255, 0, 255})
	}
	v := wmoVarying{uv: [4]imath.Vector2{{.25, .5}, {.75, .5}, {.25, .5}, {.75, .5}}, normal: imath.Vector3{1, 0, 0}}
	// Raw BGRA's red channel selects the first diffuse/height pair at UV1.
	v.color[1] = [4]float64{0, 0, 1, 0}
	a := sampleWMOProgram(wmoEffects[23], v, images, [4]float32{}, 0)
	// With no RGB weights the fourth pair must use UV4, not UV1 or UV2.
	v.color[1] = [4]float64{}
	b := sampleWMOProgram(wmoEffects[23], v, images, [4]float32{}, 0)
	if a.diffuse != [3]float64{1, 0, 0} || b.diffuse != [3]float64{0, 1, 0} {
		t.Fatalf("UV routing/height blend: %+v %+v", a, b)
	}
	v.color[1][3] = .75
	c := sampleWMOProgram(wmoEffects[23], v, images, [4]float32{}, 0)
	if math.Abs(c.diffuse[1]-.25) > 1e-9 {
		t.Fatalf("vertex AO: %+v", c)
	}
}

func TestWMOMaskedEmissiveIsIndependentFromDiffuse(t *testing.T) {
	tx := [9][4]float64{{.8, .6, .4, .25}, {.2, .4, .6, .5}, {.1, .2, .3, .75}}
	f := evaluateWMOCombiner(11, tx, [4]float64{0, 0, 0, .4}, [4]float64{})
	if math.Abs(f.emission[0]-(.8*.25*.2+.1*.75*.4)) > 1e-9 || f.diffuse[0] != .8 {
		t.Fatalf("emissive composition %+v", f)
	}
}

func TestLongUVLoopBakesOnlyFirstFourSeconds(t *testing.T) {
	gs := components.NewGlobalSequence(0, 36000)
	track := &components.Animation{GlobalSeq: &gs, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 36000: imath.Vector3{36, 0, 0}}}
	plan := makeBakeTimeline([2]*components.TextureAnim{{Translation: track}, nil}, [4]components.AnimatedOrStatic[float64]{}, 0, nil)
	if plan.period != 4000 || len(plan.moments) != 32 || plan.keys[4000] != 0 {
		t.Fatalf("unbounded bake: %+v", plan)
	}
	uv := transformBakeUV(imath.Vector2{}, bakeTransformAt(&components.TextureAnim{Translation: track}, plan.moments[31]))
	if math.Abs(uv[0]-3.875) > 1e-9 {
		t.Fatalf("stretched whole source loop rather than truncated: %v", uv)
	}
	for _, fps := range []int{12, 21, 30} {
		for _, window := range []int{4000, 12000} {
			requested := makeBakeTimeline([2]*components.TextureAnim{{Translation: track}, nil}, [4]components.AnimatedOrStatic[float64]{}, 0, nil, config.TextureBakingOptions{FPS: fps, WindowMS: window})
			if requested.period != window || len(requested.moments) != fps*window/1000 || requested.keys[window] != 0 {
				t.Fatalf("requested sampling not honored: %d FPS %d ms: %d frames/%d ms", fps, window, len(requested.moments), requested.period)
			}
			if math.Abs(requested.moments[1].time-1000.0/float64(fps)) > 1e-9 {
				t.Fatal("sample clock does not match FPS")
			}
		}
	}
	reduced := reduceBakeTimeline(plan)
	if len(reduced.moments) != 16 || reduced.keys[4000] != 0 || reduced.keys[3875] != 15 {
		t.Fatalf("downsampled timing: %+v", reduced)
	}
}

func TestLocalBakeWindowRepeatsWithoutSamplingBeyondWindow(t *testing.T) {
	track := &components.Animation{Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 12000: imath.Vector3{12, 0, 0}, 12001: imath.Vector3{20, 0, 0}, 24001: imath.Vector3{32, 0, 0}}}
	seqs := []components.Sequence{{Interval: [2]int{0, 12000}}, {Interval: [2]int{12001, 24001}}}
	plan := makeBakeTimeline([2]*components.TextureAnim{{Translation: track}, nil}, [4]components.AnimatedOrStatic[float64]{}, 0, seqs)
	if plan.keys[0] != plan.keys[4000] || plan.keys[12001] != plan.keys[16001] {
		t.Fatal("local window does not repeat")
	}
	for _, moment := range plan.moments {
		if moment.time-float64(moment.sequence.Interval[0]) >= 4000 {
			t.Fatal("baked past window")
		}
	}
	reduced := reduceBakeTimeline(plan)
	if reduced.keys[12001] == reduced.keys[0] {
		t.Fatal("two local animation states collapsed")
	}
}

func TestWMOVaryingInterpolatesColourAndUV4(t *testing.T) {
	g := &wmo.Loader{Indices: []uint16{0, 1, 2}, Vertices: []float32{0, 0, 0, 1, 0, 0, 0, 1, 0}, UVs: [][]float32{{0, 0, 1, 0, 0, 1}, {0, 0, 1, 0, 0, 1}, {0, 0, 1, 0, 0, 1}, {1, 1, 2, 1, 1, 2}}, VertexColours: [][]uint32{{0xff000000, 0xff000000, 0xff000000}, {0xff0000ff, 0xff0000ff, 0xff0000ff}}}
	v := wmoVaryingAt(g, wmo.RenderBatch{}, 0, [3]float64{.5, .25, .25})
	if v.uv[3] != (imath.Vector2{1.25, 1.25}) || v.color[1] != ([4]float64{1, 0, 0, 1}) {
		t.Fatalf("varying: %+v", v)
	}
}
