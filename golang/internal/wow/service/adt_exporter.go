package service

import (
	"context"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/export"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

// ADTExporterService implements casc.ADTExporter.
type ADTExporterService struct{}

func (ADTExporterService) Export(ctx context.Context, params casc.ADTExportParams, exportID int) (casc.ADTExportResult, error) {
	source, err := server.GlobalRuntime.GetCasc()
	if err != nil {
		return casc.ADTExportResult{}, err
	}

	quality := server.GetConfig().ExportMapQuality
	if params.Quality != nil {
		quality = *params.Quality
	}

	overrides := map[string]any{}
	if params.IncludeM2 != nil {
		overrides["mapsIncludeM2"] = *params.IncludeM2
	}
	if params.IncludeWMO != nil {
		overrides["mapsIncludeWMO"] = *params.IncludeWMO
	}
	if params.IncludeWMOSets != nil {
		overrides["mapsIncludeWMOSets"] = *params.IncludeWMOSets
	}
	if params.IncludeGameObjects != nil {
		overrides["mapsIncludeGameObjects"] = *params.IncludeGameObjects
	}
	if params.IncludeLiquid != nil {
		overrides["mapsIncludeLiquid"] = *params.IncludeLiquid
	}
	if params.IncludeFoliage != nil {
		overrides["mapsIncludeFoliage"] = *params.IncludeFoliage
	}
	if params.IncludeHoles != nil {
		overrides["mapsIncludeHoles"] = *params.IncludeHoles
	}
	if params.SplitAlphaMaps != nil {
		overrides["splitAlphaMaps"] = *params.SplitAlphaMaps
	}
	if params.SplitLargeTerrainBakes != nil {
		overrides["splitLargeTerrainBakes"] = *params.SplitLargeTerrainBakes
	}
	options := exportadt.BuildADTExportOptions(overrides)

	gameObjects := map[uint32]db.DB2Row{}
	if len(params.GameObjects) > 0 {
		for _, raw := range params.GameObjects {
			rowMap, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			row := make(db.DB2Row, len(rowMap))
			for k, v := range rowMap {
				row[k] = v
			}
			gameObjects[gameObjectID(row)] = row
		}
	} else if options.MapsIncludeGameObjects {
		startX, startY, endX, endY := exportadt.TileBounds(params.TileX, params.TileY)
		collected, err := exportadt.CollectGameObjects(ctx, uint32(params.MapID), func(obj db.DB2Row) bool {
			pos := gameObjectPos(obj)
			if len(pos) < 2 {
				return false
			}
			return pos[0] > startX && pos[0] < endX && pos[1] > startY && pos[1] < endY
		})
		if err != nil {
			// Game objects are optional; missing TACT keys must not fail terrain export.
			gameObjects = map[uint32]db.DB2Row{}
		} else {
			gameObjects = collected
		}
	}

	baseDir := writers.GetExportPath(filepath.Join("maps", params.MapDir))
	exporter := exportadt.NewExporter(params.MapID, params.MapDir, params.TileX*64+params.TileY)

	var progress *export.ProgressReporter
	if params.ProgressKey != "" && params.TileIndex != nil && params.TileCount != nil && params.StepsPerTile != nil {
		progress = export.CreateBatchExportProgress(export.BatchExportProgressParams{
			Key: params.ProgressKey, TileIndex: *params.TileIndex, TileCount: *params.TileCount,
			StepsPerTile: *params.StepsPerTile, CurrentTile: casc.TileCoord{X: params.TileX, Y: params.TileY},
		})
	}

	result, err := exporter.Export(ctx, source, baseDir, quality, options, gameObjects, progress, nil)
	if err != nil {
		return casc.ADTExportResult{}, err
	}

	relMain := ""
	if result.Path != "" {
		rel, err := filepath.Rel(server.GetConfig().ExportDirectory, result.Path)
		if err != nil {
			rel = result.Path
		}
		relMain = rel
	}

	return casc.ADTExportResult{
		ExportID: exportID, MapID: params.MapID, MapDir: params.MapDir,
		TileX: params.TileX, TileY: params.TileY,
		TileIndex:  tileIndex(params),
		ExportPath: baseDir, ExportType: "ADT_OBJ", MainFile: &relMain,
	}, nil
}

// ExportForConversion loads an ADT tile into memory for WC3 map conversion.
func (ADTExporterService) ExportForConversion(ctx context.Context, params casc.ADTExportParams) (*exportadt.ConversionOutput, error) {
	source, err := server.GlobalRuntime.GetCasc()
	if err != nil {
		return nil, err
	}

	quality := server.GetConfig().ExportMapQuality
	if params.Quality != nil {
		quality = *params.Quality
	}

	overrides := map[string]any{
		"mapsIncludeM2": true, "mapsIncludeWMO": true,
		"mapsIncludeGameObjects": true, "mapsIncludeHoles": true,
		"mapsIncludeLiquid": false, "mapsIncludeFoliage": false,
	}
	if params.IncludeM2 != nil {
		overrides["mapsIncludeM2"] = *params.IncludeM2
	}
	if params.IncludeWMO != nil {
		overrides["mapsIncludeWMO"] = *params.IncludeWMO
	}
	if params.IncludeWMOSets != nil {
		overrides["mapsIncludeWMOSets"] = *params.IncludeWMOSets
	}
	if params.IncludeGameObjects != nil {
		overrides["mapsIncludeGameObjects"] = *params.IncludeGameObjects
	}
	if params.IncludeHoles != nil {
		overrides["mapsIncludeHoles"] = *params.IncludeHoles
	}
	options := exportadt.BuildADTExportOptions(overrides)

	gameObjects := map[uint32]db.DB2Row{}
	if options.MapsIncludeGameObjects {
		startX, startY, endX, endY := exportadt.TileBounds(params.TileX, params.TileY)
		collected, err := exportadt.CollectGameObjects(ctx, uint32(params.MapID), func(obj db.DB2Row) bool {
			pos := gameObjectPos(obj)
			if len(pos) < 2 {
				return false
			}
			return pos[0] > startX && pos[0] < endX && pos[1] > startY && pos[1] < endY
		})
		if err == nil {
			gameObjects = collected
		}
	}

	exportAssetDir := params.ExportAssetDir
	if exportAssetDir == "" {
		exportAssetDir = server.GetConfig().ExportDirectory
	}

	exporter := exportadt.NewExporter(params.MapID, params.MapDir, params.TileX*64+params.TileY)
	return exporter.ExportForConversion(ctx, source, exportAssetDir, quality, options, gameObjects, nil)
}

func tileIndex(params casc.ADTExportParams) int {
	if params.TileIndex != nil {
		return *params.TileIndex
	}
	return params.TileX*64 + params.TileY
}

func gameObjectID(row db.DB2Row) uint32 {
	switch v := row["ID"].(type) {
	case float64:
		return uint32(v)
	case int:
		return uint32(v)
	case uint32:
		return v
	default:
		return 0
	}
}

func gameObjectPos(row db.DB2Row) []float64 {
	switch v := row["Pos"].(type) {
	case []float64:
		return v
	case []float32:
		out := make([]float64, len(v))
		for i, n := range v {
			out[i] = float64(n)
		}
		return out
	case []any:
		out := make([]float64, 0, len(v))
		for _, item := range v {
			if n, ok := item.(float64); ok {
				out = append(out, n)
			}
		}
		return out
	default:
		return nil
	}
}
