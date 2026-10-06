package animation

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestGlobalBoneTracksUseSourceDurationAndIndependentClock(t *testing.T) {
	a, b := uint32(100), uint32(2667)
	track := TrackData{GlobalSeq: 1, Interpolation: 1,
		Timestamps: [][]*uint32{nil, {&a, &b}},
		Values:     [][][]float64{nil, {{0, 0, 0}, {1, 2, 3}}},
	}
	rotation := track
	rotation.Values = [][][]float64{nil, {{0, 0, 0, 1}, {1, 2, 3, 4}}}
	f := File{IsLoaded: true, GlobalLoops: []uint32{0, 2667},
		Bones: []BoneData{{ParentBone: -1, Translation: track, Rotation: rotation, Scale: track}},
		// There are fewer sequences than global track rows. The second row
		// must retain raw timestamps rather than use the sequence clock.
		Animations: []AnimationData{{ID: 0, Duration: 8000}},
	}
	gs := components.NewGlobalSequence(1, 1)
	globals := []*components.GlobalSequence{&gs}
	result := f.ToMdl(&globals)
	bone := result.Bones[0]
	for _, anim := range []*components.Animation{bone.Translation, bone.Rotation, bone.Scaling} {
		if anim == nil || anim.GlobalSeq != &gs || anim.GlobalSeq.Duration != 2667 || len(anim.KeyFrames) != 2 {
			t.Fatalf("global bone animation = %#v, globals = %#v", anim, globals)
		}
		if _, ok := anim.KeyFrames[100]; !ok {
			t.Fatalf("source global timestamps were shifted: %v", anim.KeyFrames)
		}
	}
	if got := bone.Translation.KeyFrames[2667]; got != (imath.Vector3{1, -3, 2}) {
		t.Fatalf("global translation basis = %v", got)
	}
	if got := bone.Rotation.KeyFrames[2667]; got != (imath.QuaternionRotation{1, -3, 2, 4}) {
		t.Fatalf("global rotation basis = %v", got)
	}
	if got := bone.Scaling.KeyFrames[2667]; got != (imath.Vector3{1, 3, 2}) {
		t.Fatalf("global scaling basis = %v", got)
	}
}

func TestLocalBoneTrackRetainsSequenceClock(t *testing.T) {
	time := uint32(25)
	track := TrackData{GlobalSeq: uint16(config.BlizzardNull), Timestamps: [][]*uint32{nil, {&time}}, Values: [][][]float64{nil, {{1, 2, 3}}}}
	anim := &components.Animation{KeyFrames: map[int]any{}}
	applyTrack(anim, track, []AnimationData{{Duration: 1000}, {Duration: 2000}}, nil, func(v []float64) any { return imath.Vector3{v[0], v[1], v[2]} })
	if _, ok := anim.KeyFrames[1026]; !ok {
		t.Fatalf("local timestamp = %v, want second sequence offset + 25", anim.KeyFrames)
	}
}
