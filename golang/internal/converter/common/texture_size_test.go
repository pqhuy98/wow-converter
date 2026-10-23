package common

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	pngwriter "github.com/pqhuy98/wow-converter/internal/formats/png"
)

func TestTextureLimitsAreIndependentAndRefreshCachedBLPs(t *testing.T) {
	png, err := pngwriter.EncodeRGBA(make([]byte, 1024*512*4), 1024, 512)
	if err != nil {
		t.Fatal(err)
	}
	sources := t.TempDir()
	if err := os.MkdirAll(filepath.Join(sources, "texture-limits"), 0755); err != nil {
		t.Fatal(err)
	}
	manager := NewAssetManager(config.Config{ExportAssetDir: sources, AssetPrefix: "wow", MaxTextureSize: 512}, nil, nil)
	for _, rel := range []string{"texture-limits/scenery.png", "texture-limits/terrain.png"} {
		if err := os.WriteFile(filepath.Join(sources, rel), png, 0644); err != nil {
			t.Fatal(err)
		}
		manager.textures[rel] = struct{}{}
	}
	manager.SetTextureMaxSize(`wow\texture-limits\scenery.blp`, 256)
	manager.SetTextureMaxSize("texture-limits/terrain.png", 0)
	// A shared terrain reference must retain full resolution in either traversal order.
	manager.SetTextureMaxSize("texture-limits/terrain.png", 256)
	out := t.TempDir()
	check := func(sceneryWidth int) {
		t.Helper()
		if _, err := manager.ExportTextures(context.Background(), out); err != nil {
			t.Fatal(err)
		}
		for name, width := range map[string]int{"scenery": sceneryWidth, "terrain": 1024} {
			size := blp.ReadBlpSizeSync(filepath.Join(out, "wow", "texture-limits", name+".blp"))
			if size == nil || int(size.Width) != width || int(size.Height) != width/2 {
				t.Fatalf("%s dimensions: %+v, want %dx%d", name, size, width, width/2)
			}
		}
	}
	check(256)
	manager.SetTextureMaxSize("texture-limits/scenery.png", 0)
	check(1024) // Existing smaller BLPs must be rebuilt without Fresh export.
	manager.ReleaseAfterExport()
	if len(manager.textureMaxSizes) != 0 {
		t.Fatal("texture limits survived export cleanup")
	}
}
