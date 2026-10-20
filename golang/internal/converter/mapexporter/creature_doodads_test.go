package mapexporter

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestCreatureDecorationModeUsesDoodadsWithoutPathing(t *testing.T) {
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
	mm.SetTerrain(getInitialTerrain(2, 2))
	cfg := DefaultMapExportConfig()
	cfg.Creatures.AllAreDoodads = true
	NewWc3Converter(cfg).PlaceUnits(mm, m)
	if len(mm.Units) != 0 || len(mm.DestructibleTypes) != 0 || len(mm.DoodadTypes) != 1 || len(mm.Doodads) != 3 {
		t.Fatal("decoration mode must share a real doodad type across ground and airborne NPC placements")
	}
	fields := map[string]interface{}{}
	for _, field := range mm.DoodadTypes[0].Data {
		fields[field.ID] = field.Value
	}
	if fields["dfil"] != "creature-42.mdx" || fields["dptx"] != "" || fields["dvar"] != 1 {
		t.Fatal("NPC doodads need an exact model path and no pathing footprint")
	}
	if mm.Doodads[0].Position[2] == mm.Doodads[2].Position[2] {
		t.Fatal("NPC doodads must preserve their actual source elevations")
	}
}
