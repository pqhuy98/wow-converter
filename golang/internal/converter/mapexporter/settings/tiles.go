package settings

import "sort"

// TileCoord is an ADT tile coordinate.
type TileCoord struct {
	X, Y int
}

func SortTileCoords(tiles []TileCoord) {
	sort.Slice(tiles, func(i, j int) bool {
		if tiles[i].Y != tiles[j].Y {
			return tiles[i].Y < tiles[j].Y
		}
		return tiles[i].X < tiles[j].X
	})
}
