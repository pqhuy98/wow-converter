package character

import (
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

// Replace every Stand family with the selected weapon's Ready pose. The normal
// exporter has already filtered attack tags. Keep the original idle if unavailable.
// OptimizeNPCSequences subsequently prunes removed timelines and rebinds events.
func useNPCStandReady(model *mdl.MDL) {
	hasReady := false
	for _, seq := range model.Sequences {
		if seq.Name == "Stand Ready" {
			hasReady = true
			break
		}
	}
	if !hasReady {
		return
	}
	var kept []components.Sequence
	for _, seq := range model.Sequences {
		if seq.Name == "Stand Ready" {
			seq.Name, seq.Data.WC3Name = "Stand", "Stand"
			seq.NonLooping, seq.Rarity = false, nil
		} else if seq.Name == "Stand" || strings.HasPrefix(seq.Name, "Stand ") {
			continue
		}
		kept = append(kept, seq)
	}
	model.Sequences = kept
}
