package adt

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sync"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	adtfmt "github.com/pqhuy98/wow-converter/internal/wow/formats/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/export"
	exportwriters "github.com/pqhuy98/wow-converter/internal/wow/export"
	wmoexport "github.com/pqhuy98/wow-converter/internal/wow/export/wmo"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

var (
	foliageOnce      sync.Once
	foliageAvailable bool
	foliageLoaded    bool
	dbTextures       *db.WDCReader
	dbDoodads        *db.WDCReader
)

func clearFoliageTables() {
	dbTextures = nil
	dbDoodads = nil
	foliageLoaded = false
	foliageAvailable = false
	foliageOnce = sync.Once{}
}

func loadFoliageTables(ctx context.Context) {
	foliageOnce.Do(func() {
		if foliageLoaded {
			return
		}
		dbDoodads = db.NewWDCReader("DBFilesClient/GroundEffectDoodad.db2", nil)
		dbTextures = db.NewWDCReader("DBFilesClient/GroundEffectTexture.db2", nil)
		if err := dbDoodads.Parse(ctx, nil); err != nil {
			foliageAvailable = false
			log.Write("Unable to load foliage tables, foliage exporting will be unavailable for all tiles.")
			foliageLoaded = true
			return
		}
		if err := dbTextures.Parse(ctx, nil); err != nil {
			foliageAvailable = false
			log.Write("Unable to load foliage tables, foliage exporting will be unavailable for all tiles.")
			foliageLoaded = true
			return
		}
		foliageAvailable = true
		foliageLoaded = true
	})
}

func (e *Exporter) exportModelPlacements(
	ctx context.Context,
	dir string,
	options export.ADTExportOptions,
	objAdt *adtfmt.ADTLoader,
	gameObjects map[uint32]db.DB2Row,
	progress *export.ProgressReporter,
	getFile func(context.Context, uint32) ([]byte, error),
) error {
	usePosix := options.PathFormat == "posix"
	csvPath := filepath.Join(dir, "adt_"+e.TileID+"_ModelPlacementInformation.csv")
	if !options.OverwriteFiles && writers.OutputFileExists(csvPath) {
		log.Write("Skipping model placement export %s (file exists, overwrite disabled)", csvPath)
		if progress != nil {
			progress.Advance()
		}
		return nil
	}

	csv := &writers.CSVWriter{Out: csvPath}
	csv.AddField("ModelFile", "PositionX", "PositionY", "PositionZ", "RotationX", "RotationY", "RotationZ", "RotationW", "ScaleFactor", "ModelId", "Type", "FileDataID", "DoodadSetIndexes", "DoodadSetNames")

	exportDoodads := func(exportType, csvName string, objects []adtfmt.DoodadEntry) {
		log.Write("Writing %d %s placements to CSV...", len(objects), exportType)
		for i, model := range objects {
			if progress != nil {
				progress.SetLabel(fmt.Sprintf("Tile %s, %s", e.TileID, exportType), i+1, len(objects))
			}
			refName := exportwriters.ModelReferencePath(model.MmidEntry, "m2", nil)
			modelPath := exportwriters.ResolveModelStoragePath(refName, dir, options.EnableSharedChildren)
			modelFile, _ := filepath.Rel(dir, modelPath)
			if usePosix {
				modelFile = writers.Win32ToPosix(modelFile)
			}
			csv.AddRow(map[string]any{
				"ModelFile": modelFile, "PositionX": model.Position[0], "PositionY": model.Position[1], "PositionZ": model.Position[2],
				"RotationX": model.Rotation[0], "RotationY": model.Rotation[1], "RotationZ": model.Rotation[2], "RotationW": 0.0,
				"ScaleFactor": float64(model.Scale) / 1024, "ModelId": model.UniqueID, "Type": csvName, "FileDataID": model.MmidEntry,
				"DoodadSetIndexes": 0, "DoodadSetNames": "",
			})
		}
	}

	exportGameObjects := func(objects map[uint32]db.DB2Row) {
		log.Write("Writing %d game objects placements to CSV...", len(objects))
		i := 0
		for _, model := range objects {
			i++
			if progress != nil {
				progress.SetLabel(fmt.Sprintf("Tile %s, game objects", e.TileID), i, len(objects))
			}
			fileDataID := rowUint32(model["FileDataID"])
			refName := exportwriters.ModelReferencePath(fileDataID, "m2", nil)
			modelPath := exportwriters.ResolveModelStoragePath(refName, dir, options.EnableSharedChildren)
			modelFile, _ := filepath.Rel(dir, modelPath)
			if usePosix {
				modelFile = writers.Win32ToPosix(modelFile)
			}
			pos := rowFloatSlice(model["Position"])
			rot := rowFloatSlice(model["Rotation"])
			csv.AddRow(map[string]any{
				"ModelFile": modelFile,
				"PositionX": posAt(pos, 0), "PositionY": posAt(pos, 1), "PositionZ": posAt(pos, 2),
				"RotationX": posAt(rot, 0), "RotationY": posAt(rot, 1), "RotationZ": posAt(rot, 2), "RotationW": posAt(rot, 3),
				"ScaleFactor": 1.0, "ModelId": rowUint32(model["uniqueId"]), "Type": "gobj", "FileDataID": fileDataID,
				"DoodadSetIndexes": 0, "DoodadSetNames": "",
			})
		}
	}

	exportWMOs := func() {
		worldModels := objAdt.WorldModels
		log.Write("Writing %d WMO placements to CSV...", len(worldModels))
		setNameCache := make(map[uint32][]string)
		objectCache := make(map[string]struct{})
		usingNames := len(objAdt.WmoNames) > 0
		for i, model := range worldModels {
			if progress != nil {
				progress.SetLabel(fmt.Sprintf("Tile %s, WMO objects", e.TileID), i+1, len(worldModels))
			}
			useADTSets := model.Flags&0x80 != 0
			var fileDataID uint32
			if usingNames {
				offset := objAdt.WmoOffsets[model.MwidEntry]
				fileName := objAdt.WmoNames[int(offset)]
				if id, ok := archivecasc.GetByFilename(fileName); ok {
					fileDataID = uint32(id)
				}
			} else {
				fileDataID = model.MwidEntry
			}
			if fileDataID == 0 {
				continue
			}

			doodadSets := []uint16{model.DoodadSet}
			if useADTSets {
				doodadSets = objAdt.DoodadSets
			}
			cacheID := fmt.Sprintf("%d-%s", fileDataID, joinUint16(doodadSets))

			refName := exportwriters.ModelReferencePath(fileDataID, "wmo", intPtr(int(model.DoodadSet)))
			modelPath := exportwriters.ResolveModelStoragePath(refName, dir, options.EnableSharedChildren)

			if _, cached := objectCache[cacheID]; !cached {
				raw, err := getFile(ctx, fileDataID)
				if err != nil {
					log.Write("Failed to export WMO [%d]: %v", fileDataID, err)
					continue
				}
				wmoExp := wmoexport.NewExporter(raw, int(fileDataID))
				if err := wmoExp.Loader.Load(); err != nil {
					log.Write("Failed to load WMO [%d]: %v", fileDataID, err)
					continue
				}
				names := make([]string, len(wmoExp.Loader.DoodadSets))
				for si, set := range wmoExp.Loader.DoodadSets {
					names[si] = set.Name
				}
				setNameCache[fileDataID] = names

				if options.MapsIncludeWMOSets {
					mask := buildWMOSetsMask(wmoExp.Loader.DoodadSets, useADTSets, doodadSets, model.DoodadSet)
					wmoExp.SetDoodadSetMask(mask)
					if err := wmoExp.ExportDoodadPlacementCsv(modelPath, options, progress); err != nil {
						log.Write("Failed to export WMO interior CSV [%d]: %v", fileDataID, err)
					}
				}
				objectCache[cacheID] = struct{}{}
			}

			doodadNames := setNameCache[fileDataID]
			modelFile, _ := filepath.Rel(dir, modelPath)
			if usePosix {
				modelFile = writers.Win32ToPosix(modelFile)
			}
			setIndexes := joinUint16(doodadSets)
			setNames := joinDoodadSetNames(doodadSets, doodadNames)
			csv.AddRow(map[string]any{
				"ModelFile": modelFile, "PositionX": model.Position[0], "PositionY": model.Position[1], "PositionZ": model.Position[2],
				"RotationX": model.Rotation[0], "RotationY": model.Rotation[1], "RotationZ": model.Rotation[2], "RotationW": 0.0,
				"ScaleFactor": float64(model.Scale) / 1024, "ModelId": model.UniqueID, "Type": "wmo", "FileDataID": fileDataID,
				"DoodadSetIndexes": setIndexes, "DoodadSetNames": setNames,
			})
		}
	}

	if options.MapsIncludeGameObjects && len(gameObjects) > 0 {
		exportGameObjects(gameObjects)
	}
	if options.MapsIncludeM2 {
		exportDoodads("doodads", "m2", objAdt.Models)
	}
	if options.MapsIncludeWMO {
		exportWMOs()
	}
	if progress != nil {
		progress.Advance()
	}
	return csv.Write(options.OverwriteFiles)
}

func (e *Exporter) exportLiquid(rootAdt *adtfmt.ADTLoader, dir string, options export.ADTExportOptions, progress *export.ProgressReporter) error {
	if len(rootAdt.LiquidChunks) == 0 {
		return nil
	}
	liquidFile := filepath.Join(dir, "liquid_"+e.TileID+".json")
	log.Write("Exporting liquid data to %s", liquidFile)
	if progress != nil {
		progress.SetLabel("Tile " + e.TileID + ", liquid")
		progress.Advance()
	}

	enhanced := make([]map[string]any, len(rootAdt.LiquidChunks))
	for chunkIndex, chunk := range rootAdt.LiquidChunks {
		if len(chunk.Instances) == 0 {
			enhanced[chunkIndex] = liquidChunkToMap(chunk)
			continue
		}
		terrainChunk := rootAdt.Chunks[chunkIndex]
		instances := make([]map[string]any, len(chunk.Instances))
		for i, instance := range chunk.Instances {
			chunkX := terrainChunk.Position[0]
			chunkY := terrainChunk.Position[1]
			chunkZ := terrainChunk.Position[2]
			centerX := float64(instance.XOffset) + float64(instance.Width)/2
			centerY := float64(instance.YOffset) + float64(instance.Height)/2
			worldX := float64(chunkY) - centerX*float64(unitSize)
			worldY := (float64(instance.MinHeightLevel)+float64(instance.MaxHeightLevel))/2 + float64(chunkZ)
			worldZ := float64(chunkX) - centerY*float64(unitSize)
			entry := liquidInstanceToMap(instance)
			entry["worldPosition"] = []float64{worldX, worldY, worldZ}
			entry["terrainChunkPosition"] = []float32{chunkX, chunkY, chunkZ}
			instances[i] = entry
		}
		chunkMap := liquidChunkToMap(chunk)
		chunkMap["instances"] = instances
		enhanced[chunkIndex] = chunkMap
	}

	json := writers.NewJSONWriter(liquidFile)
	json.AddProperty("liquidChunks", enhanced)
	return json.Write(options.OverwriteFiles)
}

func (e *Exporter) exportFoliage(ctx context.Context, dir string, texAdt *adtfmt.ADTLoader, progress *export.ProgressReporter) error {
	if !server.GetConfig().MapsIncludeFoliage {
		return nil
	}
	loadFoliageTables(ctx)
	if !foliageAvailable {
		return nil
	}
	foliageDir := filepath.Join(dir, "foliage")
	log.Write("Exporting foliage to %s", foliageDir)
	if progress != nil {
		progress.SetLabel("Tile " + e.TileID + ", foliage")
	}

	exportCache := make(map[uint32]struct{})
	effectCache := make(map[int32]struct{})
	for _, chunk := range texAdt.TexChunks {
		if chunk.Layers == nil {
			continue
		}
		for _, layer := range chunk.Layers {
			if layer.EffectID == 0 {
				continue
			}
			groundEffectTexture := dbTextures.GetRow(uint32(layer.EffectID))
			if groundEffectTexture == nil {
				continue
			}
			doodadIDs, ok := groundEffectTexture["DoodadID"].([]uint32)
			if !ok {
				doodadIDs = rowUint32Slice(groundEffectTexture["DoodadID"])
			}
			if len(doodadIDs) == 0 {
				continue
			}

			var foliageJSON *writers.JSONWriter
			if server.GetConfig().ExportFoliageMeta {
				if _, seen := effectCache[layer.EffectID]; !seen {
					foliageJSON = writers.NewJSONWriter(filepath.Join(foliageDir, fmt.Sprintf("%d.json", layer.EffectID)))
					for k, v := range groundEffectTexture {
						foliageJSON.AddProperty(k, v)
					}
					effectCache[layer.EffectID] = struct{}{}
				}
			}

			doodadModelIDs := make(map[string]map[string]any)
			for _, doodadEntryID := range doodadIDs {
				if doodadEntryID == 0 {
					continue
				}
				groundEffectDoodad := dbDoodads.GetRow(doodadEntryID)
				if groundEffectDoodad == nil {
					continue
				}
				modelID := rowUint32(groundEffectDoodad["ModelFileID"])
				entry := map[string]any{"fileDataID": modelID}
				doodadModelIDs[fmt.Sprintf("%d", doodadEntryID)] = entry
				if modelID == 0 || hasKey(exportCache, modelID) {
					continue
				}
				exportCache[modelID] = struct{}{}
			}

			if foliageJSON != nil {
				for _, entry := range doodadModelIDs {
					if modelID, ok := entry["fileDataID"].(uint32); ok {
						name := filepath.Base(exportwriters.ModelReferencePath(modelID, "m2", nil))
						entry["fileName"] = name
					}
				}
				foliageJSON.AddProperty("DoodadModelIDs", doodadModelIDs)
				if err := foliageJSON.Write(true); err != nil {
					return err
				}
			}
		}
	}
	if progress != nil {
		progress.Advance()
	}
	return nil
}

func liquidChunkToMap(chunk adtfmt.LiquidChunk) map[string]any {
	return map[string]any{
		"attributes": map[string]any{
			"fishable": chunk.Attributes.Fishable,
			"deep":     chunk.Attributes.Deep,
		},
		"instances": liquidInstancesToMaps(chunk.Instances),
	}
}

func liquidInstancesToMaps(instances []adtfmt.LiquidInstance) []any {
	if len(instances) == 0 {
		return nil
	}
	out := make([]any, len(instances))
	for i, inst := range instances {
		out[i] = liquidInstanceToMap(inst)
	}
	return out
}

func liquidInstanceToMap(instance adtfmt.LiquidInstance) map[string]any {
	return map[string]any{
		"chunkIndex": instance.ChunkIndex, "instanceIndex": instance.InstanceIndex,
		"liquidType": instance.LiquidType, "liquidObject": instance.LiquidObject,
		"minHeightLevel": instance.MinHeightLevel, "maxHeightLevel": instance.MaxHeightLevel,
		"xOffset": instance.XOffset, "yOffset": instance.YOffset,
		"width": instance.Width, "height": instance.Height,
		"bitmap": instance.Bitmap,
		"vertexData": map[string]any{
			"height": sanitizeFloat32Slice(instance.VertexData.Height),
			"depth":  instance.VertexData.Depth,
			"uv":     instance.VertexData.UV,
		},
		"offsetExistsBitmap": instance.OffsetExistsBitmap,
		"offsetVertexData":   instance.OffsetVertexData,
	}
}

func rowUint32(v any) uint32 {
	switch x := v.(type) {
	case uint32:
		return x
	case int32:
		return uint32(x)
	case int64:
		return uint32(x)
	case int:
		return uint32(x)
	case float64:
		return uint32(x)
	default:
		return 0
	}
}

func rowUint32Slice(v any) []uint32 {
	switch x := v.(type) {
	case []uint32:
		return x
	case []int64:
		out := make([]uint32, len(x))
		for i, n := range x {
			out[i] = uint32(n)
		}
		return out
	default:
		return nil
	}
}

func rowFloatSlice(v any) []float64 {
	switch x := v.(type) {
	case []float32:
		out := make([]float64, len(x))
		for i, n := range x {
			out[i] = float64(n)
		}
		return out
	case []float64:
		return x
	default:
		return nil
	}
}

func posAt(values []float64, index int) float64 {
	if index < len(values) {
		return values[index]
	}
	return 0
}

func intPtr(v int) *int { return &v }

func joinUint16(values []uint16) string {
	if len(values) == 0 {
		return ""
	}
	out := fmt.Sprintf("%d", values[0])
	for i := 1; i < len(values); i++ {
		out += fmt.Sprintf(",%d", values[i])
	}
	return out
}

func joinStrings(values []string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for i := 1; i < len(values); i++ {
		out += "," + values[i]
	}
	return out
}

func buildWMOSetsMask(sets []wmo.DoodadSet, useADTSets bool, adtSets []uint16, modelSet uint16) []wmoexport.DoodadSetMaskEntry {
	mask := make([]wmoexport.DoodadSetMaskEntry, len(sets))
	if len(mask) > 0 {
		mask[0].Checked = true
	}
	if useADTSets {
		for _, idx := range adtSets {
			if int(idx) < len(mask) {
				mask[idx].Checked = true
			}
		}
	} else if int(modelSet) < len(mask) {
		mask[modelSet].Checked = true
	}
	return mask
}

func joinDoodadSetNames(setIndexes []uint16, names []string) string {
	if len(setIndexes) == 0 {
		return ""
	}
	parts := make([]string, 0, len(setIndexes))
	for _, idx := range setIndexes {
		if int(idx) < len(names) && names[idx] != "" {
			parts = append(parts, names[idx])
		} else {
			parts = append(parts, fmt.Sprintf("Set_%d", idx))
		}
	}
	return joinStrings(parts)
}

func hasKey(m map[uint32]struct{}, key uint32) bool {
	_, ok := m[key]
	return ok
}

func sanitizeFloat32Slice(vals []float32) []float64 {
	out := make([]float64, len(vals))
	for i, v := range vals {
		f := float64(v)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			f = 0
		}
		out[i] = f
	}
	return out
}
