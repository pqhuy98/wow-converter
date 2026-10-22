package terrain

import (
	"log"
	stdmath "math"
	"sort"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter/settings"
	"github.com/pqhuy98/wow-converter/internal/math"
)

// SuggestTerrainClamp is shared by the fast source-metadata preview and export.
// Bounds and NPC heights use the converter's raw model coordinate scale.
func SuggestTerrainClamp(min, max math.Vector3, npcHeights []float64, unitScale float64, current settings.TerrainClampPercent) settings.TerrainClampPercent {
	if len(npcHeights) == 0 || max[2] <= min[2] || unitScale <= 0 {
		return current
	}
	// Enlarge the exported scene by suggesting 80% of the original window,
	// while preserving the minimum width required by WC3 map dimensions.
	clampDiff := stdmath.Min(1, stdmath.Max(MinimumTerrainClampWidth(min, max), .8*RecommendedTerrainClampWidth(min, max, unitScale)))

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
	return settings.TerrainClampPercent{Lower: bestLower, Upper: bestUpper}
}

// recommendedTerrainClampWidth anchors ground tolerance to the full source range,
// independently of the selected clamp. Suggestions use 80% of this reference;
// keep the original reference so changing suggestions cannot reject more NPCs.
// Keep the preview equivalent in map-preview.model.ts in sync.
func RecommendedTerrainClampWidth(min, max math.Vector3, unitScale float64) float64 {
	if max[2] <= min[2] || unitScale <= 0 {
		return 1
	}
	size := math.V3Sub(max, min)
	clampDiff := config.MaxGameHeightDiff() / size[2] * unitScale
	width := size[0] / unitScale / config.DistancePerTile
	height := size[1] / unitScale / config.DistancePerTile
	return stdmath.Min(1, stdmath.Max(MinimumTerrainClampWidth(min, max), clampDiff*maxFloat(1, ceilDiv4(width)/maxTerrainTiles, ceilDiv4(height)/maxTerrainTiles)))
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
