package creature

import (
	"fmt"
	"log"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	settings "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

// PlacedUnit retains a creature for asset export, including skipped placements.
type PlacedUnit struct {
	unitIndex      int
	Player         *int                 `json:"player,omitempty"`
	Creature       azerothcore.Creature `json:"source"`
	TypeCode       string               `json:"typeCode,omitempty"`
	ExcludedReason string               `json:"excludedReason,omitempty"`
	ModelPath      string               `json:"model,omitempty"`
	IconPath       string               `json:"icon,omitempty"`
}

// PlaceUnits adds unit instances to the map.
func PlaceUnits(mapConfig settings.MapExportConfig, mm *extra.MapManager, m *common.WowObjectManager) []PlacedUnit {
	terrain := mm.GetTerrain()
	roots := m.Roots
	var units []PlacedUnit

	mapMin := imath.Vector3{float64(terrain.Map.Offset.X), float64(terrain.Map.Offset.Y), config.DataHeightToGameZ(float64(config.DataHeightMin()))}
	mapMax := imath.Vector3{
		float64(terrain.Map.Offset.X) + float64(terrain.Map.Width)*config.DistancePerTile,
		float64(terrain.Map.Offset.Y) + float64(terrain.Map.Height)*config.DistancePerTile,
		config.DataHeightToGameZ(float64(config.DataHeightMax())),
	}
	mapSize := imath.V3Sub(mapMax, mapMin)
	ext := common.ComputeAbsoluteMinMaxExtents(roots)
	min, max := ext.Min, ext.Max
	modelSize := imath.V3Sub(max, min)
	scale := mapSize[0] / modelSize[0]
	clampDiff := mapConfig.Terrain.ClampPercent.Upper - mapConfig.Terrain.ClampPercent.Lower
	// The same source ground gap stays accepted when a manual clamp stretches Z.
	// At the recommended clamp this is exactly 100 WC3 units.
	groundTolerance := 100 * mapterrain.RecommendedTerrainClampWidth(min, max, mapConfig.UnitScale) / clampDiff

	templateIdToUnitType := map[string]*extra.UnitType{}
	templateIdToDoodadType := map[string]*extra.DoodadType{}
	type candidate struct {
		index       int
		position    [3]float32
		angle       float32
		model, name string
		modelScale  float64
	}
	var candidates []candidate

	m.IterateObjects(func(obj *common.WowObject, abs common.ObjectAbsolute) {
		c := common.ObjectCreature(obj)
		if !common.IsWowUnit(obj) || c == nil {
			return
		}
		units = append(units, PlacedUnit{Creature: *c, unitIndex: -1})
		index := len(units) - 1
		absPosition := abs.Position

		if absPosition[0] < min[0]-1 || absPosition[0] > max[0]+1 ||
			absPosition[1] < min[1]-1 || absPosition[1] > max[1]+1 {
			log.Printf("%s", ansi.Redf("Creature %s is out of bounds %v", c.Template.Name, absPosition))
			log.Printf("map[min:%v max:%v]", min, max)
			units[index].ExcludedReason = "outside map bounds"
			return
		}

		percent := imath.Vector3{
			(absPosition[0] - min[0]) / modelSize[0],
			(absPosition[1] - min[1]) / modelSize[1],
			(absPosition[2] - min[2]) / modelSize[2],
		}
		inGameX := mapMin[0] + percent[0]*mapSize[0]
		inGameY := mapMin[1] + percent[1]*mapSize[1]
		inGameZ := config.DataHeightToGameZ(float64(config.DataHeightMin()) +
			float64(config.DataHeightMax()-config.DataHeightMin())/clampDiff*(percent[2]-mapConfig.Terrain.ClampPercent.Lower))
		terrainZ := config.DataHeightToGameZ(float64(mapterrain.GetTerrainHeight(terrain, percent[0], percent[1])))

		creatureModel := fmt.Sprintf("creature-%d.mdx", c.Model.CreatureDisplayID)
		creatureName := c.Template.Name
		if creatureName == "" {
			creatureName = c.Template.SubName
		}
		creatureScale := scale * c.Model.DisplayScale * mapConfig.UnitScale
		position := [3]float32{float32(inGameX), float32(inGameY), float32(inGameZ)}

		withinPlayable := percent[2] >= mapConfig.Terrain.ClampPercent.Lower && percent[2] <= mapConfig.Terrain.ClampPercent.Upper
		notOnGround := inGameZ < terrainZ-groundTolerance || inGameZ > terrainZ+groundTolerance

		// Keep every creature in the returned asset-export list, but avoid
		// sharing unit models with scenery when a spawn cannot become a unit.
		if !mapConfig.Creatures.AllAreDoodads && (!withinPlayable || notOnGround) {
			units[index].ExcludedReason = "outside playable height or too far from ground"
			return
		}
		if mapConfig.Creatures.AllAreDoodads {
			key := creatureUnitKey(*c)
			if _, ok := templateIdToDoodadType[key]; !ok {
				mods := []data.Modification{
					{ID: "bnam", Type: data.ModificationString, Value: "~U " + creatureName},
					{ID: "bfil", Type: data.ModificationString, Value: creatureModel},
					{ID: "bmas", Type: data.ModificationUnreal, Value: float32(creatureScale * 1.5)},
					{ID: "bmis", Type: data.ModificationUnreal, Value: float32(creatureScale / 1.5)},
					{ID: "bptx", Type: data.ModificationString, Value: ""},
					{ID: "bvar", Type: data.ModificationInt, Value: 1},
				}
				templateIdToDoodadType[key] = mm.AddDoodadType(mods, true)
			}
			dt := templateIdToDoodadType[key]
			units[index].TypeCode = dt.Code
			mm.AddDoodad(dt, data.Doodad{
				Variation:        0,
				Position:         position,
				Angle:            wc3.Angle(imath.Degrees(abs.Rotation[2])),
				Scale:            [3]float32{float32(creatureScale), float32(creatureScale), float32(creatureScale)},
				SkinID:           dt.Code,
				Flags:            data.DoodadFlag{Visible: true, Solid: true, CustomHeight: true},
				Life:             100,
				RandomItemSetPtr: -1,
			})
			return
		}

		candidates = append(candidates, candidate{index, position, float32(abs.Rotation[2]), creatureModel, creatureName, creatureScale})
	})
	for _, p := range candidates {
		c := &units[p.index].Creature
		key := creatureUnitKey(*c)
		if _, ok := templateIdToUnitType[key]; !ok {
			yardScale := scale * m.RawModelScale()
			mods := creatureUnitFields(*c, p.model, p.name, p.modelScale, yardScale*mapConfig.UnitScale, yardScale)
			mods = append(mods, data.Modification{ID: "unsf", Type: data.ModificationString, Value: fmt.Sprintf("template.entry=%d displayId=%d", c.Template.Entry, c.Model.CreatureDisplayID)})
			templateIdToUnitType[key] = mm.AddUnitType(false, "hfoo", mods)
		}
		ut := templateIdToUnitType[key]
		units[p.index].TypeCode = ut.Code
		units[p.index].unitIndex = len(mm.Units)
		mm.AddUnit(ut, data.Unit{
			Variation:         0,
			Position:          p.position,
			Rotation:          p.angle,
			Scale:             [3]float32{1, 1, 1},
			Skin:              ut.Code,
			Player:            0, // Assigned after all eligible faction templates are known.
			Hitpoints:         100,
			Mana:              -1,
			TargetAcquisition: -1,
			RandomItemSetPtr:  -1,
			Hero:              data.UnitHero{Level: 1, Str: 0, Agi: 0, Int: 0},
			Random:            data.UnitRandom{Type: 0},
			Color:             23,
			Waygate:           -1,
		})
	}
	return units
}
