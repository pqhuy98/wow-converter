package adt

import (
	"context"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/wow/constants"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/export"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

var gameObjectsOnce sync.Once
var gameObjectsByMap map[uint32]map[uint32]db.DB2Row
var gameObjectsErr error

// ClearGameObjectsCache drops cached GameObjects.db2 data (e.g. after CASC unload).
func ClearGameObjectsCache() {
	gameObjectsOnce = sync.Once{}
	gameObjectsByMap = nil
	gameObjectsErr = nil
}

// BuildADTExportOptions builds normalized export options from config and overrides.
func BuildADTExportOptions(overrides map[string]any) export.ADTExportOptions {
	cfg := server.GetConfig()
	pickBool := func(key string, def bool) bool {
		if v, ok := overrides[key]; ok {
			if b, ok := v.(bool); ok {
				return b
			}
		}
		switch key {
		case "enableSharedTextures":
			return cfg.EnableSharedTextures
		case "overwriteFiles":
			return cfg.OverwriteFiles
		case "splitAlphaMaps":
			return cfg.SplitAlphaMaps
		case "splitLargeTerrainBakes":
			return cfg.SplitLargeTerrainBakes
		case "mapsIncludeHoles":
			return cfg.MapsIncludeHoles
		case "enableSharedChildren":
			return cfg.EnableSharedChildren
		case "enableAbsoluteCSVPaths":
			return cfg.EnableAbsoluteCSVPaths
		case "modelsExportCollision":
			return cfg.ModelsExportCollision
		case "mapsIncludeWMO":
			return cfg.MapsIncludeWMO
		case "mapsIncludeM2":
			return cfg.MapsIncludeM2
		case "mapsIncludeWMOSets":
			return cfg.MapsIncludeWMOSets
		case "exportFoliageMeta":
			return cfg.ExportFoliageMeta
		case "mapsIncludeFoliage":
			return cfg.MapsIncludeFoliage
		case "mapsIncludeLiquid":
			return cfg.MapsIncludeLiquid
		case "mapsIncludeGameObjects":
			return cfg.MapsIncludeGameObjects
		default:
			return def
		}
	}
	pathFormat := cfg.PathFormat
	if v, ok := overrides["pathFormat"].(string); ok && v != "" {
		pathFormat = v
	}
	return export.ADTExportOptions{
		PathFormat:             pathFormat,
		EnableSharedTextures:   pickBool("enableSharedTextures", cfg.EnableSharedTextures),
		OverwriteFiles:         pickBool("overwriteFiles", cfg.OverwriteFiles),
		SplitAlphaMaps:         pickBool("splitAlphaMaps", cfg.SplitAlphaMaps),
		SplitLargeTerrainBakes: pickBool("splitLargeTerrainBakes", cfg.SplitLargeTerrainBakes),
		MapsIncludeHoles:       pickBool("mapsIncludeHoles", cfg.MapsIncludeHoles),
		EnableSharedChildren:   pickBool("enableSharedChildren", cfg.EnableSharedChildren),
		EnableAbsoluteCSVPaths: pickBool("enableAbsoluteCSVPaths", cfg.EnableAbsoluteCSVPaths),
		ModelsExportCollision:  pickBool("modelsExportCollision", cfg.ModelsExportCollision),
		MapsIncludeWMO:         pickBool("mapsIncludeWMO", cfg.MapsIncludeWMO),
		MapsIncludeM2:          pickBool("mapsIncludeM2", cfg.MapsIncludeM2),
		MapsIncludeWMOSets:     pickBool("mapsIncludeWMOSets", cfg.MapsIncludeWMOSets),
		ExportFoliageMeta:      pickBool("exportFoliageMeta", cfg.ExportFoliageMeta),
		MapsIncludeFoliage:     pickBool("mapsIncludeFoliage", cfg.MapsIncludeFoliage),
		MapsIncludeLiquid:      pickBool("mapsIncludeLiquid", cfg.MapsIncludeLiquid),
		MapsIncludeGameObjects: pickBool("mapsIncludeGameObjects", cfg.MapsIncludeGameObjects),
	}
}

// TileBounds returns world bounds for an ADT tile.
func TileBounds(tileX, tileY int) (startX, startY, endX, endY float64) {
	tileSize := constants.Game.TileSize
	mapOffset := constants.Game.MapOffset
	startX = mapOffset - float64(tileX)*tileSize - tileSize
	startY = mapOffset - float64(tileY)*tileSize - tileSize
	return startX, startY, startX + tileSize, startY + tileSize
}

// CollectGameObjects returns game objects for a map, optionally filtered.
func CollectGameObjects(ctx context.Context, mapID uint32, filter func(db.DB2Row) bool) (map[uint32]db.DB2Row, error) {
	gameObjectsOnce.Do(func() {
		gameObjectsByMap = make(map[uint32]map[uint32]db.DB2Row)
		objTable := db.NewWDCReader("DBFilesClient/GameObjects.db2", nil)
		if err := objTable.Parse(ctx, nil); err != nil {
			gameObjectsErr = err
			return
		}
		idTable := db.NewWDCReader("DBFilesClient/GameObjectDisplayInfo.db2", nil)
		if err := idTable.Parse(ctx, nil); err != nil {
			gameObjectsErr = err
			return
		}
		for _, row := range objTable.GetAllRows() {
			displayID := toUint32(row["DisplayID"])
			fidRow := idTable.GetRow(displayID)
			if fidRow == nil {
				continue
			}
			row["FileDataID"] = fidRow["FileDataID"]
			ownerID := toUint32(row["OwnerID"])
			if gameObjectsByMap[ownerID] == nil {
				gameObjectsByMap[ownerID] = make(map[uint32]db.DB2Row)
			}
			id := toUint32(row["ID"])
			if id == 0 {
				for k, v := range row {
					if k == "ID" {
						id = toUint32(v)
					}
				}
			}
			gameObjectsByMap[ownerID][id] = row
		}
	})
	if gameObjectsErr != nil {
		return nil, gameObjectsErr
	}
	result := make(map[uint32]db.DB2Row)
	for id, obj := range gameObjectsByMap[mapID] {
		if filter == nil || filter(obj) {
			result[id] = obj
		}
	}
	return result, nil
}

// GameObjectPosition returns the DB2 game object's map position.
func GameObjectPosition(row db.DB2Row) []float64 {
	return numberSlice(row["Pos"])
}

func numberSlice(value any) []float64 {
	switch values := value.(type) {
	case []float64:
		return values
	case []float32:
		out := make([]float64, len(values))
		for i, value := range values {
			out[i] = float64(value)
		}
		return out
	case []any:
		out := make([]float64, 0, len(values))
		for _, value := range values {
			number, ok := toFloat64(value)
			if !ok {
				return nil
			}
			out = append(out, number)
		}
		return out
	default:
		return nil
	}
}

func toFloat64(value any) (float64, bool) {
	switch number := value.(type) {
	case float64:
		return number, true
	case float32:
		return float64(number), true
	case int:
		return float64(number), true
	case int32:
		return float64(number), true
	case int64:
		return float64(number), true
	case uint:
		return float64(number), true
	case uint32:
		return float64(number), true
	case uint64:
		return float64(number), true
	default:
		return 0, false
	}
}

func toUint32(v any) uint32 {
	switch x := v.(type) {
	case uint32:
		return x
	case int32:
		return uint32(x)
	case int64:
		return uint32(x)
	case uint64:
		return uint32(x)
	case int:
		return uint32(x)
	case float32:
		return uint32(x)
	case float64:
		return uint32(x)
	case []uint32:
		if len(x) > 0 {
			return x[0]
		}
	case []int64:
		if len(x) > 0 {
			return uint32(x[0])
		}
	case []float32:
		if len(x) > 0 {
			return uint32(x[0])
		}
	case []float64:
		if len(x) > 0 {
			return uint32(x[0])
		}
	default:
		return 0
	}
	return 0
}
