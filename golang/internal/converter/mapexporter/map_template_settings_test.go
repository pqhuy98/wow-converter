package mapexporter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wc3/translators"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

func TestGeneratedMapInheritsTemplateSettings(t *testing.T) {
	templateDir, err := workspace.ResolveTemplateEmptyDir()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(templateDir, "war3map.w3i"))
	if err != nil {
		t.Fatal(err)
	}
	want := translators.InfoTranslator{}.WarToJSON(raw).JSON
	// The real template's title refers to WTS entry 7. Generated W3I must
	// carry its text even when the output retains a stale string table.
	want.LoadingScreen.Title = "wow-converter template map"
	for _, cached := range []bool{false, true} {
		name := "fresh"
		if cached {
			name = "cached"
		}
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "generated.w3x")
			mm := extra.NewMapManager()
			mm.SetTerrain(mapterrain.GetInitialTerrain(16, 32))
			mm.EnsureMapInfo("generated")
			bounds := mm.Info.Camera
			area := mm.Info.Map.PlayableArea
			if cached {
				mm.Info = want
				mm.Info.Water3.AlphaTileMinimapColor ^= 0xffffff
				mm.Info.Map.Flags.UseAlphaTileMinimapColor = !want.Map.Flags.UseAlphaTileMinimapColor
				mm.Info.LoadingScreen.Background = 12345
				if err := mm.Save(dir); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "war3map.wts"), []byte("STRING 7\n{\nold title\n}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			cfg := DefaultMapExportConfig()
			exporter := &MapExporter{MapExportConfig: &cfg, MapManager: mm}
			if err := exporter.SaveWar3mapFiles(context.Background(), dir, "generated"); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, "war3map.w3i"))
			if err != nil {
				t.Fatal(err)
			}
			got := translators.InfoTranslator{}.WarToJSON(raw).JSON
			if got.Map.Flags.UseAlphaTileMinimapColor != want.Map.Flags.UseAlphaTileMinimapColor {
				t.Fatal("alpha-tile minimap color enable flag was not inherited")
			}
			if got.FileVersion < want.FileVersion || got.Water3.AlphaTileMinimapColor != want.Water3.AlphaTileMinimapColor || got.LoadingScreen != want.LoadingScreen {
				t.Fatalf("template settings lost: version=%d alpha=%#x loading=%+v", got.FileVersion, uint32(got.Water3.AlphaTileMinimapColor), got.LoadingScreen)
			}
			if got.Camera != bounds || got.Map.PlayableArea != area {
				t.Fatal("template settings replaced generated map bounds")
			}
		})
	}
}

func TestTemplateLoadingScreenStrings(t *testing.T) {
	dir := t.TempDir()
	template := translators.DefaultInfo()
	template.FileVersion = 39
	template.LoadingScreen = data.LoadingScreen{Background: -1, Path: "LoadingScreen.mdx", Text: "TRIGSTR_001", Title: "TRIGSTR_002", Subtitle: "Literal subtitle"}
	if err := os.WriteFile(filepath.Join(dir, "war3map.w3i"), translators.InfoTranslator{}.JSONToWar(template).Buffer, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "war3map.wts"), []byte("STRING 1\r\n{\r\nFirst line\r\nSecond line\r\n}\r\n\r\nSTRING 2\r\n// Title\r\n{\r\nLoading title\r\n}\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mm := extra.NewMapManager()
	mm.EnsureMapInfo("generated")
	mm.Info.Map.Flags.UseAlphaTileMinimapColor = true
	exporter := &MapExporter{MapManager: mm}
	if err := exporter.inheritTemplateMapSettings(dir); err != nil {
		t.Fatal(err)
	}
	if mm.Info.Map.Flags.UseAlphaTileMinimapColor {
		t.Fatal("template with alpha-tile color disabled did not clear the flag")
	}
	want := template.LoadingScreen
	want.Text = "First line\r\nSecond line"
	want.Title = "Loading title"
	if mm.Info.LoadingScreen != want {
		t.Fatalf("loading screen = %+v, want %+v", mm.Info.LoadingScreen, want)
	}
}
