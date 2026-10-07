package character

import (
	"fmt"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
)

// SequenceSource preserves the source identity after filtering and sequence sorting.
type SequenceSource struct {
	Index      int    `json:"index"`
	Name       string `json:"name"`
	WowName    string `json:"wowName"`
	WowVariant int    `json:"wowVariant"`
}

type ModelSequenceSources struct {
	Path       string           `json:"path"`
	ModelScale float64          `json:"modelScale"`
	Sequences  []SequenceSource `json:"sequences"`
}

func (e *CharacterExporter) SequenceSources(format string) []ModelSequenceSources {
	models := make([]ModelSequenceSources, 0, len(e.Models))
	for _, pair := range e.Models {
		model := pair[0].(*mdl.MDL)
		source := ModelSequenceSources{Path: filepath.ToSlash(pair[1].(string)) + "." + format, ModelScale: e.Config.RawModelScaleUp * model.AccumScale, Sequences: make([]SequenceSource, 0, len(model.Sequences))}
		counts := map[string]int{}
		for i, seq := range model.Sequences {
			counts[seq.Name]++
			source.Sequences = append(source.Sequences, SequenceSource{Index: i, Name: fmt.Sprintf("%s %d", seq.Name, counts[seq.Name]), WowName: seq.Data.WowName, WowVariant: seq.Data.WowVariant})
		}
		models = append(models, source)
	}
	return models
}
