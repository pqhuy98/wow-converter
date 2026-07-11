package main

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestDeathTimeSec(t *testing.T) {
	if got := deathTimeSec(nil); got != 6 {
		t.Fatalf("nil mdl: got %v want 6", got)
	}
	m := mdl.New(mdl.NewMDLOptions{Name: "test"})
	m.Sequences = []components.Sequence{{Name: "Death", Interval: [2]int{0, 2500}}}
	if got := deathTimeSec(m); got != 2.5 {
		t.Fatalf("death seq: got %v want 2.5", got)
	}
}
