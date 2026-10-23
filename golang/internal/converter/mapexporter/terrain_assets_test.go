package mapexporter

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestTerrainAssetsDeduplicateAndCleanOnlyOwnedFilesAfterSuccess(t *testing.T) {
	out := t.TempDir()
	cfg := config.DefaultConfig()
	mc := DefaultMapExportConfig()
	mc.WowExportFolder = "Test"
	e := NewMapExporter(cfg, &mc, nil, nil)
	e.WowObjectManager = common.NewWowObjectManager(cfg, nil, nil)
	dir := "wow/maps/Test/"
	write := func(name string, data []byte) {
		t.Helper()
		p := filepath.Join(out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	first, duplicate, unique := dir+"tex_1_2_0.blp", dir+"tex_1_2_1.blp", dir+"tex_1_2_2.blp"
	write(first, []byte("same complete BLP"))
	write(duplicate, []byte("same complete BLP"))
	write(unique, []byte("different BLP"))
	staleModel, staleTexture := dir+"adt_1_2-tileset-0123.mdx", dir+"adt_1_2-tileset-0123-edge-atlas-00.blp"
	write(staleModel, []byte("old model"))
	write(staleTexture, []byte("old atlas"))
	custom := dir + "user-decoration.mdx"
	write(custom, []byte("keep"))
	write("wow/maps/Other/tex_1_2_1.blp", []byte("keep"))
	model := mdl.New(mdl.NewMDLOptions{Name: dir + "adt_1_2-chunks-v1-4x4-00-00"})
	model.Textures = []*components.Texture{{Image: duplicate, WowData: components.TextureWowData{PngPath: "maps/Test/tex_1_2_1.png"}}, {Image: unique}, {Image: first}}
	asset := &common.Model{RelativePath: model.Model.Name, MDL: model}
	e.WowObjectManager.AssetManager.Models()["chunk"] = asset
	write(asset.RelativePath+".mdx", []byte("current model"))
	imp := make([]byte, 8)
	write("war3map.imp", imp)
	if err := extra.EnsureCustomImports(out, []string{first, duplicate, staleModel, staleTexture, custom}); err != nil {
		t.Fatal(err)
	}
	if err := e.prepareTerrainAssets(context.Background(), out); err != nil {
		t.Fatal(err)
	}
	if model.Textures[0].Image != first || model.Textures[0].WowData.PngPath != "maps/Test/tex_1_2_1.png" || !asset.AlwaysExport {
		t.Fatal("canonical reference/source identity/cached model invalidation incorrect")
	}
	if e.terrainAssets[duplicate] || !e.terrainAssets[first] || !e.terrainAssets[unique] {
		t.Fatal("wrong kept assets")
	}
	// Deduplication cannot delete assets while saved map files still use them.
	if _, err := os.Stat(filepath.Join(out, duplicate)); err != nil {
		t.Fatal("deleted before successful save")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e.cleanTerrainAssets(ctx, out) == nil {
		t.Fatal("cleanup ignored cancellation")
	}
	if _, err := os.Stat(filepath.Join(out, staleModel)); err != nil {
		t.Fatal("cancelled cleanup deleted an old asset")
	}
	if err := e.cleanTerrainAssets(context.Background(), out); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{duplicate, staleModel, staleTexture} {
		if _, err := os.Stat(filepath.Join(out, name)); !os.IsNotExist(err) {
			t.Fatalf("obsolete asset remains: %s", name)
		}
	}
	for _, name := range []string{first, unique, custom, asset.RelativePath + ".mdx", "wow/maps/Other/tex_1_2_1.blp"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Fatal(err)
		}
	}
	imports, err := os.ReadFile(filepath.Join(out, "war3map.imp"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(imports, []byte(duplicate)) || bytes.Contains(imports, []byte(staleModel)) || bytes.Contains(imports, []byte(staleTexture)) {
		t.Fatal("obsolete imports remain")
	}
	if !bytes.Contains(imports, []byte(custom)) || !bytes.Contains(imports, []byte("adt_1_2-chunks-v1-4x4-00-00.mdx")) {
		t.Fatal("cleanup lost imports")
	}
	if err := e.cleanTerrainAssets(context.Background(), out); err != nil {
		t.Fatal(err)
	}
	again, _ := os.ReadFile(filepath.Join(out, "war3map.imp"))
	if !bytes.Equal(imports, again) {
		t.Fatal("cleanup not idempotent")
	}
	if binary.LittleEndian.Uint32(again[4:8]) != 4 {
		t.Fatalf("unexpected import count: %d", binary.LittleEndian.Uint32(again[4:8]))
	}
	for _, bad := range []string{"../Other", filepath.Join(out, "outside")} {
		e.MapExportConfig.WowExportFolder = bad
		if _, err := e.terrainAssetDir(out); err == nil {
			t.Fatal("unsafe cleanup path accepted")
		}
	}
}
