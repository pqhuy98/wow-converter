package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRepoPath(t *testing.T) {
	t.Parallel()
	root := FindRepoRoot()
	got := ResolveRepoPath("exported-assets")
	want := filepath.Join(root, "exported-assets")
	if got != want {
		t.Fatalf("ResolveRepoPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		t.Skip("not in wow-converter repo")
	}
}
