package directm2

import (
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestEveryM2EffectSelectsItsCombinerAndUVInputs(t *testing.T) {
	// Independently transcribed renderer table. Keep new table entries visible
	// in this regression, rather than silently selecting the legacy shader.
	wantPixels := []int{12, 13, 14, 15, 16, 13, 16, 17, 18, 19, 20, 21, 22, 23, 23, 20, 24, 25, 26, 33, 27, 6, 28, 29, 25, 33, 30, 31, 32, 34, 35, 35, 0, 7, 1, 36}
	for index, pixel := range wantPixels {
		shader, err := decodeM2Shader(0x8000+uint16(index), m2SamplerCounts[pixel])
		if err != nil || shader.pixel != pixel {
			t.Fatalf("effect %d: %+v %v", index, shader, err)
		}
	}
	cross, _ := decodeM2Shader(0x8012, 3)
	masked, _ := decodeM2Shader(0x8016, 4)
	glow, _ := decodeM2Shader(0x8003, 3)
	if cross.coords != [4]m2Coord{coordT1M0, coordT1M0, coordT1M0, coordT1M0} || masked.coords[3] != coordT2M1 || glow.coords[2] != coordT1 {
		t.Fatal("three/four-texture routing does not match renderer")
	}
	if _, err := decodeM2Shader(0xffff, 1); err == nil {
		t.Fatal("invalid effect silently accepted")
	}
}

func TestM2EmissionMaskAndCrossfadeAreSeparateFromDiffuse(t *testing.T) {
	tex := [4][4]float64{{.8, .6, .4, .25}, {.2, .4, .6, .5}, {1, .5, .2, .75}, {0, 0, 0, .3}}
	weights := [4]float64{1, .25, .5, 1}
	f := evaluateM2Combiner(15, tex, weights)
	if math.Abs(f.emission[0]-.375) > 1e-9 || math.Abs(f.diffuse[0]-.44) > 1e-9 || f.alpha != 1 {
		t.Fatalf("third emissive texture: %+v", f)
	}
	masked := evaluateM2Combiner(28, tex, weights)
	if math.Abs(masked.alpha-(.25*.75+.5*.25)*.5*.3-.75*.5*.3) > 1e-9 {
		t.Fatalf("masked crossfade alpha %+v", masked)
	}
	for pixel := range 37 {
		f := evaluateM2Combiner(pixel, tex, weights)
		if math.IsNaN(f.alpha) || f.alpha <= 0 {
			t.Fatalf("combiner %d invalid alpha", pixel)
		}
		for k := range 3 {
			if math.IsNaN(f.diffuse[k]+f.emission[k]) || f.diffuse[k]+f.emission[k] <= 0 {
				t.Fatalf("combiner %d has no evaluated output", pixel)
			}
		}
	}
}

func TestParticleColorAndAlphaMultipliersAreIndependent(t *testing.T) {
	tex := [3][4]float64{{.5, .5, .5, .5}, {.5, .5, .5, .5}, {.25, .25, .25, .25}}
	for _, tc := range []struct {
		flags      uint32
		rgb, alpha float64
	}{{0, .5, .125}, {0x20000000, .5, .25}, {0x40000000, .25, .125}, {0x60000000, .25, .25}} {
		out := combineParticleSamples(tex, tc.flags)
		if out[0] != tc.rgb || out[3] != tc.alpha {
			t.Fatalf("flags %#x: %v", tc.flags, out)
		}
	}
}

func TestLocalUVAtlasUsesSequenceIntervalsAndNoGlobalSequence(t *testing.T) {
	track := &components.Animation{Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 100: imath.Vector3{1, 0, 0}, 101: imath.Vector3{2, 0, 0}, 201: imath.Vector3{3, 0, 0}}}
	seqs := []components.Sequence{{Interval: [2]int{0, 100}}, {Interval: [2]int{101, 201}}}
	plan := makeBakeTimeline([2]*components.TextureAnim{{Translation: track}, nil}, [4]components.AnimatedOrStatic[float64]{}, 6, seqs)
	if plan.period != 0 || len(plan.moments) < 4 {
		t.Fatalf("local bake uses wrong timeline %+v", plan)
	}
	start, ok := plan.keys[101]
	if !ok {
		t.Fatal("second sequence has no atlas key")
	}
	uv := transformBakeUV(imath.Vector2{}, bakeTransformAt(&components.TextureAnim{Translation: track}, plan.moments[start]))
	if uv[0] != 2 {
		t.Fatalf("second sequence sampled wrong track: %v", uv)
	}
}
