package components

import (
	"strings"
	"sync/atomic"
)

var globalSequenceStableID atomic.Uint64

type GlobalSequence struct {
	ID       int
	Duration int
	RawID    int
	HasRawID bool
	StableID uint64
}

func NewGlobalSequence(id int, duration int) GlobalSequence {
	return GlobalSequence{
		ID:       id,
		Duration: duration,
		RawID:    id,
		HasRawID: true,
		StableID: globalSequenceStableID.Add(1),
	}
}

func CloneGlobalSequence(gs GlobalSequence) GlobalSequence {
	gs.StableID = globalSequenceStableID.Add(1)
	return gs
}

func GlobalSequencesToString(globalSequences []*GlobalSequence) string {
	if len(globalSequences) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("GlobalSequences ")
	b.WriteString(FVal(float64(len(globalSequences))))
	b.WriteString(" {\n")
	for _, gs := range globalSequences {
		if gs == nil {
			continue
		}
		b.WriteString("Duration ")
		b.WriteString(FVal(float64(gs.Duration)))
		b.WriteString(",\n")
	}
	b.WriteString("}")
	return b.String()
}
