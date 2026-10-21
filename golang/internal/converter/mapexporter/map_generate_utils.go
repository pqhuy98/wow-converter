package mapexporter

import (
	"log"
	stdmath "math"
	"sort"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/convertlog"
	"github.com/pqhuy98/wow-converter/internal/math"
)

const CreatureExportConcurrency = 5

// ComputeCreatureExportSteps counts creature export progress steps.
func ComputeCreatureExportSteps(uniqueCreatureCount int) int {
	if uniqueCreatureCount > 0 {
		return uniqueCreatureCount
	}
	return 0
}

// CountUniqueUnitExportsFromManager counts unique unit display IDs.
func CountUniqueUnitExportsFromManager(m *common.WowObjectManager) int {
	displayIDs := map[int]struct{}{}
	count := 0
	m.IterateObjects(func(obj *common.WowObject, _ common.ObjectAbsolute) {
		c := common.ObjectCreature(obj)
		if !common.IsWowUnit(obj) || c == nil {
			return
		}
		displayID := c.Model.CreatureDisplayID
		if displayID == 0 {
			return
		}
		if _, ok := displayIDs[displayID]; ok {
			return
		}
		displayIDs[displayID] = struct{}{}
		count++
	})
	return count
}

// AutoChooseClampPercent picks terrain clamp to fit most units.
func AutoChooseClampPercent(exporter *MapExporter, unitScale float64) {
	mapCfg := exporter.MapExportConfig
	if mapCfg == nil {
		return
	}
	var unitPos []math.Vector3
	exporter.WowObjectManager.IterateObjects(func(obj *common.WowObject, abs common.ObjectAbsolute) {
		if common.IsWowUnit(obj) {
			unitPos = append(unitPos, abs.Position)
		}
	})
	if len(unitPos) == 0 {
		log.Printf("No units found, cannot auto choose clamp percent. Defaulting to %.2f %.2f",
			mapCfg.Terrain.ClampPercent.Lower, mapCfg.Terrain.ClampPercent.Upper)
		return
	}

	_, min, max := ComputeRecommendedTerrainClampPercent(exporter.WowObjectManager.Roots)
	heights := make([]float64, len(unitPos))
	for i, p := range unitPos {
		heights[i] = p[2]
	}
	mapCfg.Terrain.ClampPercent = SuggestTerrainClamp(min, max, heights, unitScale, mapCfg.Terrain.ClampPercent)
}

// SuggestTerrainClamp is shared by the fast source-metadata preview and export.
// Bounds and NPC heights use the converter's raw model coordinate scale.
func SuggestTerrainClamp(min, max math.Vector3, npcHeights []float64, unitScale float64, current TerrainClampPercent) TerrainClampPercent {
	if len(npcHeights) == 0 || max[2] <= min[2] || unitScale <= 0 {
		return current
	}
	// Enlarge the exported scene by suggesting 80% of the original window,
	// while preserving the minimum width required by WC3 map dimensions.
	clampDiff := stdmath.Min(1, stdmath.Max(minimumTerrainClampWidth(min, max), .8*recommendedTerrainClampWidth(min, max, unitScale)))

	lower := current.Lower
	upper := current.Upper
	if upper-lower <= clampDiff {
		log.Println("Map terrain clamp is already within the recommended range, skipping auto choose.")
		return current
	}

	bestLower, bestUpper := lower, lower+clampDiff
	maxCount := 0
	unitPosRatio := make([]float64, len(npcHeights))
	for i, z := range npcHeights {
		unitPosRatio[i] = (z - min[2]) / (max[2] - min[2])
	}
	sort.Float64s(unitPosRatio)
	for lowerPercent := lower; lowerPercent <= upper-clampDiff; lowerPercent += 0.01 {
		upperPercent := lowerPercent + clampDiff
		count := sort.SearchFloat64s(unitPosRatio, stdmath.Nextafter(upperPercent, stdmath.Inf(1))) - sort.SearchFloat64s(unitPosRatio, lowerPercent)
		if count > maxCount {
			maxCount = count
			bestLower = lowerPercent
			bestUpper = upperPercent
		}
	}
	leftOutBelow, leftOutAbove := 0, 0
	for _, r := range unitPosRatio {
		if r < bestLower {
			leftOutBelow++
		}
		if r > bestUpper {
			leftOutAbove++
		}
	}
	leftOut := leftOutBelow + leftOutAbove
	remaining := len(unitPosRatio) - leftOut
	log.Printf("Chosen clamp percent: %.2f - %.2f (%d units remaining)", bestLower, bestUpper, remaining)
	log.Printf("Left out units: %d (%d below, %d above)", leftOut, leftOutBelow, leftOutAbove)
	return TerrainClampPercent{Lower: bestLower, Upper: bestUpper}
}

// PruneDepth limits object tree depth (1=adt, 2=+top m2, 3=+interiors).
func PruneDepth(exporter *MapExporter, depth int) {
	if exporter.WowObjectManager == nil || depth >= 3 {
		return
	}
	m := exporter.WowObjectManager
	nextRoots := []*common.WowObject{}
	nextObjects := map[string]*common.WowObject{}
	nextDoodads := []*common.WowObject{}
	nextTerrains := []*common.WowObject{}

	var visit func(obj *common.WowObject, hasWmoAncestor bool) *common.WowObject
	visit = func(obj *common.WowObject, hasWmoAncestor bool) *common.WowObject {
		currentHasWmo := hasWmoAncestor || obj.Type == common.WowObjectWMO
		keep := true
		switch depth {
		case 1:
			keep = obj.Type == common.WowObjectADT || obj.Type == common.WowObjectUnit
		case 2:
			if obj.Type == common.WowObjectM2 || obj.Type == common.WowObjectGobj {
				keep = !currentHasWmo
			}
		}
		if !keep {
			return nil
		}
		clone := *obj
		clone.Children = nil
		nextObjects[clone.ID] = &clone
		if clone.Type == common.WowObjectADT {
			nextTerrains = append(nextTerrains, &clone)
		} else if clone.Type != common.WowObjectUnit {
			nextDoodads = append(nextDoodads, &clone)
		}
		for _, child := range obj.Children {
			if pruned := visit(child, currentHasWmo); pruned != nil {
				clone.Children = append(clone.Children, pruned)
			}
		}
		return &clone
	}

	for _, root := range m.Roots {
		if newRoot := visit(root, false); newRoot != nil {
			nextRoots = append(nextRoots, newRoot)
		}
	}
	if depth == 1 {
		hasAdt := false
		for _, r := range nextRoots {
			if r.Type == common.WowObjectADT {
				hasAdt = true
				break
			}
		}
		if hasAdt {
			filtered := nextRoots[:0]
			for _, r := range nextRoots {
				if r.Type == common.WowObjectADT {
					filtered = append(filtered, r)
				}
			}
			nextRoots = filtered
		}
	}
	m.Roots = nextRoots
	m.Objects = nextObjects
	m.Doodads = nextDoodads
	if len(nextTerrains) > 0 {
		m.Terrains = nextTerrains
	}
}

// LogMapGeneratePhase logs a map generation phase label.
func LogMapGeneratePhase(label string) {
	convertlog.MapGenerate(label)
}

// recommendedTerrainClampWidth anchors ground tolerance to the full source range,
// independently of the selected clamp. Suggestions use 80% of this reference;
// keep the original reference so changing suggestions cannot reject more NPCs.
// Keep the preview equivalent in map-preview.model.ts in sync.
func recommendedTerrainClampWidth(min, max math.Vector3, unitScale float64) float64 {
	if max[2] <= min[2] || unitScale <= 0 {
		return 1
	}
	size := math.V3Sub(max, min)
	clampDiff := config.MaxGameHeightDiff() / size[2] * unitScale
	width := size[0] / unitScale / config.DistancePerTile
	height := size[1] / unitScale / config.DistancePerTile
	return stdmath.Min(1, stdmath.Max(minimumTerrainClampWidth(min, max), clampDiff*maxFloat(1, ceilDiv4(width)/maxTerrainTiles, ceilDiv4(height)/maxTerrainTiles)))
}

func ceilDiv4(v float64) float64 {
	return float64((int(v) + 3) / 4 * 4)
}

func maxFloat(a float64, rest ...float64) float64 {
	m := a
	for _, v := range rest {
		if v > m {
			m = v
		}
	}
	return m
}
