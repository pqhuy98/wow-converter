package terrain

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/terrainfit"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func (e *Wc3Converter) ApplyWc3Tileset(ctx context.Context, terrain *data.Terrain, manager *common.WowObjectManager, wowClient client.Client, cfg config.Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	tiles := selectedAdtTiles(manager)
	if len(tiles) == 0 {
		log.Printf("Terrain as WC3 tilesets: no ADT tiles, leaving heightfield textures unchanged")
		return nil
	}
	source, err := cascSourceFromClient(ctx, wowClient)
	if err != nil {
		return err
	}
	log.Printf("Fitting WC3 tileset for %d ADT tiles", len(tiles))
	grid, err := tilesetGrid(manager, terrain.Map.Width, terrain.Map.Height)
	if err != nil {
		return err
	}
	grid.PaintFamilies = len(wc3GroundSlots)
	fitted, err := terrainfit.FitTileset(ctx, source, e.config.CascMapDir, tiles, grid)
	if err != nil {
		return err
	}
	paintTilesetGround(terrain, fitted)
	if err := e.prepareTilesetModels(ctx, manager, terrain, len(fitted.Sheets)-1, cfg.AssetPrefix); err != nil {
		return err
	}
	slots, err := applyTilesetPalette(terrain, fitted)
	if err != nil {
		return err
	}
	e.tilesetSlots = slots
	e.Tileset = fitted
	log.Printf("WC3 tileset families=%d cells=%dx%d", len(fitted.Sheets), fitted.Width, fitted.Height)
	return nil
}

// Extended 512x256 sheets map variation 0 to full variant 0 (layer 16),
// variations 1..15 to layers 17..31, and variation 16 to full mask layer 15.
func w3eGroundVariation(variant uint8) int {
	return int(variant)
}

// Invert the actual ADT placement, just as the minimap does. This handles
// all map angles without guessing which global axis to flip.
func tilesetGrid(m *common.WowObjectManager, width, height int) (terrainfit.TilesetGrid, error) {
	ext := common.ComputeAbsoluteMinMaxExtents(m.Roots)
	var grid terrainfit.TilesetGrid
	m.IterateObjects(func(obj *common.WowObject, abs common.ObjectAbsolute) {
		if grid.Width != 0 || obj.Type != common.WowObjectADT || !adtHasFaces(obj) {
			return
		}
		lo, hi := imath.Vector3{math.Inf(1), math.Inf(1), 0}, imath.Vector3{math.Inf(-1), math.Inf(-1), 0}
		for _, g := range obj.Model.MDL.Geosets {
			for _, v := range g.Vertices {
				lo = imath.V3Min(lo, v.Position)
				hi = imath.V3Max(hi, v.Position)
			}
		}
		if hi[0] <= lo[0] || hi[1] <= lo[1] || abs.ScaleFactor <= 0 {
			return
		}
		toSource := func(x, y float64) [2]float64 {
			p := imath.V3Rotate(imath.V3Sub(imath.Vector3{x, y, abs.Position[2]}, abs.Position), imath.EulerRotation{0, 0, -abs.Rotation[2]})
			p = imath.V3Scale(p, 1/abs.ScaleFactor)
			return [2]float64{float64(obj.TileX) + (hi[0]-p[0])/(hi[0]-lo[0]), float64(obj.TileY) + (p[1]-lo[1])/(hi[1]-lo[1])}
		}
		o := toSource(ext.Min[0], ext.Max[1])
		x := toSource(ext.Min[0]+(ext.Max[0]-ext.Min[0])/float64(width), ext.Max[1])
		y := toSource(ext.Min[0], ext.Max[1]-(ext.Max[1]-ext.Min[1])/float64(height))
		grid = terrainfit.TilesetGrid{Width: width, Height: height, Origin: o, StepX: [2]float64{x[0] - o[0], x[1] - o[1]}, StepY: [2]float64{y[0] - o[0], y[1] - o[1]}}
	})
	if grid.Width == 0 {
		return grid, fmt.Errorf("no nonempty ADT frame for native tileset")
	}
	return grid, nil
}

func selectedAdtTiles(m *common.WowObjectManager) [][2]int {
	seen := map[[2]int]bool{}
	var tiles [][2]int
	m.IterateObjects(func(obj *common.WowObject, _ common.ObjectAbsolute) {
		tx, ty, ok := common.AsAdt(obj)
		if !ok || seen[[2]int{tx, ty}] {
			return
		}
		seen[[2]int{tx, ty}] = true
		tiles = append(tiles, [2]int{tx, ty})
	})
	return tiles
}

func applyTilesetPalette(terrain *data.Terrain, fitted *terrainfit.Tileset) ([]stockTile, error) {
	paintCount := len(fitted.Sheets) - 1
	if paintCount < 1 || paintCount > len(wc3GroundSlots) {
		return nil, fmt.Errorf("WC3 tileset has %d paint families, max %d stock tiles", paintCount, len(wc3GroundSlots))
	}
	// Preserve the fitted color ordering; only change which stock ID backs each
	// family. Count final W3E paint after model interiors become native Maph.
	counts := make([]int, paintCount)
	for _, row := range terrain.GroundTexture {
		for _, id := range row {
			if id >= 0 && id < paintCount {
				counts[id]++
			}
		}
	}
	order := make([]int, paintCount)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] < counts[order[j]] })
	var free, restricted []stockTile
	for _, slot := range wc3GroundSlots {
		if slot.buildable {
			free = append(free, slot)
		} else {
			restricted = append(restricted, slot)
		}
	}
	slots := make([]stockTile, paintCount)
	needRestricted := max(0, paintCount-len(free))
	for i := 0; i < needRestricted; i++ {
		slots[order[i]] = restricted[i]
	}
	next := 0
	for i := range slots {
		if slots[i].id == "" {
			slots[i] = free[next]
			next++
		}
	}
	palette := make([]string, paintCount+1)
	for i, slot := range slots {
		palette[i] = slot.id
	}
	palette[paintCount] = "Maph" // Native 3.0 alpha tile; no imported texture.
	terrain.CustomTileset = true
	terrain.TilePalette = palette
	return slots, nil
}

func paintTilesetGround(terrain *data.Terrain, fitted *terrainfit.Tileset) {
	rows := len(terrain.GroundTexture)
	if rows == 0 || fitted.Width <= 0 || fitted.Height <= 0 || len(fitted.Labels) == 0 {
		return
	}
	cols := len(terrain.GroundTexture[0])
	if cols == 0 {
		return
	}
	for i := 0; i < rows; i++ {
		y := fitted.Height - i
		for j := 0; j < cols; j++ {
			x := j
			terrain.GroundTexture[i][j] = int(fitted.Labels[y*(fitted.Width+1)+x])
			variant := uint8(0)
			if x < fitted.Width && i < fitted.Height {
				variant = fitted.Variants[(fitted.Height-1-i)*fitted.Width+x]
			}
			terrain.GroundVariation[i][j] = w3eGroundVariation(variant)
		}
	}
}

// cellVertex carries the original attributes through exact triangle/cell clipping.
type cellVertex struct {
	grid   imath.Vector3
	vertex components.GeosetVertex
}

func cellPolygon(poly []cellVertex, axis int, edge float64, greater bool) []cellVertex {
	out := make([]cellVertex, 0, len(poly)+1)
	for i, b := range poly {
		a := poly[(i+len(poly)-1)%len(poly)]
		ain, bin := a.grid[axis] >= edge, b.grid[axis] >= edge
		if !greater {
			ain, bin = a.grid[axis] <= edge, b.grid[axis] <= edge
		}
		if ain != bin {
			t := (edge - a.grid[axis]) / (b.grid[axis] - a.grid[axis])
			c := a
			for k := 0; k < 3; k++ {
				c.grid[k] += t * (b.grid[k] - a.grid[k])
				c.vertex.Position[k] += t * (b.vertex.Position[k] - a.vertex.Position[k])
				c.vertex.Normal[k] += t * (b.vertex.Normal[k] - a.vertex.Normal[k])
			}
			for k := 0; k < 2; k++ {
				c.vertex.TexPosition[k] += t * (b.vertex.TexPosition[k] - a.vertex.TexPosition[k])
			}
			out = append(out, c)
		}
		if bin {
			out = append(out, b)
		}
	}
	return out
}

func polygonArea(p []cellVertex) float64 {
	area := 0.0
	for i, a := range p {
		b := p[(i+1)%len(p)]
		area += a.grid[0]*b.grid[1] - b.grid[0]*a.grid[1]
	}
	return math.Abs(area) * .5
}

func visitAdtCells(ctx context.Context, m *common.WowObjectManager, width, height int, visit func(*common.WowObject, *components.Geoset, int, int, []cellVertex)) error {
	ext := common.ComputeAbsoluteMinMaxExtents(m.Roots)
	size := imath.V3Sub(ext.Max, ext.Min)
	var failure error
	m.IterateObjects(func(obj *common.WowObject, abs common.ObjectAbsolute) {
		if failure != nil || obj.Type != common.WowObjectADT || !adtHasFaces(obj) {
			return
		}
		for _, g := range obj.Model.MDL.Geosets {
			for fi, face := range g.Faces {
				if fi%256 == 0 {
					if failure = ctx.Err(); failure != nil {
						return
					}
				}
				tri := make([]cellVertex, 3)
				lo, hi := [2]float64{math.Inf(1), math.Inf(1)}, [2]float64{math.Inf(-1), math.Inf(-1)}
				for i, v := range face.Vertices {
					p := imath.V3Sum(abs.Position, imath.V3Rotate(imath.V3Scale(v.Position, abs.ScaleFactor), abs.Rotation))
					tri[i] = cellVertex{grid: imath.Vector3{(p[0] - ext.Min[0]) / size[0] * float64(width), (p[1] - ext.Min[1]) / size[1] * float64(height), p[2]}, vertex: *v}
					for k := 0; k < 2; k++ {
						lo[k] = math.Min(lo[k], tri[i].grid[k])
						hi[k] = math.Max(hi[k], tri[i].grid[k])
					}
				}
				for y := max(0, int(math.Floor(lo[1]))); y < min(height, int(math.Ceil(hi[1]))); y++ {
					for x := max(0, int(math.Floor(lo[0]))); x < min(width, int(math.Ceil(hi[0]))); x++ {
						p := cellPolygon(tri, 0, float64(x), true)
						p = cellPolygon(p, 0, float64(x+1), false)
						p = cellPolygon(p, 1, float64(y), true)
						p = cellPolygon(p, 1, float64(y+1), false)
						if polygonArea(p) > 1e-9 {
							visit(obj, g, x, y, p)
						}
					}
				}
			}
		}
	})
	return failure
}

// Classify whole triangle footprints, not randomly rounded source vertices.
// Keep the original tree for bounds, water, navigation and minimap generation;
// only the model referenced by the placed ADT doodad is replaced.
func (w *Wc3Converter) prepareTilesetModels(ctx context.Context, m *common.WowObjectManager, terrain *data.Terrain, clear int, assetPrefix string) error {
	width, height := terrain.Map.Width, terrain.Map.Height
	w.modelCells = make2DBool(height, width, false)
	surface := make2DFloat64(height+1, width+1, math.Inf(-1))
	ext := common.ComputeAbsoluteMinMaxExtents(m.Roots)
	lower := ext.Min[2] + (ext.Max[2]-ext.Min[2])*w.config.Terrain.ClampPercent.Lower
	upper := ext.Min[2] + (ext.Max[2]-ext.Min[2])*w.config.Terrain.ClampPercent.Upper
	err := visitAdtCells(ctx, m, width, height, func(_ *common.WowObject, _ *components.Geoset, x, y int, p []cellVertex) {
		for j := 0; j < 2; j++ {
			for i := 0; i < 2; i++ {
				for k := 1; k < len(p)-1; k++ {
					z, inside := NavigationHeight([3]imath.Vector3{p[0].grid, p[k].grid, p[k+1].grid}, float64(x+i), float64(y+j))
					if inside {
						surface[y+j][x+i] = math.Max(surface[y+j][x+i], z)
						break
					}
				}
			}
		}
		for _, v := range p {
			if v.grid[2] < lower-1e-6 || v.grid[2] > upper+1e-6 {
				w.modelCells[y][x] = true
			}
		}
	})
	if err != nil {
		return err
	}
	for y, row := range surface {
		for x, z := range row {
			if !math.IsInf(z, -1) {
				p := math.Max(0, math.Min(1, (z-lower)/(upper-lower)))
				terrain.GroundHeight[y][x] = int(math.Round(float64(config.DataHeightMin()) + p*float64(config.DataHeightMax()-config.DataHeightMin())))
			}
		}
	}
	// Only the selected height limit can allocate model cells. Tiny cracks
	// between floating-point MCNK coordinates must never create model strips.
	// Alpha paint is an interior cutout. Keep every corner touching native
	// ground unchanged, so visible native cells retain their fitted colors.
	for y := 0; y <= height; y++ {
		for x := 0; x <= width; x++ {
			surrounded := true
			for yy := max(0, y-1); yy <= min(height-1, y); yy++ {
				for xx := max(0, x-1); xx <= min(width-1, x); xx++ {
					surrounded = surrounded && w.modelCells[yy][xx]
				}
			}
			if surrounded {
				terrain.GroundTexture[y][x] = clear
			}
		}
	}
	geosets := map[*common.WowObject]map[*components.Geoset]*components.Geoset{}
	// Deduplicate clipped boundary vertices before MDX's 16-bit face indices.
	type vertexKey struct {
		p  [3]int64
		uv [2]int64
	}
	vertices := map[*components.Geoset]map[vertexKey]*components.GeosetVertex{}
	err = visitAdtCells(ctx, m, width, height, func(obj *common.WowObject, g *components.Geoset, x, y int, p []cellVertex) {
		if !w.modelCells[y][x] {
			return
		}
		if geosets[obj] == nil {
			geosets[obj] = map[*components.Geoset]*components.Geoset{}
		}
		out := geosets[obj][g]
		if out == nil {
			cp := *g
			cp.Vertices = nil
			cp.Faces = nil
			out = &cp
			geosets[obj][g] = out
			vertices[out] = map[vertexKey]*components.GeosetVertex{}
		}
		vv := make([]*components.GeosetVertex, len(p))
		for i, v := range p {
			key := vertexKey{}
			for k := 0; k < 3; k++ {
				key.p[k] = int64(math.Round(v.vertex.Position[k] * 1e6))
			}
			for k := 0; k < 2; k++ {
				key.uv[k] = int64(math.Round(v.vertex.TexPosition[k] * 1e9))
			}
			vv[i] = vertices[out][key]
			if vv[i] == nil {
				cp := v.vertex
				vv[i] = &cp
				cp.ID = len(out.Vertices)
				vertices[out][key] = vv[i]
				out.Vertices = append(out.Vertices, vv[i])
			}
		}
		for i := 1; i < len(vv)-1; i++ {
			out.Faces = append(out.Faces, components.Face{Vertices: [3]*components.GeosetVertex{vv[0], vv[i], vv[i+1]}})
		}
	})
	if err != nil {
		return err
	}
	w.adtModels = map[*common.WowObject]*common.Model{}
	for _, root := range m.Roots {
		if root.Type != common.WowObjectADT || root.Model == nil {
			continue
		}
		model := *root.Model
		model.AlwaysExport = true
		visual := *model.MDL
		visual.Geosets = nil
		for _, g := range model.MDL.Geosets {
			if clipped := geosets[root][g]; clipped != nil {
				visual.Geosets = append(visual.Geosets, clipped)
			}
		}
		if len(visual.Geosets) == 0 {
			continue
		}
		// The mask and frame own this generated asset's cache identity.
		hash := sha256.New()
		fmt.Fprintf(hash, "v2/%v/%v/%dx%d/%v/%v", ext, root.Rotation, width, height, w.config.Terrain.ClampPercent, root.Position)
		for _, row := range w.modelCells {
			for _, v := range row {
				if v {
					hash.Write([]byte{1})
				} else {
					hash.Write([]byte{0})
				}
			}
		}
		stem := strings.TrimSuffix(strings.TrimSuffix(model.RelativePath, ".mdx"), ".mdl")
		model.RelativePath = fmt.Sprintf("%s-tileset-%x", stem, hash.Sum(nil)[:8])
		visual.Model.Name = model.RelativePath
		visual.Modify = mdl.NewModify(&visual)
		model.MDL = &visual
		w.adtModels[root] = &model
		key := strings.TrimPrefix(model.RelativePath, strings.TrimRight(assetPrefix, "/\\")+"/")
		m.AssetManager.Models()[key] = &model
	}
	return nil
}

func adtHasFaces(obj *common.WowObject) bool {
	if obj == nil || obj.Model == nil || obj.Model.MDL == nil {
		return false
	}
	for _, geoset := range obj.Model.MDL.Geosets {
		if len(geoset.Faces) > 0 {
			return true
		}
	}
	return false
}

type stockTile struct {
	id, dir, file string
	buildable     bool
}

func (s stockTile) importPath() string {
	return s.dir + `\` + s.file + ".blp"
}

func (s stockTile) diskPath(outputDir string) string {
	return filepath.Join(outputDir, filepath.FromSlash(strings.ReplaceAll(s.dir, `\`, "/")), s.file+".blp")
}

// Least-painted stock IDs in the extracted melee-map corpus: 40 unrestricted
// and 8 walkable/flyable but non-buildable. Exclude every cliffSet and every
// CliffTypes ground/upper association. Native Maph occupies a separate slot.
// Reproduce with scripts/analyze-melee-terrain.ts (40 + 8 defaults).
var wc3GroundSlots = []stockTile{
	{"Olgb", `TerrainArt\Outland`, "Outland_FlatStones", true},         // 1215 playable paint corners
	{"Kfst", `TerrainArt\BlackCitadel`, "Citadel_FlatStones", true},    // 1223 playable paint corners
	{"Ksmb", `TerrainArt\BlackCitadel`, "Citadel_SmallBricks", true},   // 2044 playable paint corners
	{"Qdrr", `TerrainArt\VillageFall`, "VillageFall_DirtRough", true},  // 5023 playable paint corners
	{"Xhdg", `TerrainArt\Dalaran`, "Dalaran_GrassTrim", true},          // 5272 playable paint corners
	{"Ofst", `TerrainArt\Outland`, "Outland_DirtCracked", true},        // 5958 playable paint corners
	{"Xdtr", `TerrainArt\Dalaran`, "Dalaran_DirtRough", true},          // 5978 playable paint corners
	{"Odtr", `TerrainArt\Outland`, "Outland_DirtLight", true},          // 6192 playable paint corners
	{"Kdtr", `TerrainArt\BlackCitadel`, "Citadel_RoughDirt", true},     // 6623 playable paint corners
	{"Fdro", `TerrainArt\LordaeronFall`, "Lordf_DirtRough", true},      // 7241 playable paint corners
	{"Fdrg", `TerrainArt\LordaeronFall`, "Lordf_DirtGrass", true},      // 7356 playable paint corners
	{"Jhdg", `TerrainArt\DalaranRuins`, "DRuins_GrassTrim", true},      // 7457 playable paint corners
	{"Jdtr", `TerrainArt\DalaranRuins`, "DRuins_DirtRough", true},      // 8484 playable paint corners
	{"Vdrr", `TerrainArt\Village`, "Village_DirtRough", true},          // 10316 playable paint corners
	{"Qgrs", `TerrainArt\VillageFall`, "VillageFall_GrassShort", true}, // 10704 playable paint corners
	{"Vgrs", `TerrainArt\Village`, "Village_GrassShort", true},         // 11930 playable paint corners
	{"Kfsl", `TerrainArt\BlackCitadel`, "Citadel_DirtLight", true},     // 12065 playable paint corners
	{"Idki", `TerrainArt\Icecrown`, "Ice_DarkIce", true},               // 12658 playable paint corners
	{"Wdro", `TerrainArt\LordaeronWinter`, "Lordw_DirtRough", true},    // 12666 playable paint corners
	{"Ofsl", `TerrainArt\Outland`, "Outland_FlatStonesLight", true},    // 13515 playable paint corners
	{"Iice", `TerrainArt\Icecrown`, "Ice_Ice", true},                   // 14901 playable paint corners
	{"Ndrd", `TerrainArt\Northrend`, "North_dirtdark", true},           // 15225 playable paint corners
	{"Idtr", `TerrainArt\Icecrown`, "Ice_DirtRough", true},             // 17564 playable paint corners
	{"Clvg", `TerrainArt\Felwood`, "Felwood_Leaves", true},             // 18624 playable paint corners
	{"Nice", `TerrainArt\Northrend`, "North_ice", true},                // 20031 playable paint corners
	{"Ydtr", `TerrainArt\Cityscape`, "City_DirtRough", true},           // 20951 playable paint corners
	{"Jgsb", `TerrainArt\DalaranRuins`, "DRuins_Grass", true},          // 22800 playable paint corners
	{"Idrt", `TerrainArt\Icecrown`, "Ice_Dirt", true},                  // 22887 playable paint corners
	{"Bdrh", `TerrainArt\Barrens`, "Barrens_DirtRough", true},          // 23119 playable paint corners
	{"Adrd", `TerrainArt\Ashenvale`, "Ashen_DirtRough", true},          // 24975 playable paint corners
	{"Yhdg", `TerrainArt\Cityscape`, "City_GrassTrim", true},           // 25261 playable paint corners
	{"Wsng", `TerrainArt\LordaeronWinter`, "Lordw_SnowGrass", true},    // 26073 playable paint corners
	{"Zdrg", `TerrainArt\Ruins`, "Ruins_DirtGrass", true},              // 26973 playable paint corners
	{"Bdrg", `TerrainArt\Barrens`, "Barrens_DirtGrass", true},          // 28575 playable paint corners
	{"Bdsd", `TerrainArt\Barrens`, "Barrens_DesertDark", true},         // 28861 playable paint corners
	{"Cdrd", `TerrainArt\Felwood`, "Felwood_DirtRough", true},          // 29555 playable paint corners
	{"Zvin", `TerrainArt\Ruins`, "Ruins_GrassDark", true},              // 31189 playable paint corners
	{"Ngrs", `TerrainArt\Northrend`, "North_Grass", true},              // 31875 playable paint corners
	{"Cpos", `TerrainArt\Felwood`, "Felwood_Poison", true},             // 35526 playable paint corners
	{"Wdrt", `TerrainArt\LordaeronWinter`, "Lordw_Dirt", true},         // 36913 playable paint corners
	{"Dbrk", `TerrainArt\Dungeon`, "Cave_Brick", false},                // 0 playable paint corners
	{"Dgrs", `TerrainArt\Dungeon`, "Cave_GreyStones", false},           // 0 playable paint corners
	{"Dlav", `TerrainArt\Dungeon`, "Cave_Lava", false},                 // 0 playable paint corners
	{"Dlvc", `TerrainArt\Dungeon`, "Cave_LavaCracks", false},           // 0 playable paint corners
	{"Drds", `TerrainArt\Dungeon`, "Cave_RedStones", false},            // 0 playable paint corners
	{"Gdkr", `TerrainArt\Dungeon2`, "GDarkRocks", false},               // 0 playable paint corners
	{"Grds", `TerrainArt\Dungeon2`, "GRedStones", false},               // 0 playable paint corners
	{"Ggrs", `TerrainArt\Dungeon2`, "GGreyStones", false},              // 121 playable paint corners
}

func (e *Wc3Converter) WriteWc3Tileset(outputDir string) error {
	if e == nil || e.Tileset == nil {
		return nil
	}
	if len(e.Tileset.Sheets) > len(wc3GroundSlots)+1 {
		return fmt.Errorf("WC3 tileset has %d families, max %d ground tiles plus native alpha", len(e.Tileset.Sheets), len(wc3GroundSlots))
	}
	if len(e.tilesetSlots) != len(e.Tileset.Sheets)-1 {
		return fmt.Errorf("missing WC3 stock tile assignment")
	}
	paths := make([]string, 0, len(e.Tileset.Sheets))
	for i, sheet := range e.Tileset.Sheets {
		if i == len(e.Tileset.Sheets)-1 {
			continue // Maph uses the game's native alpha tile.
		}
		slot := e.tilesetSlots[i]
		path := slot.diskPath(outputDir)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, sheet, 0o644); err != nil {
			return err
		}
		paths = append(paths, slot.importPath())
	}
	return extra.EnsureCustomImports(outputDir, paths)
}

type clientCascSource struct {
	client client.Client
	info   client.CASCInfo
}

func cascSourceFromClient(ctx context.Context, c client.Client) (casc.Source, error) {
	if c == nil {
		return nil, fmt.Errorf("WoW data is required for terrain as WC3 tilesets")
	}
	info, err := c.GetCASCInfo(ctx)
	if err != nil {
		return nil, err
	}
	return clientCascSource{client: c, info: info}, nil
}

func (s clientCascSource) IsLoaded() bool        { return true }
func (s clientCascSource) TypeName() string      { return "client" }
func (s clientCascSource) Build() casc.BuildInfo { return s.info.Build }
func (s clientCascSource) BuildConfig() any      { return s.info.BuildConfig }
func (s clientCascSource) GetBuildName() string  { return s.info.BuildName }
func (s clientCascSource) GetBuildKey() string   { return s.info.BuildKey }
func (s clientCascSource) GetFile(ctx context.Context, id int) ([]byte, error) {
	return s.client.DownloadCascFile(ctx, id)
}
func (s clientCascSource) GetFilePartial(ctx context.Context, id int) ([]byte, error) {
	return s.GetFile(ctx, id)
}

func (s clientCascSource) GetFileByName(ctx context.Context, name string) (casc.ListfileEntry, error) {
	return s.client.GetFileByName(ctx, name)
}
