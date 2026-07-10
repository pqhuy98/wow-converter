package mapexporter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

// MapGenerateConversionOptions configures a server-side WC3 conversion run.
type MapGenerateConversionOptions struct {
	Config                   config.Config
	MapExportConfig          *MapExportConfig
	MapSaveName              string
	FreshExport              bool
	IncludeBuildingInteriors bool
	AutoClampPercent         bool
	UnitScale                float64
	WowClient                client.Client
	OnConvertStepsKnown      func(convertSteps int)
	OnProgress               func(convertCompleted int, taskName string, creatureProgress *CreatureProgress)
}

// CreatureProgress tracks creature export progress.
type CreatureProgress struct {
	Completed int
	Total     int
}

// MapGenerateConversionResult is the output of runMapGenerateConversion.
type MapGenerateConversionResult struct {
	OutputDir    string
	MapSaveName  string
	ConvertSteps int
}

// BuildMapExportConfig constructs MapExportConfig from API parameters.
func BuildMapExportConfig(params struct {
	MapID           int
	WowExportFolder string
	Min             math.Vector2
	Max             math.Vector2
	MapAngleDeg     float64
	ClampLower      float64
	ClampUpper      float64
	UnitScale       float64
	CreaturesEnable bool
	AllAreDoodads   bool
}) MapExportConfig {
	cfg := DefaultMapExportConfig()
	cfg.MapID = params.MapID
	cfg.WowExportFolder = params.WowExportFolder
	cfg.Min = params.Min
	cfg.Max = params.Max
	cfg.MapAngleDeg = params.MapAngleDeg
	cfg.Terrain.ClampPercent.Lower = params.ClampLower
	cfg.Terrain.ClampPercent.Upper = params.ClampUpper
	cfg.UnitScale = params.UnitScale
	cfg.Creatures.Enable = params.CreaturesEnable
	cfg.Creatures.AllAreDoodads = params.AllAreDoodads
	return cfg
}

// RunMapGenerateConversion executes parse, export, and save for a generated map.
func RunMapGenerateConversion(ctx context.Context, opts MapGenerateConversionOptions) (MapGenerateConversionResult, error) {
	autoClamp := opts.AutoClampPercent

	if opts.WowClient != nil {
		if err := opts.WowClient.WaitUntilReady(ctx); err != nil {
			return MapGenerateConversionResult{}, fmt.Errorf("WoW data not loaded: %w", err)
		}
	}

	mapSaveName := NormalizeMapSaveName(opts.MapSaveName)
	outputDir := filepath.Join("maps", mapSaveName)
	if opts.FreshExport {
		_ = os.RemoveAll(outputDir)
		LogMapGeneratePhase("Removed existing map folder " + outputDir)
	}

	conversionCfg := opts.Config
	conversionCfg.IsBulkExport = true
	conversionCfg.OverrideModels = opts.FreshExport
	conversionCfg.MDX = true

	convertCompleted := 0
	report := func(taskName string, creature *CreatureProgress) {
		if opts.OnProgress != nil {
			opts.OnProgress(convertCompleted, taskName, creature)
		}
	}

	exporter := NewMapExporter(conversionCfg, opts.MapExportConfig, opts.WowClient)
	LogMapGeneratePhase("Parsing map objects")
	report("Parsing map data", nil)
	if err := exporter.ParseObjects(nil); err != nil {
		return MapGenerateConversionResult{}, err
	}
	pruneDepth := 2
	if opts.IncludeBuildingInteriors {
		pruneDepth = 3
	}
	PruneDepth(exporter, pruneDepth)
	if autoClamp {
		AutoChooseClampPercent(exporter, opts.UnitScale)
	}
	convertCompleted = 1
	report("Parsed map data", nil)

	uniqueCreatureCount := 0
	if opts.MapExportConfig != nil && opts.MapExportConfig.Creatures.Enable {
		uniqueCreatureCount = CountUniqueUnitExportsFromManager(exporter.WowObjectManager)
	}
	creatureExportSteps := ComputeCreatureExportSteps(uniqueCreatureCount)
	convertSteps := 1 + 1 + creatureExportSteps + 1
	if opts.OnConvertStepsKnown != nil {
		opts.OnConvertStepsKnown(convertSteps)
	}

	LogMapGeneratePhase("Saving terrain and doodads")
	report("Saving terrain and doodads", nil)
	if err := exporter.ExportTerrainsDoodads(outputDir); err != nil {
		return MapGenerateConversionResult{}, err
	}
	convertCompleted = 2
	report("Saved terrain and doodads", nil)

	if opts.MapExportConfig != nil && opts.MapExportConfig.Creatures.Enable && creatureExportSteps > 0 {
		LogMapGeneratePhase("Exporting creature models")
		report("Exporting creature models", &CreatureProgress{Completed: 0, Total: uniqueCreatureCount})
		if err := exporter.ExportCreatures(outputDir, func(completed, total int) {
			convertCompleted = 2 + completed
			report("Exporting creature models", &CreatureProgress{Completed: completed, Total: total})
		}); err != nil {
			return MapGenerateConversionResult{}, err
		}
		convertCompleted = 2 + creatureExportSteps
		report("Exported creature models", nil)
	}

	LogMapGeneratePhase("Saving war3map files")
	report("Saving war3map files", nil)
	if err := exporter.SaveWar3mapFiles(outputDir, mapSaveName); err != nil {
		return MapGenerateConversionResult{}, err
	}
	convertCompleted = convertSteps
	report("Complete", nil)

	absOut, _ := filepath.Abs(outputDir)
	LogMapGeneratePhase("Complete: " + mapSaveName + " → " + absOut)
	return MapGenerateConversionResult{
		OutputDir:    absOut,
		MapSaveName:  mapSaveName,
		ConvertSteps: convertSteps,
	}, nil
}
