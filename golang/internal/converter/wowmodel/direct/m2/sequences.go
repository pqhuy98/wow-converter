package directm2

import (
	"context"
	"fmt"
	"sort"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// SequenceSelection addresses records in one resolved model/build, not animation IDs.
type SequenceSelection struct {
	FileDataID int    `json:"fileDataID"`
	BuildKey   string `json:"buildKey"`
	Indices    []int  `json:"indices"`
}

type ModelSequence struct {
	Index           int    `json:"index"`
	AnimationID     uint16 `json:"animationID"`
	WowVariant      uint16 `json:"wowVariant"`
	WowName         string `json:"wowName"`
	WC3Name         string `json:"wc3Name"`
	Duration        uint32 `json:"duration"`
	Looping         bool   `json:"looping"`
	DefaultIncluded bool   `json:"defaultIncluded"`
}

type ModelMetadata struct {
	FileDataID        int             `json:"fileDataID"`
	BuildKey          string          `json:"buildKey"`
	Sequences         []ModelSequence `json:"sequences"`
	BakingRecommended *bool           `json:"bakingRecommended"`
	BakingReason      string          `json:"bakingReason,omitempty"`
}

// ReadModelMetadata reads the same skeleton graph as conversion without writing assets.
func ReadModelMetadata(ctx context.Context, src FileSource, fileDataID int, keepCinematic bool, attackTag animmap.AttackTag) (ModelMetadata, error) {
	raw, err := src.GetRawFile(ctx, fileDataID)
	if err != nil {
		return ModelMetadata{}, err
	}
	loader := m2.NewLoader(buffer.From(raw), func(c context.Context, id uint32) ([]byte, error) { return src.GetRawFile(c, int(id)) })
	if err := loader.Load(ctx); err != nil {
		return ModelMetadata{}, err
	}
	buildKey, err := src.GetBuildKey(ctx)
	if err != nil {
		return ModelMetadata{}, err
	}
	graph, err := loadSkeletonGraph(ctx, loader, fileDataID, buildKey)
	if err != nil {
		return ModelMetadata{}, err
	}
	result := ModelMetadata{FileDataID: fileDataID, BuildKey: buildKey, Sequences: make([]ModelSequence, 0, len(graph.Animations))}
	for i, a := range graph.Animations {
		name := animmap.GetWowAnimName(int(a.ID))
		info := animmap.GetWc3AnimInfo(name)
		result.Sequences = append(result.Sequences, ModelSequence{Index: i, AnimationID: a.ID, WowVariant: a.VariationIndex, WowName: name, WC3Name: info.WC3Name, Duration: a.Duration, Looping: animmap.IsLoopAnimation(name), DefaultIncluded: animmap.IncludeByDefault(name, keepCinematic, attackTag)})
	}
	sortModelSequences(result.Sequences, graph.Animations)
	// ponytail: a trustworthy shader compatibility classifier is deferred; unknown
	// keeps the existing manual Advanced Textures option without misleading advice.
	result.BakingReason = "Material compatibility has not been checked; Advanced Textures is available manually."
	return result, nil
}

func sortModelSequences(sequences []ModelSequence, animations []m2.AnimationEntry) {
	// Match the converter's preference for the most frequent idle/walk variant.
	best := map[string]int{}
	for _, sequence := range sequences {
		switch sequence.WC3Name {
		case "Stand", "Stand Alternate", "Walk", "Walk Alternate":
			previous, exists := best[sequence.WC3Name]
			if !exists || animations[sequence.Index].Frequency > animations[previous].Frequency {
				best[sequence.WC3Name] = sequence.Index
			}
		}
	}
	key := func(sequence ModelSequence) components.Sequence {
		rarity := 0
		if preferred, exists := best[sequence.WC3Name]; exists && preferred != sequence.Index {
			rarity = 4
		}
		return components.Sequence{Name: sequence.WC3Name, Rarity: &rarity}
	}
	sort.SliceStable(sequences, func(i, j int) bool {
		return mdl.CompareSequences(key(sequences[i]), key(sequences[j])) < 0
	})
}

func applySequenceSelection(file *bundleanim.File, selection *SequenceSelection, fileDataID int, buildKey string) error {
	if selection.FileDataID != fileDataID || selection.BuildKey == "" || selection.BuildKey != buildKey {
		return fmt.Errorf("animation selection belongs to a different model or build; choose sequences again")
	}
	if len(selection.Indices) == 0 {
		return fmt.Errorf("select at least one animation")
	}
	selected := make(map[int]struct{}, len(selection.Indices))
	for _, index := range selection.Indices {
		if index < 0 || index >= len(file.Animations) {
			return fmt.Errorf("animation record %d is not available; choose sequences again", index)
		}
		if _, duplicate := selected[index]; duplicate {
			return fmt.Errorf("duplicate animation record %d", index)
		}
		selected[index] = struct{}{}
	}
	file.ExcludedIndices = make(map[int]struct{}, len(file.Animations)-len(selected))
	for index := range file.Animations {
		if _, include := selected[index]; !include {
			file.ExcludedIndices[index] = struct{}{}
		}
	}
	return nil
}
