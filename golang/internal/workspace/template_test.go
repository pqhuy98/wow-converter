package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTemplateEmptyDir(t *testing.T) {
	t.Parallel()
	root := FindRepoRoot()
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		t.Skip("not in wow-converter repo")
	}
	got, err := ResolveTemplateEmptyDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "maps", "template-empty.w3x")
	if got != want {
		t.Fatalf("ResolveTemplateEmptyDir = %q, want %q", got, want)
	}
}
