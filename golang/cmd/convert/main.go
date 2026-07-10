package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter"
	"github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// WowMap identifies a WoW continent/instance.
type WowMap struct {
	ID     int
	Folder string
}

var (
	WowMapAzeroth           = WowMap{ID: 0, Folder: "azeroth"}
	WowMapKalimdor          = WowMap{ID: 1, Folder: "kalimdor"}
	WowMapNorthrend         = WowMap{ID: 571, Folder: "northrend"}
	WowMapOutland           = WowMap{ID: 530, Folder: "outland"}
	WowMapDeathKnightStart  = WowMap{ID: 609, Folder: "deathknightstart"}
	WowMapIcecrownCitadel   = WowMap{ID: 631, Folder: "icecrowncitadel"}
	WowMapTheMaw            = WowMap{ID: 2456, Folder: "2456"}
	WowMapDurnhole          = WowMap{ID: 560, Folder: "HillsbradPast"}
	WowMapStratholmeRaid    = WowMap{ID: 533, Folder: "stratholmeraid"}
)

type mapEntry struct {
	mapInfo   WowMap
	min       math.Vector2
	max       math.Vector2
	output    string
	lowerPct  float64
	upperPct  float64
	angleDeg  float64
	wmoSet    []string
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	if root, err := workspace.ChdirRepoRoot(); err != nil {
		return fmt.Errorf("chdir repo root: %w", err)
	} else {
		log.Printf("Working directory: %s", root)
	}
	_ = workspace.LoadEnvFile(".env")

	entries := []mapEntry{
		{
			mapInfo:  WowMapNorthrend,
			min:      math.Vector2{21, 27},
			max:      math.Vector2{22, 28},
			output:   "valiancekeep.w3x",
			lowerPct: 0,
			upperPct: 1,
			angleDeg: 0,
		},
	}
	chosen := entries[0]
	autoChooseClamp := true
	creatureScaleUp := 2.0
	depth := 3
	mapOutputDir := fmt.Sprintf("maps/%s", chosen.output)

	cfg := config.DefaultConfig()
	cfg.IsBulkExport = true
	cfg.OverrideModels = false
	cfg.MDX = true

	mapCfg := mapexporter.DefaultMapExportConfig()
	mapCfg.MapID = chosen.mapInfo.ID
	mapCfg.WowExportFolder = chosen.mapInfo.Folder
	mapCfg.Min = chosen.min
	mapCfg.Max = chosen.max
	mapCfg.MapAngleDeg = chosen.angleDeg
	mapCfg.Terrain.ClampPercent.Lower = chosen.lowerPct
	mapCfg.Terrain.ClampPercent.Upper = chosen.upperPct
	mapCfg.Creatures.Enable = true
	mapCfg.Creatures.AllAreDoodads = true
	mapCfg.UnitScale = creatureScaleUp

	ctx := context.Background()
	handler, err := bootstrap.StartWowDataServer(ctx)
	if err != nil {
		return err
	}
	wowClient := client.NewInProcessClient(handler)

	cfg.ExportAssetDir = mapexporter.SyncExportAssetDir(ctx, wowClient, cfg.ExportAssetDir)
	log.Printf("Export asset dir: %s", cfg.ExportAssetDir)

	if err := mapexporter.EnsureADTTilesExported(ctx, cfg.ExportAssetDir, mapCfg, wowClient); err != nil {
		return err
	}

	start := time.Now()
	exporter := mapexporter.NewMapExporter(cfg, &mapCfg, wowClient)
	if err := exporter.ParseObjects(nil); err != nil {
		return err
	}
	mapexporter.PruneDepth(exporter, depth)
	if autoChooseClamp {
		mapexporter.AutoChooseClampPercent(exporter, creatureScaleUp)
	}
	if err := exporter.ExportTerrainsDoodads(mapOutputDir); err != nil {
		return err
	}
	if err := exporter.ExportCreatures(mapOutputDir, nil); err != nil {
		return err
	}
	if err := exporter.SaveWar3mapFiles(mapOutputDir, ""); err != nil {
		return err
	}
	log.Printf("Total map export time: %.2f s", time.Since(start).Seconds())
	return nil
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(0)
}

// Reference to match examples/convert.ts side-effect import.
var _ = mapexporter.GameZToPercent
