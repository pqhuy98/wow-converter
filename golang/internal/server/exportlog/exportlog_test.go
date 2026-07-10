package exportlog

import (
	"bytes"
	"log"
	"testing"
)

func TestCaptureKeepsRollingWindow(t *testing.T) {
	buf := &bytes.Buffer{}
	underlying = buf
	log.SetOutput(captureWriter{})

	Begin()
	log.Print("line 1")
	log.Print("line 2")
	log.Print("line 3")

	got := Snapshot()
	if len(got) != 2 {
		t.Fatalf("len(Snapshot()) = %d, want 2", len(got))
	}
	if got[0] != "line 2" || got[1] != "line 3" {
		t.Fatalf("Snapshot() = %#v, want [line 2 line 3]", got)
	}

	End()
	log.Print("line 4")
	if snap := Snapshot(); len(snap) != 2 || snap[1] != "line 3" {
		t.Fatalf("after End(), Snapshot() = %#v, want last captured lines unchanged", snap)
	}
}

func TestStripLogPrefix(t *testing.T) {
	got := stripLogPrefix("2026/07/08 16:41:00 Exporting character foo")
	if got != "Exporting character foo" {
		t.Fatalf("stripLogPrefix() = %q", got)
	}
}
