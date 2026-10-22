package mapexporter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestNavigationWithoutBlockersClearsCachedPathing(t *testing.T) {
	if DefaultMapExportConfig().PlacePathBlockers {
		t.Fatal("path blockers must be opt-in")
	}
	dir := t.TempDir()
	r := &mapterrain.TerrainNavigation{Width: 2, Height: 2, Pathing: []byte{14, 14, 14, 14}}
	if err := r.Write(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := r.Write(dir, false); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultMapExportConfig()
	mm := extra.NewMapManager()
	mm.SetTerrain(mapterrain.GetInitialTerrain(2, 3))
	exporter := &MapExporter{MapExportConfig: &cfg, MapManager: mm}
	if err := exporter.SaveWar3mapFiles(context.Background(), dir, "no-blockers"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "war3map.wpm")); !os.IsNotExist(err) {
		t.Fatalf("disabled blockers left cached engine pathing: %v", err)
	}
	for _, name := range []string{"terrain-navigation.json", "terrain-navigation.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}
