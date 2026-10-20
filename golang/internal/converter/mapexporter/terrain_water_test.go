package mapexporter

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"

	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func testWater(t data.Terrain, cells ...waterCell) *mapWater {
	w := &mapWater{width: t.Map.Width, height: t.Map.Height, ox: float64(t.Map.Offset.X), oy: float64(t.Map.Offset.Y), cells: cells, report: waterReport{Types: map[uint16]int{}, UnsupportedReasons: map[string]int{}, MinZ: math.Inf(1), MaxZ: math.Inf(-1)}}
	w.index()
	return w
}

func waterQuad(x0, y0, x1, y1, z float64) waterCell {
	return waterCell{bank: 0, typeID: 5, v: [4]imath.Vector3{{x0, y0, z}, {x1, y0, z}, {x1, y1, z}, {x0, y1, z}}}
}

func TestWaterNativeSlopePathingAndBridge(t *testing.T) {
	tn := getInitialTerrain(4, 4)
	for y := range tn.GroundHeight {
		for x := range tn.GroundHeight[y] {
			tn.GroundHeight[y][x] = config.GameZToDataHeight(0)
		}
	}
	c := waterQuad(-256, -256, 256, 256, 100)
	for i := range c.v {
		c.v[i][2] += .1 * c.v[i][0]
	}
	w := testWater(tn, c)
	if err := w.project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	for _, p := range []imath.Vector3{{-200, -100, 80}, {0, 0, 100}, {160, 20, 116}} {
		z, ok := nativeWaterHeight(tn, p[0], p[1])
		if !ok || math.Abs(z-p[2]) > .126 {
			t.Fatal("sloped water flattened", p, z)
		}
	}
	if w.report.UnsupportedCells != 0 {
		t.Fatal("planar water should fit natively")
	}
	corners := w.report.NativeCorners
	if err := w.project(context.Background(), tn); err != nil || w.report.NativeCorners != corners {
		t.Fatal("repeated projection must not accumulate water", err)
	}
	if !w.blocksGround(0, 0, 0) || w.blocksGround(0, 0, 150) {
		t.Fatal("ground water depth must retain dry bridges")
	}
	dir := t.TempDir()
	if err := w.write(context.Background(), tn, nil, dir, false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "war3map.wpm"))
	if int(binary.LittleEndian.Uint32(b[8:])) != 16 {
		t.Fatal("wrong pathing resolution")
	}
	for _, flags := range b[16:] {
		if flags != 64|8|2 {
			t.Fatalf("deep water must block walkers/buildings and allow aircraft: %d", flags)
		}
	}
	// An upper bridge floor must not inherit the water mask below it.
	for y := range tn.GroundHeight {
		for x := range tn.GroundHeight[y] {
			tn.GroundHeight[y][x] = config.GameZToDataHeight(150)
		}
	}
	if err := w.write(context.Background(), tn, nil, dir, false); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, "war3map.wpm"))
	for _, flags := range b[16:] {
		if flags != 0 {
			t.Fatal("bridge was flooded", flags)
		}
	}
}

func TestWaterNativeOnlySmallPoolsAndUnsupportedLayers(t *testing.T) {
	tn := getInitialTerrain(4, 4)
	for y := range tn.GroundHeight {
		for x := range tn.GroundHeight[y] {
			tn.GroundHeight[y][x] = config.GameZToDataHeight(0)
		}
	}
	w := testWater(tn, waterQuad(12, 12, 20, 20, 50), waterQuad(-200, -200, -180, -180, 5000))
	if err := w.project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	if _, present := nativeWaterHeight(tn, 16, 16); !present {
		t.Fatal("small pool must use the native water grid")
	}
	if w.report.UnsupportedCells != 1 || w.report.UnsupportedReasons["height-range"] != 1 {
		t.Fatal("unsupported water height must be reported", w.report)
	}
	w = testWater(tn, waterQuad(-256, -256, 256, 256, 20), waterQuad(-256, -256, 256, 256, 100))
	if err := w.project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	if w.report.ApproximatedCells == 0 {
		t.Fatal("single native water layer must report stacked-source approximation")
	}
	lava := waterQuad(-256, -256, 256, 256, 30)
	lava.bank = 2
	w = testWater(tn, lava)
	if err := w.project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	if w.report.UnsupportedReasons["material"] != 1 || w.report.NativeCorners != 0 {
		t.Fatal("magma must not silently become ordinary native water", w.report)
	}
	dir := t.TempDir()
	if err := w.write(context.Background(), tn, nil, dir, false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "war3map.wpm"))
	for _, flags := range b[16:] {
		if flags != 10 {
			t.Fatal("magma must not become swimming water", flags)
		}
	}
}

func TestWaterNativeNarrowRiverAndChunkSeams(t *testing.T) {
	tn := getInitialTerrain(4, 4)
	for y := range tn.GroundHeight {
		for x := range tn.GroundHeight[y] {
			tn.GroundHeight[y][x] = config.GameZToDataHeight(0)
		}
	}
	// The former strict mask clipping cleared all corners of a narrow river.
	// Independent float32 chunk origins can also leave subpixel gaps at x=0.
	w := testWater(tn, waterQuad(-256, -60, -.02, 60, 100), waterQuad(.02, -60, 256, 60, 100))
	if _, _, wet := w.sample(0, 16, 0, true); !wet {
		t.Fatal("float32 chunk seam was treated as a real dry strip")
	}
	if _, _, wet := w.sample(0, 61, 0, true); wet {
		t.Fatal("seam tolerance must not erase real source mask edges")
	}
	if err := w.project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	for x := -240.; x < 256; x += 32 {
		if z, present := nativeWaterHeight(tn, x, 16); !present || math.Abs(z-100) > .126 {
			t.Fatal("native river contains a missing-water strip", x, z, present)
		}
	}
}
func TestWaterSaveIndependentOfNPCsAndOptionalBlockers(t *testing.T) {
	tn := getInitialTerrain(4, 4)
	for y := range tn.GroundHeight {
		for x := range tn.GroundHeight[y] {
			tn.GroundHeight[y][x] = config.GameZToDataHeight(0)
		}
	}
	w := testWater(tn, waterQuad(-256, -256, 256, 256, 20))
	if err := w.project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	mm := extra.NewMapManager()
	mm.SetTerrain(tn)
	cfg := DefaultMapExportConfig()
	e := &MapExporter{MapExportConfig: &cfg, MapManager: mm, water: w}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "water"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"liquid-0000.mdx", "liquid-0000.mdl", "liquid-0.tga", "custom.mdl", "liquid-custom.mdx"} {
		if err := os.WriteFile(filepath.Join(dir, "water", name), []byte("old import"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.SaveWar3mapFiles(context.Background(), dir, "water"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "water"))
	if err != nil || len(entries) != 2 || entries[0].Name() != "custom.mdl" || entries[1].Name() != "liquid-custom.mdx" || len(mm.Doodads) != 0 {
		t.Fatal("native water must remove legacy generated meshes and preserve other imports", entries, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "war3map.wpm"))
	if err != nil {
		t.Fatal("water pathing was removed by blocker opt-out", err)
	}
	for _, flags := range b[16:] {
		if flags != 64|8 {
			t.Fatal("shallow water must remain walkable with blockers disabled", flags)
		}
	}
	j, _ := os.ReadFile(filepath.Join(dir, "terrain-water.json"))
	var report waterReport
	if err = json.Unmarshal(j, &report); err != nil {
		t.Fatal(err)
	}
	if report.WaterPathingCells != 256 || report.DeepPathingCells != 0 {
		t.Fatal("wrong water diagnostics", report)
	}
	e.water = nil
	if err := e.SaveWar3mapFiles(context.Background(), dir, "dry"); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, "war3map.wpm")); !os.IsNotExist(err) {
		t.Fatal("stale water pathing retained")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = w.project(ctx, tn); err == nil {
		t.Fatal("ignored cancellation")
	}
}
