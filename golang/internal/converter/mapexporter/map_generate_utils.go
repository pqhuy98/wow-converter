package mapexporter

import (
	"log"

	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/convertlog"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
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

	_, min, max := mapterrain.ComputeRecommendedTerrainClampPercent(exporter.WowObjectManager.Roots)
	heights := make([]float64, len(unitPos))
	for i, p := range unitPos {
		heights[i] = p[2]
	}
	mapCfg.Terrain.ClampPercent = mapterrain.SuggestTerrainClamp(min, max, heights, unitScale, mapCfg.Terrain.ClampPercent)
}

// PruneDepth limits object tree depth (1=ADT, 2=+top M2, 3=+WMO children).
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
