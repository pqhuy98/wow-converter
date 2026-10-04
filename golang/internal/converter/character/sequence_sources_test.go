package character

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestSequenceSourcesPreserveAmbiguousNamesAndVariant(t *testing.T) {
	exporter := &CharacterExporter{Models: [][2]interface{}{{&mdl.MDL{Sequences: []components.Sequence{
		{Name: "Attack", Data: components.SequenceData{WowName: "Attack1H", WowVariant: 2}},
		{Name: "Attack", Data: components.SequenceData{WowName: "Attack2H", WowVariant: 0}},
	}}, "model"}}}
	sources := exporter.SequenceSources("mdx")
	if sources[0].Path != "model.mdx" || sources[0].Sequences[0].Name != "Attack 1" || sources[0].Sequences[1].Name != "Attack 2" {
		t.Fatalf("wrong exported names: %+v", sources)
	}
	if sources[0].Sequences[0].WowName != "Attack1H" || sources[0].Sequences[0].WowVariant != 2 || sources[0].Sequences[1].WowName != "Attack2H" {
		t.Fatalf("lost source identity: %+v", sources)
	}
}
