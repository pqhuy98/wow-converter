package mapexporter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

// TileCoord is an ADT tile coordinate.
type TileCoord struct {
	X, Y int
}

// TileLoadFailure identifies an ADT tile that could not be loaded.
type TileLoadFailure struct {
	Tile TileCoord
	Err  error
}

// LoadADTTilesForConversion loads ADT tiles into memory for WC3 conversion.
// CASC access goes through wowClient (wow-data-server); in-process CASC is used only when wowClient is nil.
func LoadADTTilesForConversion(
	ctx context.Context,
	wowClient client.Client,
	exportAssetDir string,
	mc MapExportConfig,
	quality int,
	tiles []TileCoord,
	includeInteriors bool,
	registry *common.TileRegistry,
	onTileDone func(completed, total int, tile TileCoord),
) error {
	_, _, err := LoadADTTilesForConversionSummary(
		ctx, wowClient, exportAssetDir, mc, quality, tiles, includeInteriors, registry, onTileDone,
	)
	return err
}

// LoadADTTilesForConversionSummary loads all requested tiles and returns
// deterministic success and failure lists for API job reporting.
func LoadADTTilesForConversionSummary(
	ctx context.Context,
	wowClient client.Client,
	exportAssetDir string,
	mc MapExportConfig,
	quality int,
	tiles []TileCoord,
	includeInteriors bool,
	registry *common.TileRegistry,
	onTileDone func(completed, total int, tile TileCoord),
) ([]TileCoord, []TileLoadFailure, error) {
	if registry == nil {
		return nil, nil, fmt.Errorf("tile registry is nil")
	}
	if wowClient == nil && server.GlobalRuntime.GetCascOptional() == nil {
		return nil, nil, fmt.Errorf("no CASC source has been loaded")
	}

	includeWMOSets := includeInteriors
	var completed atomic.Int32
	var resultMu sync.Mutex
	loaded := make([]TileCoord, 0, len(tiles))
	failures := make([]TileLoadFailure, 0)
	tasks := make([]func() error, 0, len(tiles))
	for _, tile := range tiles {
		tile := tile
		tasks = append(tasks, func() error {
			snapshot, err := loadTileSnapshot(ctx, wowClient, exportAssetDir, mc, quality, tile, includeInteriors, includeWMOSets)
			if err != nil {
				resultMu.Lock()
				failures = append(failures, TileLoadFailure{Tile: tile, Err: err})
				resultMu.Unlock()
				return nil
			}
			registry.Register(snapshot)
			registry.RegisterTerrainTexturesFor(snapshot.ObjectPath)
			resultMu.Lock()
			loaded = append(loaded, tile)
			resultMu.Unlock()
			n := int(completed.Add(1))
			if onTileDone != nil {
				onTileDone(n, len(tiles), tile)
			}
			log.Printf("Loaded ADT tile %d/%d: %s (%d,%d)", n, len(tiles), mc.WowExportFolder, tile.X, tile.Y)
			return nil
		})
	}
	_ = common.WorkerPool(MapExportWorkerCount(), tasks)
	sortTileCoords(loaded)
	if len(failures) > 0 {
		sort.Slice(failures, func(i, j int) bool {
			if failures[i].Tile.Y != failures[j].Tile.Y {
				return failures[i].Tile.Y < failures[j].Tile.Y
			}
			return failures[i].Tile.X < failures[j].Tile.X
		})
		errs := make([]error, len(failures))
		for i, failure := range failures {
			errs[i] = fmt.Errorf("load ADT %d,%d: %w", failure.Tile.X, failure.Tile.Y, failure.Err)
		}
		return loaded, failures, errors.Join(errs...)
	}
	return loaded, failures, nil
}

func sortTileCoords(tiles []TileCoord) {
	sort.Slice(tiles, func(i, j int) bool {
		if tiles[i].Y != tiles[j].Y {
			return tiles[i].Y < tiles[j].Y
		}
		return tiles[i].X < tiles[j].X
	})
}

func loadTileSnapshot(
	ctx context.Context,
	wowClient client.Client,
	exportAssetDir string,
	mc MapExportConfig,
	quality int,
	tile TileCoord,
	includeInteriors, includeWMOSets bool,
) (*exportadt.ConversionOutput, error) {
	cascMapDir := mc.CascMapDir
	if cascMapDir == "" {
		cascMapDir = mc.WowExportFolder
	}
	if wowClient != nil {
		includeM2, includeWMO, includeGO := true, true, true
		params := casc.ADTExportParams{
			MapID: mc.MapID, MapDir: cascMapDir, TileX: tile.X, TileY: tile.Y,
			Quality: &quality, ExportAssetDir: exportAssetDir,
			IncludeM2: &includeM2, IncludeWMO: &includeWMO, IncludeWMOSets: &includeWMOSets,
			IncludeGameObjects: &includeGO, IncludeHoles: boolPtr(true),
			IncludeLiquid: boolPtr(false), IncludeFoliage: boolPtr(false),
		}
		return wowClient.ExportADTForConversion(ctx, params)
	}

	source, err := server.GlobalRuntime.GetCasc()
	if err != nil {
		return nil, err
	}
	opts := exportadt.BuildADTExportOptions(map[string]any{
		"mapsIncludeM2": true, "mapsIncludeWMO": true, "mapsIncludeWMOSets": includeWMOSets,
		"mapsIncludeGameObjects": true, "mapsIncludeHoles": true,
		"mapsIncludeLiquid": false, "mapsIncludeFoliage": false,
	})
	gameObjects := map[uint32]db.DB2Row{}
	if opts.MapsIncludeGameObjects {
		startX, startY, endX, endY := exportadt.TileBounds(tile.X, tile.Y)
		collected, err := exportadt.CollectGameObjects(ctx, uint32(mc.MapID), func(obj db.DB2Row) bool {
			pos := exportadt.GameObjectPosition(obj)
			if len(pos) < 2 {
				return false
			}
			return pos[0] > startX && pos[0] < endX && pos[1] > startY && pos[1] < endY
		})
		if err == nil {
			gameObjects = collected
		}
	}
	exporter := exportadt.NewExporter(mc.MapID, cascMapDir, tile.X*64+tile.Y)
	return exporter.ExportForConversion(ctx, source, exportAssetDir, quality, opts, gameObjects, nil)
}

func boolPtr(v bool) *bool { return &v }

// TryLoadADTTilesForConversion loads tiles when a data client or in-process CASC is available.
func TryLoadADTTilesForConversion(
	ctx context.Context,
	wowClient client.Client,
	exportAssetDir string,
	mc MapExportConfig,
	quality int,
	tiles []TileCoord,
	includeInteriors bool,
	registry *common.TileRegistry,
	onTileDone func(completed, total int, tile TileCoord),
) (loaded bool, err error) {
	if wowClient == nil && server.GlobalRuntime.GetCascOptional() == nil {
		return false, nil
	}
	if err := LoadADTTilesForConversion(ctx, wowClient, exportAssetDir, mc, quality, tiles, includeInteriors, registry, onTileDone); err != nil {
		return false, err
	}
	return true, nil
}

// TilesFromBounds enumerates tile coords in a min/max rectangle.
func TilesFromBounds(minX, minY, maxX, maxY int) []TileCoord {
	var out []TileCoord
	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			out = append(out, TileCoord{X: x, Y: y})
		}
	}
	return out
}
