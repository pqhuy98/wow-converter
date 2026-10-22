package settings

import "github.com/pqhuy98/wow-converter/internal/math"

// TerrainClampPercent configures Z clamp for terrain generation.
type TerrainClampPercent struct {
	Lower float64
	Upper float64
}

// DoodadEnable toggles doodad types during export.
type DoodadEnable struct {
	ADT    bool
	WMO    bool
	M2     bool
	Gobj   bool
	Unit   bool
	Others bool
}

// MapExportConfig drives a single map export run.
type MapExportConfig struct {
	MapID           int
	CascMapDir      string
	WowExportFolder string
	WMOSet          []string
	Min             math.Vector2
	Max             math.Vector2
	MapAngleDeg     float64
	Terrain         struct {
		ClampPercent TerrainClampPercent
	}
	Doodads struct {
		Enable DoodadEnable
	}
	Creatures struct {
		Enable        bool
		AllAreDoodads bool
	}
	PlayerFaction        string
	UnitScale            float64
	Navigation           NavigationConfig
	PlacePathBlockers    bool
	TerrainAsWc3Tilesets bool
}

// DefaultMapExportConfig returns defaults excluding map-specific fields.
func DefaultMapExportConfig() MapExportConfig {
	cfg := MapExportConfig{
		Terrain: struct{ ClampPercent TerrainClampPercent }{
			ClampPercent: TerrainClampPercent{Lower: 0, Upper: 1},
		},
		PlayerFaction: "Alliance",
		UnitScale:     1,
		Navigation:    DefaultNavigationConfig(),
	}
	cfg.Doodads.Enable = DoodadEnable{ADT: true, WMO: true, M2: true, Gobj: true, Unit: true, Others: true}
	cfg.Creatures.Enable = true
	return cfg
}
