package adt

import (
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/constants"
)

func TestBakeChunkCoversFullCanvas(t *testing.T) {
	tileSize := constants.Game.TileSize
	chunkSize := tileSize / 16
	unitSize := chunkSize / 8
	unitHalf := unitSize / 2
	chunkSizePx := 64

	chunkX := float32(16000)
	chunkY := float32(-16000)
	firstChunkX := chunkX
	firstChunkY := chunkY
	deltaX := float64(chunkY) - tileSize
	deltaY := float64(chunkX) - tileSize
	ofsX := -deltaX - (chunkSize * 7.5)
	ofsY := -deltaY - (chunkSize * 7.5)

	vertices := make([]float32, 145*3)
	uvsBake := make([]float32, 145*2)
	vertexColors := make([]float32, 145*4)
	idx := 0
	for row := 0; row < 17; row++ {
		isShort := row%2 != 0
		colCount := 9
		if isShort {
			colCount = 8
		}
		for col := 0; col < colCount; col++ {
			vx := chunkY - float32(col)*float32(unitSize)
			vy := float32(100)
			vz := chunkX - float32(row)*float32(unitHalf)
			if isShort {
				vx -= float32(unitHalf)
			}
			vertices[idx*3] = vx
			vertices[idx*3+1] = vy
			vertices[idx*3+2] = vz
			vertexColors[idx*4], vertexColors[idx*4+1], vertexColors[idx*4+2], vertexColors[idx*4+3] = 0.5, 0.5, 0.5, 1
			uRaw := -float64(vx-firstChunkX) / tileSize
			vRaw := float64(vz-firstChunkY) / tileSize
			uvsBake[idx*2] = float32(uRaw)
			uvsBake[idx*2+1] = float32(vRaw)
			idx++
		}
	}

	indices := make([]int, 0, 136*6)
	ofs := 0
	for j := 9; j < 145; j++ {
		indOfs := ofs + j
		indices = append(indices, indOfs, indOfs-9, indOfs+8, indOfs, indOfs-8, indOfs-9, indOfs, indOfs+9, indOfs-8, indOfs, indOfs+8, indOfs+9)
		if (j+1)%(9+8) == 0 {
			j += 9
		}
	}

	canvas := make([]byte, chunkSizePx*chunkSizePx*4)
	for i := range canvas {
		if i%4 == 3 {
			canvas[i] = 255
		}
	}

	mat := &BakeMaterial{Scale: 1, DiffuseTex: &CPUMipTexture{Mips: []MipLevel{{
		Data: make([]byte, 4*4*4), Width: 4, Height: 4,
	}}}}
	for i := 0; i < 4*4*4; i += 4 {
		mat.DiffuseTex.Mips[0].Data[i], mat.DiffuseTex.Mips[0].Data[i+1], mat.DiffuseTex.Mips[0].Data[i+2], mat.DiffuseTex.Mips[0].Data[i+3] = 200, 150, 100, 255
	}

	BakeChunk(ChunkBakeParams{
		Canvas: canvas, CanvasSize: chunkSizePx, Indices: indices,
		Vertices: vertices, UvsBake: uvsBake, VertexColors: vertexColors,
		Translation: [2]float64{ofsX, ofsY}, TileSize: tileSize, Zoom: 0.0625,
		Layers: [4]*BakeMaterial{mat, nil, nil, nil},
	})

	minY, maxY := chunkSizePx, -1
	for y := 0; y < chunkSizePx; y++ {
		for x := 0; x < chunkSizePx; x++ {
			o := (y*chunkSizePx + x) * 4
			if canvas[o] != 0 || canvas[o+1] != 0 || canvas[o+2] != 0 {
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	rotated := Rotate180(canvas, chunkSizePx)
	rotMinY, rotMaxY := chunkSizePx, -1
	for y := 0; y < chunkSizePx; y++ {
		for x := 0; x < chunkSizePx; x++ {
			o := (y*chunkSizePx + x) * 4
			if rotated[o] != 0 || rotated[o+1] != 0 || rotated[o+2] != 0 {
				if y < rotMinY {
					rotMinY = y
				}
				if y > rotMaxY {
					rotMaxY = y
				}
			}
		}
	}

	t.Logf("pre-rotate Y coverage: %d..%d (canvas %d)", minY, maxY, chunkSizePx)
	t.Logf("post-rotate Y coverage: %d..%d", rotMinY, rotMaxY)

	// Expect full canvas height coverage before rotation.
	if minY > 0 || maxY < chunkSizePx-1 {
		t.Fatalf("expected full canvas coverage before rotate, got Y %d..%d", minY, maxY)
	}

	// Log transform Y range for corners.
	zoom := 0.0625
	transformY := func(vi int) float64 {
		z := float64(vertices[vi*3+2])
		cy := ((((z + ofsY) / tileSize) * 2) - 1) * -1
		ndcY := cy / zoom
		return (1 - (ndcY+1)/2) * float64(chunkSizePx)
	}
	t.Logf("vertex 0 pixelY=%.2f, vertex %d pixelY=%.2f", transformY(0), idx-1, transformY(idx-1))
}

func TestBakeChunkTransformYSpan(t *testing.T) {
	tileSize := constants.Game.TileSize
	chunkSize := tileSize / 16
	chunkX := float32(16000)
	deltaY := float64(chunkX) - tileSize
	ofsY := -deltaY - (chunkSize * 7.5)
	zoom := 0.0625

	zMin := float64(chunkX) - chunkSize
	zMax := float64(chunkX)
	cyMin := ((((zMin + ofsY) / tileSize) * 2) - 1) * -1
	cyMax := ((((zMax + ofsY) / tileSize) * 2) - 1) * -1
	t.Logf("cy span: %.6f .. %.6f (expected ~%.6f .. %.6f)", cyMin, cyMax, -zoom, zoom)
	if math.Abs(math.Abs(cyMax-cyMin)-2*zoom) > 1e-6 {
		t.Fatalf("cy span %.6f != 2*zoom", math.Abs(cyMax-cyMin))
	}
	if math.Abs((cyMin+cyMax)/2) > 1e-4 {
		t.Fatalf("cy not centered on 0: min=%.6f max=%.6f", cyMin, cyMax)
	}
}
