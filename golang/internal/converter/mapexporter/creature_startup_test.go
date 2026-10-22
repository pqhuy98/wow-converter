package mapexporter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter/creature"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestFactionPlacementsSaveForBothUserSides(t *testing.T) {
	for _, side := range []string{"Horde", "Alliance"} {
		t.Run(side, func(t *testing.T) {
			m := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
			model := mdl.New(mdl.NewMDLOptions{Name: "terrain.mdx"})
			model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{{Position: [3]float64{}}, {Position: [3]float64{100, 100, 100}}}}}
			root := &common.WowObject{Type: common.WowObjectADT, ScaleFactor: 1, Model: &common.Model{MDL: model}}
			// A Stormwind bear is named; a humanoid Monster is generic. Creature type
			// must have no effect on faction-based ownership.
			for _, g := range []azerothcore.CreatureGameplay{{Type: 1, Faction: 12}, {Type: 7, Faction: 14}, {Type: 1, Faction: 32}, {Type: 1, Faction: 38}, {Type: 7, Faction: 35}} {
				root.Children = append(root.Children, &common.WowObject{Type: common.WowObjectUnit, ScaleFactor: 1, Position: [3]float64{50, 50, 50}, Creature: &azerothcore.Creature{Template: azerothcore.CreatureTemplate{Entry: 1, Name: "Fixture"}, Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 42, DisplayScale: 1}, Gameplay: &g}})
			}
			m.Roots = []*common.WowObject{root}
			mm := extra.NewMapManager()
			terrain := mapterrain.GetInitialTerrain(2, 2)
			for y := range terrain.GroundHeight {
				for x := range terrain.GroundHeight[y] {
					terrain.GroundHeight[y][x] = config.GameZToDataHeight(640)
				}
			}
			mm.SetTerrain(terrain)
			cfg := DefaultMapExportConfig()
			cfg.PlayerFaction = side
			cfg.Terrain.ClampPercent = TerrainClampPercent{Lower: .25, Upper: .75}
			units := creature.PlaceUnits(cfg, mm, m)
			if len(mm.Units) != 5 {
				t.Fatal("fixture NPCs not placed")
			}
			e := &MapExporter{MapExportConfig: &cfg, MapManager: mm}
			if plan, err := creature.ConvertFactionPlayers(e.MapExportConfig.PlayerFaction, e.MapManager, units); err != nil {
				t.Fatal(err)
			} else {
				e.factions = plan
			}
			want := []int{2, 24, 27, 24, 27}
			for i, u := range mm.Units {
				if u.Player != want[i] || units[i].Player == nil || *units[i].Player != want[i] {
					t.Fatalf("wrong owner %d: %d", i, u.Player)
				}
			}
			dir := t.TempDir()
			for range 2 {
				if err := e.SaveWar3mapFiles(context.Background(), dir, "factions"); err != nil {
					t.Fatal(err)
				}
				saved := extra.NewMapManager()
				if err := saved.Load(dir); err != nil {
					t.Fatal(err)
				}
				if len(saved.Info.Players) != 2 || saved.Info.Players[0].PlayerNum != e.factions.UserSlot || saved.Info.Players[0].Type != 1 || saved.Info.Players[1].Type != 2 {
					t.Fatalf("bad player settings: %+v", saved.Info.Players)
				}
				race := 2
				if side == "Alliance" {
					race = 1
				}
				if saved.Info.Players[0].Name != "Player ("+side+")" {
					t.Fatal("user name mismatch")
				}
				if saved.Info.Players[0].Race != race {
					t.Fatal("user race mismatch")
				}
				for i, owner := range want {
					if saved.MapData.Units[i].Player != owner {
						t.Fatal("DOO owner lost")
					}
				}
				starts := 0
				for _, u := range saved.MapData.Units {
					if u.Type == "sloc" {
						starts++
						found := false
						for _, player := range saved.Info.Players {
							if u.Player == player.PlayerNum && u.Position[0] == player.StartingPos.X && u.Position[1] == player.StartingPos.Y {
								found = true
							}
						}
						if !found {
							t.Fatal("start marker disagrees with player metadata")
						}
					}
				}
				if starts != len(saved.Info.Players) {
					t.Fatal("missing/duplicate player start")
				}
				for _, filename := range []string{"war3map.lua", "war3map.wct"} {
					raw, err := os.ReadFile(filepath.Join(dir, filename))
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Contains(raw, []byte("function WowConverterInitFactions()")) || !bytes.Contains(raw, []byte("    WowConverterInitFactions()")) || !bytes.Contains(raw, []byte(fmt.Sprintf("setAllianceState(player_%s_%d, player_Stormwind_2,", side, e.factions.UserSlot))) {
						t.Fatalf("%s missing persistent alliance setup", filename)
					}
				}
				raw, _ := os.ReadFile(filepath.Join(dir, "war3map.lua"))
				if !bytes.Contains(raw, []byte(fmt.Sprintf("SetPlayerController(Player(%d), MAP_CONTROL_USER)", e.factions.UserSlot))) {
					t.Fatal("wrong active user")
				}
			}
		})
	}
}

func TestIrrelevantFactionUsesHumanPlayerOne(t *testing.T) {
	for _, side := range []string{"Horde", "Alliance"} {
		t.Run(side, func(t *testing.T) {
			cfg := DefaultMapExportConfig()
			cfg.PlayerFaction = side
			mm := extra.NewMapManager()
			mm.SetTerrain(mapterrain.GetInitialTerrain(2, 2))
			// Reuse the exporter/output across modes to catch stale faction plans.
			plan, err := creature.MakeFactionPlan(side, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			e := &MapExporter{MapExportConfig: &cfg, MapManager: mm, factions: plan}
			dir := t.TempDir()
			for _, destructibles := range []bool{false, true} {
				cfg.Creatures.Enable = destructibles
				cfg.Creatures.AllAreDoodads = destructibles
				if err := e.SaveWar3mapFiles(context.Background(), dir, "generic-player"); err != nil {
					t.Fatal(err)
				}
				saved := extra.NewMapManager()
				if err := saved.Load(dir); err != nil {
					t.Fatal(err)
				}
				if len(saved.Info.Players) != 1 {
					t.Fatalf("unexpected players: %+v", saved.Info.Players)
				}
				user := saved.Info.Players[0]
				if user.PlayerNum != 0 || user.Name != "Player 1" || user.Race != 1 || user.Type != 1 {
					t.Fatalf("incorrect generic player: %+v", user)
				}
				raw, err := os.ReadFile(filepath.Join(dir, "war3map.lua"))
				if err != nil {
					t.Fatal(err)
				}
				script := string(raw)
				human := "SetPlayerRacePreference(Player(0), RACE_PREF_HUMAN)"
				if !strings.Contains(script, `SetPlayerName(Player(0), "Player 1")`) ||
					!strings.Contains(script, "SetPlayerController(Player(0), MAP_CONTROL_USER)") ||
					strings.LastIndex(script, human) <= strings.LastIndex(script, "SetPlayerRacePreference(Player(0), RACE_PREF_ORC)") ||
					strings.Contains(script, "SetPlayerController(Player(1), MAP_CONTROL_USER)") {
					t.Fatal("runtime player disagrees with generic Human slot 1")
				}
				starts := 0
				for _, u := range saved.MapData.Units {
					if u.Type == "sloc" {
						starts++
						if u.Player != 0 {
							t.Fatal("wrong start owner")
						}
					}
				}
				if starts != 1 || e.factions.UserSlot != 0 || e.factions.PlayerFaction != "" {
					t.Fatal("faction or start marker leaked into generic export")
				}
			}
		})
	}
}
