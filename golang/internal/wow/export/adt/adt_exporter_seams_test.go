package adt

import "testing"

func TestTerrainSharedEdgesUseIdenticalCoordinates(t *testing.T) {
	for _, origin := range []float64{1600, -1600, 16000, -16000, 0} {
		first := float32(origin)
		for n := 0; n < 15; n++ {
			a, b := float32(origin-float64(n)*chunkSize), float32(origin-float64(n+1)*chunkSize)
			for r := 0; r <= 16; r += 2 {
				x0, z0 := terrainVertexXZ(first, first, a, a, r, 8)
				x1, z1 := terrainVertexXZ(first, first, a, b, r, 0)
				if x0 != x1 || z0 != z1 {
					t.Fatalf("horizontal shared edge differs: %g,%g / %g,%g", x0, z0, x1, z1)
				}
			}
			for c := 0; c <= 8; c++ {
				x0, z0 := terrainVertexXZ(first, first, a, a, 16, c)
				x1, z1 := terrainVertexXZ(first, first, b, a, 0, c)
				if x0 != x1 || z0 != z1 {
					t.Fatalf("vertical shared edge differs: %g,%g / %g,%g", x0, z0, x1, z1)
				}
			}
		}
	}
}

func TestTerrainAdjacentADTSharedEdge(t *testing.T) {
	for tile := -32; tile < 32; tile++ {
		first := float32(float64(tile) * tileSize)
		next := float32(float64(tile-1) * tileSize)
		lastChunk := float32(float64(tile)*tileSize - 15*chunkSize)
		x0, z0 := terrainVertexXZ(first, first, first, lastChunk, 0, 8)
		x1, z1 := terrainVertexXZ(first, next, first, next, 0, 0)
		if x0 != x1 || z0 != z1 {
			t.Fatalf("tile %d: shared edge differs: %g,%g / %g,%g", tile, x0, z0, x1, z1)
		}
	}
}
