package pickfolder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveInitialDirectory(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "missing", "child")

	if got := ResolveInitialDirectory(""); got != "" {
		t.Fatalf("empty input = %q, want empty", got)
	}
	if got := ResolveInitialDirectory(child); got != dir {
		t.Fatalf("missing child = %q, want %q", got, dir)
	}
	if got := ResolveInitialDirectory(dir); got != dir {
		t.Fatalf("existing dir = %q, want %q", got, dir)
	}

	if wd, err := os.Getwd(); err == nil {
		if got := ResolveInitialDirectory(filepath.Join(wd, "definitely-missing-subdir")); got != wd {
			t.Fatalf("missing subdir of cwd = %q, want %q", got, wd)
		}
	}
}
