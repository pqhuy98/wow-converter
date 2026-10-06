package metadata

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func timestamp(value uint32) *uint32 { return &value }

func TestM2GlobalTextureTrackDoesNotRequireModelSequences(t *testing.T) {
	f := &File{globalLoops: []uint32{4200}}
	track := m2TrackRaw{
		GlobalSeq: 0,
		Timestamps: [][]*uint32{{
			timestamp(0), timestamp(1750), timestamp(4200),
		}},
		Values: [][][]float64{{
			{0, 0, 0}, {0.5, -0.25, 0}, {1, 0, 0},
		}},
	}
	anim := f.m2TrackToAnimation(track, components.AnimTypeOthers, func(v []float64) any {
		return imath.Vector3{v[0], v[1], v[2]}
	})
	if anim == nil {
		t.Fatal("global-only texture track was dropped without ordinary animation sequences")
	}
	if anim.GlobalSeq == nil || anim.GlobalSeq.Duration != 4200 {
		t.Fatalf("global sequence = %#v, want duration 4200", anim.GlobalSeq)
	}
	if len(anim.KeyFrames) != 3 || anim.KeyFrames[1750] != (imath.Vector3{0.5, -0.25, 0}) {
		t.Fatalf("global keyframes = %#v, want source timestamps and values unchanged", anim.KeyFrames)
	}
}

func TestM2GlobalTextureTrackGroupsKeepGlobalTimestamps(t *testing.T) {
	f := &File{globalLoops: []uint32{5000}}
	track := m2TrackRaw{
		GlobalSeq: 0,
		Timestamps: [][]*uint32{
			{timestamp(0), timestamp(250)},
			{timestamp(100), timestamp(250)},
		},
		Values: [][][]float64{
			{{0, 0, 0}, {1, 0, 0}},
			{{2, 0, 0}, {3, 0, 0}},
		},
	}
	anim := f.m2TrackToAnimation(track, components.AnimTypeOthers, func(v []float64) any {
		return imath.Vector3{v[0], v[1], v[2]}
	})
	if anim == nil {
		t.Fatal("global texture track with multiple groups was dropped")
	}
	if len(anim.KeyFrames) != 3 {
		t.Fatalf("global groups yielded %d keys, want merged timestamps without group offsets: %#v", len(anim.KeyFrames), anim.KeyFrames)
	}
	if got := anim.KeyFrames[250]; got != (imath.Vector3{3, 0, 0}) {
		t.Fatalf("duplicate global timestamp value = %#v, want last source group value", got)
	}
	if _, ok := anim.KeyFrames[350]; ok {
		t.Fatalf("global track incorrectly applied local sequence offset: %#v", anim.KeyFrames)
	}
}

func TestM2LocalTextureTrackKeepsSequenceOffsets(t *testing.T) {
	f := &File{m2Animations: []m2AnimMeta{{Duration: 1000}, {Duration: 2000}}}
	track := m2TrackRaw{
		GlobalSeq: uint16(config.BlizzardNull),
		Timestamps: [][]*uint32{
			{timestamp(50), timestamp(900)},
			{timestamp(75), timestamp(1800)},
		},
		Values: [][][]float64{
			{{1, 0, 0}, {2, 0, 0}},
			{{3, 0, 0}, {4, 0, 0}},
		},
	}
	anim := f.m2TrackToAnimation(track, components.AnimTypeOthers, func(v []float64) any {
		return imath.Vector3{v[0], v[1], v[2]}
	})
	if anim == nil {
		t.Fatal("local texture track was dropped")
	}
	if _, ok := anim.KeyFrames[50]; !ok {
		t.Fatalf("first sequence start missing from keyframes: %#v", anim.KeyFrames)
	}
	if got := anim.KeyFrames[1076]; got != (imath.Vector3{3, 0, 0}) {
		t.Fatalf("second sequence key = %#v, want offset 1076 with source value", got)
	}
	if got := anim.KeyFrames[1000]; got != (imath.Vector3{2, 0, 0}) {
		t.Fatalf("first sequence endpoint = %#v, want final source value", got)
	}
}
