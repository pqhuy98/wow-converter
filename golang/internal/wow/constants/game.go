package constants

// Game holds WoW world/map coordinate constants.
type gameConstants struct {
	MapSize      int
	MapSizeSq    int
	MapCoordBase float64
	TileSize     float64
	MapOffset    float64
}

// Game holds map/tile sizing used by ADT/WDT loaders and exporters.
var Game = gameConstants{
	MapSize:      64,
	MapSizeSq:    4096,
	MapCoordBase: 51200.0 / 3.0,
	TileSize:     (51200.0 / 3.0) / 32.0,
	MapOffset:    17066,
}

// Magic chunk identifiers.
const (
	MagicMD21 = 0x3132444D
	MagicMD20 = 0x3032444D
)
