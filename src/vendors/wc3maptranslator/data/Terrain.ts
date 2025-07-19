interface Terrain {
  tileset: string
  customTileset: boolean
  tilePalette: string[]
  cliffTilePalette: string[]
  map: MapSize
  // "Masks"
  groundHeight: number[][]
  waterHeight: number[][]
  boundaryFlag: boolean[][]
  flags: number[][]
  groundTexture: number[][]
  groundVariation: number[][]
  cliffVariation: number[][]
  cliffTexture: number[][]
  layerHeight: number[][]
}

interface MapSize {
  width: number
  height: number
  offset: Offset
}

interface Offset {
  x: number
  y: number
}

export type { Terrain, MapSize, Offset };

export enum TerrainFlag {
  Unwalkable = 0x0002,
  Unflyable = 0x0004,
  Unbuildable = 0x0008,
  Ramp = 0x0010, // ramp flag (used to set a ramp between two layers)
  Blight = 0x0020, // blight flag (ground will look like Undead's ground)
  Water = 0x0040, // water flag (enable water)
  Boundary = 0x0080, // boundary flag 2 (used on "camera bounds" area. Usually set by the World Editor "boundary" tool.)
}
