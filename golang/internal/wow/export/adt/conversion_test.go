package adt

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/export"
	adtfmt "github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
)

func TestParallelConversionKeepsEveryChunkTexture(t *testing.T) {
	conv := &ConversionOutput{ExportAssetDir: t.TempDir()}
	const chunks = 256
	var workers sync.WaitGroup
	start := make(chan struct{})
	errors := make(chan error, chunks)
	for i := 0; i < chunks; i++ {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			<-start
			path := filepath.Join(conv.ExportAssetDir, "maps", "Azeroth", fmt.Sprintf("tex_34_49_%d.png", i))
			errors <- storePNG(path, []byte{byte(i), 100, 200, 255}, 1, 1, conv)
		}(i)
	}
	close(start)
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(conv.Textures) != chunks {
		t.Fatalf("lost parallel terrain textures: got %d, want %d", len(conv.Textures), chunks)
	}
	seen := map[string]bool{}
	for _, texture := range conv.Textures {
		if seen[texture.RelPath] {
			t.Fatalf("duplicate texture %s", texture.RelPath)
		}
		seen[texture.RelPath] = true
		if _, err := png.Decode(bytes.NewReader(texture.PNG)); err != nil {
			t.Fatalf("invalid texture %s: %v", texture.RelPath, err)
		}
	}
}

func TestSplitConversionKeepsPNGWhenDiskCacheAlreadyExists(t *testing.T) {
	conv := &ConversionOutput{ExportAssetDir: t.TempDir()}
	dir := filepath.Join(conv.ExportAssetDir, "maps", "Azeroth")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(dir, "tex_34_49_108.png")
	if err := os.WriteFile(cached, []byte("old disk cache"), 0644); err != nil {
		t.Fatal(err)
	}
	root := &adtfmt.ADTLoader{Chunks: make([]adtfmt.ADTChunk, 256)}
	tex := &adtfmt.ADTLoader{TexChunks: make([]adtfmt.TexChunk, 256)}
	exporter := NewExporter(0, "Azeroth", 34*64+49)
	// Tiny empty chunks exercise the real split-bake queue without CASC or an 8K allocation.
	if err := exporter.exportLargeBake(context.Background(), nil, dir, 16, export.ADTExportOptions{}, root, tex, adtfmt.ADTChunk{}, make([][]int, 256), nil, nil, nil, true, nil, conv); err != nil {
		t.Fatal(err)
	}
	if len(conv.Textures) != 256 {
		t.Fatalf("incomplete split bake: got %d textures, want 256", len(conv.Textures))
	}
	seen := map[string]bool{}
	for _, texture := range conv.Textures {
		if seen[texture.RelPath] {
			t.Fatalf("duplicate split texture %s", texture.RelPath)
		}
		seen[texture.RelPath] = true
		if _, err := png.Decode(bytes.NewReader(texture.PNG)); err != nil {
			t.Fatalf("invalid split texture %s: %v", texture.RelPath, err)
		}
	}
	if !seen["maps/Azeroth/tex_34_49_108.png"] {
		t.Fatal("chunk 108 was skipped because an unrelated disk artifact existed")
	}
	raw, err := os.ReadFile(cached)
	if err != nil || string(raw) != "old disk cache" {
		t.Fatal("in-memory conversion changed the disk cache")
	}
}
