package mdl

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestSyntheticDeathStopsEmittersAndPreservesStand(t *testing.T) {
	m := New(NewMDLOptions{Name: "impact"})
	m.Sequences = []components.Sequence{{Name: "Stand", Interval: [2]int{0, 1000}}}
	visibility := &components.Animation{Interpolation: components.InterpDontInterp, KeyFrames: map[int]any{0: 1.0, 667: 0.0, 1000: 1.0}}
	m.ParticleEmitter2s = []*components.ParticleEmitter2{
		{Visibility: visibility, EmissionRate: components.AnimatedOrStatic[float64]{Static: true, Value: 20}},
		{}, // Missing visibility means enabled in existing sequences.
	}
	m.RibbonEmitters = []*components.RibbonEmitter{{}}
	m.Modify.AddDoodadDeathAnimation().OptimizeKeyFrames()
	death := m.Sequences[1]
	if death.Name != "Death" || !death.NonLooping || m.Sequences[0].NonLooping {
		t.Fatal("synthetic Death must not change Stand looping")
	}
	for _, anim := range []*components.Animation{m.ParticleEmitter2s[0].Visibility, m.ParticleEmitter2s[1].Visibility, m.RibbonEmitters[0].Visibility} {
		for _, timestamp := range []int{death.Interval[0], death.Interval[0] + 500, death.Interval[1]} {
			if got := InterpolateKeyFramesFloat(death, anim.KeyFrames, timestamp, false, 1); got != 0 {
				t.Fatalf("Death visibility at %d = %v, want 0", timestamp, got)
			}
		}
		if got := InterpolateKeyFramesFloat(m.Sequences[0], anim.KeyFrames, 0, false, 1); got != 1 {
			t.Fatal("Stand visibility changed")
		}
	}
	if visibility.KeyFrames[667] != 0.0 || m.ParticleEmitter2s[0].EmissionRate.Value != 20 {
		t.Fatal("original particle timing/rate changed")
	}
	before := m.ToMdl()
	m.Modify.AddDoodadDeathAnimation()
	if m.ToMdl() != before {
		t.Fatal("existing Death must be left untouched")
	}
}
