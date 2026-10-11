package character

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestCustomSelectionOverridesCinematicAndAttackPruning(t *testing.T) {
	for _, custom := range []bool{false, true} {
		model := mdl.New(mdl.NewMDLOptions{})
		model.Sequences = []components.Sequence{
			{Name: "Stand", Interval: [2]int{0, 100}, Data: components.SequenceData{WowName: "Stand", SourceIndices: []int{0}}},
			{Name: "Cinematic Test", Interval: [2]int{101, 200}, Data: components.SequenceData{WowName: "Cinematic01", SourceIndices: []int{1}}},
			{Name: "Attack", Interval: [2]int{201, 300}, Data: components.SequenceData{WowName: "Attack2H", AttackTag: "2H", SourceIndices: []int{2}}},
		}
		char := Character{AttackTag: "1H", NoDecay: true}
		if custom {
			char.SequenceSelection = &directm2.SequenceSelection{Indices: []int{0, 1, 2}}
		}
		e := NewCharacterExporter(config.DefaultConfig(), nil)
		e.postProcessModel(model, char, &ExportContext{})
		want := 1
		if custom {
			want = 3
		}
		if len(model.Sequences) != want {
			t.Fatalf("custom=%t sequences=%+v", custom, model.Sequences)
		}
	}
}
