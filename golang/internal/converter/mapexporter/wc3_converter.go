package mapexporter

import (
	"fmt"
	"log"
	"math"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

const (
	terrainFlagUnwalkable  = 0x0002
	terrainFlagUnflyable   = 0x0004
	terrainFlagUnbuildable = 0x0008
)

// PlacedUnit ties a creature to map placement.
type PlacedUnit struct {
	Creature azerothcore.Creature
}

// Wc3Converter converts WoW objects to WC3 map data.
type Wc3Converter struct {
	config MapExportConfig
}

// NewWc3Converter creates a converter.
func NewWc3Converter(cfg MapExportConfig) *Wc3Converter {
	return &Wc3Converter{config: cfg}
}

// GenerateTerrainWithHeight builds war3map.w3e terrain from ADT roots.
func (w *Wc3Converter) GenerateTerrainWithHeight(m *common.WowObjectManager) data.Terrain {
	roots := m.Roots
	log.Printf("Generating terrain from %d objects", len(roots))

	heightMap, height, width := computeTerrainHeightMap(roots, w.config)
	log.Printf("Map size height=%d width=%d", height, width)
	if width > 480 || height > 480 {
		panic("Map size is too large!")
	}

	terrain := getInitialTerrain(height, width)
	for i := 0; i < len(heightMap); i++ {
		for j := 0; j < len(heightMap[i]); j++ {
			if heightMap[i][j] == -1 {
				continue
			}
			terrain.GroundHeight[i][j] = int(math.Ceil(heightMap[i][j]*float64(config.DataHeightMax()-config.DataHeightMin()) + float64(config.DataHeightMin())))
		}
	}
	for i := range terrain.GroundHeight {
		for j := range terrain.GroundHeight[i] {
			if terrain.GroundHeight[i][j] >= config.DataHeightMax() {
				terrain.Flags[i][j] |= terrainFlagUnflyable | terrainFlagUnwalkable | terrainFlagUnbuildable
			}
		}
	}
	return terrain
}

// PlaceDoodads adds doodad instances to the map.
func (w *Wc3Converter) PlaceDoodads(mm *extra.MapManager, m *common.WowObjectManager, filter func(*common.WowObject) bool) (doodadTypesWithPitchRoll int, err error) {
	terrain := mm.GetTerrain()
	log.Printf("Placing doodads")
	roots := m.Roots

	ext := common.ComputeAbsoluteMinMaxExtents(roots)
	min, max := ext.Min, ext.Max
	mapMin := imath.Vector3{float64(terrain.Map.Offset.X), float64(terrain.Map.Offset.Y), config.DataHeightToGameZ(float64(config.DataHeightMin()))}
	mapMax := imath.Vector3{
		float64(terrain.Map.Offset.X) + float64(terrain.Map.Width)*config.DistancePerTile,
		float64(terrain.Map.Offset.Y) + float64(terrain.Map.Height)*config.DistancePerTile,
		config.DataHeightToGameZ(float64(config.DataHeightMax())),
	}
	mapSize := imath.V3Sub(mapMax, mapMin)
	modelSize := imath.V3Sub(max, min)
	clampDiff := w.config.Terrain.ClampPercent.Upper - w.config.Terrain.ClampPercent.Lower
	rootScale := imath.Vector3{
		mapSize[0] / modelSize[0],
		mapSize[1] / modelSize[1],
		mapSize[2] / (modelSize[2] * clampDiff),
	}
	log.Printf("rootScale=%v", rootScale)

	modelPathToDoodadType := map[string]*extra.DoodadType{}
	doodadsPlaced := 0
	doodadsOutOfBounds := 0

	m.IterateObjects(func(obj *common.WowObject, abs common.ObjectAbsolute) {
		if !filter(obj) {
			return
		}
		doodadsPlaced++
		wc3Roll := modNegPi(((-abs.Rotation[0]) - math.Floor((-abs.Rotation[0])/(2*math.Pi))*(2*math.Pi)) - 2*math.Pi)
		wc3Pitch := modNegPi(((-abs.Rotation[1]) - math.Floor((-abs.Rotation[1])/(2*math.Pi))*(2*math.Pi)) - 2*math.Pi)
		hasRollPitch := math.Abs(wc3Roll) > w.config.Doodads.PitchRollThresholdRadians &&
			math.Abs(wc3Pitch) > w.config.Doodads.PitchRollThresholdRadians

		if obj.Model == nil {
			log.Printf("%s", ansi.Redf("Doodad has no model %v", obj))
			panic("Doodad has no model")
		}
		fileName := obj.Model.RelativePath
		hashKey := fileName
		if hasRollPitch {
			hashKey = fmt.Sprintf("%s;%.2f;%.2f", fileName, abs.Rotation[0], abs.Rotation[1])
		}

		percent := imath.Vector3{
			(abs.Position[0] - min[0]) / modelSize[0],
			(abs.Position[1] - min[1]) / modelSize[1],
			(abs.Position[2] - min[2]) / modelSize[2],
		}
		inGameX := mapMin[0] + percent[0]*mapSize[0]
		inGameY := mapMin[1] + percent[1]*mapSize[1]
		zDiff := float64(config.DataHeightMax()-config.DataHeightMin()) * (percent[2] - w.config.Terrain.ClampPercent.Lower) / clampDiff
		inGameZ := config.DataHeightToGameZ(float64(config.DataHeightMin()) + zDiff)

		if inGameX < mapMin[0] || inGameX > mapMax[0] || inGameY < mapMin[1] || inGameY > mapMax[1] {
			doodadsOutOfBounds++
			return
		}

		if _, ok := modelPathToDoodadType[hashKey]; !ok {
			dt := mm.AddDoodadType(nil, false)
			doodadName := fmt.Sprintf("~D %s -- %s -- %s", filepath.Base(obj.Model.RelativePath), obj.Type, dt.Code)
			dt.Data = append(dt.Data,
				data.Modification{ID: "dfil", Type: data.ModificationString, Level: 0, Column: 0, Value: fileName},
				data.Modification{ID: "dnam", Type: data.ModificationString, Level: 0, Column: 0, Value: doodadName},
				data.Modification{ID: "dmas", Type: data.ModificationUnreal, Value: float32(abs.ScaleFactor * maxFloat(rootScale[0], rootScale[1], rootScale[2]) * 1.5)},
				data.Modification{ID: "dmis", Type: data.ModificationUnreal, Value: float32(abs.ScaleFactor * minFloat(rootScale[0], rootScale[1], rootScale[2]) / 1.5)},
				data.Modification{ID: "danf", Type: data.ModificationInt, Level: 0, Column: 0, Value: 1},
				data.Modification{ID: "dshf", Type: data.ModificationInt, Level: 0, Column: 0, Value: 1},
			)
			if hasRollPitch {
				dt.Data = append(dt.Data,
					data.Modification{ID: "dmar", Type: data.ModificationUnreal, Level: 0, Column: 0, Value: float32(wc3Roll)},
					data.Modification{ID: "dmap", Type: data.ModificationUnreal, Level: 0, Column: 0, Value: float32(wc3Pitch)},
				)
				doodadTypesWithPitchRoll++
			}
			modelPathToDoodadType[hashKey] = dt
		}
		dt := modelPathToDoodadType[hashKey]
		id4 := dt.Code
		if len(id4) > 4 {
			id4 = id4[:4]
		}
		mm.AddDoodad(dt, data.Doodad{
			Variation: 0,
			Position:  [3]float32{float32(inGameX), float32(inGameY), float32(inGameZ)},
			Angle:     wc3.Angle(imath.Degrees(abs.Rotation[2])),
			Scale:     [3]float32{float32(abs.ScaleFactor * rootScale[0]), float32(abs.ScaleFactor * rootScale[1]), float32(abs.ScaleFactor * rootScale[2])},
			SkinID:    id4,
			Flags:     data.DoodadFlag{Visible: true, Solid: true, CustomHeight: true},
			Life:      100,
			RandomItemSetPtr: -1,
		})
	})

	if doodadsOutOfBounds > 0 {
		log.Printf("%d/%d objects are outside of map bounds.", doodadsOutOfBounds, doodadsPlaced)
	}
	return doodadTypesWithPitchRoll, nil
}

// PlaceUnits adds unit instances to the map.
func (w *Wc3Converter) PlaceUnits(mm *extra.MapManager, m *common.WowObjectManager) []PlacedUnit {
	mapConfig := w.config
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

	templateIdToUnitType := map[int]*extra.UnitType{}
	templateIdToDoodadType := map[int]*extra.DoodadType{}

	m.IterateObjects(func(obj *common.WowObject, abs common.ObjectAbsolute) {
		c := common.ObjectCreature(obj)
		if !common.IsWowUnit(obj) || c == nil {
			return
		}
		units = append(units, PlacedUnit{Creature: *c})
		absPosition := abs.Position

		if absPosition[0] < min[0]-1 || absPosition[0] > max[0]+1 ||
			absPosition[1] < min[1]-1 || absPosition[1] > max[1]+1 {
			log.Printf("%s", ansi.Redf("Creature %s is out of bounds %v", c.Template.Name, absPosition))
			log.Printf("map[min:%v max:%v]", min, max)
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
		terrainZ := config.DataHeightToGameZ(float64(GetTerrainHeight(terrain, percent[0], percent[1])))

		creatureModel := fmt.Sprintf("creature-%d.mdx", c.Model.CreatureDisplayID)
		creatureName := c.Template.Name
		if creatureName == "" {
			creatureName = c.Template.SubName
		}
		creatureScale := scale * c.Model.DisplayScale * mapConfig.UnitScale
		position := [3]float32{float32(inGameX), float32(inGameY), float32(inGameZ)}

		withinPlayable := percent[2] >= mapConfig.Terrain.ClampPercent.Lower && percent[2] <= mapConfig.Terrain.ClampPercent.Upper
		notOnGround := inGameZ < terrainZ-100 || inGameZ > terrainZ+100

		if mapConfig.Creatures.AllAreDoodads || !withinPlayable || notOnGround {
			if _, ok := templateIdToDoodadType[c.Template.Entry]; !ok {
				templateIdToDoodadType[c.Template.Entry] = mm.AddDoodadType([]data.Modification{
					{ID: "bnam", Type: data.ModificationString, Value: "~U " + creatureName},
					{ID: "bfil", Type: data.ModificationString, Value: creatureModel},
					{ID: "bmas", Type: data.ModificationUnreal, Value: float32(creatureScale * 1.5)},
					{ID: "bmis", Type: data.ModificationUnreal, Value: float32(creatureScale / 1.5)},
				}, true)
			}
			dt := templateIdToDoodadType[c.Template.Entry]
			mm.AddDoodad(dt, data.Doodad{
				Variation: 0,
				Position:  position,
				Angle:     wc3.Angle(imath.Degrees(abs.Rotation[2])),
				Scale:     [3]float32{float32(creatureScale), float32(creatureScale), float32(creatureScale)},
				SkinID:    dt.Code,
				Flags:     data.DoodadFlag{Visible: true, Solid: true, CustomHeight: true},
				Life:      100,
				RandomItemSetPtr: -1,
			})
			return
		}

		if _, ok := templateIdToUnitType[c.Template.Entry]; !ok {
			templateIdToUnitType[c.Template.Entry] = mm.AddUnitType(false, "hfoo", []data.Modification{
				{ID: "unam", Type: data.ModificationString, Value: creatureName},
				{ID: "unsf", Type: data.ModificationString, Value: fmt.Sprintf("guid=%d template.entry=%d displayId=%d phaseMask=%d",
					c.Creature.GUID, c.Template.Entry, c.Model.CreatureDisplayID, c.Creature.PhaseMask)},
				{ID: "umdl", Type: data.ModificationString, Value: creatureModel},
				{ID: "uabi", Type: data.ModificationString, Value: ""},
				{ID: "usca", Type: data.ModificationReal, Value: float32(creatureScale)},
				{ID: "uhpm", Type: data.ModificationInt, Value: c.Creature.CurHealth},
				{ID: "umpm", Type: data.ModificationInt, Value: c.Creature.CurMana},
				{ID: "umpi", Type: data.ModificationInt, Value: c.Creature.CurMana},
				{ID: "ulev", Type: data.ModificationInt, Value: c.Template.MaxLevel},
			})
		}
		ut := templateIdToUnitType[c.Template.Entry]
		mm.AddUnit(ut, data.Unit{
			Variation: 0,
			Position:  position,
			Rotation:  float32(abs.Rotation[2]),
			Scale:     [3]float32{1, 1, 1},
			Skin:      ut.Code,
			Player:    0,
			Hitpoints: 100,
			Mana:      0,
			RandomItemSetPtr: -1,
			Hero:      data.UnitHero{Level: c.Template.MaxLevel, Str: 0, Agi: 0, Int: 0},
			Random:    data.UnitRandom{Type: 0},
			Color:     23,
			Waygate:   -1,
		})
	})
	return units
}

// GetTerrainHeight bilinearly samples terrain height at normalized coordinates.
func GetTerrainHeight(terrain data.Terrain, percentX, percentY float64) float64 {
	u := math.Min(1, math.Max(0, percentX))
	v := math.Min(1, math.Max(0, percentY))
	gridHeight := len(terrain.GroundHeight) - 1
	gridWidth := len(terrain.GroundHeight[0]) - 1
	x := u * float64(gridWidth)
	y := v * float64(gridHeight)
	x0 := int(math.Floor(x))
	x1 := int(math.Min(float64(gridWidth), math.Ceil(x)))
	y0 := int(math.Floor(y))
	y1 := int(math.Min(float64(gridHeight), math.Ceil(y)))
	wx1 := x - float64(x0)
	wx0 := 1 - wx1
	wy1 := y - float64(y0)
	wy0 := 1 - wy1
	h00 := float64(terrain.GroundHeight[y0][x0])
	h10 := float64(terrain.GroundHeight[y0][x1])
	h01 := float64(terrain.GroundHeight[y1][x0])
	h11 := float64(terrain.GroundHeight[y1][x1])
	return h00*wx0*wy0 + h10*wx1*wy0 + h01*wx0*wy1 + h11*wx1*wy1
}

// ComputeRecommendedTerrainClampPercent returns ratio and bounds from roots.
func ComputeRecommendedTerrainClampPercent(roots []*common.WowObject) (ratio float64, min, max imath.Vector3) {
	ext := common.ComputeAbsoluteMinMaxExtents(roots)
	size := imath.V3Sub(ext.Max, ext.Min)
	ratio = (config.DataHeightToGameZ(float64(config.DataHeightMax())) - config.DataHeightToGameZ(float64(config.DataHeightMin()))) / size[2]
	return ratio, ext.Min, ext.Max
}

func computeTerrainHeightMap(roots []*common.WowObject, cfg MapExportConfig) ([][]float64, int, int) {
	if len(roots) == 0 {
		heightMap := make2DFloat64(64+1, 64+1, -1)
		return heightMap, 64, 64
	}
	log.Printf("Computing terrain height map...")
	ext := common.ComputeAbsoluteMinMaxExtents(roots)
	min, max := ext.Min, ext.Max
	log.Printf("map[min:%v max:%v]", min, max)
	terrainSize := imath.V3Sub(max, min)
	log.Printf("map[terrainSize:%v]", terrainSize)
	recommendedClamp := (config.DataHeightToGameZ(float64(config.DataHeightMax())) - config.DataHeightToGameZ(float64(config.DataHeightMin()))) / terrainSize[2]
	log.Printf("Recommended terrain clamp percent difference %v", recommendedClamp)
	clampDiff := cfg.Terrain.ClampPercent.Upper - cfg.Terrain.ClampPercent.Lower
	ratioZ := config.MaxGameHeightDiff() / (terrainSize[2] * clampDiff)
	ratioXY := ratioZ
	width := int(math.Ceil(terrainSize[0]/config.DistancePerTile*ratioXY/4)) * 4
	height := int(math.Ceil(terrainSize[1]/config.DistancePerTile*ratioXY/4)) * 4
	log.Printf("map[ratio:%v height:%d width:%d]", ratioZ, height, width)
	heightMap := make2DFloat64(height+1, width+1, -1)
	random := imath.SeededRandom(fmt.Sprintf("terrain-height-map:%dx%d", width, height))

	for _, root := range roots {
		if root.Model == nil {
			continue
		}
		for _, geoset := range root.Model.MDL.Geosets {
			for _, v := range geoset.Vertices {
				rotatedV := imath.V3Rotate(v.Position, root.Rotation)
				position := imath.V3Sum(root.Position, rotatedV)
				percent := imath.Vector3{
					(position[0] - min[0]) / terrainSize[0],
					(position[1] - min[1]) / terrainSize[1],
					(position[2] - (min[2] + terrainSize[2]*cfg.Terrain.ClampPercent.Lower)) / (terrainSize[2] * clampDiff),
				}
				if percent[0] < 0 || percent[0] > 1 || percent[1] < 0 || percent[1] > 1 {
					panic("Out of bounds")
				}
				var iX, iY int
				if random() > 0.5 {
					iX = int(math.Round(percent[0] * float64(width)))
					iY = int(math.Round(percent[1] * float64(height)))
				} else {
					iX = int(math.Floor(percent[0] * float64(width)))
					iY = int(math.Floor(percent[1] * float64(height)))
				}
				z := math.Max(0, math.Min(1, percent[2]))
				if heightMap[iY][iX] < z {
					heightMap[iY][iX] = z
				}
			}
		}
	}

	floodBrushSize := 5
	for k := floodBrushSize*2 + 1; k >= 1; k-- {
		k := k
		for i := range heightMap {
			for j := range heightMap[i] {
				if heightMap[i][j] != -1 {
					continue
				}
				sum, cnt := 0.0, 0
				for i2 := maxInt(0, i-floodBrushSize); i2 <= minInt(len(heightMap)-1, i+floodBrushSize); i2++ {
					for j2 := maxInt(0, j-floodBrushSize); j2 < minInt(len(heightMap[i])-1, j+floodBrushSize); j2++ {
						if heightMap[i2][j2] >= 0 {
							sum += heightMap[i2][j2]
							cnt++
						}
					}
				}
				if cnt >= k {
					heightMap[i][j] = sum / float64(cnt)
				}
			}
		}
	}
	return heightMap, height, width
}

func getInitialTerrain(height, width int) data.Terrain {
	defaultHeight := (config.DataHeightMin() + config.DataHeightMax()) >> 1
	waterHeight := config.DataHeightMin() + 728/4
	terrain := data.Terrain{
		Tileset:          "L",
		CustomTileset:    true,
		TilePalette:      []string{"Ldrt", "Ldro", "Ldrg", "Lrok", "Lgrs", "Lgrd"},
		CliffTilePalette: []string{"CLdi", "CLgr"},
		Map: data.MapSize{
			Width:  width,
			Height: height,
			Offset: data.Offset{
				X: float32(-config.DistancePerTile / 2 * float64(width)),
				Y: float32(-config.DistancePerTile / 2 * float64(height)),
			},
		},
	}
	terrain.GroundHeight = make2DInt(height+1, width+1, defaultHeight)
	terrain.WaterHeight = make2DInt(height+1, width+1, waterHeight)
	terrain.BoundaryFlag = make2DBool(height+1, width+1, false)
	terrain.Flags = make2DUint16(height+1, width+1, 0)
	terrain.GroundTexture = make2DInt(height+1, width+1, 0)
	terrain.GroundVariation = make2DInt(height+1, width+1, 0)
	terrain.CliffVariation = make2DInt(height+1, width+1, 0)
	terrain.CliffTexture = make2DInt(height+1, width+1, 240)
	terrain.LayerHeight = make2DInt(height+1, width+1, config.DefaultLayer)
	return terrain
}

func make2DInt(h, w, fill int) [][]int {
	out := make([][]int, h)
	for i := range out {
		out[i] = make([]int, w)
		for j := range out[i] {
			out[i][j] = fill
		}
	}
	return out
}

func make2DFloat64(h, w int, fill float64) [][]float64 {
	out := make([][]float64, h)
	for i := range out {
		out[i] = make([]float64, w)
		for j := range out[i] {
			out[i][j] = fill
		}
	}
	return out
}

func make2DBool(h, w int, fill bool) [][]bool {
	out := make([][]bool, h)
	for i := range out {
		out[i] = make([]bool, w)
		for j := range out[i] {
			out[i][j] = fill
		}
	}
	return out
}

func make2DUint16(h, w int, fill uint16) [][]uint16 {
	out := make([][]uint16, h)
	for i := range out {
		out[i] = make([]uint16, w)
		for j := range out[i] {
			out[i][j] = fill
		}
	}
	return out
}

func modNegPi(v float64) float64 {
	v = math.Mod(v, 2*math.Pi)
	if v > 0 {
		v -= 2 * math.Pi
	}
	return v
}

func minFloat(a float64, rest ...float64) float64 {
	m := a
	for _, v := range rest {
		if v < m {
			m = v
		}
	}
	return m
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// suppress unused import
var _ = strings.TrimSpace
