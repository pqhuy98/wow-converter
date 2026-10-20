package mapexporter

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

func TestMapStartupSurvivesCachedExportAndEditorRegeneration(t *testing.T) {
	dir := t.TempDir()
	// Reproduce a cached export from the old JASS template, including folders.
	if err := os.WriteFile(filepath.Join(dir, "war3map.j"), []byte("old script"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "war3map.wtg"), []byte("old triggers"), 0o644); err != nil {
		t.Fatal(err)
	}
	minimap := []byte("editor generated minimap")
	if err := os.WriteFile(filepath.Join(dir, "war3mapMap.blp"), minimap, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultMapExportConfig()
	mm := extra.NewMapManager()
	mm.SetTerrain(getInitialTerrain(16, 32))
	unitType := mm.AddUnitType(false, "hfoo", nil)
	for i := 0; i < 2; i++ {
		mm.AddUnit(unitType, data.Unit{Position: [3]float32{float32(i * 64), 32, 0}, Scale: [3]float32{1, 1, 1}, Hitpoints: 100, Color: -1})
	}
	exporter := &MapExporter{MapExportConfig: &cfg, MapManager: mm, preloadLocations: [][2]float32{{123.5, -987.25}}}
	for i := 0; i < 2; i++ {
		if err := exporter.SaveWar3mapFiles(context.Background(), dir, "startup"); err != nil {
			t.Fatal(err)
		}
		lua, err := os.ReadFile(filepath.Join(dir, "war3map.lua"))
		if err != nil {
			t.Fatal(err)
		}
		wct, err := os.ReadFile(filepath.Join(dir, "war3map.wct"))
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range [][]byte{lua, wct} {
			if bytes.Contains(text, []byte("{123.5, -987.25}")) != (i == 0) ||
				!bytes.Contains(text, []byte(`BlzChangeMinimapTerrainTex("minimap.blp")`)) {
				t.Fatalf("stale or missing startup data after export %d", i)
			}
		}
		if i == 0 && !bytes.Contains(lua, []byte("{123.5, -987.25},\r\n")) {
			t.Fatal("injected locations are missing CRLF")
		}
		if hasBareLF(lua) {
			t.Fatal("war3map.lua has LF without CR")
		}
		wantUnits := 0
		if i == 0 {
			wantUnits = 2
		}
		if bytes.Count(lua, []byte("u = BlzCreateUnitWithSkin(")) != wantUnits {
			t.Fatal("runtime unit creation does not follow the exported placements")
		}
		if bytes.Contains(wct, []byte(unitCreationStart)) {
			t.Fatal("creation must not persist in custom triggers and duplicate editor-generated units")
		}
		// Parse the WCT lengths after injection and verify each complete custom
		// trigger remains present verbatim in the runnable map script.
		_, err = rewriteCustomTriggerText(wct, func(script []byte) ([]byte, error) {
			text := bytes.TrimRight(script, "\x00")
			if hasBareLF(text) {
				t.Fatal("wct custom trigger has LF without CR")
			}
			if len(script) > 0 && !bytes.Contains(lua, text) {
				t.Fatal("editor custom trigger differs from runtime Lua")
			}
			return script, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		exporter.preloadLocations = nil
		mm.Units = nil
	}
	if mm.Info.ScriptLanguage != data.ScriptLanguageLua {
		t.Fatal("map info still selects JASS")
	}
	if _, err := os.Stat(filepath.Join(dir, "war3map.j")); !os.IsNotExist(err) {
		t.Fatal("stale JASS script retained")
	}
	got, _ := os.ReadFile(filepath.Join(dir, "war3mapMap.blp"))
	if !bytes.Equal(got, minimap) {
		t.Fatal("overwrote editor minimap")
	}
	imports, _ := os.ReadFile(filepath.Join(dir, "war3map.imp"))
	if bytes.Count(imports, []byte("minimap.blp\x00")) != 1 || binary.LittleEndian.Uint32(imports[4:]) != 2 {
		t.Fatal("custom minimap import missing or duplicated")
	}
	templateDir, err := workspace.ResolveTemplateEmptyDir()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(filepath.Join(templateDir, "war3map.wtg"))
	got, _ = os.ReadFile(filepath.Join(dir, "war3map.wtg"))
	if !bytes.Equal(got, want) {
		t.Fatal("cached trigger structure not refreshed")
	}
}

func TestCustomTriggerTextRejectsTruncation(t *testing.T) {
	for _, raw := range [][]byte{nil, {4, 0, 0, 128, 1, 0, 0, 0}, {4, 0, 0, 128, 1, 0, 0, 0, 0, 255, 0, 0, 0}} {
		if _, err := rewriteCustomTriggerText(raw, func(b []byte) ([]byte, error) { return b, nil }); err == nil {
			t.Fatal("accepted truncated WCT")
		}
	}
}

func TestWindowsNewlinesDoesNotDoubleCR(t *testing.T) {
	got := windowsNewlines([]byte("a\nb\r\nc\n"))
	if !bytes.Equal(got, []byte("a\r\nb\r\nc\r\n")) {
		t.Fatalf("got %q", got)
	}
}

func hasBareLF(b []byte) bool {
	for i, c := range b {
		if c == '\n' && (i == 0 || b[i-1] != '\r') {
			return true
		}
	}
	return false
}
