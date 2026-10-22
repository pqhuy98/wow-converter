package terrain

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	settings "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func navigationFixture(w, h int) *navigationGrid {
	g := &navigationGrid{width: w, height: h, minZ: -1000, maxZ: 1000, limits: settings.NavigationConfig{
		Radius: 4, Height: 40, StepHeight: 8, MaxSlopeDegrees: 45, JumpHeight: 24, JumpDistance: 96, MaxDrop: 64, SeedSnap: 16, AirBlockHeight: 80}}
	g.bins = make([][]int, w*h)
	g.cells = make([][]int, w*h)
	return g
}
func navigationFloor(g *navigationGrid, x0, y0, x1, y1, z float64) {
	a, b, c, d := imath.Vector3{x0, y0, z}, imath.Vector3{x1, y0, z}, imath.Vector3{x1, y1, z}, imath.Vector3{x0, y1, z}
	g.addTriangle([3]imath.Vector3{a, b, c})
	g.addTriangle([3]imath.Vector3{a, c, d})
}
func navigationSearchFixture(t *testing.T, g *navigationGrid, seeds ...imath.Vector3) *TerrainNavigation {
	t.Helper()
	if err := g.rasterize(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := &TerrainNavigation{Width: g.width, Height: g.height}
	q := g.seed(seeds, r)
	if err := g.search(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	g.projectJumps()
	return r
}
func navigationCellReached(g *navigationGrid, x, y int) bool {
	for _, id := range g.cells[y*g.width+x] {
		if g.spans[id].owner >= 0 {
			return true
		}
	}
	return false
}

func TestNavigationCannotWalkOrJumpThroughTallWall(t *testing.T) {
	g := navigationFixture(8, 4)
	navigationFloor(g, 0, 0, 256, 128, 0)
	g.addTriangle([3]imath.Vector3{{128, 0, 0}, {128, 128, 0}, {128, 128, 160}})
	g.addTriangle([3]imath.Vector3{{128, 0, 0}, {128, 128, 160}, {128, 0, 160}})
	navigationSearchFixture(t, g, imath.Vector3{16, 16, 0})
	if !navigationCellReached(g, 3, 1) || navigationCellReached(g, 4, 1) {
		t.Fatal("wall must separate the reachable floor")
	}
}

func TestNavigationWalksGentlySlopingRoadWithRadius(t *testing.T) {
	g := navigationFixture(8, 4)
	g.limits.Radius = 10
	g.addTriangle([3]imath.Vector3{{0, 0, 0}, {256, 0, 25.6}, {256, 128, 25.6}})
	g.addTriangle([3]imath.Vector3{{0, 0, 0}, {256, 128, 25.6}, {0, 128, 0}})
	r := navigationSearchFixture(t, g, imath.Vector3{48, 48, 4.8})
	if r.GroundNPCs != 1 || !navigationCellReached(g, 6, 1) {
		t.Fatalf("normal road slope must support an NPC and remain connected: %+v", r)
	}
}

func TestNavigationProjectsSlopingRoadAtNativeCorners(t *testing.T) {
	g := navigationFixture(16, 8)
	g.addTriangle([3]imath.Vector3{{0, 0, 0}, {512, 0, 256}, {512, 256, 320}})
	g.addTriangle([3]imath.Vector3{{0, 0, 0}, {512, 256, 320}, {0, 256, 64}})
	r := navigationSearchFixture(t, g, imath.Vector3{48, 48, 36})
	terrain := GetInitialTerrain(2, 4)
	g.project(terrain, r)
	if r.Reachable != 16*8 || r.ProjectionBlocked != 0 {
		t.Fatalf("a representable planar road must stay entirely walkable: %+v", r)
	}
	for y, row := range terrain.GroundHeight {
		for x, z := range row {
			if math.Abs(config.DataHeightToGameZ(float64(z))-(float64(x)*64+float64(y)*32)) > .25 {
				t.Fatalf("corner (%d,%d) must match road plane", x, y)
			}
		}
	}
}

func TestNavigationWalksOverRoadCrestWithoutJumping(t *testing.T) {
	g := navigationFixture(8, 4)
	g.limits.JumpHeight = 0
	g.addTriangle([3]imath.Vector3{{0, 0, 0}, {128, 0, 32}, {128, 128, 32}})
	g.addTriangle([3]imath.Vector3{{0, 0, 0}, {128, 128, 32}, {0, 128, 0}})
	g.addTriangle([3]imath.Vector3{{128, 0, 32}, {256, 0, 0}, {256, 128, 0}})
	g.addTriangle([3]imath.Vector3{{128, 0, 32}, {256, 128, 0}, {128, 128, 32}})
	r := navigationSearchFixture(t, g, imath.Vector3{48, 48, 12})
	if r.GroundNPCs != 1 || !navigationCellReached(g, 6, 1) {
		t.Fatal("walking must follow the floor over a crest rather than intersect its chord")
	}
}

func TestNavigationCeilingUndersideIsNotGround(t *testing.T) {
	g := navigationFixture(4, 4)
	g.addTriangle([3]imath.Vector3{{0, 0, 0}, {128, 128, 0}, {128, 0, 0}})
	g.addTriangle([3]imath.Vector3{{0, 0, 0}, {0, 128, 0}, {128, 128, 0}})
	r := navigationSearchFixture(t, g, imath.Vector3{48, 48, 0})
	if r.GroundNPCs != 0 || len(g.spans) != 0 {
		t.Fatal("downward collision faces cannot seed a walkable surface")
	}
}

func TestNavigationNarrowRaisedRoadFitsAllSupportingCorners(t *testing.T) {
	g := navigationFixture(16, 8)
	navigationFloor(g, 96, 0, 128, 256, 200)
	r := navigationSearchFixture(t, g, imath.Vector3{112, 48, 200})
	terrain := GetInitialTerrain(2, 4)
	g.project(terrain, r)
	for y := 0; y < g.height; y++ {
		if r.Pathing[y*g.width+3] != 0 {
			t.Fatal("all native corners supporting a narrow raised road must follow it")
		}
	}
}

func TestNavigationJumpDiscoversLandingAndProjectsGap(t *testing.T) {
	g := navigationFixture(8, 4)
	navigationFloor(g, 0, 0, 64, 128, 0)
	navigationFloor(g, 96, 0, 256, 128, 0)
	navigationSearchFixture(t, g, imath.Vector3{16, 16, 0})
	if !navigationCellReached(g, 5, 1) || !navigationCellReached(g, 2, 1) || len(g.jumps) == 0 {
		t.Fatal("short jump must connect the landing and make the gap walkable")
	}
	g = navigationFixture(8, 4)
	g.limits.JumpHeight = 0
	navigationFloor(g, 0, 0, 64, 128, 0)
	navigationFloor(g, 96, 0, 256, 128, 0)
	navigationSearchFixture(t, g, imath.Vector3{16, 16, 0})
	if navigationCellReached(g, 5, 1) {
		t.Fatal("walking cannot cross a gap without jumping")
	}
}

func TestNavigationJumpConnectsTwoSeededPlatforms(t *testing.T) {
	g := navigationFixture(8, 4)
	navigationFloor(g, 0, 0, 64, 128, 0)
	navigationFloor(g, 96, 0, 256, 128, 0)
	navigationSearchFixture(t, g, imath.Vector3{16, 16, 0}, imath.Vector3{144, 16, 0})
	if !navigationCellReached(g, 2, 1) {
		t.Fatal("jump gap must be projected even if both landings already have NPC seeds")
	}
}

func TestNavigationWallSeparatesTwoSeededRooms(t *testing.T) {
	g := navigationFixture(8, 4)
	navigationFloor(g, 0, 0, 256, 128, 0)
	g.addTriangle([3]imath.Vector3{{128, 0, 0}, {128, 128, 0}, {128, 128, 160}})
	g.addTriangle([3]imath.Vector3{{128, 0, 0}, {128, 128, 160}, {128, 0, 160}})
	r := navigationSearchFixture(t, g, imath.Vector3{16, 16, 0}, imath.Vector3{208, 16, 0})
	terrain := GetInitialTerrain(1, 2)
	g.project(terrain, r)
	if r.Pathing[1*g.width+4]&2 == 0 {
		t.Fatal("wall needs a blocker even when both rooms are reachable from their own NPCs")
	}
	if r.Pathing[1*g.width+4]&4 == 0 {
		t.Fatal("tall vertical walls without a horizontal cap must also block flight")
	}
}

func TestNavigationPrefersFloorSupportedByMostNPCs(t *testing.T) {
	g := navigationFixture(8, 8)
	navigationFloor(g, 0, 0, 256, 256, 0)
	navigationFloor(g, 0, 0, 256, 256, 100)
	r := navigationSearchFixture(t, g, imath.Vector3{16, 16, 0}, imath.Vector3{48, 16, 0}, imath.Vector3{80, 16, 0}, imath.Vector3{16, 16, 100})
	terrain := GetInitialTerrain(2, 2)
	g.project(terrain, r)
	if r.GroundNPCs != 4 || r.Conflicts != 64 {
		t.Fatalf("unexpected seeds/overlaps: %+v", r)
	}
	for _, row := range terrain.GroundHeight {
		for _, z := range row {
			if config.DataHeightToGameZ(float64(z)) != 0 {
				t.Fatal("upper floor displaced NPC-supported lower floor")
			}
		}
	}
	if r.Reachable != 64 {
		t.Fatalf("selected floor should remain walkable: %+v", r)
	}
}

func TestNavigationRejectsFloatingNPCAndLowCeiling(t *testing.T) {
	g := navigationFixture(8, 4)
	navigationFloor(g, 0, 0, 256, 128, 0)
	navigationFloor(g, 128, 0, 256, 128, 30)
	r := navigationSearchFixture(t, g, imath.Vector3{16, 16, 100}, imath.Vector3{144, 16, 0}, imath.Vector3{16, 16, 0})
	if r.GroundNPCs != 1 || r.RejectedNPCs != 2 {
		t.Fatalf("airborne/low-clearance seeds must be rejected: %+v", r)
	}
}

func TestNavigationClassifiesTallAndLowUnreachableGround(t *testing.T) {
	g := navigationFixture(12, 4)
	navigationFloor(g, 0, 0, 128, 128, 0)
	navigationFloor(g, 128, 0, 256, 128, 60)
	navigationFloor(g, 256, 0, 384, 128, 200)
	g.limits.JumpHeight = 0
	r := navigationSearchFixture(t, g, imath.Vector3{16, 16, 0})
	terrain := GetInitialTerrain(1, 3)
	g.project(terrain, r)
	if r.Pathing[2*g.width+5]&4 != 0 || r.Pathing[2*g.width+9]&4 == 0 {
		t.Fatalf("low obstacle must allow flight; tall obstacle must block it: %+v", r)
	}
}

func TestNavigationCancellation(t *testing.T) {
	g := navigationFixture(8, 4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if g.rasterize(ctx) != context.Canceled {
		t.Fatal("rasterization must propagate cancellation")
	}
}

func TestBlockerPackingExactCoverAndDoodads(t *testing.T) {
	w, h := 80, 60
	cells := make([]byte, w*h)
	for i := range cells {
		cells[i] = 10
	}
	rects := packBlockers(cells, w, h)
	if len(rects) != 4 {
		t.Fatalf("80x60 rectangle should pack into four solid squares, got %d", len(rects))
	}
	for y := 12; y < 44; y++ {
		for x := 20; x < 55; x++ {
			cells[y*w+x] = 0
		}
	}
	for y := 5; y < 10; y++ {
		for x := 4; x < 70; x++ {
			cells[y*w+x] = 14
		}
	}
	rects = packBlockers(cells, w, h)
	actual := make([]byte, len(cells))
	for _, r := range rects {
		for y := r.y; y < r.y+r.size; y++ {
			for x := r.x; x < r.x+r.size; x++ {
				i := y*w + x
				if actual[i] != 0 && actual[i] != r.flags {
					t.Fatal("overlapping squares must have the same blocking class")
				}
				actual[i] = r.flags
			}
		}
	}
	for i := range cells {
		if cells[i] != actual[i] {
			t.Fatalf("incorrect coverage at %d", i)
		}
	}
	mm := extra.NewMapManager()
	mm.SetTerrain(GetInitialTerrain(h/4, w/4))
	dir := t.TempDir()
	r := &TerrainNavigation{Width: w, Height: h, Pathing: cells}
	if err := r.PlaceBlockers(mm, dir); err != nil {
		t.Fatal(err)
	}
	assertPathBlockerModel(t, mm, dir)
	if len(mm.DestructibleTypes) != 0 || len(mm.Doodads) != r.BlockerInstances {
		t.Fatal("blockers must be packed doodads, never destructibles")
	}
	for _, typ := range mm.DoodadTypes {
		if typ.IsDestructible {
			t.Fatal("destructible blocker")
		}
	}
	mm.EnsureMapInfo("navigation-test")
	if err := mm.Save(dir); err != nil {
		t.Fatal(err)
	}
	loaded := extra.NewMapManager()
	if err := loaded.Load(dir); err != nil {
		t.Fatal(err)
	}
	if len(loaded.DoodadTypes) != len(mm.DoodadTypes) || len(loaded.DestructibleTypes) != 0 || len(loaded.Doodads) != r.BlockerInstances {
		t.Fatal("serialized blockers must remain doodad types and instances")
	}
	assertSavedBlockers(t, dir, loaded, r.Pathing, w, h)
	b, err := os.ReadFile(filepath.Join(dir, "war3map.wpm"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b[:4]) != "MP3W" || int(binary.LittleEndian.Uint32(b[8:])) != w || len(b) != 16+w*h {
		t.Fatal("incorrect WPM layout")
	}
	tga := blockerTGA(blockerSquare{size: 3, flags: 14})
	if len(tga) != 18+3*3*3 || tga[19] != 255 {
		t.Fatal("both-blocker TGA must include flight channel")
	}
	if blockerTGA(blockerSquare{size: 1, flags: 10})[19] != 0 {
		t.Fatal("ground-only blocker must allow flight")
	}
	// Native object fields must use the doodad table, including dptx.
	found := false
	for _, mod := range mm.DoodadTypes[0].Data {
		if mod.ID == "dptx" && mod.Type == data.ModificationString {
			found = true
		}
	}
	if !found {
		t.Fatal("missing doodad pathing texture")
	}
}
