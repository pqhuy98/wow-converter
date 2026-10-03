package client

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExpireOtherRawBuildCaches(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for _, name := range []string{"live", "recent", "stale"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	liveFile := filepath.Join(dir, "live", "106695")
	if err := os.WriteFile(liveFile, []byte("m2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, "recent"), now.Add(-2*24*time.Hour), now.Add(-2*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(dir, "stale"), now.Add(-20*24*time.Hour), now.Add(-20*24*time.Hour)); err != nil {
		t.Fatal(err)
	}

	removed, err := expireOtherRawBuildCaches(dir, "live", now)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed %d, want 1", removed)
	}
	if _, err := os.Stat(liveFile); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "recent")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "note.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale")); !os.IsNotExist(err) {
		t.Fatalf("stale still present: %v", err)
	}

	removed, err = expireOtherRawBuildCaches(dir, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("empty key removed %d", removed)
	}
	if _, err := os.Stat(liveFile); err != nil {
		t.Fatal(err)
	}

	removed, err = expireOtherRawBuildCaches(filepath.Join(dir, "missing"), "live", now)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Fatalf("missing dir removed %d", removed)
	}
}
