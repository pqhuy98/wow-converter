//go:build integration_tests

package mapexporter

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

// Opt in to a real loaded-data export. Writes only to the requested validation
// directory, leaving user maps alone; the active CASC product is not changed.
func TestLoadedAdtChunkAssets(t *testing.T) {
	out := os.Getenv("WOW_ADT_CHUNK_VALIDATION_DIR")
	if out == "" {
		t.Skip("set WOW_ADT_CHUNK_VALIDATION_DIR for a real terrain export")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := workspace.LoadEnvFile(filepath.Join(workspace.FindRepoRoot(), ".env")); err != nil {
		t.Fatal(err)
	}
	handler, err := bootstrap.StartWowDataServer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := client.NewInProcessClient(handler)
	if _, err := c.GetCASCInfo(ctx); err != nil {
		t.Fatal(err)
	}
	cfg := config.DefaultConfig()
	cfg.OverrideModels = false
	mc := DefaultMapExportConfig()
	mc.MapID = 609
	mc.CascMapDir = "DeathKnightStart"
	mc.WowExportFolder = "DeathKnightStart"
	mc.Min = [2]float64{41, 28}
	mc.Max = [2]float64{42, 29}
	tiles := TilesFromBounds(41, 28, 42, 29)
	mc.Creatures.Enable = false
	mc.PlacePathBlockers = false
	mc.Navigation.Disabled = true
	mc.Doodads.Enable = DoodadEnable{ADT: true}
	var before map[string][32]byte
	var timestamps map[string]time.Time
	for pass, clamp := range []TerrainClampPercent{{Lower: 0, Upper: 1}, {Lower: .2, Upper: .8}} {
		mc.Terrain.ClampPercent = clamp
		registry := common.NewTileRegistry()
		if err := LoadADTTilesForConversion(ctx, c, cfg.ExportAssetDir, mc, 8192, tiles, false, registry, nil); err != nil {
			t.Fatal(err)
		}
		e := NewMapExporter(cfg, &mc, c, registry)
		if err := e.ParseObjects(ctx, func(_ string, typ common.WowObjectType) bool { return typ == common.WowObjectADT }); err != nil {
			t.Fatal(err)
		}
		if err := e.ExportTerrainsDoodads(ctx, out); err != nil {
			t.Fatal(err)
		}
		// Minimal import list is enough to exercise final cleanup without a
		// template/minimap export unrelated to this terrain regression.
		if pass == 0 {
			if err := os.WriteFile(filepath.Join(out, "war3map.imp"), make([]byte, 8), 0644); err != nil {
				t.Fatal(err)
			}
		}
		if err := e.MapManager.Save(out); err != nil {
			t.Fatal(err)
		}
		if err := e.cleanTerrainAssets(ctx, out); err != nil {
			t.Fatal(err)
		}
		current := map[string][32]byte{}
		mtime := map[string]time.Time{}
		models, textures, bytes := 0, 0, int64(0)
		for path := range e.terrainAssets {
			raw, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(path)))
			if err != nil {
				t.Fatal(err)
			}
			current[path] = sha256.Sum256(raw)
			if strings.HasSuffix(path, ".mdx") {
				models++
				info, _ := os.Stat(filepath.Join(out, filepath.FromSlash(path)))
				mtime[path] = info.ModTime()
			} else if strings.HasSuffix(path, ".blp") {
				textures++
				bytes += int64(len(raw))
				if binary.LittleEndian.Uint32(raw[12:16]) != 512 {
					t.Fatal("source texture resolution changed")
				}
			}
		}
		t.Logf("pass %d: %d centered models, %d unique full-resolution textures, %.2f MiB", pass, models, textures, float64(bytes)/1048576)
		if models == 0 || models != len(e.MapManager.Doodads) {
			t.Fatalf("expected independently placed terrain parts, got %d models and %d placements", models, len(e.MapManager.Doodads))
		}
		if textures >= len(tiles)*256 {
			t.Fatalf("real repeating terrain was not deduplicated: %d", textures)
		}
		if before != nil {
			if len(before) != len(current) {
				t.Fatal("clamp changed asset set")
			}
			for path, hash := range before {
				if current[path] != hash {
					t.Fatalf("clamp changed asset %s", path)
				}
				if old, ok := timestamps[path]; ok && !mtime[path].Equal(old) {
					t.Fatalf("unchanged model rewritten: %s", path)
				}
			}
		}
		before, timestamps = current, mtime
	}
}
