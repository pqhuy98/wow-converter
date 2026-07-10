package bench

import (
	"crypto/sha256"
	"runtime"

	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	pngwriter "github.com/pqhuy98/wow-converter/internal/formats/png"
)

func benchConcurrency() int {
	n := runtime.NumCPU()
	if n <= 1 {
		return 1
	}
	return n - 1
}

func cpuWork(seed int) [32]byte {
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(seed ^ i)
	}
	return sha256.Sum256(data)
}

func syntheticRGBA(size int) []byte {
	data := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			i := (y*size + x) * 4
			data[i] = byte(x * 3)
			data[i+1] = byte(y * 5)
			data[i+2] = byte((x + y) * 2)
			data[i+3] = 255
		}
	}
	return data
}

func syntheticPNG(size int) ([]byte, error) {
	return pngwriter.EncodeRGBA(syntheticRGBA(size), size, size)
}

func syntheticChunkBakeJobs(count int) []adt.ChunkBakeParams {
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

	mat := &adt.BakeMaterial{Scale: 1, DiffuseTex: &adt.CPUMipTexture{Mips: []adt.MipLevel{{
		Data: make([]byte, 4*4*4), Width: 4, Height: 4,
	}}}}
	for i := 0; i < 4*4*4; i += 4 {
		mat.DiffuseTex.Mips[0].Data[i], mat.DiffuseTex.Mips[0].Data[i+1], mat.DiffuseTex.Mips[0].Data[i+2], mat.DiffuseTex.Mips[0].Data[i+3] = 200, 150, 100, 255
	}

	jobs := make([]adt.ChunkBakeParams, count)
	for n := 0; n < count; n++ {
		x := n / 16
		y := n % 16
		ofsX := -deltaX - (float64(chunkSize) * 7.5) + (float64(y) * float64(chunkSize))
		ofsY := -deltaY - (float64(chunkSize) * 7.5) + (float64(x) * float64(chunkSize))
		canvas := make([]byte, chunkSizePx*chunkSizePx*4)
		for i := range canvas {
			if i%4 == 3 {
				canvas[i] = 255
			}
		}
		jobs[n] = adt.ChunkBakeParams{
			Canvas: canvas, CanvasSize: chunkSizePx, Indices: indices,
			Vertices: vertices, UvsBake: uvsBake, VertexColors: vertexColors,
			Translation: [2]float64{ofsX, ofsY}, TileSize: tileSize, Zoom: 0.0625,
			Layers: [4]*adt.BakeMaterial{mat, nil, nil, nil},
		}
	}
	return jobs
}
