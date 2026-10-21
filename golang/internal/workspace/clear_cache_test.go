package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClearCacheSeparatesBrowseAndCASCAndPreservesExports(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if FindRepoRoot() != root {
		t.Fatal("cache fixture must resolve within the temporary directory")
	}
	files := map[string]string{
		".cache/wow/data.bin":                 "cache",
		"exported-assets-browse/nested/a.blp": "browse",
		"exported-assets/model.mdx":           "keep export",
		"maps/test.w3x/war3map.lua":           "keep map",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, kind := range []string{"casc", "browse"} {
		want := int64(len("cache"))
		if kind == "browse" {
			want = int64(len("browse"))
		}
		size, err := ProjectCacheDirSize(kind)
		if err != nil || size != want {
			t.Fatalf("%s size = %d, %v, want %d", kind, size, err, want)
		}
		if err := ClearProjectCacheDir(kind); err != nil {
			t.Fatal(err)
		}
		size, err = ProjectCacheDirSize(kind)
		if err != nil || size != 0 {
			t.Fatalf("cleared %s size = %d, %v", kind, size, err)
		}
		if kind == "casc" {
			size, err = ProjectCacheDirSize("browse")
			if err != nil || size != int64(len("browse")) {
				t.Fatal("CASC clear deleted browse assets")
			}
			if err := os.WriteFile(filepath.Join(root, ".cache", "keep"), []byte("cache"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if content, err := os.ReadFile(filepath.Join(root, ".cache", "keep")); err != nil || string(content) != "cache" {
		t.Fatal("browse clear deleted CASC cache")
	}
	for _, name := range []string{"exported-assets/model.mdx", "maps/test.w3x/war3map.lua"} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil || string(content) != files[name] {
			t.Fatalf("changed protected output %s: %v", name, err)
		}
	}
	if err := ClearProjectCacheDir("browse"); err != nil {
		t.Fatal(err)
	}
	size, err := ProjectCacheDirSize("browse")
	if err != nil || size != 0 {
		t.Fatalf("cleared cache size = %d, %v", size, err)
	}
	if err := ClearProjectCacheDir("casc"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "exported-assets-browse", "new.blp"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	size, err = ProjectCacheDirSize("browse")
	if err != nil || size != 3 {
		t.Fatalf("missing .cache hid browse assets: %d, %v", size, err)
	}
	for _, kind := range []string{"", "../maps", "all"} {
		if err := ClearProjectCacheDir(kind); err == nil {
			t.Fatalf("accepted invalid cache kind %q", kind)
		}
	}
}
