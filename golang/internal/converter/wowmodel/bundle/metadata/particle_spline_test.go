package metadata

import (
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestSplineParticleBirthsFollowCurveAndPreserveRate(t *testing.T) {
	const k = float32(.55228475)
	points := [][3]float32{{1, 0, 0}, {1, k, 0}, {k, 1, 0}, {0, 1, 0}, {-k, 1, 0}, {-1, k, 0}, {-1, 0, 0}, {-1, -k, 0}, {-k, -1, 0}, {0, -1, 0}, {k, -1, 0}, {1, -k, 0}, {1, 0, 0}}
	f := &File{mdl: mdl.New(mdl.NewMDLOptions{Name: "spline"})}
	node := components.NewParticleEmitter2("ring")
	node.Width = components.AnimatedOrStatic[float64]{Static: true, Value: 1}
	node.Length = components.AnimatedOrStatic[float64]{Static: true}
	node.EmissionRate = components.AnimatedOrStatic[float64]{Static: true, Value: 160}
	variants := f.particleSplineVariants(node, m2.ParticleEmitterEntry{EmitterType: 3, SplinePoints: points})
	if len(variants) != 32 {
		t.Fatalf("samples=%d", len(variants))
	}
	total := 0.0
	for _, variant := range variants {
		p := variant.Translation.KeyFrames[0].(imath.Vector3)
		if math.Abs(math.Hypot(p[0], p[1])-1) > .002 || p[2] != 0 {
			t.Fatalf("birth off ring: %v", p)
		}
		if variant.Width.Value != 0 || variant.Length.Value != 0 {
			t.Fatal("normalized spline interval became physical birth area")
		}
		total += variant.EmissionRate.Value
	}
	if math.Abs(total-160) > 1e-8 || node.EmissionRate.Value != 160 || node.Width.Value != 1 {
		t.Fatal("rate or input mutated")
	}
	node.Width = components.AnimatedOrStatic[float64]{Anim: &components.Animation{Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: float64(0), 1000: float64(1)}}}
	animated := f.particleSplineVariants(node, m2.ParticleEmitterEntry{EmitterType: 3, SplinePoints: points})
	for _, variant := range animated {
		p := variant.Translation.KeyFrames[500].(imath.Vector3)
		if math.Abs(math.Hypot(p[0], p[1])-1) > .002 {
			t.Fatalf("birth crossed a chord instead of following curve: %v", p)
		}
	}
	clone := cloneParticles([]m2.ParticleEmitterEntry{{SplinePoints: points}})
	clone[0].SplinePoints[0][0] = 10
	if points[0][0] != 1 {
		t.Fatal("spline metadata aliases loader")
	}
}
