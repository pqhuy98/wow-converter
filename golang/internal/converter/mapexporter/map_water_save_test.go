package mapexporter

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/liquid"
)

func TestWaterSaveIndependentOfNPCsAndOptionalBlockers(t *testing.T) {
	tn := mapterrain.GetInitialTerrain(4, 4)
	for y := range tn.GroundHeight {
		for x := range tn.GroundHeight[y] {
			tn.GroundHeight[y][x] = config.GameZToDataHeight(0)
		}
	}
	w := mapterrain.NewMapWater(tn, []liquid.Surface{{TypeID: 5, Cells: [][4]imath.Vector3{{{-256, -256, 20}, {256, -256, 20}, {256, 256, 20}, {-256, 256, 20}}}}}, 0)
	if err := w.Project(context.Background(), tn); err != nil {
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
	var report mapterrain.WaterReport
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
	if err = w.Project(ctx, tn); err == nil {
		t.Fatal("ignored cancellation")
	}
}
