package directm2

import (
	"reflect"
	"testing"

	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestMetadataUsesExportSequenceOrderingWithoutChangingSourceIndices(t *testing.T) {
	sequences := []ModelSequence{
		{Index: 0, WC3Name: "Cinematic EmoteLaugh"},
		{Index: 1, WC3Name: "Attack Slam"},
		{Index: 2, WC3Name: "Stand"},
		{Index: 3, WC3Name: "Walk"},
		{Index: 4, WC3Name: "Stand"},
		{Index: 5, WC3Name: "Attack"},
	}
	animations := make([]m2.AnimationEntry, len(sequences))
	animations[4].Frequency = 100
	sortModelSequences(sequences, animations)
	var indices []int
	for _, sequence := range sequences {
		indices = append(indices, sequence.Index)
	}
	if !reflect.DeepEqual(indices, []int{4, 2, 3, 5, 1, 0}) {
		t.Fatalf("export ordering/source indices: %v", indices)
	}
}

func TestSelectionDistinguishesVariantsAndRejectsStaleSource(t *testing.T) {
	file := &bundleanim.File{Animations: []bundleanim.AnimationData{{ID: 0, VariationIndex: 0}, {ID: 0, VariationIndex: 1}, {ID: 1}}}
	selection := &SequenceSelection{FileDataID: 42, BuildKey: "build", Indices: []int{1}}
	if err := applySequenceSelection(file, selection, 42, "build"); err != nil {
		t.Fatal(err)
	}
	if _, excluded := file.ExcludedIndices[1]; excluded || len(file.ExcludedIndices) != 2 {
		t.Fatalf("variant selection = %v", file.ExcludedIndices)
	}
	for _, invalid := range []SequenceSelection{
		{FileDataID: 43, BuildKey: "build", Indices: []int{1}},
		{FileDataID: 42, BuildKey: "old", Indices: []int{1}},
		{FileDataID: 42, BuildKey: "build"},
		{FileDataID: 42, BuildKey: "build", Indices: []int{3}},
		{FileDataID: 42, BuildKey: "build", Indices: []int{-1}},
		{FileDataID: 42, BuildKey: "build", Indices: []int{1, 1}},
	} {
		if err := applySequenceSelection(file, &invalid, 42, "build"); err == nil {
			t.Fatalf("accepted %+v", invalid)
		}
	}
}
