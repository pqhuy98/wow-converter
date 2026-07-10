package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
)

type exportAdtTile struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type mapExportTileSuccess struct {
	TileX  int                  `json:"tileX"`
	TileY  int                  `json:"tileY"`
	Result casc.ADTExportResult `json:"result"`
}

type mapExportTileFailure struct {
	TileX int    `json:"tileX"`
	TileY int    `json:"tileY"`
	Error string `json:"error"`
}

func dedupeTiles(tiles []exportAdtTile) []exportAdtTile {
	seen := map[string]exportAdtTile{}
	for _, t := range tiles {
		seen[tileKey(t.X, t.Y)] = t
	}
	out := make([]exportAdtTile, 0, len(seen))
	for _, t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}

func newJobID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type errString string

func (e errString) Error() string { return string(e) }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func includeBuildingInteriorsEnabled(v *bool) bool {
	if v == nil {
		return true
	}
	return *v
}

// exportADTTilesParallel exports tiles with bounded concurrency.
func exportADTTilesParallel(
	ctx context.Context,
	d *Deps,
	tiles []exportAdtTile,
	makeParams func(tile exportAdtTile, tileIndex int) casc.ADTExportParams,
	onTileDone func(completed int, tile exportAdtTile),
) ([]mapExportTileSuccess, []mapExportTileFailure) {
	concurrency := mapexporter.MapExportWorkerCount()
	succeeded := make([]mapExportTileSuccess, 0, len(tiles))
	failed := make([]mapExportTileFailure, 0)
	var mu sync.Mutex
	var completed atomic.Int32

	tasks := make([]func() error, 0, len(tiles))
	for i, tile := range tiles {
		i, tile := i, tile
		tasks = append(tasks, func() error {
			params := makeParams(tile, i)
			result, err := d.Client.ExportADT(ctx, params)
			n := int(completed.Add(1))
			if onTileDone != nil {
				onTileDone(n, tile)
			}
			mu.Lock()
			if err != nil {
				failed = append(failed, mapExportTileFailure{TileX: tile.X, TileY: tile.Y, Error: err.Error()})
			} else {
				succeeded = append(succeeded, mapExportTileSuccess{TileX: tile.X, TileY: tile.Y, Result: result})
			}
			mu.Unlock()
			return nil
		})
	}
	_ = common.WorkerPool(concurrency, tasks)
	return succeeded, failed
}
