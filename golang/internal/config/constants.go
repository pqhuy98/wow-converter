package config

import "runtime"

// Map generation constants from src/lib/constants.ts.

const (
	DefaultLayer     = 7
	DistancePerTile  = 128
	BlizzardNull     = 65535
)

var (
	dataHeightMin = 512
	dataHeightMax = 8192*2 - 512
)

// DataHeightMin returns the current terrain data height minimum.
func DataHeightMin() int { return dataHeightMin }

// DataHeightMax returns the current terrain data height maximum.
func DataHeightMax() int { return dataHeightMax }

// SetDataHeightLimit updates terrain height limits.
func SetDataHeightLimit(min, max int) {
	dataHeightMin = min
	dataHeightMax = max
}

// MaxGameHeightDiff is the usable Z range for terrain clamp percent.
func MaxGameHeightDiff() float64 {
	return float64(dataHeightMax-dataHeightMin) / 4
}

// DataHeightToGameZ converts WC3 terrain height to WoW game Z.
func DataHeightToGameZ(dataHeight float64) float64 {
	return (dataHeight - 8192 + float64(DefaultLayer-2)*512) / 4
}

// GameZToDataHeight converts WoW game Z to WC3 terrain height.
func GameZToDataHeight(gameZ float64) int {
	return int(gameZ*4 + 8192 - float64(DefaultLayer-2)*512)
}

// GameZToWaterHeight converts game Z to water height data value.
func GameZToWaterHeight(waterZ float64) int {
	return int(waterZ + 89.6*4 + 8192)
}

// WaterHeightToGameZ converts water height data to game Z.
func WaterHeightToGameZ(waterHeight float64) float64 {
	return (waterHeight - 8192) / 4 - 89.6
}

// GameZToPercent maps game Z into 0..1 terrain clamp percent range.
func GameZToPercent(z float64) float64 {
	return (z - DataHeightToGameZ(float64(dataHeightMin))) / MaxGameHeightDiff()
}

// MaxConcurrency returns worker pool size (CPU count - 1, min 1).
func MaxConcurrency() int {
	n := runtime.NumCPU()
	if n <= 1 {
		return 1
	}
	return n - 1
}
