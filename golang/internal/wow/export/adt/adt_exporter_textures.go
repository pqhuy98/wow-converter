package adt

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	adtfmt "github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	appconfig "github.com/pqhuy98/wow-converter/internal/config"
	blpfmt "github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/wow/export"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

func (e *Exporter) exportTextures(
	ctx context.Context,
	source getFileFunc,
	dir string,
	quality int,
	options export.ADTExportOptions,
	rootAdt, texAdt *adtfmt.ADTLoader,
	firstChunk adtfmt.ADTChunk,
	chunkMeshes [][]int,
	vertices, uvsBake, vertexColors []float32,
	progress *export.ProgressReporter,
	conv *ConversionOutput,
) error {
	cfg := server.GetConfig()
	isAlphaMaps := quality == -1
	isLargeBake := quality >= 8192
	isSplittingAlphaMaps := isAlphaMaps && cfg.SplitAlphaMaps
	isSplittingTextures := isLargeBake && cfg.SplitLargeTerrainBakes

	if isAlphaMaps {
		return e.exportAlphaMaps(ctx, source, dir, options, rootAdt, texAdt, isSplittingAlphaMaps, progress, conv)
	}
	if quality <= 512 {
		return e.exportMinimap(ctx, source, dir, quality, options, progress, conv)
	}
	return e.exportLargeBake(ctx, source, dir, quality, options, rootAdt, texAdt, firstChunk, chunkMeshes, vertices, uvsBake, vertexColors, isSplittingTextures, progress, conv)
}

func (e *Exporter) exportAlphaMaps(
	ctx context.Context,
	getFile getFileFunc,
	dir string,
	options export.ADTExportOptions,
	rootAdt, texAdt *adtfmt.ADTLoader,
	split bool,
	progress *export.ProgressReporter,
	conv *ConversionOutput,
) error {
	usePosix := options.PathFormat == "posix"
	materialIDs := texAdt.DiffuseTextureFileDataIDs
	heightIDs := texAdt.HeightTextureFileDataIDs
	texParams := texAdt.TexParams

	type alphaMaterial struct {
		Scale            float64 `json:"scale"`
		FileDataID       uint32  `json:"fileDataID"`
		File             string  `json:"file,omitempty"`
		HeightFile       string  `json:"heightFile,omitempty"`
		HeightFileDataID uint32  `json:"heightFileDataID,omitempty"`
		HeightScale      float32 `json:"heightScale,omitempty"`
		HeightOffset     float32 `json:"heightOffset,omitempty"`
	}

	saveLayerTexture := func(fileDataID uint32) (string, error) {
		raw, err := getFile(ctx, fileDataID)
		if err != nil {
			return "", err
		}
		img, err := blpfmt.NewBLPImage(buffer.From(raw))
		if err != nil {
			return "", err
		}
		rgba, err := img.ToUInt8Array(0, 0b1111)
		if err != nil {
			return "", err
		}
		fileName, ok := archivecasc.GetByID(int(fileDataID))
		if !ok {
			fileName = archivecasc.FormatUnknownFile(int(fileDataID), ".png")
		} else {
			fileName = writers.ReplaceExtension(fileName, ".png")
		}
		var texPath string
		var texFileOut string
		if options.EnableSharedTextures {
			texPath = writers.GetExportPath(fileName)
			texFileOut, _ = filepath.Rel(dir, texPath)
		} else {
			texPath = filepath.Join(dir, filepath.Base(fileName))
			texFileOut = filepath.Base(texPath)
		}
		if err := storePNG(texPath, rgba, img.ScaledWidth, img.ScaledHeight, conv); err != nil {
			return "", err
		}
		if usePosix {
			return writers.Win32ToPosix(texFileOut), nil
		}
		return texFileOut, nil
	}

	materials := make([]*alphaMaterial, len(materialIDs))
	for i, diffuseFileDataID := range materialIDs {
		if diffuseFileDataID == 0 {
			continue
		}
		mat := &alphaMaterial{Scale: 1, FileDataID: diffuseFileDataID}
		materials[i] = mat
		file, err := saveLayerTexture(diffuseFileDataID)
		if err != nil {
			return err
		}
		mat.File = file
		if i < len(heightIDs) && heightIDs[i] > 0 {
			heightFile, err := saveLayerTexture(heightIDs[i])
			if err != nil {
				return err
			}
			mat.HeightFile = heightFile
			mat.HeightFileDataID = heightIDs[i]
		}
		if texParams != nil && i < len(texParams) {
			params := texParams[i]
			mat.Scale = mathPow2(float64((params.Flags & 0xF0) >> 4))
			if params.Height != 0 || params.Offset != 1 {
				mat.HeightScale = params.Height
				mat.HeightOffset = params.Offset
			}
		}
	}

	canvasSize := 64 * 16
	if split {
		canvasSize = 64
	}
	canvas := make([]byte, canvasSize*canvasSize*4)
	layers := make([]map[string]any, 0)
	chunkVertexColors := make([]map[string]any, 0)

	type alphaChunkJob struct {
		chunkIndex int
		imageData  []byte
		layerRows  []map[string]any
		vertexRow  map[string]any
	}
	jobs := make([]alphaChunkJob, 0, len(texAdt.TexChunks))
	for chunkIndex := 0; chunkIndex < len(texAdt.TexChunks); chunkIndex++ {
		texChunk := texAdt.TexChunks[chunkIndex]
		rootChunk := rootAdt.Chunks[chunkIndex]
		fixAlphaMap := rootChunk.Flags&(1<<15) == 0
		alphaLayers := texChunk.AlphaLayers
		imageData := make([]byte, 64*64*4)
		for i := 1; i < len(alphaLayers); i++ {
			layer := alphaLayers[i]
			for j := 0; j < len(layer) && j < 64*64; j++ {
				isLastColumn := j%64 == 63
				isLastRow := j >= 63*64
				channel := (j * 4) + (i - 1)
				if fixAlphaMap {
					switch {
					case isLastColumn && !isLastRow:
						imageData[channel] = layer[j-1]
					case isLastRow:
						imageData[channel] = layer[j-64]
					default:
						imageData[channel] = layer[j]
					}
				} else {
					imageData[channel] = layer[j]
				}
			}
		}
		for i := 0; i < 64*64; i++ {
			imageData[(i*4)+3] = 255
		}
		job := alphaChunkJob{chunkIndex: chunkIndex, imageData: imageData}
		for i, layer := range texChunk.Layers {
			if int(layer.TextureID) >= len(materials) {
				continue
			}
			mat := materials[layer.TextureID]
			if mat == nil {
				continue
			}
			entry := map[string]any{
				"index": i, "effectID": layer.EffectID,
				"scale": mat.Scale, "fileDataID": mat.FileDataID, "file": mat.File,
			}
			if !split {
				entry["chunkIndex"] = chunkIndex
			}
			if mat.HeightFile != "" {
				entry["heightFile"] = mat.HeightFile
				entry["heightFileDataID"] = mat.HeightFileDataID
			}
			if mat.HeightScale != 0 || mat.HeightOffset != 0 {
				entry["heightScale"] = mat.HeightScale
				entry["heightOffset"] = mat.HeightOffset
			}
			job.layerRows = append(job.layerRows, entry)
		}
		if len(rootChunk.VertexShading) > 0 {
			job.vertexRow = map[string]any{
				"chunkIndex": chunkIndex,
				"shading":    rgbaInts(rootChunk.VertexShading),
			}
		}
		jobs = append(jobs, job)
	}

	var progressMu sync.Mutex
	var assembleMu sync.Mutex
	var firstErr atomic.Value
	if err := formats.Queue(jobs, func(job alphaChunkJob) error {
		if split {
			if progress != nil {
				progressMu.Lock()
				progress.SetLabel(fmt.Sprintf("Tile %s, alpha maps", e.TileID), job.chunkIndex+1, len(texAdt.TexChunks))
				progressMu.Unlock()
			}
			filePrefix := fmt.Sprintf("%s_%d", e.TileID, job.chunkIndex)
			if err := storePNG(filepath.Join(dir, "tex_"+filePrefix+".png"), job.imageData, 64, 64, conv); err != nil {
				firstErr.Store(err)
				return err
			}
			if conv == nil {
				json := writers.NewJSONWriter(filepath.Join(dir, "tex_"+filePrefix+".json"))
				json.AddProperty("layers", job.layerRows)
				if job.vertexRow != nil {
					if shading, ok := job.vertexRow["shading"]; ok {
						json.AddProperty("vertexColors", shading)
					}
				}
				if err := json.Write(options.OverwriteFiles); err != nil {
					firstErr.Store(err)
					return err
				}
			}
			return nil
		}
		if progress != nil {
			progressMu.Lock()
			progress.SetLabel(fmt.Sprintf("Tile %s, alpha maps", e.TileID), job.chunkIndex+1, len(texAdt.TexChunks))
			progressMu.Unlock()
		}
		cx := job.chunkIndex % 16
		cy := job.chunkIndex / 16
		for row := 0; row < 64; row++ {
			srcOfs := row * 64 * 4
			dstOfs := ((cy*64+row)*canvasSize + cx*64) * 4
			copy(canvas[dstOfs:dstOfs+64*4], job.imageData[srcOfs:srcOfs+64*4])
		}
		assembleMu.Lock()
		layers = append(layers, job.layerRows...)
		if job.vertexRow != nil {
			chunkVertexColors = append(chunkVertexColors, job.vertexRow)
		}
		assembleMu.Unlock()
		return nil
	}, appconfig.MaxConcurrency()); err != nil {
		return err
	}
	if v := firstErr.Load(); v != nil {
		return v.(error)
	}

	if split {
		return nil
	}
	if err := storePNG(filepath.Join(dir, "tex_"+e.TileID+".png"), canvas, canvasSize, canvasSize, conv); err != nil {
		return err
	}
	if conv == nil {
		json := writers.NewJSONWriter(filepath.Join(dir, "tex_"+e.TileID+".json"))
		json.AddProperty("layers", layers)
		if len(chunkVertexColors) > 0 {
			json.AddProperty("vertexColors", chunkVertexColors)
		}
		return json.Write(options.OverwriteFiles)
	}
	return nil
}

func (e *Exporter) exportMinimap(
	ctx context.Context,
	getFile getFileFunc,
	dir string,
	quality int,
	options export.ADTExportOptions,
	progress *export.ProgressReporter,
	conv *ConversionOutput,
) error {
	_ = progress
	paddedX := fmt.Sprintf("%02d", e.TileY)
	paddedY := fmt.Sprintf("%02d", e.TileX)
	tilePath := fmt.Sprintf("world/minimaps/%s/map%s_%s.blp", e.MapDir, paddedX, paddedY)
	tileOutPath := filepath.Join(dir, "tex_"+e.TileID+".png")
	if conv == nil && !options.OverwriteFiles && writers.OutputFileExists(tileOutPath) {
		log.Write("Skipping ADT bake of %s (file exists, overwrite disabled)", tileOutPath)
		return nil
	}
	id, ok := archivecasc.GetByFilename(tilePath)
	if !ok {
		return fmt.Errorf("minimap %s not in listfile", tilePath)
	}
	raw, err := getFile(ctx, uint32(id))
	if err != nil {
		return err
	}
	img, err := blpfmt.NewBLPImage(buffer.From(raw))
	if err != nil {
		return err
	}
	rgba, err := img.ToUInt8Array(0, 0b0111)
	if err != nil {
		return err
	}
	scaled := ResizeBilinear(rgba, img.ScaledWidth, img.ScaledHeight, quality, quality)
	return storePNG(tileOutPath, scaled, quality, quality, conv)
}

func (e *Exporter) exportLargeBake(
	ctx context.Context,
	getFile getFileFunc,
	dir string,
	quality int,
	options export.ADTExportOptions,
	rootAdt, texAdt *adtfmt.ADTLoader,
	firstChunk adtfmt.ADTChunk,
	chunkMeshes [][]int,
	vertices, uvsBake, vertexColors []float32,
	split bool,
	progress *export.ProgressReporter,
	conv *ConversionOutput,
) error {
	tileOutPath := filepath.Join(dir, "tex_"+e.TileID+".png")
	if !split && conv == nil && !options.OverwriteFiles && writers.OutputFileExists(tileOutPath) {
		log.Write("Skipping ADT bake of %s (file exists, overwrite disabled)", tileOutPath)
		return nil
	}

	chunkSizePx := quality / 16
	if chunkSizePx < 1 {
		chunkSizePx = 1
	}
	materialIDs := texAdt.DiffuseTextureFileDataIDs
	texParams := texAdt.TexParams
	materials := make([]*BakeMaterial, len(materialIDs))
	if progress != nil {
		progress.SetLabel(fmt.Sprintf("Tile %s, loading textures", e.TileID), 0, len(materialIDs))
	}
	for i, diffuseFileDataID := range materialIDs {
		if diffuseFileDataID == 0 {
			continue
		}
		if progress != nil {
			progress.SetLabel(fmt.Sprintf("Tile %s, loading textures", e.TileID), i+1, len(materialIDs))
		}
		mat := &BakeMaterial{HeightScale: 0, HeightOffset: 1, Scale: 1}
		materials[i] = mat
		tex, err := LoadBakeTexture(ctx, getFile, diffuseFileDataID)
		if err != nil {
			continue
		}
		mat.DiffuseTex = tex
		if texParams != nil && i < len(texParams) {
			params := texParams[i]
			mat.Scale = mathPow2(float64((params.Flags & 0xF0) >> 4))
			if params.Height != 0 || params.Offset != 1 {
				mat.HeightScale = float64(params.Height)
				mat.HeightOffset = float64(params.Offset)
			}
		}
	}

	deltaX := float64(firstChunk.Position[1]) - float64(tileSize)
	deltaY := float64(firstChunk.Position[0]) - float64(tileSize)
	var composite []byte
	if !split {
		composite = make([]byte, quality*quality*4)
		for i := range composite {
			if i%4 == 3 {
				composite[i] = 255
			}
		}
	}

	type chunkBakeJob struct {
		x, y, chunkIndex, jobIndex int
	}
	jobs := make([]chunkBakeJob, 0, 256)
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			jobs = append(jobs, chunkBakeJob{
				x: x, y: y, chunkIndex: x*16 + y, jobIndex: x*16 + y + 1,
			})
		}
	}

	var progressMu sync.Mutex
	var assembleMu sync.Mutex
	var firstErr atomic.Value
	if err := formats.Queue(jobs, func(job chunkBakeJob) error {
		if progress != nil {
			progressMu.Lock()
			progress.SetLabel(fmt.Sprintf("Tile %s, baking textures", e.TileID), job.jobIndex, 256)
			progressMu.Unlock()
		}
		ofsX := -deltaX - (float64(chunkSize) * 7.5) + (float64(job.y) * float64(chunkSize))
		ofsY := -deltaY - (float64(chunkSize) * 7.5) + (float64(job.x) * float64(chunkSize))
		texChunk := texAdt.TexChunks[job.chunkIndex]
		fixAlphaMap := rootAdt.Chunks[job.chunkIndex].Flags&(1<<15) == 0
		alphaLayers := FixChunkAlphaLayers(texChunk.AlphaLayers, fixAlphaMap)
		var layerMaterials [4]*BakeMaterial
		for i, layer := range texChunk.Layers {
			if i >= 4 {
				break
			}
			if int(layer.TextureID) < len(materials) {
				layerMaterials[i] = materials[layer.TextureID]
			}
		}
		chunkCanvas := make([]byte, chunkSizePx*chunkSizePx*4)
		for i := range chunkCanvas {
			if i%4 == 3 {
				chunkCanvas[i] = 255
			}
		}
		BakeChunk(ChunkBakeParams{
			Canvas: chunkCanvas, CanvasSize: chunkSizePx, Indices: chunkMeshes[job.chunkIndex],
			Vertices: vertices, UvsBake: uvsBake, VertexColors: vertexColors,
			Translation: [2]float64{ofsX, ofsY}, TileSize: float64(tileSize), Zoom: 0.0625,
			Layers: layerMaterials, AlphaLayers: alphaLayers,
		})
		rotated := Rotate180(chunkCanvas, chunkSizePx)
		if split {
			outPath := filepath.Join(dir, fmt.Sprintf("tex_%s_%d.png", e.TileID, job.chunkIndex))
			if options.OverwriteFiles || !writers.OutputFileExists(outPath) {
				if err := storePNG(outPath, rotated, chunkSizePx, chunkSizePx, conv); err != nil {
					firstErr.Store(err)
					return err
				}
			}
			return nil
		}
		cx := job.chunkIndex % 16
		cy := job.chunkIndex / 16
		assembleMu.Lock()
		for row := 0; row < chunkSizePx; row++ {
			srcOfs := row * chunkSizePx * 4
			dstOfs := ((cy*chunkSizePx+row)*quality + cx*chunkSizePx) * 4
			copy(composite[dstOfs:dstOfs+chunkSizePx*4], rotated[srcOfs:srcOfs+chunkSizePx*4])
		}
		assembleMu.Unlock()
		return nil
	}, appconfig.MaxConcurrency()); err != nil {
		return err
	}
	if v := firstErr.Load(); v != nil {
		return v.(error)
	}
	if !split {
		if progress != nil {
			progress.SetLabel("Tile " + e.TileID + ", saving terrain texture")
		}
		return storePNG(tileOutPath, composite, quality, quality, conv)
	}
	return nil
}

func rgbaInts(colors []adtfmt.RGBA) []int {
	out := make([]int, len(colors))
	for i, c := range colors {
		out[i] = int(c.R)<<24 | int(c.G)<<16 | int(c.B)<<8 | int(c.A)
	}
	return out
}

type getFileFunc func(context.Context, uint32) ([]byte, error)
