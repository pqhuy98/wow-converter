package character

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
)

func TestCancelledModelWriteDoesNotSerializeOrCreateOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	exporter := &CharacterExporter{Models: [][2]interface{}{{(*mdl.MDL)(nil), "cancelled"}}}
	if _, err := exporter.WriteAllModels(ctx, dir, "mdx"); !errors.Is(err, context.Canceled) {
		t.Fatalf("write error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("cancelled export created output")
	}
}

func TestWriteAllModelsReturnsWrittenPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	exporter := &CharacterExporter{Config: config.DefaultConfig()}
	exporter.IncludeMdlToOutput(mdl.New(mdl.NewMDLOptions{Name: "test"}), "the-lich-king")

	paths, err := exporter.WriteAllModels(context.Background(), dir, "mdx")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 {
		t.Fatalf("paths len = %d, want 1", len(paths))
	}
	got := paths[0]
	if !strings.HasSuffix(got, ".mdx") {
		t.Fatalf("path %q missing .mdx suffix", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("written model missing on disk: %v", err)
	}
	rel, err := filepath.Rel(dir, got)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.ToSlash(rel) != "the-lich-king.mdx" {
		t.Fatalf("rel = %q, want the-lich-king.mdx", rel)
	}
}
