package directm2

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestPhysicsBakeFallsMovesLoopsAndSurvivesOptimization(t *testing.T) {
	makeModel := func() (*mdl.MDL, *m2.PhysicsData) {
		model := mdl.New(mdl.NewMDLOptions{FormatVersion: 1000, Name: "pendulum"})
		head, hair, jaw := components.NewBone("Head"), components.NewBone("Hair"), components.NewBone("Jaw")
		head.PivotPoint = imath.Vector3{0, 0, 2}
		hair.PivotPoint = imath.Vector3{0, 0, 2.2}
		hair.Parent = head
		jaw.Parent = head
		head.Translation = &components.Animation{Type: components.AnimTypeTranslation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 250: imath.Vector3{.15, 0, .05}, 500: imath.Vector3{}, 750: imath.Vector3{-.15, 0, -.05}, 1000: imath.Vector3{}}}
		jaw.Translation = &components.Animation{Type: components.AnimTypeTranslation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 500: imath.Vector3{0, .1, 0}, 1000: imath.Vector3{}}}
		model.Bones = []*components.Bone{head, hair, jaw}
		model.Sequences = []components.Sequence{{Name: "Stand", Interval: [2]int{0, 1000}}}
		frame := m2.PhysicsFrame{Axes: [3]imath.Vector3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}}
		data := &m2.PhysicsData{Bodies: []m2.PhysicsBody{{Bone: 0, Position: head.PivotPoint}, {Bone: 1, Type: 1, Position: hair.PivotPoint, Gravity: 1, Inertia: 1, LinearDamping: 3, AngularDamping: 5, Shapes: []m2.PhysicsShape{{A: imath.Vector3{-.01, 0, .01}, B: imath.Vector3{-.2, 0, .1}, Radius: .02}}}}, Joints: []m2.PhysicsJoint{{A: 0, B: 1, Type: 1, FrameA: frame, FrameB: frame, Lower: -10, Upper: 10, Cone: 150}}}
		data.Joints[0].FrameA.Origin = imath.Vector3{0, 0, .2}
		return model, data
	}
	m, data := makeModel()
	if err := bakePhysics(context.Background(), m, data, 1); err != nil {
		t.Fatal(err)
	}
	hair := m.Bones[1]
	if !hair.PhysicsDriven || m.Bones[2].PhysicsDriven {
		t.Fatal("wrong physics preservation scope")
	}
	q0 := hair.Rotation.KeyFrames[0].(imath.QuaternionRotation)
	if physicsRotate(q0, physicsShapeCenter(data.Bodies[1].Shapes))[2] >= 0 {
		t.Fatal("gravity did not turn the upward rest shape downward")
	}
	changed := false
	for _, v := range hair.Rotation.KeyFrames {
		q := v.(imath.QuaternionRotation)
		norm := 0.0
		dot := 0.0
		for i, x := range q {
			if math.IsNaN(x) || math.IsInf(x, 0) {
				t.Fatal("nonfinite quaternion")
			}
			norm += x * x
			dot += x * q0[i]
		}
		if math.Abs(norm-1) > 1e-9 {
			t.Fatal("non-unit quaternion")
		}
		if math.Abs(dot) < .9999 {
			changed = true
		}
	}
	if !changed {
		t.Fatal("moving anchor produced no secondary motion")
	}
	if !reflect.DeepEqual(hair.Rotation.KeyFrames[0], hair.Rotation.KeyFrames[1000]) || !reflect.DeepEqual(hair.Translation.KeyFrames[0], hair.Translation.KeyFrames[1000]) {
		t.Fatal("loop seam is open")
	}
	for _, v := range hair.Translation.KeyFrames {
		if imath.V3Distance(v.(imath.Vector3), imath.Vector3{}) > 1e-6 {
			t.Fatal("anchored hair stretched away from its parent")
		}
	}
	count := len(hair.Rotation.KeyFrames)
	m.Modify.FreezeHeadOutsidePortrait()
	if len(hair.Rotation.KeyFrames) != count {
		t.Fatal("head freeze erased physics")
	}
	if _, ok := m.Bones[2].Translation.KeyFrames[500]; ok {
		t.Fatal("face freezing stopped working")
	}
	m.Modify.OptimizeKeyFrames().ReduceKeyFramesConservative()
	if len(hair.Rotation.KeyFrames) < 3 {
		t.Fatal("optimization erased secondary motion")
	}
	a, b := makeModel()
	if err := bakePhysics(context.Background(), a, b, 1); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Bones[1].Rotation.KeyFrames, func() map[int]any {
		c, d := makeModel()
		if err := bakePhysics(context.Background(), c, d, 1); err != nil {
			t.Fatal(err)
		}
		return c.Bones[1].Rotation.KeyFrames
	}()) {
		t.Fatal("bake is nondeterministic")
	}
	static, rig := makeModel()
	static.Sequences[0].Interval = [2]int{0, 0}
	if err := bakePhysics(context.Background(), static, rig, 1); err != nil {
		t.Fatal(err)
	}
	if len(static.Bones[1].Rotation.KeyFrames) != 1 {
		t.Fatal("single-frame pose was not baked")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a, b = makeModel()
	if err := bakePhysics(ctx, a, b, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation ignored: %v", err)
	}
}

func TestPhysicsContactsAndLimits(t *testing.T) {
	data := &m2.PhysicsData{Bodies: []m2.PhysicsBody{{Shapes: []m2.PhysicsShape{{A: imath.Vector3{}, B: imath.Vector3{0, 0, 1}, Radius: .2}}}}}
	state := []physicsState{{q: imath.QuatNoRotation()}}
	colliders := makePhysicsColliders(data)
	updatePhysicsColliders(colliders, state)
	push := physicsContact(imath.Vector3{.1, 0, .5}, .05, -1, colliders)
	if math.Abs(push[0]-.15) > 1e-9 {
		t.Fatalf("bad capsule contact %v", push)
	}
	push = physicsContact(imath.Vector3{1, 0, -.1}, .05, -1, colliders)
	if math.Abs(push[2]-.15) > 1e-9 {
		t.Fatalf("bad ground contact %v", push)
	}
	link := physicsLink{joint: m2.PhysicsJoint{Type: 1, Lower: -5, Upper: 5, Cone: 45}, frameA: imath.QuatNoRotation(), frameB: imath.QuatNoRotation()}
	q := limitPhysicsJoint(imath.QuaternionRotation{0, math.Sin(math.Pi / 3), 0, math.Cos(math.Pi / 3)}, imath.QuatNoRotation(), link)
	angle := 2 * math.Acos(q[3])
	if math.Abs(angle-math.Pi/4) > 1e-8 {
		t.Fatalf("cone limit ignored: %f", angle)
	}
}

func TestPhysicsUnsupportedSharedBoneLeavesAuthoredAnimationIntact(t *testing.T) {
	m := mdl.New(mdl.NewMDLOptions{})
	b := components.NewBone("shared")
	b.Rotation = &components.Animation{Type: components.AnimTypeRotation, KeyFrames: map[int]any{0: imath.QuatNoRotation()}}
	m.Bones = []*components.Bone{b}
	original := b.Rotation
	data := &m2.PhysicsData{Bodies: []m2.PhysicsBody{{Bone: 0}, {Bone: 0, Type: 1}}, Joints: []m2.PhysicsJoint{{A: 0, B: 1}}}
	if err := bakePhysics(context.Background(), m, data, 1); !errors.Is(err, m2.ErrUnsupportedPhysics) {
		t.Fatalf("expected skippable rig: %v", err)
	}
	if b.Rotation != original || b.PhysicsDriven || b.Translation != nil {
		t.Fatal("unsupported rig modified authored animation")
	}
}
