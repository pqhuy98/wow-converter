package mdl

import (
	"fmt"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestNPCAnimationDedupBeforeVariantLimitAndTimelinePruning(t *testing.T) {
	m := New(NewMDLOptions{})
	b := components.NewBone("Body")
	b.Translation = translationCurve()
	// The first two attacks are practically identical, with different key
	// layouts. The next four really differ, so four distinct variants survive.
	for i, start := range []float64{0, 0.01, 1, 2, 3, 4} {
		lo := i * 200
		m.Sequences = append(m.Sequences, components.Sequence{Name: "Attack", Interval: [2]int{lo, lo + 100}})
		b.Translation.KeyFrames[lo] = imath.Vector3{start, 0, 0}
		b.Translation.KeyFrames[lo+100] = imath.Vector3{start + 1, 0, 0}
		if i == 1 {
			b.Translation.KeyFrames[lo+50] = imath.Vector3{start + .5, 0, 0}
		}
	}
	for i, name := range []string{"Mount", "Mount MountChopper", "Sleep"} {
		lo := 1500 + i*200
		m.Sequences = append(m.Sequences, components.Sequence{Name: name, Interval: [2]int{lo, lo + 100}})
		b.Translation.KeyFrames[lo] = imath.Vector3{10, 0, 0}
	}
	m.Sequences = append(m.Sequences,
		components.Sequence{Name: "Stand Alternate", Interval: [2]int{2200, 2300}, Data: components.SequenceData{WowName: "MountFlightIdle"}},
		components.Sequence{Name: "Walk Swim", Interval: [2]int{2400, 2500}, Data: components.SequenceData{SourceAnimations: []components.SourceAnimation{{WowName: "MountSwimRun"}}}},
		components.Sequence{Name: "Death", Interval: [2]int{2600, 2700}, Data: components.SequenceData{WowName: "SleepDown"}},
	)
	b.Scaling = translationCurve(0, 50, 100)
	b.Scaling.GlobalSeq = &components.GlobalSequence{Duration: 100}
	m.Bones = []*components.Bone{b}
	m.Modify.OptimizeNPCSequences()
	if len(m.Sequences) != 4 {
		t.Fatalf("got %d variants, want four distinct attacks", len(m.Sequences))
	}
	for i, want := range []int{0, 400, 600, 800} {
		if m.Sequences[i].Interval[0] != want {
			t.Fatalf("variant %d starts at %d, want %d", i, m.Sequences[i].Interval[0], want)
		}
	}
	if len(b.Translation.KeyFrames) != 8 || len(b.Scaling.KeyFrames) != 3 {
		t.Fatal("removed timelines/global keys were not handled correctly")
	}
}

func TestNPCAnimationDedupPreservesTimingEffectsAndEvents(t *testing.T) {
	for _, difference := range []string{"duration", "loop", "speed", "effect", "event", "nonlinear", "family"} {
		t.Run(difference, func(t *testing.T) {
			m := New(NewMDLOptions{})
			m.Sequences = []components.Sequence{{Name: "Attack", Interval: [2]int{0, 100}}, {Name: "Attack", Interval: [2]int{200, 300}}}
			b := components.NewBone("Body")
			b.Translation = &components.Animation{Type: components.AnimTypeTranslation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{
				0: imath.Vector3{}, 100: imath.Vector3{1, 0, 0}, 200: imath.Vector3{}, 300: imath.Vector3{1, 0, 0},
			}}
			m.Bones = []*components.Bone{b}
			switch difference {
			case "duration":
				m.Sequences[1].Interval[1]++
			case "loop":
				m.Sequences[1].NonLooping = true
			case "speed":
				m.Sequences[1].MoveSpeed = 270
			case "family":
				m.Sequences[1].Name = "Spell"
			case "effect":
				m.GeosetAnims = []components.GeosetAnim{{Alpha: &components.AnimatedOrStatic[float64]{Anim: &components.Animation{Type: components.AnimTypeAlpha, Interpolation: components.InterpDontInterp, KeyFrames: map[int]any{0: 1.0, 100: 1.0, 200: 0.0, 300: 0.0}}}}}
			case "event":
				e := components.NewEventObject("SND")
				e.Track = []components.EventTrackEntry{{Sequence: &m.Sequences[0], Offset: 20}, {Sequence: &m.Sequences[1], Offset: 30}}
				m.EventObjects = []*components.EventObject{e}
			case "nonlinear":
				b.Translation.Interpolation = components.InterpHermite
				b.Translation.InOutTans = map[int]components.InOutTan{200: {InTan: imath.Vector3{1, 0, 0}}}
			}
			m.Modify.OptimizeNPCSequences()
			if len(m.Sequences) != 2 {
				t.Fatalf("lost meaningful %s difference", difference)
			}
			if difference == "event" && m.EventObjects[0].Track[1].Sequence != &m.Sequences[1] {
				t.Fatal("event sequence pointer was not rebound")
			}
		})
	}
}

func TestNPCVariantBudgetAppliesSeparatelyToEachWC3Family(t *testing.T) {
	m := New(NewMDLOptions{})
	for _, name := range []string{"Spell", "Attack Slam"} {
		for i := 0; i < 6; i++ {
			start := len(m.Sequences) * 100
			m.Sequences = append(m.Sequences, components.Sequence{Name: fmt.Sprintf("%s %d", name, i+1), Interval: [2]int{start, start + 50 + i}})
		}
	}
	m.Modify.OptimizeNPCSequences()
	if len(m.Sequences) != 8 {
		t.Fatalf("got %d sequences, want four in each family", len(m.Sequences))
	}
}
