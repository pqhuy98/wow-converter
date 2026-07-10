package mapexporter

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

type adtTileCoord struct {
	x, y, index, total int
}

// EnsureADTTilesExported exports any missing ADT OBJ tiles for the configured region.
func EnsureADTTilesExported(ctx context.Context, exportDir string, mc MapExportConfig, wowClient client.Client) error {
	var tiles []adtTileCoord
	total := 0
	idx := 0
	for x := int(mc.Min[0]); x <= int(mc.Max[0]); x++ {
		for y := int(mc.Min[1]); y <= int(mc.Max[1]); y++ {
			total++
			objPath := filepath.Join(exportDir, "maps", mc.WowExportFolder, fmt.Sprintf("adt_%d_%d.obj", x, y))
			if _, err := os.Stat(objPath); err == nil {
				continue
			}
			tiles = append(tiles, adtTileCoord{x: x, y: y, index: idx, total: total})
			idx++
		}
	}
	if len(tiles) == 0 {
		return nil
	}

	var completed atomic.Int32
	var firstErr atomic.Value
	var errMu sync.Mutex
	tasks := make([]func() error, 0, len(tiles))
	for _, tile := range tiles {
		tile := tile
		tasks = append(tasks, func() error {
			n := int(completed.Add(1))
			log.Printf("Exporting ADT tile %d/%d: %s (%d,%d)", n, len(tiles), mc.WowExportFolder, tile.x, tile.y)
			tileIndex := tile.index
			tileCount := tile.total
			_, err := wowClient.ExportADT(ctx, casc.ADTExportParams{
				MapID:     mc.MapID,
				MapDir:    mc.WowExportFolder,
				TileX:     tile.x,
				TileY:     tile.y,
				TileIndex: &tileIndex,
				TileCount: &tileCount,
			})
			if err != nil {
				errMu.Lock()
				if firstErr.Load() == nil {
					firstErr.Store(fmt.Errorf("export ADT %d,%d: %w", tile.x, tile.y, err))
				}
				errMu.Unlock()
			}
			return nil
		})
	}
	_ = common.WorkerPool(MapExportWorkerCount(), tasks)
	if v := firstErr.Load(); v != nil {
		return v.(error)
	}
	return nil
}

// SyncExportAssetDir returns exportDirectory from wow-data-server config when available.
func SyncExportAssetDir(ctx context.Context, wowClient client.Client, fallback string) string {
	cfg, err := wowClient.GetConfig(ctx, "exportDirectory")
	if err != nil {
		return fallback
	}
	if v, ok := cfg["exportDirectory"].(string); ok && v != "" {
		return workspace.ResolveExportAssetDir(v)
	}
	return workspace.ResolveExportAssetDir(fallback)
}
