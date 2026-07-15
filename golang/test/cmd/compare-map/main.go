package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run ./test/cmd/compare-map <ts-map-dir> <go-map-dir>")
		os.Exit(2)
	}
	tsMap := loadMap(os.Args[1])
	goMap := loadMap(os.Args[2])

	failed := false
	failed = compareTerrain(tsMap, goMap) || failed
	failed = compareMultiset("doodads", doodadKeys(tsMap), doodadKeys(goMap)) || failed
	failed = compareMultiset("units", unitKeys(tsMap), unitKeys(goMap)) || failed
	failed = compareMultiset("doodad types", doodadTypeKeys(tsMap), doodadTypeKeys(goMap)) || failed
	failed = compareMultiset("unit types", unitTypeKeys(tsMap), unitTypeKeys(goMap)) || failed
	if failed {
		fmt.Println("SEMANTIC MAP PARITY: FAIL")
		os.Exit(1)
	}
	fmt.Println("SEMANTIC MAP PARITY: PASS")
}

func loadMap(dir string) *extra.MapManager {
	manager := extra.NewMapManager()
	if err := manager.Load(dir); err != nil {
		fmt.Fprintf(os.Stderr, "load %s: %v\n", dir, err)
		os.Exit(1)
	}
	return manager
}

func compareTerrain(tsMap, goMap *extra.MapManager) bool {
	ts := tsMap.GetTerrain()
	goTerrain := goMap.GetTerrain()
	failed := false
	if ts.Tileset != goTerrain.Tileset ||
		ts.CustomTileset != goTerrain.CustomTileset ||
		ts.Map.Width != goTerrain.Map.Width ||
		ts.Map.Height != goTerrain.Map.Height ||
		ts.Map.Offset != goTerrain.Map.Offset ||
		!equalStrings(ts.TilePalette, goTerrain.TilePalette) ||
		!equalStrings(ts.CliffTilePalette, goTerrain.CliffTilePalette) {
		fmt.Printf("terrain metadata differs:\n  TS=%+v\n  Go=%+v\n", ts.Map, goTerrain.Map)
		failed = true
	}
	layers := []struct {
		name string
		ts   [][]int
		goV  [][]int
	}{
		{"ground height", ts.GroundHeight, goTerrain.GroundHeight},
		{"water height", ts.WaterHeight, goTerrain.WaterHeight},
		{"flags", intMatrix(ts.Flags), intMatrix(goTerrain.Flags)},
		{"ground texture", ts.GroundTexture, goTerrain.GroundTexture},
		{"ground variation", ts.GroundVariation, goTerrain.GroundVariation},
		{"cliff variation", ts.CliffVariation, goTerrain.CliffVariation},
		{"cliff texture", ts.CliffTexture, goTerrain.CliffTexture},
		{"layer height", ts.LayerHeight, goTerrain.LayerHeight},
	}
	for _, layer := range layers {
		different, maxDelta := matrixDiff(layer.ts, layer.goV)
		fmt.Printf("terrain %-18s different=%d maxDelta=%d\n", layer.name, different, maxDelta)
		if different > 0 {
			failed = true
		}
	}
	return failed
}

func compareMultiset(label string, tsKeys, goKeys []string) bool {
	tsCounts := counts(tsKeys)
	goCounts := counts(goKeys)
	missing, extra := multisetDifference(tsCounts, goCounts), multisetDifference(goCounts, tsCounts)
	fmt.Printf("%-14s TS=%d Go=%d missing=%d extra=%d\n", label, len(tsKeys), len(goKeys), missing, extra)
	return missing > 0 || extra > 0
}

func doodadKeys(manager *extra.MapManager) []string {
	out := make([]string, 0, len(manager.Doodads))
	for _, instance := range manager.Doodads {
		doodad := instance.Doodad
		out = append(out, fmt.Sprintf(
			"%s|%d|%.3f,%.3f,%.3f|%.5f|%.3f,%.3f,%.3f|%s|%t,%t,%t|%d",
			doodadTypeKey(instance.Type), doodad.Variation,
			doodad.Position[0], doodad.Position[1], doodad.Position[2],
			doodad.Angle, doodad.Scale[0], doodad.Scale[1], doodad.Scale[2],
			doodad.SkinID, doodad.Flags.Visible, doodad.Flags.Solid, doodad.Flags.CustomHeight, doodad.Life,
		))
	}
	return out
}

func unitKeys(manager *extra.MapManager) []string {
	out := make([]string, 0, len(manager.Units))
	for _, instance := range manager.Units {
		unit := instance.Unit
		out = append(out, fmt.Sprintf(
			"%s|%d|%.3f,%.3f,%.3f|%.5f|%.3f,%.3f,%.3f|%s|%d|%d|%d",
			unitTypeKey(instance.Type), unit.Variation,
			unit.Position[0], unit.Position[1], unit.Position[2],
			unit.Rotation, unit.Scale[0], unit.Scale[1], unit.Scale[2],
			unit.Skin, unit.Player, unit.Hitpoints, unit.Mana,
		))
	}
	return out
}

func doodadTypeKeys(manager *extra.MapManager) []string {
	out := make([]string, 0, len(manager.DoodadTypes))
	for i := range manager.DoodadTypes {
		out = append(out, objectDataKey(manager.DoodadTypes[i].ObjectData))
	}
	return out
}

func unitTypeKeys(manager *extra.MapManager) []string {
	out := make([]string, 0, len(manager.UnitTypes))
	for i := range manager.UnitTypes {
		out = append(out, objectDataKey(manager.UnitTypes[i].ObjectData))
	}
	return out
}

func doodadTypeKey(value any) string {
	switch typed := value.(type) {
	case *extra.DoodadType:
		return objectDataKey(typed.ObjectData)
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}

func unitTypeKey(value any) string {
	switch typed := value.(type) {
	case *extra.UnitType:
		return objectDataKey(typed.ObjectData)
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}

func objectDataKey(value extra.ObjectData) string {
	mods := append([]data.Modification(nil), value.Data...)
	sort.Slice(mods, func(i, j int) bool {
		return modificationKey(mods[i]) < modificationKey(mods[j])
	})
	parts := make([]string, len(mods))
	for i, mod := range mods {
		parts[i] = modificationKey(mod)
	}
	return value.Parent + "|" + strings.Join(parts, ";")
}

func modificationKey(mod data.Modification) string {
	value, _ := json.Marshal(mod.Value)
	return fmt.Sprintf("%s|%s|%d|%d|%d|%s", mod.ID, mod.Type, mod.Level, mod.Column, mod.Variation, value)
}

func counts(values []string) map[string]int {
	out := make(map[string]int, len(values))
	for _, value := range values {
		out[value]++
	}
	return out
}

func multisetDifference(left, right map[string]int) int {
	total := 0
	for key, count := range left {
		if remaining := count - right[key]; remaining > 0 {
			total += remaining
		}
	}
	return total
}

func matrixDiff(left, right [][]int) (different, maxDelta int) {
	if len(left) != len(right) {
		return max(len(left), len(right)), 0
	}
	for y := range left {
		if len(left[y]) != len(right[y]) {
			different += max(len(left[y]), len(right[y]))
			continue
		}
		for x := range left[y] {
			delta := left[y][x] - right[y][x]
			if delta < 0 {
				delta = -delta
			}
			if delta > 0 {
				different++
			}
			if delta > maxDelta {
				maxDelta = delta
			}
		}
	}
	return different, maxDelta
}

func intMatrix(values [][]uint16) [][]int {
	out := make([][]int, len(values))
	for y := range values {
		out[y] = make([]int, len(values[y]))
		for x := range values[y] {
			out[y][x] = int(values[y][x])
		}
	}
	return out
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
