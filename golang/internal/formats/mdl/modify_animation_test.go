package mdl

import (
	"math"
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func translationCurve(times ...int) *components.Animation {
	a := &components.Animation{Type: components.AnimTypeTranslation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{}}
	for _, timestamp := range times {
		a.KeyFrames[timestamp] = imath.Vector3{float64(timestamp), 0, 0}
	}
	return a
}

func TestFreezeHeadPreservesHeadBodyPortraitAndIndependentTracks(t *testing.T) {
	m := New(NewMDLOptions{})
	m.Sequences = []components.Sequence{{Name: "Stand", Interval: [2]int{0, 100}}, {Name: "Portrait Talk", Interval: [2]int{200, 300}}}
	head, jaw, body := components.NewBone("Head"), components.NewBone("Jaw"), components.NewBone("Body")
	jaw.Parent = head
	head.Translation = translationCurve(0, 50, 100, 200, 250, 300)
	jaw.Translation = translationCurve(0, 50, 100, 200, 250, 300)
	body.Translation = translationCurve(0, 50, 100, 200, 250, 300)
	jaw.Scaling = translationCurve(0, 50, 100)
	jaw.Scaling.GlobalSeq = &components.GlobalSequence{Duration: 100}
	jaw.Rotation = translationCurve(0, 50, 100)
	jaw.Rotation.Interpolation = components.InterpHermite
	m.Bones = []*components.Bone{head, jaw, body}
	m.Modify.FreezeHeadOutsidePortrait()
	if len(jaw.Translation.KeyFrames) != 5 || jaw.Translation.KeyFrames[100] != (imath.Vector3{}) {
		t.Fatal("ordinary head-child motion was not frozen to its first pose")
	}
	if _, ok := jaw.Translation.KeyFrames[50]; ok {
		t.Fatal("redundant face key retained")
	}
	for _, timestamp := range []int{200, 250, 300} {
		if jaw.Translation.KeyFrames[timestamp] != (imath.Vector3{float64(timestamp), 0, 0}) {
			t.Fatal("portrait motion changed")
		}
	}
	if len(head.Translation.KeyFrames) != 6 || len(body.Translation.KeyFrames) != 6 || len(jaw.Scaling.KeyFrames) != 3 || len(jaw.Rotation.KeyFrames) != 3 {
		t.Fatal("head/body/global/nonlinear tracks changed")
	}
	// Shared Stand/Portrait intervals must retain portrait expressions.
	m.Sequences[1].Interval = [2]int{0, 100}
	jaw.Translation = translationCurve(0, 50, 100)
	m.Modify.FreezeHeadOutsidePortrait()
	if len(jaw.Translation.KeyFrames) != 3 {
		t.Fatal("overlapping portrait timeline was frozen")
	}
}

func TestConservativeReductionPreservesBoundariesAndCurvedMotion(t *testing.T) {
	m := New(NewMDLOptions{})
	m.Sequences = []components.Sequence{{Name: "Stand", Interval: [2]int{0, 100}}, {Name: "Walk", Interval: [2]int{200, 300}}}
	b := components.NewBone("Body")
	b.Translation = translationCurve(0, 25, 50, 75, 100, 200, 250, 300)
	b.Translation.KeyFrames[50] = imath.Vector3{50, 0.1, 0}
	b.Rotation = &components.Animation{Type: components.AnimTypeRotation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{}}
	for _, timestamp := range []int{0, 25, 50, 75, 100} {
		angle := float64(timestamp) * math.Pi / 200
		q := imath.QuaternionRotation{0, 0, math.Sin(angle / 2), math.Cos(angle / 2)}
		if timestamp == 50 { // q and -q are the same orientation.
			for i := range q {
				q[i] = -q[i]
			}
		}
		b.Rotation.KeyFrames[timestamp] = q
	}
	b.Scaling = translationCurve(0, 25, 50, 75, 100)
	b.Scaling.Type = components.AnimTypeScaling
	b.Scaling.GlobalSeq = &components.GlobalSequence{Duration: 100}
	m.Bones = []*components.Bone{b}
	before := components.CloneKeyFrames(b.Translation.KeyFrames)
	m.Modify.ReduceKeyFramesConservative()
	if len(b.Translation.KeyFrames) >= len(before) || len(b.Rotation.KeyFrames) != 2 || len(b.Scaling.KeyFrames) != 2 {
		t.Fatal("linear/global/slerp redundancy was not reduced")
	}
	for _, timestamp := range []int{0, 50, 100, 200, 300} {
		if _, ok := b.Translation.KeyFrames[timestamp]; !ok {
			t.Fatalf("lost boundary/curve key %d", timestamp)
		}
	}
	curve := newAnimationCurve(b.Translation)
	for timestamp, value := range before {
		if error := animationValueError(value, curve.sample(float64(timestamp), components.InterpLinear)); error > 0.005 {
			t.Fatalf("translation error %g at %d", error, timestamp)
		}
	}
	after := components.CloneKeyFrames(b.Translation.KeyFrames)
	m.Modify.ReduceKeyFramesConservative()
	if !reflect.DeepEqual(after, b.Translation.KeyFrames) {
		t.Fatal("reduction is not deterministic/idempotent")
	}
}

func TestConservativeReductionSkipsNonlinearAndPreservesOverlappingEndpoints(t *testing.T) {
	m := New(NewMDLOptions{})
	m.Sequences = []components.Sequence{{Name: "Stand", Interval: [2]int{0, 100}}, {Name: "Portrait", Interval: [2]int{50, 100}}}
	b := components.NewBone("Body")
	b.Translation = translationCurve(0, 25, 50, 75, 100)
	b.Scaling = translationCurve(0, 25, 50, 75, 100)
	b.Scaling.Interpolation = components.InterpBezier
	m.Bones = []*components.Bone{b}
	m.Modify.ReduceKeyFramesConservative()
	if len(b.Translation.KeyFrames) != 3 || b.Translation.KeyFrames[50] == nil || len(b.Scaling.KeyFrames) != 5 {
		t.Fatal("overlapping endpoint or nonlinear keys changed")
	}
}
