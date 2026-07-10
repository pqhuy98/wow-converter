package mdl

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestScaleSequenceDurationMatchesTSFlooring(t *testing.T) {
	m := New(NewMDLOptions{Name: "parity"})
	m.Sequences = []components.Sequence{
		{
			Name:      "Walk",
			Interval:  [2]int{1435, 2335},
			MoveSpeed: 588,
		},
		{
			Name:     "Stand",
			Interval: [2]int{3395, 6061},
		},
	}
	m.Bones = []*components.Bone{{
		NodeBase: components.NodeBase{
			Name: "Root",
			Type: "Bone",
		},
	}}
	m.Bones[0].Translation = &components.Animation{
		Interpolation: components.InterpLinear,
		Type:          components.AnimTypeTranslation,
		KeyFrames: map[int]any{
			1435: imath.Vector3{0, 0, 0},
			2335: imath.Vector3{1, 0, 0},
			3395: imath.Vector3{2, 0, 0},
		},
	}

	m.Modify.ScaleSequenceDuration(&m.Sequences[0], 588.0/270.0)

	if got, want := m.Sequences[0].Interval, [2]int{1435, 3394}; got != want {
		t.Fatalf("walk interval mismatch: got %v want %v", got, want)
	}
	if got, want := m.Sequences[1].Interval, [2]int{4454, 7120}; got != want {
		t.Fatalf("next sequence interval mismatch: got %v want %v", got, want)
	}
	if got := components.SortedKeyInts(m.Bones[0].Translation.KeyFrames); len(got) != 3 || got[0] != 1435 || got[1] != 3395 || got[2] != 4454 {
		t.Fatalf("unexpected scaled keyframes: got %v", got)
	}
}

func TestScaleSequenceDurationPreservesTSCollisionOrder(t *testing.T) {
	m := New(NewMDLOptions{Name: "parity"})
	m.Sequences = []components.Sequence{{
		Name:      "Walk",
		Interval:  [2]int{1435, 2335},
		MoveSpeed: 588,
	}}
	m.Bones = []*components.Bone{{
		NodeBase: components.NodeBase{
			Name: "Root",
			Type: "Bone",
		},
	}}
	m.Bones[0].Translation = &components.Animation{
		Interpolation: components.InterpLinear,
		Type:          components.AnimTypeTranslation,
		KeyFrames: map[int]any{
			1435: imath.Vector3{0, 0, 0},
			2335: imath.Vector3{1, 0, 0},
			2336: imath.Vector3{2, 0, 0},
		},
	}

	m.Modify.ScaleSequenceDuration(&m.Sequences[0], 588.0/270.0)

	got, ok := m.Bones[0].Translation.KeyFrames[3395]
	if !ok {
		t.Fatalf("expected collision result at 3395, got %v", components.SortedKeyInts(m.Bones[0].Translation.KeyFrames))
	}
	if got != (imath.Vector3{2, 0, 0}) {
		t.Fatalf("expected later timestamp to win collision at 3395, got %#v", got)
	}
	if _, ok := m.Bones[0].Translation.KeyFrames[3394]; ok {
		t.Fatalf("did not expect synthesized boundary key at 3394, got %v", components.SortedKeyInts(m.Bones[0].Translation.KeyFrames))
	}
}
