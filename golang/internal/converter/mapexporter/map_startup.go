package mapexporter

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	creature "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/creature"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

const preloadLocationsStart = "-- WOW_CONVERTER_PRELOAD_LOCATIONS_BEGIN"
const preloadLocationsEnd = "-- WOW_CONVERTER_PRELOAD_LOCATIONS_END"

// Always start from the template: repeated exports must replace old coordinates,
// and WE must see the same custom trigger source as the runnable map script.
func (e *MapExporter) saveStartupScripts(ctx context.Context, templateDir, outputDir string) error {
	var locations strings.Builder
	locations.WriteString(preloadLocationsStart + "\n    local locations = {\n")
	points := append([][2]float32(nil), e.preloadLocations...)
	sort.Slice(points, func(i, j int) bool {
		if points[i][0] != points[j][0] {
			return points[i][0] < points[j][0]
		}
		return points[i][1] < points[j][1]
	})
	for _, p := range points {
		fmt.Fprintf(&locations, "        {%.9g, %.9g},\n", p[0], p[1])
	}
	locations.WriteString("    }\n    " + preloadLocationsEnd)
	payload := string(windowsNewlines([]byte(locations.String())))
	for _, name := range []string{"war3map.lua", "war3map.wct", "war3map.wtg"} {
		if err := ctx.Err(); err != nil {
			return err
		}
		raw, err := os.ReadFile(filepath.Join(templateDir, name))
		if err != nil {
			return err
		}
		if name == "war3map.wct" {
			raw, err = rewriteCustomTriggerText(raw, func(script []byte) ([]byte, error) {
				script, err := e.prepareTerrainStartup(script, payload)
				if err != nil {
					return nil, err
				}
				script = creature.InjectFactionAlliances(script, e.factions)
				return windowsNewlines(creature.InjectCreatureDeathSounds(script, e.deathSounds)), nil
			})
		} else if name == "war3map.lua" {
			raw, err = e.prepareTerrainStartup(raw, payload)
			if err == nil {
				raw = creature.InjectFactionAlliances(raw, e.factions)
				raw = creature.InjectCreatureDeathSounds(raw, e.deathSounds)
				raw, err = injectUnitCreation(raw, e.MapManager.MapData.Units)
			}
			// InitBlizzard reads these bounds before the initialization
			// triggers reveal the playable area. WE regenerates this from W3I.
			var call strings.Builder
			call.WriteString("SetCameraBounds(")
			for i, v := range e.MapManager.Info.Camera.Bounds {
				if i > 0 {
					call.WriteString(", ")
				}
				fmt.Fprintf(&call, "%.9g", v)
			}
			call.WriteString(")")
			raw = regexp.MustCompile(`(?m)^SetCameraBounds\([^\r\n]*\)`).ReplaceAllLiteral(raw, []byte(call.String()))
			if err == nil {
				raw, err = creature.ConfigureFactionScript(raw, e.MapManager, e.factions)
			}
			raw = windowsNewlines(raw)
		}
		if err != nil {
			return fmt.Errorf("prepare %s: %w", name, err)
		}
		if name != "war3map.wtg" && !e.MapExportConfig.TerrainAsWc3Tilesets && !bytes.Contains(raw, []byte(payload)) {
			return fmt.Errorf("%s is missing preload location markers", name)
		}
		if err := os.WriteFile(filepath.Join(outputDir, name), raw, 0o644); err != nil {
			return err
		}
	}
	imports := []string{"minimap.blp"}
	for _, path := range e.deathSounds {
		imports = append(imports, strings.ReplaceAll(path, "/", `\`))
	}
	if err := extra.EnsureCustomImports(outputDir, imports); err != nil {
		return err
	}
	// A cached export from the former JASS template must not keep both scripts.
	if err := os.Remove(filepath.Join(outputDir, "war3map.j")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (e *MapExporter) prepareTerrainStartup(script []byte, locations string) ([]byte, error) {
	if e.MapExportConfig.TerrainAsWc3Tilesets {
		// Keep the template trigger registered, but skip the entire camera/fade
		// preload action in native terrain mode, including after an editor save.
		action := regexp.MustCompile(`(?ms)^function Trig_LUA_0s_elapse_Actions\(\)\r?\n.*?^end\b`)
		return action.ReplaceAllLiteral(script, []byte("function Trig_LUA_0s_elapse_Actions()\r\nend")), nil
	}
	script, err := injectPreloadLocations(script, locations)
	if err != nil {
		return nil, err
	}
	init := []byte("function Trig_LUA_map_init_Actions()")
	setup := append([]byte(nil), init...)
	setup = append(setup, "\r\n    BlzShowTerrain(false)\r\n    FogMaskEnable(false)\r\n    FogEnable(false)"...)
	return bytes.ReplaceAll(script, init, setup), nil
}

// World Editor custom-text triggers store Windows newlines. Same as the
// campaign WCT injector: turn lone LF into CRLF without doubling existing CRLF.
func windowsNewlines(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	return bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
}

func injectPreloadLocations(script []byte, locations string) ([]byte, error) {
	start := bytes.Index(script, []byte(preloadLocationsStart))
	if start < 0 {
		return script, nil // Other custom triggers have no generated data.
	}
	end := bytes.Index(script[start:], []byte(preloadLocationsEnd))
	if end < 0 {
		return nil, fmt.Errorf("unterminated preload location markers")
	}
	end += start + len(preloadLocationsEnd)
	result := append([]byte(nil), script[:start]...)
	result = append(result, locations...)
	return append(result, script[end:]...), nil
}

// The template's modern WCT (0x80000004) has a version/subversion, a
// NUL-terminated comment, then length-prefixed global and per-trigger scripts.
func rewriteCustomTriggerText(raw []byte, rewrite func([]byte) ([]byte, error)) ([]byte, error) {
	if len(raw) < 9 || binary.LittleEndian.Uint32(raw) != 0x80000004 {
		return nil, fmt.Errorf("unsupported custom trigger text version")
	}
	commentEnd := bytes.IndexByte(raw[8:], 0)
	if commentEnd < 0 {
		return nil, fmt.Errorf("unterminated custom trigger comment")
	}
	pos := 8 + commentEnd + 1
	out := append([]byte(nil), raw[:pos]...)
	for pos < len(raw) {
		if len(raw)-pos < 4 {
			return nil, fmt.Errorf("truncated custom trigger length")
		}
		n := int(binary.LittleEndian.Uint32(raw[pos:]))
		pos += 4
		if n > len(raw)-pos {
			return nil, fmt.Errorf("truncated custom trigger text")
		}
		script, err := rewrite(raw[pos : pos+n])
		if err != nil {
			return nil, err
		}
		out = binary.LittleEndian.AppendUint32(out, uint32(len(script)))
		out = append(out, script...)
		pos += n
	}
	return out, nil
}
