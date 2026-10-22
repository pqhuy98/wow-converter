package settings

import (
	"fmt"
	"math"
)

// NavigationConfig describes a humanoid in WoW units, independent of export scale.
// Jump-only links are projected as walkable terrain, including short gap ramps.
type NavigationConfig struct {
	Disabled        bool    `json:"disabled"`
	Radius          float64 `json:"radius"`
	Height          float64 `json:"height"`
	MaxSlopeDegrees float64 `json:"maxSlopeDegrees"`
	StepHeight      float64 `json:"stepHeight"`
	JumpHeight      float64 `json:"jumpHeight"`
	JumpDistance    float64 `json:"jumpDistance"`
	MaxDrop         float64 `json:"maxDrop"`
	SeedSnap        float64 `json:"seedSnap"`
	AirBlockHeight  float64 `json:"airBlockHeight"`
}

// DefaultNavigationConfig returns conservative humanoid movement limits.
func DefaultNavigationConfig() NavigationConfig {
	return NavigationConfig{Radius: 0.35, Height: 1.8, MaxSlopeDegrees: 45,
		StepHeight: 0.6, JumpHeight: 1.5, JumpDistance: 3, MaxDrop: 3,
		SeedSnap: 1.25, AirBlockHeight: 6}
}

// Validate rejects non-finite, negative and unbounded traversal parameters.
func (c NavigationConfig) Validate() error {
	for _, v := range []float64{c.Radius, c.Height, c.MaxSlopeDegrees, c.StepHeight, c.JumpHeight, c.JumpDistance, c.MaxDrop, c.SeedSnap, c.AirBlockHeight} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
			return fmt.Errorf("navigation limits must be finite and between 0 and 100 WoW units")
		}
	}
	if c.Radius <= 0 || c.Height <= 0 || c.SeedSnap <= 0 || c.AirBlockHeight <= 0 || c.MaxSlopeDegrees <= 0 || c.MaxSlopeDegrees >= 90 {
		return fmt.Errorf("navigation radius, height, seedSnap and airBlockHeight must be positive; maxSlopeDegrees must be between 0 and 90")
	}
	return nil
}
