package mapexporter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	creature "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/creature"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestPlayerStartMarkerSurvivesExportAndRepeatedSave(t *testing.T) {
	cfg := DefaultMapExportConfig()
	mm := extra.NewMapManager()
	terrain := mapterrain.GetInitialTerrain(16, 32)
	terrain.Map.Offset.X += 1000
	terrain.Map.Offset.Y -= 2000
	mm.SetTerrain(terrain)
	npc := mm.AddUnitType(false, "hfoo", nil)
	mm.AddUnit(npc, data.Unit{Player: 0, Position: [3]float32{40, 50, 60}, Scale: [3]float32{1, 1, 1}, Color: -1})
	exporter := &MapExporter{MapExportConfig: &cfg, MapManager: mm}
	root := t.TempDir()
	for pass := 0; pass < 2; pass++ {
		if err := exporter.SaveWar3mapFiles(context.Background(), root, "start-marker"); err != nil {
			t.Fatal(err)
		}
		loaded := extra.NewMapManager()
		if err := loaded.Load(root); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, u := range loaded.MapData.Units {
			if u.Type == "sloc" && u.Player == exporter.factions.UserSlot {
				count++
				if u.Position[0] != 1000 || u.Position[1] != -2000 || u.Skin != "sloc" {
					t.Fatalf("incorrect saved marker: %+v", u)
				}
			}
		}
		if count != 1 || len(loaded.MapData.Units) != 2 {
			t.Fatalf("save %d: %d starts, %d placements", pass, count, len(loaded.MapData.Units))
		}
		start := loaded.Info.Players[0].StartingPos
		if start.X != 1000 || start.Y != -2000 || !start.Fixed {
			t.Fatalf("map metadata disagrees: %+v", start)
		}
		script, err := os.ReadFile(filepath.Join(root, "war3map.lua"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(script), fmt.Sprintf("DefineStartLocation(0, %.9g, %.9g)", start.X, start.Y)) {
			t.Fatal("Lua start differs from editor marker")
		}
		if strings.Contains(string(script), "FourCC(\"sloc\")") || strings.Count(string(script), "u = BlzCreateUnitWithSkin(") != 1 {
			t.Fatal("start marker created as a gameplay unit or NPC placement changed")
		}
	}
}

func TestExistingPlayerStartIsPreservedAndLoadedPlayersUpdated(t *testing.T) {
	mm := extra.NewMapManager()
	mm.SetTerrain(mapterrain.GetInitialTerrain(16, 32))
	mm.EnsureMapInfo("existing")
	mm.Players = append([]data.Player(nil), mm.Info.Players...)
	mm.Units = append(mm.Units, extra.UnitInstance{Type: "sloc", Unit: data.Unit{Type: "sloc", Player: 0, Position: [3]float32{123, 456, 7}}})
	mm.Units = append(mm.Units, extra.UnitInstance{Type: "sloc", Unit: data.Unit{Type: "sloc", Player: 1, Position: [3]float32{700, 800, 9}}})
	ensurePlayerStartLocation(mm, 0)
	ensurePlayerStartLocation(mm, 0)
	if len(mm.Units) != 2 || mm.Units[0].Position != [3]float32{123, 456, 7} || mm.Units[1].Position != [3]float32{700, 800, 9} {
		t.Fatal("existing starts moved or duplicated")
	}
	if mm.Players[0].StartingPos.X != 123 || mm.Info.Players[0].StartingPos.Y != 456 {
		t.Fatal("loaded player list would overwrite synchronized position")
	}
}

func TestAllFactionStartMarkersAreSpreadAndSynchronized(t *testing.T) {
	cfg := DefaultMapExportConfig()
	mm := extra.NewMapManager()
	terrain := mapterrain.GetInitialTerrain(32, 32)
	terrain.Map.Offset.X += 1000
	terrain.Map.Offset.Y -= 2000
	mm.SetTerrain(terrain)
	plan := &creature.FactionPlan{UserSlot: 1}
	for slot := 0; slot < 24; slot++ {
		plan.Players = append(plan.Players, creature.FactionPlayer{Slot: slot, Name: fmt.Sprintf("Player %d", slot), FactionName: "Fixture"})
		plan.Reactions = append(plan.Reactions, make([]creature.FactionReaction, 24))
	}
	exporter := &MapExporter{MapExportConfig: &cfg, MapManager: mm, factions: plan}
	dir := t.TempDir()
	for range 2 {
		if err := exporter.SaveWar3mapFiles(context.Background(), dir, "all-starts"); err != nil {
			t.Fatal(err)
		}
		loaded := extra.NewMapManager()
		if err := loaded.Load(dir); err != nil {
			t.Fatal(err)
		}
		if len(loaded.MapData.Units) != 24 {
			t.Fatalf("want one marker per player, got %d", len(loaded.MapData.Units))
		}
		script, err := os.ReadFile(filepath.Join(dir, "war3map.lua"))
		if err != nil {
			t.Fatal(err)
		}
		seen := map[int]bool{}
		for _, marker := range loaded.MapData.Units {
			if marker.Type != "sloc" || marker.Skin != "sloc" || seen[marker.Player] {
				t.Fatal("wrong or duplicate start marker")
			}
			seen[marker.Player] = true
			start := loaded.Info.Players[marker.Player].StartingPos
			if !start.Fixed || start.X != marker.Position[0] || start.Y != marker.Position[1] {
				t.Fatal("marker and W3I disagree")
			}
			if marker.Player == plan.UserSlot && (start.X != 1000 || start.Y != -2000) {
				t.Fatal("user start moved away from map center")
			}
			dx, dy := start.X-1000, start.Y+2000
			if dx < -400 || dx > 400 || dy < -400 || dy > 400 {
				t.Fatal("start marker too far from map center")
			}
			if !strings.Contains(string(script), fmt.Sprintf("DefineStartLocation(%d, %.9g, %.9g)", marker.Player, start.X, start.Y)) {
				t.Fatal("Lua start differs from editor marker")
			}
			for _, other := range loaded.MapData.Units {
				if marker.Player == other.Player {
					continue
				}
				dx, dy := marker.Position[0]-other.Position[0], marker.Position[1]-other.Position[1]
				if dx*dx+dy*dy < 200*200 {
					t.Fatal("start markers less than 200 units apart")
				}
			}
		}
	}
}
