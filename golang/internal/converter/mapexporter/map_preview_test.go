package mapexporter

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/liquid"
)

func TestPreviewLiquidMaskHeightsAndCache(t *testing.T) {
	root := adt.NewADTLoader(nil)
	root.Chunks = []adt.ADTChunk{{Position: [3]float32{10, 20, 500}}}
	root.LiquidChunks = []adt.LiquidChunk{{Instances: []adt.LiquidInstance{{LiquidType: 2, Width: 2, Height: 1, Bitmap: []byte{1}}}}}
	surfaces, err := root.LiquidSurfaces()
	if err != nil {
		t.Fatal(err)
	}
	origin := imath.Vector3{-25, 5, 450}
	surfaces = liquid.Transform(surfaces, func(p imath.Vector3) imath.Vector3 { return imath.V3Sub(imath.Vector3{-p[1], p[0], p[2]}, origin) })
	b := previewBuilder{ctx: context.Background(), options: PreviewOptions{CacheDir: t.TempDir()}}
	a := PreviewAsset{ID: "tile"}
	if err := b.addLiquids(&a, surfaces); err != nil {
		t.Fatal(err)
	}
	if len(a.Parts) != 1 || a.Parts[0].Vertices != 6 {
		t.Fatalf("dry mask cells became water: %+v", a)
	}
	p := a.Parts[0]
	raw, err := os.ReadFile(b.full(p.Mesh))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float32{5, 5, -450} {
		got := math.Float32frombits(binary.LittleEndian.Uint32(raw[i*4:]))
		if got != want {
			t.Fatalf("sea level moved or chunk Z was added: axis %d = %v, want %v", i, got, want)
		}
	}
	// The instance reverses the origin offset, preserving the absolute sea level.
	world := previewApply(previewTransform(origin, imath.Vector3{}, 1), p.Liquid.Min)
	if world[2] != 0 || p.Liquid.TypeID != 2 {
		t.Fatal("liquid placement or type changed", world)
	}
	scene := MapPreview{Assets: []PreviewAsset{a}, Instances: []PreviewInstance{{Asset: "tile"}}}
	if !b.assetsPresent(&scene) {
		t.Fatal("water mesh should be reusable")
	}
	if err := os.WriteFile(b.full(p.Mesh), []byte{1}, 0644); err != nil {
		t.Fatal(err)
	}
	if b.assetsPresent(&scene) {
		t.Fatal("truncated water meshes must invalidate the scene")
	}
}

func TestRecommendedTerrainClampWidth(t *testing.T) {
	for _, tc := range []struct {
		name            string
		max             imath.Vector3
		unitScale, want float64
	}{
		{"normal", imath.Vector3{10000, 10000, 10000}, 2, .768},
		{"wide", imath.Vector3{122881, 100, 10000}, 1, .76800625},
		{"tall", imath.Vector3{100, 122881, 10000}, 1, .76800625},
		{"rounded", imath.Vector3{61952, 100, 10000}, 1, .384 * 484 / 480},
		{"full range", imath.Vector3{100, 100, 100}, 2, 1},
		{"flat", imath.Vector3{100, 100, 0}, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			width := recommendedTerrainClampWidth(imath.Vector3{}, tc.max, tc.unitScale)
			if math.Abs(width-tc.want) > 1e-12 {
				t.Fatalf("width=%g, want %g", width, tc.want)
			}
			if tc.max[2] > 0 {
				recommended := SuggestTerrainClamp(imath.Vector3{}, tc.max, []float64{tc.max[2] / 2}, tc.unitScale, TerrainClampPercent{Upper: 1})
				wantSuggestion := math.Min(1, math.Max(minimumTerrainClampWidth(imath.Vector3{}, tc.max), .8*width))
				if math.Abs(recommended.Upper-recommended.Lower-wantSuggestion) > 1e-12 {
					t.Fatalf("suggestion must use 80%% of reference unless map-size limited: %+v vs %g", recommended, wantSuggestion)
				}
			}
		})
	}
}

func TestPreviewTerrainBoundsExcludeHoleOnlyVertices(t *testing.T) {
	for _, chunk := range []previewChunk{
		{Holes: 1},
		{Flags: 0x10000, HighHoles: []uint8{1}},
	} {
		used := previewTerrainVertices(chunk)
		if used[9] || used[0] || !used[144] {
			t.Fatal("hole-only vertices must not enlarge export bounds, while solid terrain remains included")
		}
	}
	used := previewTerrainVertices(previewChunk{})
	for i, value := range used {
		if !value {
			t.Fatalf("solid chunk vertex %d was excluded", i)
		}
	}
}

func TestSuggestTerrainClamp(t *testing.T) {
	min, max := imath.Vector3{}, imath.Vector3{1000, 1000, 15360}
	heights := []float64{.05 * 15360, .6 * 15360, .62 * 15360, .65 * 15360}
	current := TerrainClampPercent{Lower: 0, Upper: 1}
	got := SuggestTerrainClamp(min, max, heights, 1, current)
	if math.Abs(got.Lower-.45) > 1e-8 || math.Abs(got.Upper-.65) > 1e-8 {
		t.Fatalf("expected earliest range containing the dense group, got %+v", got)
	}
	wide := max
	wide[0] = 200000
	limited := SuggestTerrainClamp(min, wide, heights, 1, current)
	width := wide[0] * config.MaxGameHeightDiff() / (wide[2] * (limited.Upper - limited.Lower)) / config.DistancePerTile
	if width > 480 {
		t.Fatalf("suggested range exceeds native map width: %f", width)
	}
	for _, heights := range [][]float64{nil, {1, 2}} {
		flat := max
		flat[2] = 0
		if result := SuggestTerrainClamp(min, flat, heights, 2, current); result != current {
			t.Fatal("flat or empty data must keep the current range")
		}
	}
	preview := MapPreview{Min: imath.Vector3{}, Max: imath.Vector3{1000. / 56, 1000. / 56, 15360. / 56}, RawModelScale: 56}
	for _, h := range heights {
		preview.NPCs = append(preview.NPCs, PreviewNPC{Position: imath.Vector3{0, 0, h / 56}})
	}
	if result := SuggestPreviewClamp(preview, 1); math.Abs(result.Upper-got.Upper) > 1e-8 || math.Abs(result.Lower-got.Lower) > 1e-8 {
		t.Fatalf("preview and production must choose the same range: %+v vs %+v", result, got)
	}
}

type previewNoWMOClient struct {
	client.Client
	calls int
}

func (c *previewNoWMOClient) GetFileByID(context.Context, int) (casc.ListfileEntry, error) {
	c.calls++
	return casc.ListfileEntry{}, errors.New("unexpected WMO read")
}

func TestPreviewSuggestionSkipsWMOs(t *testing.T) {
	c := &previewNoWMOClient{}
	opts := PreviewOptions{Client: c, CacheDir: t.TempDir(), MapDir: "azeroth", Tiles: []TileCoord{{X: 0, Y: 0}}, SuggestOnly: true}
	var progress []PreviewProgress
	opts.Progress = func(p PreviewProgress) { progress = append(progress, p) }
	b := previewBuilder{ctx: context.Background(), options: opts, prefix: MapPreviewCacheDirectory + "/" + previewHash("")}
	stem := b.prefix + "/adt-bounds-v2-" + previewHash("azeroth") + "-0-0"
	tile := previewTile{Min: imath.Vector3{}, Max: imath.Vector3{100, 100, 100}, Chunks: []previewChunk{{Heights: make([]float32, 145)}}, Placements: []previewPlacement{{ID: 123, UID: 1}}, Asset: PreviewAsset{ID: stem, Parts: []PreviewPart{{Mesh: stem + ".bin", Vertices: 3}}}}
	if err := b.write(stem+".bin", make([]byte, 96)); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(tile)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.write(stem+".json", raw); err != nil {
		t.Fatal(err)
	}
	result, err := BuildMapPreview(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if c.calls != 0 || len(result.Instances) != 1 {
		t.Fatal("clamp suggestion must not wait for WMO processing")
	}
	found := false
	for _, p := range progress {
		if p.Message == "Preparing terrain" && p.Completed == 1 && p.Total == 1 {
			found = true
		}
	}
	if !found {
		t.Fatal("terrain completion was not reported", progress)
	}
	progress = nil
	if _, err := BuildMapPreview(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if len(progress) != 2 || progress[1].Message != "Preview ready" {
		t.Fatal("warm cache should skip completed preparation", progress)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts.SuggestOnly = false
	opts.Progress = func(p PreviewProgress) {
		if p.Message == "Preparing terrain" && p.Completed == 1 {
			cancel()
		}
	}
	if _, err := BuildMapPreview(ctx, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("progress cancellation did not stop work: %v", err)
	}
	if c.calls != 0 || !b.assetsPresent(&tile) {
		t.Fatal("cancel must stop WMO work and preserve completed assets")
	}
}

func TestPreviewWMOBinaryFloorCache(t *testing.T) {
	b := previewBuilder{ctx: context.Background(), prefix: "preview", options: PreviewOptions{CacheDir: t.TempDir()}}
	stem := "preview/wmo-7"
	metadata := previewWMO{Asset: PreviewAsset{ID: stem}, FloorFile: stem + "-floors.bin", FloorCount: 1}
	if err := b.write(metadata.FloorFile, previewVertexBytes([]float32{1, 2, 3, 4, 5, 6, 7, 8, 9})); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.write(stem+".json", raw); err != nil {
		t.Fatal(err)
	}
	result, err := b.wmo(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Floors) != 1 || result.Floors[0].V[2] != (imath.Vector3{7, 8, 9}) {
		t.Fatalf("cached WMO floor coordinates changed: %+v", result.Floors)
	}
	if err := os.WriteFile(b.full(metadata.FloorFile), []byte{1}, 0644); err != nil {
		t.Fatal(err)
	}
	if b.assetsPresent(&metadata) {
		t.Fatal("truncated floor data must invalidate the WMO cache")
	}
}

func TestPreviewTerrainLODAndSourceFloor(t *testing.T) {
	heights := make([]float32, 145)
	heights[9] = 20 // Center of the first quad, intentionally above its corners.
	ch := previewChunk{Heights: heights, Position: [3]float32{0, 0, 10}, Flags: 0x10000, HighHoles: make([]uint8, 8)}
	tile := previewTile{Min: imath.Vector3{0, -100, 10}, Max: imath.Vector3{100, 0, 30}, Chunks: []previewChunk{ch}}
	mesh := previewTerrainMesh(tile)
	if len(mesh)/8/3 != 32 {
		t.Fatalf("a high-resolution-hole flag with no holes must still use thumbnail LOD, got %d triangles", len(mesh)/8/3)
	}
	z, ok := previewGround(tile.Chunks, imath.Vector3{533.3333333333333 / 256, -533.3333333333333 / 256, 30})
	if !ok || z != 30 {
		t.Fatalf("NPC floors must retain exact center heights, got %f,%t", z, ok)
	}
	ch.HighHoles[0] = 1
	if _, ok := previewGround([]previewChunk{ch}, imath.Vector3{1, -1, 30}); ok {
		t.Fatal("terrain holes must not provide NPC floors")
	}
	tile.Chunks = []previewChunk{ch}
	if triangles := len(previewTerrainMesh(tile)) / 8 / 3; triangles != 126 {
		t.Fatalf("hole chunk must retain fine edges and skip its hole: %d", triangles)
	}
	for i := 0; i < len(mesh); i += 8 {
		if mesh[i+5] <= 0 || mesh[i+6] < 0 || mesh[i+6] > 1 || mesh[i+7] < 0 || mesh[i+7] > 1 {
			t.Fatalf("bad terrain normal/UV at vertex %d", i/8)
		}
	}
}

func TestPreviewCacheReuseRepairAndCancellation(t *testing.T) {
	b := previewBuilder{ctx: context.Background(), options: PreviewOptions{CacheDir: t.TempDir()}}
	builds := 0
	var out previewTile
	build := func() error {
		builds++
		out = previewTile{Chunks: []previewChunk{{Heights: make([]float32, 145)}}, Asset: PreviewAsset{ID: "tile", Parts: []PreviewPart{{Mesh: "tile.bin", Vertices: 3}}}}
		return b.write("tile.bin", make([]byte, 96))
	}
	if err := b.cached("tile.json", &out, build); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(b.full("tile.bin"))
	out = previewTile{}
	if err := b.cached("tile.json", &out, build); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(b.full("tile.bin"))
	if builds != 1 || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("warm previews must reuse assets")
	}
	if err := os.WriteFile(b.full("tile.bin"), []byte{0}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := b.cached("tile.json", &out, build); err != nil {
		t.Fatal(err)
	}
	if builds != 2 {
		t.Fatal("truncated assets must be regenerated")
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.ctx = ctx
	err := b.cached("cancelled.json", &out, func() error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("late success must not persist a cancelled preview: %v", err)
	}
	if _, err := os.Stat(b.full("cancelled.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("cancelled preview manifest was written")
	}
}
