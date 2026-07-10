package directm2

import (
	"path/filepath"
	"testing"
)

func TestRelPathDeepOutDir(t *testing.T) {
	exportRoot := filepath.Join("D:", "Projects", "wow-converter", ".cache", "wow-export")
	outDir := filepath.Join(exportRoot, "azeroth", "32_48", "world", "maps", "zone", "model")
	texPath := filepath.Join(exportRoot, "world", "textures", "brick.png")
	texPath2 := filepath.Join(exportRoot, "world", "other", "brick.png")

	got := relPath(outDir, texPath)
	got2 := relPath(outDir, texPath2)
	t.Logf("rel1=%q rel2=%q", got, got2)
	if got == got2 {
		t.Fatalf("different textures collapsed to same relative path: %q", got)
	}
	if filepath.Base(got) == got && filepath.Base(got2) == got2 {
		t.Fatalf("relPath fell back to basename only: %q and %q", got, got2)
	}

	// Roundtrip used by metadata: join(parentDir, fileNameExternal) should resolve under exportRoot.
	parentDir := filepath.Dir(filepath.Join(outDir, "model.m2"))
	joined := filepath.Clean(filepath.Join(parentDir, got))
	joined2 := filepath.Clean(filepath.Join(parentDir, got2))
	if joined == joined2 {
		t.Fatalf("metadata join collapsed textures: %q", joined)
	}
}

func TestMetadataPngPathRoundtrip(t *testing.T) {
	exportRoot := filepath.Join("D:", "Projects", "wow-converter", ".cache", "wow-export")
	exportPath := filepath.Join(exportRoot, "azeroth", "32_48", "world", "maps", "zone", "model.m2")
	parentDir := filepath.Dir(exportPath)

	texPaths := []string{
		filepath.Join(exportRoot, "world", "textures", "brick.png"),
		filepath.Join(exportRoot, "world", "other", "stone.png"),
	}
	seen := map[string]struct{}{}
	for _, texPath := range texPaths {
		texFile := relPath(parentDir, texPath)
		joined := filepath.Clean(filepath.Join(parentDir, texFile))
		pngRel, err := filepath.Rel(exportRoot, joined)
		if err != nil {
			t.Fatalf("relExport failed for %s: %v", texPath, err)
		}
		pngRel = filepath.ToSlash(pngRel)
		if _, ok := seen[pngRel]; ok {
			t.Fatalf("duplicate png path %q", pngRel)
		}
		seen[pngRel] = struct{}{}
		t.Logf("%s -> %s", texPath, pngRel)
	}
}
