package extra_test

import (
	"os"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wc3/translators"
)

func TestEnsureMapInfoMatchesHiveWEDefaults(t *testing.T) {
	t.Parallel()
	mm := extra.NewMapManager()
	mm.SetTerrain(data.Terrain{
		Tileset: "L",
		Map: data.MapSize{
			Width:  64,
			Height: 64,
			Offset: data.Offset{X: -4096, Y: -4096},
		},
	})
	mm.EnsureMapInfo("test-map.w3x")

	if mm.Info.Saves != 1 || mm.Info.EditorVersion != 6116 {
		t.Fatalf("version metadata = saves %d editor %d", mm.Info.Saves, mm.Info.EditorVersion)
	}
	if mm.Info.Map.MainTileType != "L" {
		t.Fatalf("tileset = %q", mm.Info.Map.MainTileType)
	}
	if mm.Info.Map.PlayableArea.Width != 64 || mm.Info.Map.PlayableArea.Height != 64 {
		t.Fatalf("playable area = %dx%d", mm.Info.Map.PlayableArea.Width, mm.Info.Map.PlayableArea.Height)
	}
	if len(mm.Info.Players) != 1 || mm.Info.Players[0].Type != 1 {
		t.Fatalf("players = %+v", mm.Info.Players)
	}
	if len(mm.Info.Forces) != 1 || mm.Info.Forces[0].Players != -1 {
		t.Fatalf("forces = %+v", mm.Info.Forces)
	}
	if mm.Info.Fog.Color[3] != 255 || mm.Info.Water[0] != 255 {
		t.Fatalf("fog/water defaults wrong: fog=%v water=%v", mm.Info.Fog.Color, mm.Info.Water)
	}

	dir := t.TempDir()
	if err := mm.Save(dir); err != nil {
		t.Fatal(err)
	}
	w3i, err := os.ReadFile(dir + "/war3map.w3i")
	if err != nil {
		t.Fatal(err)
	}
	parsed := translators.InfoTranslator{}.WarToJSON(w3i).JSON
	if parsed.Map.Name != "test-map" {
		t.Fatalf("map name = %q", parsed.Map.Name)
	}
	if parsed.Map.MainTileType != "L" {
		t.Fatalf("parsed tileset = %q", parsed.Map.MainTileType)
	}
}
