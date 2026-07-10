package adt

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sync"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	adtfmt "github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/export"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/png"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

var wdtCache sync.Map

var (
	mapSize   = 64
	tileSize  = constants.Game.TileSize
	chunkSize = tileSize / 16
	unitSize  = chunkSize / 8
	unitHalf  = unitSize / 2
)

// Exporter exports ADT terrain tiles.
type Exporter struct {
	MapID     int
	MapDir    string
	TileX     int
	TileY     int
	TileID    string
	TileIndex int
}

// NewExporter creates an exporter for a tile index.
func NewExporter(mapID int, mapDir string, tileIndex int) *Exporter {
	tileX := tileIndex % mapSize
	tileY := tileIndex / mapSize
	return &Exporter{
		MapID: mapID, MapDir: mapDir, TileX: tileX, TileY: tileY,
		TileID: fmt.Sprintf("%d_%d", tileY, tileX), TileIndex: tileIndex,
	}
}

// ExportResult is the output path of the main OBJ file.
type ExportResult struct {
	Path string
}

// Export exports the ADT tile to dir.
func (e *Exporter) Export(ctx context.Context, source casc.Source, dir string, quality int, options export.ADTExportOptions, gameObjects map[uint32]db.DB2Row, progress *export.ProgressReporter) (ExportResult, error) {
	cfg := server.GetConfig()
	getFile := func(_ context.Context, fileDataID uint32) ([]byte, error) {
		return source.GetFile(ctx, int(fileDataID))
	}

	prefix := fmt.Sprintf("world/maps/%s/%s", e.MapDir, e.MapDir)
	var wdt *adtfmt.WDTLoader
	if cached, ok := wdtCache.Load(e.MapDir); ok {
		wdt = cached.(*adtfmt.WDTLoader)
	} else {
		wdtPath := prefix + ".wdt"
		wdtData, err := getFileByName(ctx, source, wdtPath)
		if err != nil {
			return ExportResult{}, err
		}
		wdt = adtfmt.NewWDTLoader(buffer.From(wdtData))
		wdt.Load()
		wdtCache.Store(e.MapDir, wdt)
	}

	tilePrefix := fmt.Sprintf("%s_%s", prefix, e.TileID)
	maid := wdt.Entries[e.TileIndex]
	rootID := resolveFileDataID(maid.RootADT, tilePrefix+".adt")
	tex0ID := resolveFileDataID(maid.Tex0ADT, tilePrefix+"_tex0.adt")
	obj0ID := resolveFileDataID(maid.Obj0ADT, tilePrefix+"_obj0.adt")
	obj1ID := resolveFileDataID(maid.Obj1ADT, tilePrefix+"_obj1.adt")
	if rootID == 0 || tex0ID == 0 || obj0ID == 0 || obj1ID == 0 {
		return ExportResult{}, fmt.Errorf("missing fileDataID for ADT files: %d,%d,%d,%d", rootID, tex0ID, obj0ID, obj1ID)
	}
	_ = obj1ID

	rootRaw, err := getFile(ctx, rootID)
	if err != nil {
		return ExportResult{}, err
	}
	texRaw, err := getFile(ctx, tex0ID)
	if err != nil {
		return ExportResult{}, err
	}
	objRaw, err := getFile(ctx, obj0ID)
	if err != nil {
		return ExportResult{}, err
	}

	rootAdt := adtfmt.NewADTLoader(buffer.From(rootRaw))
	rootAdt.LoadRoot()
	texAdt := adtfmt.NewADTLoader(buffer.From(texRaw))
	texAdt.LoadTex(wdt)
	objAdt := adtfmt.NewADTLoader(buffer.From(objRaw))
	objAdt.LoadObj()
	if progress != nil {
		progress.Advance()
		progress.SetLabel("Tile " + e.TileID + ", terrain mesh")
	}

	vertices := make([]float32, 16*16*145*3)
	normals := make([]float32, 16*16*145*3)
	uvs := make([]float32, 16*16*145*2)
	uvsBake := make([]float32, 16*16*145*2)
	vertexColors := make([]float32, 16*16*145*4)
	chunkMeshes := make([][]int, 256)

	objOut := filepath.Join(dir, "adt_"+e.TileID+".obj")
	mtlOut := filepath.Join(dir, "adt_"+e.TileID+".mtl")
	obj := &writers.OBJWriter{Out: objOut}
	mtl := &writers.MTLWriter{Out: mtlOut}

	firstChunk := rootAdt.Chunks[0]
	firstChunkX := firstChunk.Position[0]
	firstChunkY := firstChunk.Position[1]

	isAlphaMaps := quality == -1
	isLargeBake := quality >= 8192
	isSplittingAlphaMaps := isAlphaMaps && cfg.SplitAlphaMaps
	isSplittingTextures := isLargeBake && cfg.SplitLargeTerrainBakes
	includeHoles := cfg.MapsIncludeHoles

	var uvBounds *struct{ minU, maxU, minV, maxV float64 }
	if quality != 0 && !isSplittingTextures && !isSplittingAlphaMaps {
		b := e.calculateUVBounds(rootAdt, firstChunkX, firstChunkY)
		uvBounds = &b
	}

	ofs, chunkID := 0, 0
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			indices := []int{}
			chunkIndex := x*16 + y
			chunk := rootAdt.Chunks[chunkIndex]
			chunkX, chunkY, chunkZ := chunk.Position[0], chunk.Position[1], chunk.Position[2]
			idx, midX := 0, ofs
			for row := 0; row < 17; row++ {
				isShort := row%2 != 0
				colCount := 9
				if isShort {
					colCount = 8
				}
				for col := 0; col < colCount; col++ {
					vx := chunkY - float32(col)*float32(unitSize)
					vy := chunk.Vertices[idx] + chunkZ
					vz := chunkX - float32(row)*float32(unitHalf)
					if isShort {
						vx -= float32(unitHalf)
					}
					vIndex := midX * 3
					vertices[vIndex] = vx
					vertices[vIndex+1] = vy
					vertices[vIndex+2] = vz
					normal := chunk.Normals[idx]
					normals[vIndex] = float32(normal[0]) / 127
					normals[vIndex+1] = float32(normal[1]) / 127
					normals[vIndex+2] = float32(normal[2]) / 127
					cIndex := midX * 4
					if len(chunk.VertexShading) > 0 {
						color := chunk.VertexShading[idx]
						vertexColors[cIndex] = float32(color.B) / 255
						vertexColors[cIndex+1] = float32(color.G) / 255
						vertexColors[cIndex+2] = float32(color.R) / 255
						vertexColors[cIndex+3] = float32(color.A) / 255
					} else {
						vertexColors[cIndex], vertexColors[cIndex+1], vertexColors[cIndex+2] = 0.5, 0.5, 0.5
						vertexColors[cIndex+3] = 1
					}
					uvIdx := float32(col)
					if isShort {
						uvIdx += 0.5
					}
					uvIndex := midX * 2
					uRaw := -float64(vx-firstChunkX) / tileSize
					vRaw := float64(vz-firstChunkY) / tileSize
					uvsBake[uvIndex] = float32(uRaw)
					uvsBake[uvIndex+1] = float32(vRaw)
					if quality == 0 {
						uvs[uvIndex] = uvIdx / 8
						uvs[uvIndex+1] = float32(row) * 0.5 / 8
					} else if isSplittingTextures || isSplittingAlphaMaps {
						uvs[uvIndex] = uvIdx / 8
						uvs[uvIndex+1] = 1 - float32(row)/16
					} else if uvBounds != nil {
						denU := uvBounds.maxU - uvBounds.minU
						denV := uvBounds.maxV - uvBounds.minV
						if denU != 0 {
							uvs[uvIndex] = float32((uRaw - uvBounds.minU) / denU)
						}
						if denV != 0 {
							uvs[uvIndex+1] = float32((vRaw - uvBounds.minV) / denV)
						}
					} else {
						uvs[uvIndex], uvs[uvIndex+1] = float32(uRaw), float32(vRaw)
					}
					idx++
					midX++
				}
			}
			xx, yy := 0, 0
			for j := 9; j < 145; j++ {
				if xx >= 8 {
					xx = 0
					yy++
				}
				isHole := true
				if includeHoles {
					if chunk.Flags&0x10000 == 0 {
						current := 1 << (xx/2 + (yy/2)*4)
						if chunk.HolesLowRes&uint16(current) == 0 {
							isHole = false
						}
					} else {
						var hr uint8
						if yy < len(chunk.HolesHighRes) {
							hr = chunk.HolesHighRes[yy]
						}
						if (hr>>xx)&1 == 0 {
							isHole = false
						}
					}
				} else {
					isHole = false
				}
				if !isHole {
					indOfs := ofs + j
					indices = append(indices, indOfs, indOfs-9, indOfs+8, indOfs, indOfs-8, indOfs-9, indOfs, indOfs+9, indOfs-8, indOfs, indOfs+8, indOfs+9)
				}
				xx++
				if (j+1)%(9+8) == 0 {
					j += 9
				}
			}
			ofs = midX
			if isSplittingTextures || isSplittingAlphaMaps {
				objName := fmt.Sprintf("%s_%d", e.TileID, chunkID)
				matName := "tex_" + objName
				mtl.AddMaterial(matName, matName+".png")
				obj.AddMesh(objName, indices, matName)
			} else {
				obj.AddMesh(fmt.Sprintf("%d", chunkID), indices, "tex_"+e.TileID)
			}
			chunkMeshes[chunkIndex] = indices
			chunkID++
		}
	}

	if quality != 0 && ((!isAlphaMaps && !isSplittingTextures) || (isAlphaMaps && !isSplittingAlphaMaps)) {
		mtl.AddMaterial("tex_"+e.TileID, "tex_"+e.TileID+".png")
	}
	obj.SetVertArray(vertices)
	obj.SetNormalArray(normals)
	obj.AddUVArray(uvs)
	if !mtl.IsEmpty() {
		obj.SetMaterialLibrary(filepath.Base(mtl.Out))
	}
	if err := obj.Write(options.OverwriteFiles); err != nil {
		return ExportResult{}, err
	}
	if err := mtl.Write(options.OverwriteFiles); err != nil {
		return ExportResult{}, err
	}
	if progress != nil {
		progress.Advance()
		progress.SetLabel("Tile " + e.TileID + ", textures")
	}

	if quality != 0 && !isAlphaMaps {
		if err := e.exportTextures(ctx, getFile, dir, quality, options, rootAdt, texAdt, firstChunk, chunkMeshes, vertices, uvsBake, vertexColors, progress); err != nil {
			log.Write("Terrain bake warning: %v", err)
		}
		ClearBakeTextureCache()
		if progress != nil {
			progress.Advance()
		}
	}

	if options.MapsIncludeWMO || options.MapsIncludeM2 || options.MapsIncludeGameObjects {
		if progress != nil {
			progress.SetLabel("Tile " + e.TileID + ", model placements")
		}
		if err := e.exportModelPlacements(ctx, dir, options, objAdt, gameObjects, progress, getFile); err != nil {
			log.Write("Model placement export warning: %v", err)
		}
	}

	if options.MapsIncludeLiquid {
		if err := e.exportLiquid(rootAdt, dir, options, progress); err != nil {
			log.Write("Liquid export warning: %v", err)
		}
	}

	if options.MapsIncludeFoliage {
		if err := e.exportFoliage(ctx, dir, texAdt, progress); err != nil {
			log.Write("Foliage export warning: %v", err)
		}
	}

	_ = objAdt
	if progress != nil {
		progress.SyncTileComplete()
	}
	return ExportResult{Path: objOut}, nil
}

func (e *Exporter) calculateUVBounds(rootAdt *adtfmt.ADTLoader, firstChunkX, firstChunkY float32) struct{ minU, maxU, minV, maxV float64 } {
	minU, maxU := math.Inf(1), math.Inf(-1)
	minV, maxV := math.Inf(1), math.Inf(-1)
	for x := 0; x < 16; x++ {
		for y := 0; y < 16; y++ {
			chunk := rootAdt.Chunks[x*16+y]
			if len(chunk.Vertices) == 0 {
				continue
			}
			chunkX, chunkY := chunk.Position[0], chunk.Position[1]
			idx := 0
			for row := 0; row < 17; row++ {
				isShort := row%2 != 0
				colCount := 9
				if isShort {
					colCount = 8
				}
				for col := 0; col < colCount; col++ {
					vx := chunkY - float32(col)*float32(unitSize)
					vz := chunkX - float32(row)*float32(unitHalf)
					if isShort {
						vx -= float32(unitHalf)
					}
					u := -float64(vx-firstChunkX) / tileSize
					v := float64(vz-firstChunkY) / tileSize
					minU, maxU = math.Min(minU, u), math.Max(maxU, u)
					minV, maxV = math.Min(minV, v), math.Max(maxV, v)
					idx++
				}
			}
		}
	}
	return struct{ minU, maxU, minV, maxV float64 }{minU, maxU, minV, maxV}
}

func writePNG(path string, pixels []byte, width, height int) error {
	w := png.NewWriter(width, height)
	copy(w.Pixels(), pixels)
	data, err := w.Encode()
	if err != nil {
		return err
	}
	return writers.WriteOutputFile(path, data)
}

func getFileByName(ctx context.Context, source casc.Source, fileName string) ([]byte, error) {
	id, ok := archivecasc.GetByFilename(fileName)
	if !ok {
		return nil, fmt.Errorf("file %s not in listfile", fileName)
	}
	return source.GetFile(ctx, id)
}

func resolveFileDataID(maidID uint32, fallbackName string) uint32 {
	if maidID > 0 {
		return maidID
	}
	id, ok := archivecasc.GetByFilename(fallbackName)
	if ok {
		return uint32(id)
	}
	return 0
}

// ClearExporterCache clears WDT cache.
func ClearExporterCache() {
	wdtCache = sync.Map{}
	clearFoliageTables()
}

func mathPow2(exp float64) float64 {
	return math.Pow(2, exp)
}
