package metadata

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestSelectedMaterialTrackDropsExcludedFramesButPreservesGlobalFrames(t *testing.T) {
	f := &File{Animation: &bundleanim.File{Animations: []bundleanim.AnimationData{{Duration: 1000}, {Duration: 2000}}, ExcludedIndices: map[int]struct{}{0: {}}}, globalLoops: []uint32{5000}}
	track := m2TrackRaw{GlobalSeq: uint16(config.BlizzardNull), Timestamps: [][]*uint32{{timestamp(25)}, {timestamp(25)}}, Values: [][][]float64{{{0.25}}, {{0.75}}}}
	convert := func(v []float64) any { return v[0] }
	local := f.m2TrackToAnimation(track, components.AnimTypeAlpha, convert)
	if _, leaked := local.KeyFrames[25]; leaked {
		t.Fatalf("excluded material key leaked: %v", local.KeyFrames)
	}
	if local.KeyFrames[1026] != 0.75 {
		t.Fatalf("selected material clock shifted: %v", local.KeyFrames)
	}
	track.GlobalSeq = 0
	global := f.m2TrackToAnimation(track, components.AnimTypeAlpha, convert)
	if global.GlobalSeq == nil || global.GlobalSeq.Duration != 5000 || global.KeyFrames[25] != 0.75 {
		t.Fatalf("global material dependency lost: %+v", global)
	}
}
