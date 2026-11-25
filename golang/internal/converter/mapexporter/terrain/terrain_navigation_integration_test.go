//go:build integration_tests

package terrain_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/transport"
)

// Uses the existing data server without changing its loaded build.
// The generated map stays in maps/navigation-stormwind.w3x for WC3 verification.
func TestStormwindCollisionTerrain(t *testing.T) {
	if os.Getenv("WOW_DATA_SERVER_URL") == "" {
		transport.ConfigureBundled()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	c := client.NewHTTPClient("")
	info, err := c.GetCASCInfo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if product := os.Getenv("WOW_NAV_PRODUCT"); product != "" && info.Build.Product != product {
		t.Fatalf("loaded product %s does not match requested %s", info.Build.Product, product)
	}
	t.Logf("WoW build: %s (%s)", info.BuildName, info.Build.Product)
	cfg := config.DefaultConfig()
	cfg.IsBulkExport = true
	mapCfg := mapexporter.DefaultMapExportConfig()
	mapCfg.MapID = 0
	mapCfg.CascMapDir = "azeroth"
	mapCfg.WowExportFolder = "azeroth"
	mapCfg.Min = imath.Vector2{30, 48}
	mapCfg.Max = mapCfg.Min
	mapCfg.Creatures.Enable = true
	mapCfg.Creatures.AllAreDoodads = true
	mapCfg.UnitScale = 2
	mapCfg.PlacePathBlockers = true
	registry := common.NewTileRegistry()
	if err := mapexporter.LoadADTTilesForConversion(ctx, c, cfg.ExportAssetDir, mapCfg, 512, []mapexporter.TileCoord{{30, 48}}, true, registry, nil); err != nil {
		t.Fatal(err)
	}
	result, err := mapexporter.RunMapGenerateConversion(ctx, mapexporter.MapGenerateConversionOptions{
		Config: cfg, MapExportConfig: &mapCfg, MapSaveName: "navigation-stormwind.w3x",
		AutoClampPercent: true, UnitScale: 2, WowClient: c, TileRegistry: registry, TileQuality: 512,
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(result.OutputDir, "terrain-navigation.json"))
	if err != nil {
		t.Fatal(err)
	}
	var r terrain.TerrainNavigation
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if r.GroundNPCs == 0 || r.Reachable == 0 || r.BlockerInstances == 0 {
		t.Fatalf("Stormwind must contain grounded seeds, reachable roads and blockers: %s", b)
	}
	if r.BlockerInstances >= r.GroundBlocked+r.AirBlocked {
		t.Fatal("blockers were not packed")
	}
	if _, err := os.Stat(filepath.Join(result.OutputDir, "war3map.w3d")); err != nil {
		t.Fatal(err)
	}
	mapData := extra.NewMapManager()
	if err := mapData.Load(result.OutputDir); err != nil {
		t.Fatal(err)
	}
	wpm, err := os.ReadFile(filepath.Join(result.OutputDir, "war3map.wpm"))
	if err != nil {
		t.Fatal(err)
	}
	terrain.AssertSavedBlockers(t, result.OutputDir, mapData, wpm[16:], r.Width, r.Height)
	unitsJSON, err := os.ReadFile(filepath.Join(result.OutputDir, "units.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Units []struct {
			TypeCode string `json:"typeCode"`
			Model    string `json:"model"`
		} `json:"units"`
	}
	if err := json.Unmarshal(unitsJSON, &report); err != nil {
		t.Fatal(err)
	}
	type npcDestructible struct {
		model      string
		placements int
	}
	expected := map[string]npcDestructible{}
	for _, unit := range report.Units {
		if unit.TypeCode == "" {
			continue
		}
		if unit.Model == "" {
			t.Fatalf("NPC destructible %s has no exported model", unit.TypeCode)
		}
		npc := expected[unit.TypeCode]
		npc.model = unit.Model
		npc.placements++
		expected[unit.TypeCode] = npc
	}
	if len(expected) == 0 {
		t.Fatal("generated map must contain NPC destructibles")
	}
	savedTypes := map[string]bool{}
	for _, typ := range mapData.DestructibleTypes {
		npc, ok := expected[typ.Code]
		if !ok {
			continue
		}
		for _, field := range typ.Data {
			if field.ID != "bfil" {
				continue
			}
			if field.Value != strings.ReplaceAll(npc.model, "/", `\`) {
				t.Fatalf("NPC destructible %s uses model %v, want %s", typ.Code, field.Value, npc.model)
			}
			if _, err := os.Stat(filepath.Join(result.OutputDir, filepath.FromSlash(npc.model))); err != nil {
				t.Fatal(err)
			}
			savedTypes[typ.Code] = true
		}
	}
	placements := map[string]int{}
	for _, placed := range mapData.Doodads {
		placements[placed.Doodad.Type]++
	}
	npcs := 0
	for code, npc := range expected {
		if !savedTypes[code] {
			t.Fatalf("missing NPC destructible definition or model field for %s", code)
		}
		if got := placements[code]; got != npc.placements {
			t.Fatalf("NPC destructible %s has %d placements, want %d", code, got, npc.placements)
		}
		npcs += npc.placements
	}
	t.Logf("verified %d NPC destructible placements across %d types", npcs, len(expected))
	t.Logf("Stormwind map: %s\n%s", filepath.Join(workspace.FindRepoRoot(), "maps", "navigation-stormwind.w3x"), b)
}
