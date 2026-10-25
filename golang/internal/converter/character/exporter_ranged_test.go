package character

import (
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"testing"
)

func TestSingleRangedAttackRetainsSourceIdentity(t *testing.T) {
	for _, tag := range []string{"Thrown", "Bow", "Rifle"} {
		m := mdl.New(mdl.NewMDLOptions{})
		m.Sequences = []components.Sequence{{Name: "Attack", Interval: [2]int{10, 1010}, Data: components.SequenceData{WowName: "Attack" + tag, WowVariant: 0, AttackTag: tag}}}
		concatenateAttackSequences(m)
		if len(m.Sequences) != 1 || m.Sequences[0].Data.WowName != "Attack"+tag || m.Sequences[0].Interval != [2]int{10, 1010} {
			t.Fatalf("%s single attack lost its source identity or interval", tag)
		}
	}
}
