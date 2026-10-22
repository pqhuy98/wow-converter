package terrain

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

func testWater(t data.Terrain, cells ...waterCell) *MapWater {
	w := &MapWater{width: t.Map.Width, height: t.Map.Height, ox: float64(t.Map.Offset.X), oy: float64(t.Map.Offset.Y), cells: cells, report: WaterReport{Types: map[uint16]int{}, UnsupportedReasons: map[string]int{}, MinZ: math.Inf(1), MaxZ: math.Inf(-1)}}
	w.index()
	return w
}

func waterQuad(x0, y0, x1, y1, z float64) waterCell {
	return waterCell{bank: 0, typeID: 5, v: [4]imath.Vector3{{x0, y0, z}, {x1, y0, z}, {x1, y1, z}, {x0, y1, z}}}
}

func TestWaterNativeSlopePathingAndBridge(t *testing.T) {
	tn := GetInitialTerrain(4, 4)
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
	if err := w.Project(context.Background(), tn); err != nil {
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
	if err := w.Project(context.Background(), tn); err != nil || w.report.NativeCorners != corners {
		t.Fatal("repeated projection must not accumulate water", err)
	}
	if !w.blocksGround(0, 0, 0) || w.blocksGround(0, 0, 150) {
		t.Fatal("ground water depth must retain dry bridges")
	}
	dir := t.TempDir()
	if err := w.Write(context.Background(), tn, nil, dir, false); err != nil {
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
	if err := w.Write(context.Background(), tn, nil, dir, false); err != nil {
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
	tn := GetInitialTerrain(4, 4)
	for y := range tn.GroundHeight {
		for x := range tn.GroundHeight[y] {
			tn.GroundHeight[y][x] = config.GameZToDataHeight(0)
		}
	}
	w := testWater(tn, waterQuad(12, 12, 20, 20, 50), waterQuad(-200, -200, -180, -180, 5000))
	if err := w.Project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	if _, present := nativeWaterHeight(tn, 16, 16); !present {
		t.Fatal("small pool must use the native water grid")
	}
	if w.report.UnsupportedCells != 1 || w.report.UnsupportedReasons["height-range"] != 1 {
		t.Fatal("unsupported water height must be reported", w.report)
	}
	w = testWater(tn, waterQuad(-256, -256, 256, 256, 20), waterQuad(-256, -256, 256, 256, 100))
	if err := w.Project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	if w.report.ApproximatedCells == 0 {
		t.Fatal("single native water layer must report stacked-source approximation")
	}
	lava := waterQuad(-256, -256, 256, 256, 30)
	lava.bank = 2
	w = testWater(tn, lava)
	if err := w.Project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	if w.report.UnsupportedReasons["material"] != 1 || w.report.NativeCorners != 0 {
		t.Fatal("magma must not silently become ordinary native water", w.report)
	}
	dir := t.TempDir()
	if err := w.Write(context.Background(), tn, nil, dir, false); err != nil {
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
	tn := GetInitialTerrain(4, 4)
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
	if err := w.Project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	for x := -240.; x < 256; x += 32 {
		if z, present := nativeWaterHeight(tn, x, 16); !present || math.Abs(z-100) > .126 {
			t.Fatal("native river contains a missing-water strip", x, z, present)
		}
	}
}

func TestWaterCoversEveryCornerOfPartiallyWetNativeQuads(t *testing.T) {
	for _, source := range []waterCell{
		waterQuad(-2, -2, 2, 2, 50),     // A tiny pool straddles four native quads.
		waterQuad(120, 24, 129, 28, 50), // Only one unit overlaps the right-hand quad; no sample center hits it.
	} {
		tn := GetInitialTerrain(4, 4)
		for y := range tn.GroundHeight {
			for x := range tn.GroundHeight[y] {
				tn.GroundHeight[y][x] = config.GameZToDataHeight(0)
			}
		}
		w := testWater(tn, source)
		if err := w.Project(context.Background(), tn); err != nil {
			t.Fatal(err)
		}
		wetQuads := 0
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				if !waterTouchesWetTerrain(source, w.ox+float64(x)*128, w.oy+float64(y)*128, tn) {
					continue
				}
				wetQuads++
				for _, d := range [][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
					xx, yy := x+d[0], y+d[1]
					if tn.Flags[yy][xx]&uint16(data.TerrainWater) == 0 || math.Abs(config.WaterHeightToGameZ(float64(tn.WaterHeight[yy][xx]))-50) > .126 {
						t.Fatalf("partially wet quad %d,%d has a dry/uninitialized corner %d,%d", x, y, xx, yy)
					}
				}
			}
		}
		if wetQuads < 2 {
			t.Fatal("fixture must exercise multiple touched quads", wetQuads)
		}
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				if _, present := nativeWaterHeight(tn, w.ox+(float64(x)+.5)*128, w.oy+(float64(y)+.5)*128); !present {
					continue
				}
				for _, d := range [][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
					if math.Abs(config.WaterHeightToGameZ(float64(tn.WaterHeight[y+d[1]][x+d[0]]))-50) > .126 {
						t.Fatal("rounded shore slopes to an uninitialized outer corner", x, y, d)
					}
				}
			}
		}
	}
}

func TestWaterMinorityShoreAndFullyDryBridge(t *testing.T) {
	tn := GetInitialTerrain(4, 4)
	for y := range tn.GroundHeight {
		for x := range tn.GroundHeight[y] {
			tn.GroundHeight[y][x] = config.GameZToDataHeight(100)
		}
	}
	tn.GroundHeight[3][3] = config.GameZToDataHeight(0)
	source := waterQuad(0, 0, 128, 128, 50)
	w := testWater(tn, source)
	if err := w.Project(context.Background(), tn); err != nil {
		t.Fatal(err)
	}
	for _, d := range [][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		if tn.Flags[2+d[1]][2+d[0]]&uint16(data.TerrainWater) == 0 {
			t.Fatal("mostly-land shoreline quad must remain fully water-enabled")
		}
	}
	if tn.GroundHeight[2][2] != config.GameZToDataHeight(100) {
		t.Fatal("shoreline coverage must not lower the land")
	}
	tn.GroundHeight[3][3] = config.GameZToDataHeight(100)
	if err := w.Project(context.Background(), tn); err != nil || w.report.NativeCorners != 0 {
		t.Fatal("a bridge above all source water must stay dry", err, w.report)
	}
}

func TestWaterOverlapChecksWetEdgeExtremaAndIgnoresBoundaryOnlyContact(t *testing.T) {
	tn := GetInitialTerrain(2, 2)
	for y := range tn.GroundHeight {
		for x := range tn.GroundHeight[y] {
			tn.GroundHeight[y][x] = config.GameZToDataHeight(100)
		}
	}
	tn.GroundHeight[0][1], tn.GroundHeight[1][0] = config.GameZToDataHeight(0), config.GameZToDataHeight(0)
	// Every source corner is above dry land, but the middle of its diagonal
	// edge crosses a wet low point in the bilinear terrain.
	c := waterCell{bank: 0, v: [4]imath.Vector3{{-128, -128, 55}, {-127, -128, 55}, {0, 0, 55}, {-1, 0, 55}}}
	if !waterTouchesWetTerrain(c, -128, -128, tn) {
		t.Fatal("wet edge interior was missed")
	}
	if waterTouchesWetTerrain(c, 0, -128, tn) {
		t.Fatal("zero-area boundary contact should not add an extra water quad")
	}
}
