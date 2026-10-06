package common

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	pngwriter "github.com/pqhuy98/wow-converter/internal/formats/png"
)

func TestExportTextureIgnoreAlphaCompactsWithoutChangingQuantization(t *testing.T) {
	rgba := make([]byte, 4*4*4)
	colors := [][4]byte{{255, 0, 0, 10}, {0, 255, 0, 70}, {0, 0, 255, 130}, {255, 255, 0, 190}}
	for i := 0; i < 16; i++ {
		copy(rgba[i*4:i*4+4], colors[i%len(colors)][:])
	}
	pngData, err := pngwriter.EncodeRGBA(rgba, 4, 4)
	if err != nil {
		t.Fatal(err)
	}
	const regularRel, ignoredRel = "alpha/regular.png", "alpha/ignored.png"
	texturesource.Register(regularRel, texturesource.Source{Kind: texturesource.KindPNG, PNG: pngData, PreserveAlpha: true})
	texturesource.Register(ignoredRel, texturesource.Source{Kind: texturesource.KindPNG, PNG: pngData, PreserveAlpha: true, IgnoreAlpha: true})
	t.Cleanup(func() {
		texturesource.Unregister(regularRel)
		texturesource.Unregister(ignoredRel)
	})

	assetRoot := t.TempDir()
	newManager := func() *AssetManager {
		return NewAssetManager(config.Config{ExportAssetDir: t.TempDir(), AssetPrefix: "wow", OverrideTextures: true}, nil, nil)
	}
	regularManager, ignoredManager := newManager(), newManager()
	regularManager.textures[regularRel] = struct{}{}
	ignoredManager.textures[ignoredRel] = struct{}{}
	regularDir := filepath.Join(assetRoot, "regular")
	ignoredDir := filepath.Join(assetRoot, "ignored")
	if _, err := regularManager.ExportTextures(regularDir); err != nil {
		t.Fatal(err)
	}
	if _, err := ignoredManager.ExportTextures(ignoredDir); err != nil {
		t.Fatal(err)
	}
	regular, err := os.ReadFile(filepath.Join(regularDir, "wow", "alpha", "regular.blp"))
	if err != nil {
		t.Fatal(err)
	}
	ignored, err := os.ReadFile(filepath.Join(ignoredDir, "wow", "alpha", "ignored.blp"))
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(regular[8:12]); got != 8 {
		t.Fatalf("regular alphaBits = %d, want 8-bit alpha", got)
	}
	if got := binary.LittleEndian.Uint32(ignored[8:12]); got != 0 {
		t.Fatalf("ignored alphaBits = %d, want no alpha plane", got)
	}
	if string(regular[156:1180]) != string(ignored[156:1180]) {
		t.Fatal("ignoring alpha changed the palette")
	}
	regularMip := int(binary.LittleEndian.Uint32(regular[28:32]))
	ignoredMip := int(binary.LittleEndian.Uint32(ignored[28:32]))
	if string(regular[regularMip:regularMip+4]) != string(ignored[ignoredMip:ignoredMip+4]) {
		t.Fatal("ignoring alpha changed color indices")
	}
}
