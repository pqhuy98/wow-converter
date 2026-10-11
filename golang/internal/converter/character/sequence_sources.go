package character

import (
	"fmt"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

// SequenceSource preserves the source identity after filtering and sequence sorting.
type SequenceSource struct {
	Index         int    `json:"index"`
	Name          string `json:"name"`
	WowName       string `json:"wowName"`
	WowVariant    int    `json:"wowVariant"`
	SourceIndices []int  `json:"sourceIndices,omitempty"`
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
		names := components.SequenceNames(model.Sequences)
		for i, seq := range model.Sequences {
			source.Sequences = append(source.Sequences, SequenceSource{Index: i, Name: names[i], WowName: seq.Data.WowName, WowVariant: seq.Data.WowVariant, SourceIndices: append([]int(nil), seq.Data.SourceIndices...)})
		}
		models = append(models, source)
	}
	return models
}

type AnimationRename struct {
	WowName    string `json:"wowName"`
	WowVariant int    `json:"wowVariant"`
	NewWC3Name string `json:"newWc3Name"`
}

// RenameAnimations runs after sorting and generation, so names cannot change order
// or interfere with attack/decay generation. Only the base output is renamed.
func (e *CharacterExporter) RenameAnimations(outputFile string, renames []AnimationRename) error {
	names := make(map[components.SourceAnimation]string, len(renames))
	for _, rename := range renames {
		names[components.SourceAnimation{WowName: rename.WowName, WowVariant: rename.WowVariant}] = rename.NewWC3Name
	}
	for _, pair := range e.Models {
		if pair[1].(string) != outputFile {
			continue
		}
		model := pair[0].(*mdl.MDL)
		for i := range model.Sequences {
			sequence := &model.Sequences[i]
			name := ""
			for _, source := range sequence.Data.SourceAnimations {
				if renamed := names[source]; renamed != "" {
					if name != "" && name != renamed {
						return fmt.Errorf("combined animation %s has conflicting renames; use the same name for its source animations", sequence.Name)
					}
					name = renamed
				}
			}
			if name != "" {
				sequence.Name = name
				sequence.NameOverridden = true
			}
		}
	}
	return nil
}
