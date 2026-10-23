package terrain

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
	settings "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	"github.com/pqhuy98/wow-converter/internal/converter/terrainfit"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

const (
	maxTerrainTiles        = 480
	terrainFlagUnwalkable  = 0x0002
	terrainFlagUnflyable   = 0x0004
	terrainFlagUnbuildable = 0x0008
)

// Wc3Converter converts WoW objects to WC3 map data.
type Wc3Converter struct {
	config           settings.MapExportConfig
	GameObjects      []gameObjectPlacement
	PreloadLocations [][2]float32
	modelCells       [][]bool
	adtModels        map[*common.WowObject]*common.Model
	adtChunks        map[*common.WowObject][]adtChunk
	Tileset          *terrainfit.Tileset
	tilesetSlots     []stockTile
}

type gameObjectPlacement struct {
	ID       string                  `json:"id"`
	Source   *azerothcore.GameObject `json:"source,omitempty"`
	TypeCode string                  `json:"typeCode"`
	Model    string                  `json:"model"`
	Position [3]float32              `json:"position"`
	Angle    wc3.Angle               `json:"angleDegrees"`
	Roll     float32                 `json:"rollRadians"`
	Pitch    float32                 `json:"pitchRadians"`
	Scale    [3]float32              `json:"scale"`
}

// NewWc3Converter creates a converter.
func NewWc3Converter(cfg settings.MapExportConfig) *Wc3Converter {
	return &Wc3Converter{config: cfg}
}

// GenerateTerrainWithHeight builds war3map.w3e terrain from ADT roots.
func (w *Wc3Converter) GenerateTerrainWithHeight(m *common.WowObjectManager) (data.Terrain, error) {
	roots := m.Roots
	log.Printf("Generating terrain from %d objects", len(roots))

	heightMap, height, width, err := computeTerrainHeightMap(roots, w.config)
	if err != nil {
		return data.Terrain{}, err
	}
	log.Printf("Map size height=%d width=%d", height, width)

	terrain := GetInitialTerrain(height, width)
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
	return terrain, nil
}

// PlaceDoodads adds doodad instances to the map.
func (w *Wc3Converter) PlaceDoodads(mm *extra.MapManager, m *common.WowObjectManager, filter func(*common.WowObject) bool) (tiltedDoodads int, err error) {
	terrain := mm.GetTerrain()
	log.Printf("Placing doodads")
	roots := m.Roots
	preloadRoots := map[*common.WowObject]bool{}
	for _, root := range roots {
		if root.Type != common.WowObjectADT || root.Model == nil {
			continue
		}
		for _, g := range root.Model.MDL.Geosets {
			if len(g.Vertices) > 0 {
				preloadRoots[root] = true
				break
			}
		}
	}
	if len(preloadRoots) == 0 {
		for _, root := range roots {
			if root.Type == common.WowObjectWMO {
				preloadRoots[root] = true
			}
		}
	}
	w.PreloadLocations = nil

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

	editorFields := map[*common.WowObject]string{}
	hasADT := false
	m.IterateObjects(func(obj *common.WowObject, _ common.ObjectAbsolute) {
		if obj.Type != common.WowObjectADT {
			return
		}
		hasADT = true
		editorFields[obj] = "dimc"
		for _, child := range obj.Children {
			if child.Type != common.WowObjectADT && child.Type != common.WowObjectUnit && child.Type != common.WowObjectGobj {
				editorFields[child] = "doct"
			}
		}
	})
	if !hasADT {
		for _, root := range roots {
			if root.Type == common.WowObjectWMO {
				editorFields[root] = "dimc"
			}
		}
	}

	modelPathToDoodadType := map[string]*extra.DoodadType{}
	doodadsPlaced := 0
	doodadsOutOfBounds := 0

	place := func(obj *common.WowObject, abs common.ObjectAbsolute, visual *common.Model, center imath.Vector3) {
		doodadsPlaced++
		yaw, roll, pitch := doodadInstanceRotation(abs.Rotation)

		if obj.Model == nil {
			log.Printf("%s", ansi.Redf("Doodad has no model %v", obj))
			panic("Doodad has no model")
		}
		fileName := visual.RelativePath
		isDestructible := obj.Type == common.WowObjectGobj
		if isDestructible {
			// Map generation writes MDX files. Give World Editor the actual
			// imported filename, rather than an extensionless model stem.
			if filepath.Ext(fileName) == "" {
				fileName += ".mdx"
			}
			fileName = strings.ReplaceAll(fileName, "/", "\\")
		}
		// A model can be both a direct ADT child and a nested prop. Keep their
		// object types separate so editor fields do not leak across placements.
		hashKey := fmt.Sprintf("%t;%s;%s", isDestructible, fileName, editorFields[obj])
		if obj.GameObject != nil {
			hashKey = fmt.Sprintf("%s;entry=%d", hashKey, obj.GameObject.Entry)
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
		// Match the model's native scale-then-rotate transform when moving its
		// origin. This also preserves placement with nonuniform map XY scales.
		offset := imath.Vector3{center[0] * abs.ScaleFactor * rootScale[0], center[1] * abs.ScaleFactor * rootScale[1], center[2] * abs.ScaleFactor * rootScale[2]}
		offset = imath.V3Rotate(offset, abs.Rotation)
		inGameX += offset[0]
		inGameY += offset[1]
		inGameZ += offset[2]

		if inGameX < mapMin[0] || inGameX > mapMax[0] || inGameY < mapMin[1] || inGameY > mapMax[1] {
			doodadsOutOfBounds++
			return
		}

		if _, ok := modelPathToDoodadType[hashKey]; !ok {
			dt := mm.AddDoodadType(nil, isDestructible)
			doodadName := fmt.Sprintf("~D %s -- %s -- %s", filepath.Base(visual.RelativePath), obj.Type, dt.Code)
			if obj.GameObject != nil {
				doodadName = fmt.Sprintf("~G %s (%d)", obj.GameObject.Name, obj.GameObject.Entry)
			}
			prefix := "d"
			if isDestructible {
				prefix = "b"
			}
			dt.Data = append(dt.Data,
				data.Modification{ID: prefix + "fil", Type: data.ModificationString, Level: 0, Column: 0, Value: fileName},
				data.Modification{ID: prefix + "nam", Type: data.ModificationString, Level: 0, Column: 0, Value: doodadName},
				data.Modification{ID: prefix + "mas", Type: data.ModificationUnreal, Value: float32(abs.ScaleFactor * maxFloat(rootScale[0], rootScale[1], rootScale[2]) * 1.5)},
				data.Modification{ID: prefix + "mis", Type: data.ModificationUnreal, Value: float32(abs.ScaleFactor * minFloat(rootScale[0], rootScale[1], rootScale[2]) / 1.5)},
				// Disable inherited terrain alignment; instance rotation owns the tilt.
				data.Modification{ID: prefix + "mar", Type: data.ModificationUnreal, Value: float32(0)},
				data.Modification{ID: prefix + "map", Type: data.ModificationUnreal, Value: float32(0)},
			)
			if isDestructible {
				dt.Data = append(dt.Data,
					data.Modification{ID: "bvar", Type: data.ModificationInt, Value: 1},
					data.Modification{ID: "bhps", Type: data.ModificationUnreal, Value: float32(100)},
					data.Modification{ID: "bptx", Type: data.ModificationString, Value: ""},
					data.Modification{ID: "bptd", Type: data.ModificationString, Value: ""},
					data.Modification{ID: "btxf", Type: data.ModificationString, Value: ""},
					data.Modification{ID: "btxi", Type: data.ModificationInt, Value: 0},
					data.Modification{ID: "bfxr", Type: data.ModificationUnreal, Value: float32(-1)},
					data.Modification{ID: "btar", Type: data.ModificationString, Value: "ground"},
				)
			} else {
				dt.Data = append(dt.Data,
					data.Modification{ID: "danf", Type: data.ModificationInt, Value: 1},
					data.Modification{ID: "dshf", Type: data.ModificationInt, Value: 1},
				)
				switch editorFields[obj] {
				case "dimc":
					dt.Data = append(dt.Data, data.Modification{ID: "dimc", Type: data.ModificationInt, Value: 1})
				case "doct":
					// World Editor's occlusionType2 stores the enum name, not its index.
					dt.Data = append(dt.Data, data.Modification{ID: "doct", Type: data.ModificationString, Value: "Auto"})
				}
			}
			modelPathToDoodadType[hashKey] = dt
		}
		dt := modelPathToDoodadType[hashKey]
		id4 := dt.Code
		if len(id4) > 4 {
			id4 = id4[:4]
		}
		placement := data.Doodad{
			Variation:        0,
			Position:         [3]float32{float32(inGameX), float32(inGameY), float32(inGameZ)},
			Angle:            yaw,
			Roll:             roll,
			Pitch:            pitch,
			Scale:            [3]float32{float32(abs.ScaleFactor * rootScale[0]), float32(abs.ScaleFactor * rootScale[1]), float32(abs.ScaleFactor * rootScale[2])},
			SkinID:           id4,
			Flags:            data.DoodadFlag{Visible: true, Solid: true, CustomHeight: true},
			Life:             100,
			RandomItemSetPtr: -1,
		}
		mm.AddDoodad(dt, placement)
		if roll != 0 || pitch != 0 {
			tiltedDoodads++
		}
		if preloadRoots[obj] {
			w.PreloadLocations = append(w.PreloadLocations, [2]float32{placement.Position[0], placement.Position[1]})
		}
		if isDestructible {
			w.GameObjects = append(w.GameObjects, gameObjectPlacement{
				ID: obj.ID, Source: obj.GameObject, TypeCode: dt.Code, Model: fileName, Position: placement.Position, Angle: placement.Angle, Roll: roll, Pitch: pitch, Scale: placement.Scale,
			})
		}
	}
	m.IterateObjects(func(obj *common.WowObject, abs common.ObjectAbsolute) {
		if !filter(obj) {
			return
		}
		if chunks, prepared := w.adtChunks[obj]; prepared {
			for _, chunk := range chunks {
				place(obj, abs, chunk.model, chunk.center)
			}
			return
		}
		visual := obj.Model
		if w.config.TerrainAsWc3Tilesets && obj.Type == common.WowObjectADT {
			visual = w.adtModels[obj]
			if visual == nil {
				return
			}
		}
		place(obj, abs, visual, imath.Vector3{})
	})

	if doodadsOutOfBounds > 0 {
		log.Printf("%d/%d objects are outside of map bounds.", doodadsOutOfBounds, doodadsPlaced)
	}
	return tiltedDoodads, nil
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

// minimumTerrainClampWidth is independent of unit scale: narrowing the height
// range enlarges XY by the same factor. Compute this before any grid allocation.
func MinimumTerrainClampWidth(min, max imath.Vector3) float64 {
	size := imath.V3Sub(max, min)
	return config.MaxGameHeightDiff() * math.Max(size[0], size[1]) / (size[2] * config.DistancePerTile * maxTerrainTiles)
}

func TerrainDimensions(min, max imath.Vector3, clamp settings.TerrainClampPercent) (height, width int, err error) {
	diff := clamp.Upper - clamp.Lower
	if math.IsNaN(diff) || math.IsInf(diff, 0) || clamp.Lower < 0 || clamp.Upper > 1 || diff <= 0 {
		return 0, 0, fmt.Errorf("terrain height clamp must have 0 <= lower < upper <= 1")
	}
	size := imath.V3Sub(max, min)
	for _, span := range size {
		if math.IsNaN(span) || math.IsInf(span, 0) || span <= 0 {
			return 0, 0, fmt.Errorf("terrain bounds must have a finite positive width, length and height")
		}
	}
	minWidth := MinimumTerrainClampWidth(min, max)
	if minWidth > 1 {
		return 0, 0, fmt.Errorf("map size is too large even with the full height range (maximum %d tiles per side); select fewer cells", maxTerrainTiles)
	}
	if diff+1e-12 < minWidth {
		return 0, 0, fmt.Errorf("map size is too large (maximum %d tiles per side): terrain clamp must span at least %.3f%%; widen the height range or select fewer cells", maxTerrainTiles, math.Ceil(minWidth*100000)/1000)
	}
	ratio := config.MaxGameHeightDiff() / (size[2] * diff) / config.DistancePerTile
	// Ignore floating-point noise at an exact four-tile boundary.
	w := math.Ceil(size[0]*ratio/4-1e-10) * 4
	h := math.Ceil(size[1]*ratio/4-1e-10) * 4
	if math.IsNaN(w) || math.IsNaN(h) || w > maxTerrainTiles || h > maxTerrainTiles {
		return 0, 0, fmt.Errorf("map size is too large (maximum %d tiles per side); widen the terrain height range", maxTerrainTiles)
	}
	width, height = maxInt(4, int(w)), maxInt(4, int(h))
	return height, width, nil
}

func computeTerrainHeightMap(roots []*common.WowObject, cfg settings.MapExportConfig) ([][]float64, int, int, error) {
	if len(roots) == 0 {
		heightMap := make2DFloat64(64+1, 64+1, -1)
		return heightMap, 64, 64, nil
	}
	log.Printf("Computing terrain height map...")
	ext := common.ComputeAbsoluteMinMaxExtents(roots)
	min, max := ext.Min, ext.Max
	height, width, err := TerrainDimensions(min, max, cfg.Terrain.ClampPercent)
	if err != nil {
		return nil, 0, 0, err
	}
	log.Printf("map[min:%v max:%v]", min, max)
	terrainSize := imath.V3Sub(max, min)
	log.Printf("map[terrainSize:%v]", terrainSize)
	recommendedClamp := (config.DataHeightToGameZ(float64(config.DataHeightMax())) - config.DataHeightToGameZ(float64(config.DataHeightMin()))) / terrainSize[2]
	log.Printf("Recommended terrain clamp percent difference %v", recommendedClamp)
	clampDiff := cfg.Terrain.ClampPercent.Upper - cfg.Terrain.ClampPercent.Lower
	ratioZ := config.MaxGameHeightDiff() / (terrainSize[2] * clampDiff)
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
					return nil, 0, 0, fmt.Errorf("terrain vertex is outside the map bounds")
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
	return heightMap, height, width, nil
}

func GetInitialTerrain(height, width int) data.Terrain {
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

func doodadInstanceRotation(rotation imath.EulerRotation) (yaw wc3.Angle, roll, pitch float32) {
	// Source Euler order is Rz(yaw)*Ry(pitch)*Rx(roll). WC3 3.0 doodads
	// use Rz(yaw)*Rx(roll)*Ry(pitch), so mixed-axis rotations need conversion.
	// See Antares' Doodad Handler orientation conversion:
	// https://www.hiveworkshop.com/threads/doodad-handler-preplaced-special-effect.374394/
	sr, cr := math.Sincos(rotation[0])
	sp, cp := math.Sincos(rotation[1])
	sy, cy := math.Sincos(rotation[2])
	r := math.Asin(math.Max(-1, math.Min(1, cp*sr)))
	p := math.Atan2(sp, cp*cr)
	y := math.Atan2(sy*cr-cy*sp*sr, cy*cr+sy*sp*sr)
	if math.Abs(math.Cos(r)) < 1e-8 {
		// At gimbal lock choose zero pitch; only the combined heading is unique.
		p = 0
		y = math.Atan2(sy*cp, cy*cp)
	}
	return wc3.Angle(imath.Degrees(y)), float32(r), float32(p)
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
