package animation

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestSelectedVariantDropsLocalBoneKeysAndKeepsGlobalClock(t *testing.T) {
	time := uint32(25)
	local := TrackData{GlobalSeq: uint16(config.BlizzardNull), Timestamps: [][]*uint32{{&time}, {&time}}, Values: [][][]float64{{{1, 2, 3}}, {{4, 5, 6}}}}
	global := local
	global.GlobalSeq = 0
	f := File{IsLoaded: true, GlobalLoops: []uint32{4000}, ExcludedIndices: map[int]struct{}{0: {}},
		Animations: []AnimationData{{ID: 0, VariationIndex: 0, Duration: 1000}, {ID: 0, VariationIndex: 1, Duration: 2000}},
		Bones:      []BoneData{{ParentBone: -1, Translation: local, Scale: global}},
	}
	var globals []*components.GlobalSequence
	result := f.ToMdl(&globals)
	if len(result.Sequences) != 1 || result.Sequences[0].Data.WowVariant != 1 || result.Sequences[0].Data.SourceIndices[0] != 1 {
		t.Fatalf("selected sequences = %+v", result.Sequences)
	}
	bone := result.Bones[0]
	if _, leaked := bone.Translation.KeyFrames[25]; leaked {
		t.Fatalf("excluded local keys leaked: %v", bone.Translation.KeyFrames)
	}
	if _, kept := bone.Translation.KeyFrames[1026]; !kept {
		t.Fatalf("retained clock shifted: %v", bone.Translation.KeyFrames)
	}
	if bone.Scaling.GlobalSeq == nil || bone.Scaling.GlobalSeq.Duration != 4000 {
		t.Fatalf("global dependency lost: %+v", bone.Scaling)
	}
}
