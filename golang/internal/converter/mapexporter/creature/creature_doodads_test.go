package creature

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	settings "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestCreatureDecorationModeUsesDestructiblesWithoutPathing(t *testing.T) {
	m := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	model := mdl.New(mdl.NewMDLOptions{Name: "terrain.mdx"})
	model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{{Position: [3]float64{0, 0, 0}}, {Position: [3]float64{100, 100, 100}}}}}
	root := &common.WowObject{Type: common.WowObjectADT, ScaleFactor: 1, Model: &common.Model{MDL: model}}
	for _, z := range []float64{0, 50, 100} {
		root.Children = append(root.Children, &common.WowObject{Type: common.WowObjectUnit, ScaleFactor: 1, Position: [3]float64{50, 50, z},
			Creature: &azerothcore.Creature{Template: azerothcore.CreatureTemplate{Entry: 1, Name: "Guard"},
				Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 42, DisplayScale: 1}}})
	}
	m.Roots = []*common.WowObject{root}
	mm := extra.NewMapManager()
	mm.SetTerrain(mapterrain.GetInitialTerrain(2, 2))
	cfg := settings.DefaultMapExportConfig()
	cfg.Creatures.AllAreDoodads = true
	PlaceUnits(cfg, mm, m)
	if len(mm.Units) != 0 || len(mm.DestructibleTypes) != 1 || len(mm.DoodadTypes) != 0 || len(mm.Doodads) != 3 {
		t.Fatal("decoration mode must share a destructible type across ground and airborne NPC placements")
	}
	fields := map[string]interface{}{}
	for _, field := range mm.DestructibleTypes[0].Data {
		fields[field.ID] = field.Value
	}
	if fields["bfil"] != "creature-42.mdx" || fields["bptx"] != "" || fields["bvar"] != 1 {
		t.Fatal("NPC destructibles need an exact model path and no pathing footprint")
	}
	if mm.Doodads[0].Position[2] == mm.Doodads[2].Position[2] {
		t.Fatal("NPC destructibles must preserve their actual source elevations")
	}
}

func TestCreatureUnitModeSkipsMisplacedSpawnsButRetainsModels(t *testing.T) {
	m := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	model := mdl.New(mdl.NewMDLOptions{Name: "terrain.mdx"})
	model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{{Position: [3]float64{0, 0, 0}}, {Position: [3]float64{100, 100, 100}}}}}
	root := &common.WowObject{Type: common.WowObjectADT, ScaleFactor: 1, Model: &common.Model{MDL: model}}
	// On-ground, off-ground, above clamp, below clamp. Every model remains needed.
	for i, z := range []float64{50, 60, 90, 10} {
		root.Children = append(root.Children, &common.WowObject{Type: common.WowObjectUnit, ScaleFactor: 1, Position: [3]float64{50, 50, z},
			Creature: &azerothcore.Creature{Template: azerothcore.CreatureTemplate{Entry: i + 1, Name: "Guard"}, Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: i + 42, DisplayScale: 1}}})
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
	cfg := settings.DefaultMapExportConfig()
	cfg.Terrain.ClampPercent = settings.TerrainClampPercent{Lower: .25, Upper: .75}
	assets := PlaceUnits(cfg, mm, m)
	if len(assets) != 4 || len(mm.Units) != 1 || len(mm.UnitTypes) != 1 || len(mm.Doodads) != 0 || len(mm.DoodadTypes) != 0 || len(mm.DestructibleTypes) != 0 {
		t.Fatalf("misplaced NPCs must only export assets: assets=%d units=%d scenery=%d", len(assets), len(mm.Units), len(mm.Doodads))
	}
	for i, asset := range assets {
		if asset.Creature.Model.CreatureDisplayID != i+42 {
			t.Fatal("skipped NPC model was lost")
		}
	}
}

func TestCreatureGroundToleranceDoesNotDependOnClamp(t *testing.T) {
	m := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	model := mdl.New(mdl.NewMDLOptions{Name: "terrain.mdx"})
	model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{{Position: [3]float64{0, 0, 0}}, {Position: [3]float64{10000, 10000, 10000}}}}}
	root := &common.WowObject{Type: common.WowObjectADT, ScaleFactor: 1, Model: &common.Model{MDL: model}}
	// +/-150 raw model units from the floor; the third NPC is genuinely airborne.
	for i, z := range []float64{2150, 1850, 2250} {
		root.Children = append(root.Children, &common.WowObject{Type: common.WowObjectUnit, ScaleFactor: 1, Position: [3]float64{5000, 5000, z},
			Creature: &azerothcore.Creature{Template: azerothcore.CreatureTemplate{Entry: i + 1, Name: "Guard"}, Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: i + 42, DisplayScale: 1}}})
	}
	m.Roots = []*common.WowObject{root}
	for _, tc := range []struct {
		lower, upper float64
		want         int
	}{{0, 1, 2}, {0, .768, 2}, {0, .5, 2}, {.1, .3, 2}, {0, .2, 1}, {.19, .21, 0}} {
		mm := extra.NewMapManager()
		terrain := mapterrain.GetInitialTerrain(2, 2)
		// The same source floor fitted to each selected native height range.
		floor := float64(config.DataHeightMin()) + float64(config.DataHeightMax()-config.DataHeightMin())*(.2-tc.lower)/(tc.upper-tc.lower)
		for y := range terrain.GroundHeight {
			for x := range terrain.GroundHeight[y] {
				terrain.GroundHeight[y][x] = int(floor)
			}
		}
		mm.SetTerrain(terrain)
		cfg := settings.DefaultMapExportConfig()
		cfg.UnitScale = 2
		cfg.Terrain.ClampPercent = settings.TerrainClampPercent{Lower: tc.lower, Upper: tc.upper}
		assets := PlaceUnits(cfg, mm, m)
		if len(mm.Units) != tc.want || len(assets) != 3 || len(mm.Doodads) != 0 || len(mm.DestructibleTypes) != 0 {
			t.Fatalf("clamp %g..%g: units=%d, want %d; assets=%d", tc.lower, tc.upper, len(mm.Units), tc.want, len(assets))
		}
	}
}

func TestCreatureMovementUsesTerrainScaleWithoutVisualEnlargement(t *testing.T) {
	m := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	model := mdl.New(mdl.NewMDLOptions{Name: "terrain.mdx"})
	// A 56-yard scene fitted to 2048 WC3 units gives 36.57 WC3 units/yard.
	model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{{Position: [3]float64{}}, {Position: [3]float64{3136, 3136, 100}}}}}
	root := &common.WowObject{Type: common.WowObjectADT, ScaleFactor: 1, Model: &common.Model{MDL: model}}
	for i, g := range []azerothcore.CreatureGameplay{
		{Ground: 1, RunSpeed: 8.0 / 7},
		{Ground: 1, RunSpeed: 1},
		{Ground: 1, RunSpeed: 5.0 / 7},
		{Flight: 1, FlightSpeed: 9.0 / 7},
		{Swim: 1, SwimSpeed: 1},
		{Ground: 1, Rooted: 1, RunSpeed: 1},
		{Ground: 1, RunSpeed: 3},
	} {
		root.Children = append(root.Children, &common.WowObject{Type: common.WowObjectUnit, ScaleFactor: 1, Position: [3]float64{1568, 1568, 50},
			Creature: &azerothcore.Creature{Template: azerothcore.CreatureTemplate{Entry: i + 1, Name: "Fixture"},
				Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: i + 1, DisplayScale: 1}, Gameplay: &g}})
	}
	m.Roots = []*common.WowObject{root}
	want := []int{293, 256, 183, 329, 173, 0, 522}
	for _, unitScale := range []float64{.5, 1, 2, 3} {
		mm := extra.NewMapManager()
		mm.SetTerrain(mapterrain.GetInitialTerrain(16, 16))
		cfg := settings.DefaultMapExportConfig()
		cfg.UnitScale = unitScale
		PlaceUnits(cfg, mm, m)
		if len(mm.UnitTypes) != len(want) {
			t.Fatalf("unit scale %g: got %d types", unitScale, len(mm.UnitTypes))
		}
		for i, ut := range mm.UnitTypes {
			f := fieldValues(t, ut.Data)
			if f["umvs"] != want[i] {
				t.Fatalf("unit scale %g, actor %d: movement=%v, want %d", unitScale, i, f["umvs"], want[i])
			}
			if f["usca"] != float32(2048.0/3136*unitScale) || f["ussc"] != float32(2*1.2*(2048.0/56)*unitScale/72) {
				t.Fatalf("unit scale %g: visual sizing changed: %v/%v", unitScale, f["usca"], f["ussc"])
			}
		}
	}
}
