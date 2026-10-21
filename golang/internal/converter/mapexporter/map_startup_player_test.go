package mapexporter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestPlayerStartMarkerSurvivesExportAndRepeatedSave(t *testing.T) {
	cfg := DefaultMapExportConfig()
	mm := extra.NewMapManager()
	terrain := getInitialTerrain(16, 32)
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
			if u.Type == "sloc" && u.Player == 0 {
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
	mm.SetTerrain(getInitialTerrain(16, 32))
	mm.EnsureMapInfo("existing")
	mm.Players = append([]data.Player(nil), mm.Info.Players...)
	mm.Units = append(mm.Units, extra.UnitInstance{Type: "sloc", Unit: data.Unit{Type: "sloc", Player: 0, Position: [3]float32{123, 456, 7}}})
	mm.Units = append(mm.Units, extra.UnitInstance{Type: "sloc", Unit: data.Unit{Type: "sloc", Player: 1, Position: [3]float32{700, 800, 9}}})
	ensurePlayerStartLocation(mm)
	ensurePlayerStartLocation(mm)
	if len(mm.Units) != 2 || mm.Units[0].Position != [3]float32{123, 456, 7} || mm.Units[1].Position != [3]float32{700, 800, 9} {
		t.Fatal("existing starts moved or duplicated")
	}
	if mm.Players[0].StartingPos.X != 123 || mm.Info.Players[0].StartingPos.Y != 456 {
		t.Fatal("loaded player list would overwrite synchronized position")
	}
}
