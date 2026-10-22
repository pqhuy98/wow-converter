package terrain

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	"github.com/pqhuy98/wow-converter/internal/converter/terrainfit"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestStockGroundSlotsAreRankedAndUnrestricted(t *testing.T) {
	var candidates []struct {
		ID, Dir, File                       string
		Buildable, Walkable, Flyable, Cliff bool
	}
	raw, err := os.ReadFile("testdata/stock_ground_candidates.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &candidates); err != nil {
		t.Fatal(err)
	}
	var free, restricted []stockTile
	for _, c := range candidates {
		if c.Walkable && c.Flyable && !c.Cliff {
			slot := stockTile{c.ID, c.Dir, c.File, c.Buildable}
			if c.Buildable {
				free = append(free, slot)
			} else {
				restricted = append(restricted, slot)
			}
		}
	}
	if len(free) != 53 || len(restricted) != 51 || len(wc3GroundSlots) != 48 {
		t.Fatalf("candidate selection mismatch: unrestricted=%d, restricted=%d, selected=%d", len(free), len(restricted), len(wc3GroundSlots))
	}
	expected := append(free[:40:40], restricted[:8]...)
	ids, paths := map[string]bool{}, map[string]bool{}
	for i, slot := range wc3GroundSlots {
		if slot != expected[i] {
			t.Fatalf("slot %d=%+v, expected ranked tile %+v", i, slot, expected[i])
		}
		if ids[slot.id] || paths[slot.importPath()] {
			t.Fatalf("duplicate stock ID/path %+v", slot)
		}
		ids[slot.id] = true
		paths[slot.importPath()] = true
	}

}

func TestW3EGroundVariationPreservesExtendedVariants(t *testing.T) {
	if got := w3eGroundVariation(0); got != 0 {
		t.Fatalf("empty mask: %d", got)
	}
	if got := w3eGroundVariation(3); got != 3 {
		t.Fatalf("kept variant: %d", got)
	}
}

func TestApplyTilesetPaletteReplacesStockGround(t *testing.T) {
	terrain := GetInitialTerrain(4, 4)
	if _, err := applyTilesetPalette(&terrain, &terrainfit.Tileset{Sheets: make([][]byte, 3)}); err != nil {
		t.Fatal(err)
	}
	if !terrain.CustomTileset || len(terrain.TilePalette) != 3 || terrain.TilePalette[2] != "Maph" {
		t.Fatalf("palette: %+v", terrain.TilePalette)
	}
}

func TestApplyTilesetPaletteRejectsTooManyFamilies(t *testing.T) {
	terrain := GetInitialTerrain(4, 4)
	if _, err := applyTilesetPalette(&terrain, &terrainfit.Tileset{Sheets: make([][]byte, len(wc3GroundSlots)+2)}); err == nil {
		t.Fatal("expected overflow error")
	}
}

func TestWriteWc3TilesetReplacesStockBlp(t *testing.T) {
	dir := t.TempDir()
	imp := make([]byte, 8)
	binary.LittleEndian.PutUint32(imp[4:], 0)
	if err := os.WriteFile(filepath.Join(dir, "war3map.imp"), imp, 0o644); err != nil {
		t.Fatal(err)
	}
	e := &Wc3Converter{Tileset: &terrainfit.Tileset{Sheets: [][]byte{[]byte("blp0"), []byte("blp1"), nil}}, tilesetSlots: wc3GroundSlots[:2]}
	if err := e.WriteWc3Tileset(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(wc3GroundSlots[1].diskPath(dir))
	if err != nil || string(got) != "blp1" {
		t.Fatalf("stock blp: %q %v", got, err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "TerrainArt"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("native alpha tile must not import a texture: %v, %v", entries, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "war3map.imp"))
	if err != nil || binary.LittleEndian.Uint32(raw[4:]) != 2 {
		t.Fatalf("expected only two color imports: %v", err)
	}
}

func TestPaintTilesetGroundResamplesLabelGrid(t *testing.T) {
	terrain := GetInitialTerrain(4, 2)
	fitted := &terrainfit.Tileset{
		Width:    2,
		Height:   4,
		Labels:   make([]uint8, 3*5),
		Variants: make([]uint8, 2*4),
		Sheets:   make([][]byte, 8),
	}
	fitted.Labels[4*3] = 7
	fitted.Labels[2] = 1
	paintTilesetGround(&terrain, fitted)
	if terrain.GroundTexture[0][0] != 7 || terrain.GroundTexture[4][2] != 1 {
		t.Fatalf("labels: %v", terrain.GroundTexture)
	}
	if terrain.GroundVariation[0][0] != 0 {
		t.Fatalf("variation: %d", terrain.GroundVariation[0][0])
	}
}

func TestTilesetModeSkipsEmptyAdtDoodads(t *testing.T) {
	model := mdl.New(mdl.NewMDLOptions{Name: "terrain.mdx"})
	model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{{Position: [3]float64{0, 0, 0}}}}}
	root := &common.WowObject{Type: common.WowObjectADT, ScaleFactor: 1, Model: &common.Model{RelativePath: "terrain.mdx", MDL: model}}
	manager := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	manager.Roots = []*common.WowObject{root}
	mm := extra.NewMapManager()
	mm.SetTerrain(GetInitialTerrain(2, 2))
	cfg := settings.DefaultMapExportConfig()
	cfg.TerrainAsWc3Tilesets = true
	if _, err := NewWc3Converter(cfg).PlaceDoodads(mm, manager, func(*common.WowObject) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if len(mm.Doodads) != 0 {
		t.Fatalf("empty clipped ADT should not be placed: %d", len(mm.Doodads))
	}
}

func TestParseGenerateWc3BodyDefaultsTilesetOff(t *testing.T) {
	// API default is covered in maps_generate_validate_test; this guards the exporter zero value.
	if settings.DefaultMapExportConfig().TerrainAsWc3Tilesets {
		t.Fatal("tileset export must default off")
	}
}

// Sparse source vertices cover many native cells. Every cell beyond the
// selected height window must retain exactly its source triangle area.
func TestTilesetModelCoverageAndClipping(t *testing.T) {
	for _, angle := range []float64{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2} {
		cfg := settings.DefaultMapExportConfig()
		cfg.TerrainAsWc3Tilesets = true
		cfg.Terrain.ClampPercent.Upper = .45
		manager := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
		model := mdl.New(mdl.NewMDLOptions{Name: "terrain"})
		a := &components.GeosetVertex{Position: imath.Vector3{0, 0, 0}, TexPosition: imath.Vector2{0, 0}}
		b := &components.GeosetVertex{Position: imath.Vector3{8, 0, 8}, TexPosition: imath.Vector2{1, 0}}
		c := &components.GeosetVertex{Position: imath.Vector3{8, 8, 8}, TexPosition: imath.Vector2{1, 1}}
		d := &components.GeosetVertex{Position: imath.Vector3{0, 8, 0}, TexPosition: imath.Vector2{0, 1}}
		g := &components.Geoset{Vertices: []*components.GeosetVertex{a, b, c, d}, Faces: []components.Face{{Vertices: [3]*components.GeosetVertex{a, b, c}}, {Vertices: [3]*components.GeosetVertex{a, c, d}}}}
		model.Geosets = []*components.Geoset{g}
		root := &common.WowObject{Type: common.WowObjectADT, TileX: 32, TileY: 48, Model: &common.Model{RelativePath: "terrain", MDL: model}, ScaleFactor: 1, Rotation: imath.EulerRotation{0, 0, angle}}
		manager.Roots = []*common.WowObject{root}
		ext := common.ComputeAbsoluteMinMaxExtents(manager.Roots)
		terrain := GetInitialTerrain(8, 8)
		wc := NewWc3Converter(cfg)
		if err := wc.prepareTilesetModels(context.Background(), manager, &terrain, 63, ""); err != nil {
			t.Fatal(err)
		}
		if root.Model.MDL != model || len(g.Faces) != 2 || common.ComputeAbsoluteMinMaxExtents(manager.Roots) != ext {
			t.Fatal("clipping changed original source or bounds")
		}
		visual := wc.adtModels[root]
		if visual == nil {
			t.Fatal("no fallback model")
		}
		// Compute model area directly in local XY: five full columns = 40.
		total := 0.0
		for _, gs := range visual.MDL.Geosets {
			for _, f := range gs.Faces {
				p, q, r := f.Vertices[0].Position, f.Vertices[1].Position, f.Vertices[2].Position
				total += math.Abs((q[0]-p[0])*(r[1]-p[1])-(q[1]-p[1])*(r[0]-p[0])) * .5
			}
		}
		if math.Abs(total-40) > 1e-6 {
			t.Fatalf("angle %g: missing/overlapping source area: %g, want 40", angle, total)
		}
		cells := 0
		for _, row := range wc.modelCells {
			for _, v := range row {
				if v {
					cells++
				}
			}
		}
		if cells != 40 {
			t.Fatalf("angle %g: sparse classification: %d, want 40", angle, cells)
		}
		// Final navigation can move native ground. The model-side boundary
		// must meet that final height, with no new footprint on native cells.
		for y := range terrain.GroundHeight {
			for x := range terrain.GroundHeight[y] {
				terrain.GroundHeight[y][x] = config.DataHeightMin()
			}
		}
		fitted := &terrainfit.Tileset{Width: 8, Height: 8, Labels: make([]uint8, 81)}
		if err := wc.FinishTilesetModels(context.Background(), manager, terrain, fitted, config.DefaultConfig()); err != nil {
			t.Fatal(err)
		}
		for _, id := range fitted.Labels {
			if id != 0 {
				t.Fatal("alpha cutout destroyed hypothetical native paint")
			}
		}
		welded := 0
		for _, gs := range visual.MDL.Geosets {
			for _, v := range gs.Vertices {
				p := imath.V3Rotate(v.Position, root.Rotation)
				gx := (p[0] - ext.Min[0]) / (ext.Max[0] - ext.Min[0]) * 8
				gy := (p[1] - ext.Min[1]) / (ext.Max[1] - ext.Min[1]) * 8
				d, _, _ := nearestNative(wc.modelCells, gx, gy)
				if d < 1e-5 {
					welded++
					if math.Abs(p[2]) > 1e-5 {
						t.Fatalf("unwelded boundary: %v", p)
					}
				}
			}
		}
		if welded == 0 {
			t.Fatal("fixture exercised no model boundary")
		}
		grid, err := tilesetGrid(manager, 8, 8)
		if err != nil {
			t.Fatal(err)
		}
		// Independent forward transform of an asymmetric source landmark.
		world := imath.V3Rotate(imath.Vector3{6, 2, 6}, root.Rotation)
		gx := (world[0] - ext.Min[0]) / (ext.Max[0] - ext.Min[0]) * 8
		gy := (ext.Max[1] - world[1]) / (ext.Max[1] - ext.Min[1]) * 8
		u := grid.Origin[0] + gx*grid.StepX[0] + gy*grid.StepY[0]
		v := grid.Origin[1] + gx*grid.StepX[1] + gy*grid.StepY[1]
		if math.Abs(u-32.25) > 1e-8 || math.Abs(v-48.25) > 1e-8 {
			t.Fatalf("angle %g: landmark %.4f %.4f", angle, u, v)
		}
	}
}

func TestChunkPrecisionGapsNeverAllocateModels(t *testing.T) {
	cfg := settings.DefaultMapExportConfig()
	cfg.TerrainAsWc3Tilesets = true
	manager := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	model := mdl.New(mdl.NewMDLOptions{Name: "terrain"})
	g := &components.Geoset{}
	for _, bounds := range [][2]float64{{0, 4 - 1e-4}, {4 + 1e-4, 8}} {
		a := &components.GeosetVertex{Position: imath.Vector3{bounds[0], 0, bounds[0]}}
		b := &components.GeosetVertex{Position: imath.Vector3{bounds[1], 0, bounds[1]}}
		c := &components.GeosetVertex{Position: imath.Vector3{bounds[1], 8, bounds[1]}}
		d := &components.GeosetVertex{Position: imath.Vector3{bounds[0], 8, bounds[0]}}
		g.Vertices = append(g.Vertices, a, b, c, d)
		g.Faces = append(g.Faces, components.Face{Vertices: [3]*components.GeosetVertex{a, b, c}}, components.Face{Vertices: [3]*components.GeosetVertex{a, c, d}})
	}
	model.Geosets = []*components.Geoset{g}
	root := &common.WowObject{Type: common.WowObjectADT, Model: &common.Model{RelativePath: "terrain", MDL: model}, ScaleFactor: 1}
	manager.Roots = []*common.WowObject{root}
	terrain := GetInitialTerrain(16, 16)
	wc := NewWc3Converter(cfg)
	if err := wc.prepareTilesetModels(context.Background(), manager, &terrain, 63, ""); err != nil {
		t.Fatal(err)
	}
	if len(wc.adtModels) != 0 {
		t.Fatal("models allocated on playable terrain")
	}
	for _, row := range terrain.GroundTexture {
		for _, id := range row {
			if id == 63 {
				t.Fatal("chunk crack painted as alpha")
			}
		}
	}
}

func TestBoundaryDistanceStaysOnModelSide(t *testing.T) {
	mask := [][]bool{{false, true, true}, {false, true, true}}
	d, x, y := nearestNative(mask, 1, .5)
	if d > 1e-5 || x >= 1 || y != .5 || boundaryWeight(d) > 1e-8 {
		t.Fatalf("boundary does not match native edge: %g %g %g", d, x, y)
	}
	if d, _, _ := nearestNative(mask, 2, .5); d != 1 || boundaryWeight(d) != 1 {
		t.Fatal("model interior must retain source detail")
	}
}

func TestNonBuildableStockIDsBackOnlyTheLeastPaintedFamilies(t *testing.T) {
	terrain := GetInitialTerrain(4, 4)
	least := []int{12, 1, 42, 7, 47, 0, 3, 20}
	counts := make([]int, 48)
	for i := range counts {
		counts[i] = 100 + i
	}
	for i, id := range least {
		counts[id] = i + 1
	}
	terrain.GroundTexture = [][]int{{}}
	for id, count := range counts {
		for i := 0; i < count; i++ {
			terrain.GroundTexture[0] = append(terrain.GroundTexture[0], id)
		}
	}
	// Model interiors are not painted with any of these 48 material families.
	terrain.GroundTexture[0] = append(terrain.GroundTexture[0], 48, 48, 48)
	slots, err := applyTilesetPalette(&terrain, &terrainfit.Tileset{Sheets: make([][]byte, 49)})
	if err != nil {
		t.Fatal(err)
	}
	expected := map[int]bool{}
	for _, id := range least {
		expected[id] = true
	}
	restricted := 0
	for id, slot := range slots {
		if !slot.buildable {
			restricted++
		}
		if slot.buildable == expected[id] {
			t.Fatalf("family %d (%d corners) got buildable=%v", id, counts[id], slot.buildable)
		}
		if terrain.TilePalette[id] != slot.id {
			t.Fatal("palette changed fitted family order")
		}
	}
	if restricted != 8 || terrain.TilePalette[48] != "Maph" {
		t.Fatalf("wrong mixed palette: restricted=%d palette=%v", restricted, terrain.TilePalette)
	}
}
