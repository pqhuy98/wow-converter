package data

// Offset is map tile offset within the full terrain grid.
type Offset struct {
	X float32
	Y float32
}

// MapSize describes terrain dimensions and offset.
type MapSize struct {
	Width  int
	Height int
	Offset Offset
}

// TerrainFlag is a war3map.w3e tile flag bit.
type TerrainFlag uint16

const (
	TerrainUnwalkable  TerrainFlag = 0x0002
	TerrainUnflyable   TerrainFlag = 0x0004
	TerrainUnbuildable TerrainFlag = 0x0008
	TerrainRamp        TerrainFlag = 0x0010
	TerrainBlight      TerrainFlag = 0x0020
	TerrainWater       TerrainFlag = 0x0040
	TerrainBoundary    TerrainFlag = 0x0080
)

// Terrain is war3map.w3e terrain data.
type Terrain struct {
	Tileset           string
	CustomTileset     bool
	TilePalette       []string
	CliffTilePalette  []string
	Map               MapSize
	GroundHeight      [][]int
	WaterHeight       [][]int
	BoundaryFlag      [][]bool
	Flags             [][]uint16
	GroundTexture     [][]int
	GroundVariation   [][]int
	CliffVariation    [][]int
	CliffTexture      [][]int
	LayerHeight       [][]int
}
