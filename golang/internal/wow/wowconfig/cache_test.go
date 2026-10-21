package wowconfig

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowseCacheClearDoesNotUnloadWoW(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	for _, name := range []string{".cache/keep", "exported-assets-browse/remove"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("asset"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// No client: any attempted unload/restart would fail this test.
	svc := &Service{}
	if err := svc.ClearCache(context.Background(), "browse"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".cache", "keep")); err != nil {
		t.Fatal("browse cleanup removed CASC data", err)
	}
	if _, err := os.Stat(filepath.Join(root, "exported-assets-browse", "remove")); !os.IsNotExist(err) {
		t.Fatal("browse asset was not removed")
	}
}
