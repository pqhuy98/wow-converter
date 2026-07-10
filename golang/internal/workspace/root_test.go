package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveAppRootModes(t *testing.T) {
	t.Parallel()
	repoRoot := devRepoRoot(t)

	desktop := filepath.Join(t.TempDir(), "dist-go")
	writeShippedBundle(t, desktop)

	tests := []struct {
		name   string
		exeDir string
		cwd    string
		want   string
	}{
		{
			name:   "desktop app",
			exeDir: desktop,
			cwd:    repoRoot,
			want:   desktop,
		},
		{
			name:   "dev:go air",
			exeDir: filepath.Join(repoRoot, "golang", "tmp"),
			cwd:    filepath.Join(repoRoot, "golang"),
			want:   repoRoot,
		},
		{
			name:   "dev:go-data-server",
			exeDir: filepath.Join(repoRoot, "golang", "tmp"),
			cwd:    filepath.Join(repoRoot, "golang"),
			want:   repoRoot,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := resolveAppRoot(tc.exeDir, tc.cwd)
			if got != tc.want {
				t.Fatalf("resolveAppRoot(%q, %q) = %q, want %q", tc.exeDir, tc.cwd, got, tc.want)
			}
		})
	}
}

func TestResolveAppRootDesktopIgnoresParentDevRepo(t *testing.T) {
	t.Parallel()
	repoRoot := devRepoRoot(t)
	desktop := filepath.Join(repoRoot, "dist-go")
	if !isShippedBundleAt(desktop) {
		t.Skip("dist-go bundle not built")
	}
	got := resolveAppRoot(desktop, repoRoot)
	if got != desktop {
		t.Fatalf("resolveAppRoot(dist-go inside repo) = %q, want %q", got, desktop)
	}
}

func TestResolveRepoPath(t *testing.T) {
	t.Parallel()
	root := AppRoot()
	got := ResolveRepoPath("exported-assets")
	want := filepath.Join(root, "exported-assets")
	if got != want {
		t.Fatalf("ResolveRepoPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		t.Skip("not in wow-converter repo")
	}
}

func devRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if isDevRepoRoot(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("not in wow-converter repo")
		}
		dir = parent
	}
}

func writeShippedBundle(t *testing.T, root string) {
	t.Helper()
	mustMkdirAll(t, filepath.Join(root, "webui", "out"))
	mustWriteFile(t, filepath.Join(root, "webui", "out", "index.html"), "x")
	mustMkdirAll(t, filepath.Join(root, "resources", "template-empty.w3x"))
	mustMkdirAll(t, filepath.Join(root, "bin"))
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
