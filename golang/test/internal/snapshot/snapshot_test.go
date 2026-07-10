package snapshot_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/test/internal/snapshot"
)

func TestCreateAndCompareIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := snapshot.Create(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(manifest.Files))
	}

	summary, err := snapshot.CompareManifestToDir(manifest, dir, snapshot.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !summary.Pass() {
		t.Fatalf("expected pass, got %+v", summary)
	}
	if len(summary.Identical) != 1 {
		t.Fatalf("expected 1 identical file, got %d", len(summary.Identical))
	}
}

func TestCompareDetectsMissingAndExtra(t *testing.T) {
	base := t.TempDir()
	target := t.TempDir()

	if err := os.WriteFile(filepath.Join(base, "keep.txt"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "gone.txt"), []byte("gone"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep.txt"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "new.txt"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	manifest, err := snapshot.Create(base)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := snapshot.CompareManifestToDir(manifest, target, snapshot.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Pass() {
		t.Fatal("expected failure")
	}
	if len(summary.Missing) != 1 || summary.Missing[0] != "gone.txt" {
		t.Fatalf("unexpected missing: %#v", summary.Missing)
	}
	if len(summary.Extra) != 1 || summary.Extra[0] != "new.txt" {
		t.Fatalf("unexpected extra: %#v", summary.Extra)
	}
}

func TestIsPlaceholderSHA(t *testing.T) {
	cases := map[string]bool{
		"":           true,
		"placeholder": true,
		"TODO":        true,
		"0000000000000000000000000000000000000000000000000000000000000000": true,
		"abc123": false,
	}
	for input, want := range cases {
		if got := snapshot.IsPlaceholderSHA(input); got != want {
			t.Fatalf("IsPlaceholderSHA(%q) = %v, want %v", input, got, want)
		}
	}
}
