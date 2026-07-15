package api

import (
	"context"
	"log"
	"net/http"
	"runtime"
	"runtime/debug"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/converter/mapexporter"
	"github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/server/util"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
)

type generateWc3Body struct {
	Tiles        []exportAdtTile `json:"tiles"`
	Quality      int             `json:"quality"`
	MapSaveName  string          `json:"mapSaveName"`
	ClampLower       float64 `json:"clampLower"`
	ClampUpper       float64 `json:"clampUpper"`
	AutoClampPercent bool    `json:"autoClampPercent"`
	MapAngleDeg      float64 `json:"mapAngleDeg"`
	UnitScale    float64         `json:"unitScale"`
	IncludeBuildingInteriors *bool           `json:"includeBuildingInteriors"`
	FreshExport              bool            `json:"freshExport"`
	Creatures    struct {
		Enable        bool `json:"enable"`
		AllAreDoodads bool `json:"allAreDoodads"`
	} `json:"creatures"`
}

type mapGenerateJobRequest struct {
	MapDir       string
	MapID        int
	Body         generateWc3Body
	OrderedTiles []exportAdtTile
	TileBounds   tileBounds
}

type tileBounds struct {
	Min [2]int `json:"min"`
	Max [2]int `json:"max"`
}

type mapGenerateJobResult struct {
	ID          string                 `json:"id"`
	Map         string                 `json:"map"`
	MapID       int                    `json:"mapID"`
	MapSaveName string                 `json:"mapSaveName"`
	OutputDir   string                 `json:"outputDir"`
	Quality     int                    `json:"quality"`
	Total       int                    `json:"total"`
	StepsPerTile int                   `json:"stepsPerTile"`
	TotalSteps  int                    `json:"totalSteps"`
	Succeeded   []mapExportTileSuccess `json:"succeeded"`
	Failed      []mapExportTileFailure `json:"failed"`
}

type mapGenerateProgressView struct {
	CompletedSteps    int        `json:"completedSteps"`
	TotalSteps        int        `json:"totalSteps"`
	Phase             string     `json:"phase"`
	TileIndex         *int       `json:"tileIndex,omitempty"`
	TileCount         *int       `json:"tileCount,omitempty"`
	StepsPerTile      *int       `json:"stepsPerTile,omitempty"`
	CurrentTile       *tileCoord `json:"currentTile,omitempty"`
	TaskName          string     `json:"taskName,omitempty"`
	CreatureCompleted *int       `json:"creatureCompleted,omitempty"`
	CreatureTotal     *int       `json:"creatureTotal,omitempty"`
	Percent           int        `json:"percent"`
}

type mapGenerateJobStatus struct {
	ID           string                   `json:"id"`
	Status       util.JobStatus           `json:"status"`
	Position     *int                     `json:"position,omitempty"`
	MapSaveName  string                   `json:"mapSaveName,omitempty"`
	MapDir       string                   `json:"mapDir,omitempty"`
	QueuePending *int                     `json:"queuePending,omitempty"`
	Result       *mapGenerateJobResult    `json:"result,omitempty"`
	Error        string                   `json:"error,omitempty"`
	SubmittedAt  int64                    `json:"submittedAt"`
	StartedAt    *int64                   `json:"startedAt,omitempty"`
	FinishedAt   *int64                   `json:"finishedAt,omitempty"`
	Progress     *mapGenerateProgressView `json:"progress,omitempty"`
}

const initialConvertSteps = 3

func registerMapGenerateRoutes(r Router, d *Deps) {
	queue := util.NewJobQueue(mapGenerateQueueConfig(), func(job *util.Job[mapGenerateJobRequest, mapGenerateJobResult]) (mapGenerateJobResult, error) {
		return runMapGenerateJob(context.Background(), d, job)
	})

	r.Post("/maps/{map}/generate-wc3", func(w http.ResponseWriter, req *http.Request) {
		if err := assertDesktopOnly(d.Config.IsSharedHosting); err != nil {
			sendError(w, http.StatusForbidden, err.Error())
			return
		}
		mapID, mapDir, ok := resolveMap(chi.URLParam(req, "map"))
		if !ok {
			sendError(w, http.StatusNotFound, "Unknown map")
			return
		}
		var body generateWc3Body
		if err := readJSONBody(req, &body); err != nil || len(body.Tiles) == 0 || body.MapSaveName == "" {
			sendError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		body.MapSaveName = mapexporter.NormalizeMapSaveName(body.MapSaveName)
		if body.MapSaveName == "" {
			sendError(w, http.StatusBadRequest, "Invalid map save name")
			return
		}
		if body.ClampUpper < body.ClampLower {
			sendError(w, http.StatusBadRequest, "clampUpper must be >= clampLower")
			return
		}

		ordered := dedupeTiles(body.Tiles)
		job := &util.Job[mapGenerateJobRequest, mapGenerateJobResult]{
			ID: newJobID(),
			Request: mapGenerateJobRequest{
				MapDir: mapDir, MapID: mapID, Body: body,
				OrderedTiles: ordered, TileBounds: tileBoundsFromTiles(ordered),
			},
			Status: util.JobPending, SubmittedAt: time.Now().UnixMilli(), NoTimeout: true,
		}
		queue.AddJob(job)
		log.Printf("generate-wc3 queued %s %s %d tiles → %s", mapDir, job.ID, len(ordered), body.MapSaveName)
		status := buildMapGenerateStatus(d, queue, job.ID)
		sendJSON(w, http.StatusOK, status)
	})

	r.Get("/maps/generate-wc3/status/{jobId}", func(w http.ResponseWriter, req *http.Request) {
		if err := assertDesktopOnly(d.Config.IsSharedHosting); err != nil {
			sendError(w, http.StatusForbidden, err.Error())
			return
		}
		status := buildMapGenerateStatus(d, queue, chi.URLParam(req, "jobId"))
		if status == nil {
			sendError(w, http.StatusNotFound, "Generate job not found")
			return
		}
		sendJSON(w, http.StatusOK, status)
	})

	r.Get("/maps/generate-wc3/active", func(w http.ResponseWriter, _ *http.Request) {
		if err := assertDesktopOnly(d.Config.IsSharedHosting); err != nil {
			sendError(w, http.StatusForbidden, err.Error())
			return
		}
		ids := queue.ListActiveJobIDs()
		out := make([]*mapGenerateJobStatus, 0, len(ids))
		for _, id := range ids {
			if status := buildMapGenerateStatus(d, queue, id); status != nil {
				out = append(out, status)
			}
		}
		sendJSON(w, http.StatusOK, out)
	})
}

func mapGenerateQueueConfig() util.QueueConfig[mapGenerateJobRequest, mapGenerateJobResult] {
	return util.QueueConfig[mapGenerateJobRequest, mapGenerateJobResult]{
		Concurrency: 1, MaxPendingJobs: 10, JobTTL: 10 * time.Minute, JobTimeout: 2 * time.Hour,
	}
}

func runMapGenerateJob(ctx context.Context, d *Deps, job *util.Job[mapGenerateJobRequest, mapGenerateJobResult]) (mapGenerateJobResult, error) {
	req := job.Request
	progressKey := job.ID
	includeInteriors := includeBuildingInteriorsEnabled(req.Body.IncludeBuildingInteriors)
	stepsPerTile := 1
	tileCount := len(req.OrderedTiles)
	adtTotalSteps := tileCount * stepsPerTile

	initMapGenerateProgress(progressKey, adtTotalSteps, initialConvertSteps, tileCount, stepsPerTile)

	mapExportCfg := mapexporter.BuildMapExportConfig(struct {
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
	}{
		MapID:           req.MapID,
		WowExportFolder: req.MapDir,
		Min:             math.Vector2{float64(req.TileBounds.Min[0]), float64(req.TileBounds.Min[1])},
		Max:             math.Vector2{float64(req.TileBounds.Max[0]), float64(req.TileBounds.Max[1])},
		MapAngleDeg:     req.Body.MapAngleDeg,
		ClampLower:      req.Body.ClampLower,
		ClampUpper:      req.Body.ClampUpper,
		UnitScale:       req.Body.UnitScale,
		CreaturesEnable: req.Body.Creatures.Enable,
		AllAreDoodads:   req.Body.Creatures.AllAreDoodads,
	})

	registry := common.NewTileRegistry()
	tileCoords := make([]mapexporter.TileCoord, len(req.OrderedTiles))
	for i, t := range req.OrderedTiles {
		tileCoords[i] = mapexporter.TileCoord{X: t.X, Y: t.Y}
	}
	if err := mapexporter.LoadADTTilesForConversion(ctx, d.Client, d.ExportAssetDir(), mapExportCfg, req.Body.Quality, tileCoords, includeInteriors, registry, func(completed, total int, tile mapexporter.TileCoord) {
		syncAdtProgress(progressKey, minInt(completed*stepsPerTile, adtTotalSteps), completed-1, tileCount, stepsPerTile,
			&tileCoord{X: tile.X, Y: tile.Y}, "Loading tiles")
	}); err != nil {
		clearMapGenerateProgress(progressKey)
		return mapGenerateJobResult{}, err
	}
	exportadt.ReleaseAdtExportBatchMemory()
	runtime.GC()
	debug.FreeOSMemory()

	succeeded := make([]mapExportTileSuccess, 0, len(req.OrderedTiles))
	for _, t := range req.OrderedTiles {
		succeeded = append(succeeded, mapExportTileSuccess{
			TileX: t.X, TileY: t.Y,
			Result: casc.ADTExportResult{ExportType: "ADT_DIRECT", MapID: req.MapID, MapDir: req.MapDir, TileX: t.X, TileY: t.Y},
		})
	}
	failed := make([]mapExportTileFailure, 0)

	setMapGeneratePhase(progressKey, phaseConvert, "Converting to WC3 map")
	syncAdtProgress(progressKey, adtTotalSteps, tileCount-1, tileCount, stepsPerTile, nil, "Tiles loaded")

	cfg := config.DefaultConfig()
	cfg.ExportAssetDir = d.ExportAssetDir()
	convertSteps := initialConvertSteps
	conversion, err := mapexporter.RunMapGenerateConversion(ctx, mapexporter.MapGenerateConversionOptions{
		Config:                   cfg,
		MapExportConfig:          &mapExportCfg,
		MapSaveName:              req.Body.MapSaveName,
		FreshExport:              req.Body.FreshExport,
		IncludeBuildingInteriors: includeInteriors,
		AutoClampPercent:         req.Body.AutoClampPercent,
		UnitScale:        req.Body.UnitScale,
		WowClient:        d.Client,
		TileRegistry:     registry,
		TileQuality:      req.Body.Quality,
		OnConvertStepsKnown: func(steps int) {
			convertSteps = steps
			updateMapGenerateTotalSteps(progressKey, convertSteps, adtTotalSteps)
		},
		OnProgress: func(convertCompleted int, taskName string, creature *mapexporter.CreatureProgress) {
			var cc, ct *int
			if creature != nil {
				cc, ct = &creature.Completed, &creature.Total
			}
			advanceMapGenerateProgress(progressKey, adtTotalSteps, convertCompleted, taskName, cc, ct)
		},
	})
	if err != nil {
		clearMapGenerateProgress(progressKey)
		return mapGenerateJobResult{}, err
	}

	totalSteps := adtTotalSteps + conversion.ConvertSteps
	advanceMapGenerateProgress(progressKey, adtTotalSteps, conversion.ConvertSteps, "Complete", nil, nil)

	return mapGenerateJobResult{
		ID:           "WC3_MAP_GENERATE_SUMMARY",
		Map:          req.MapDir,
		MapID:        req.MapID,
		MapSaveName:  conversion.MapSaveName,
		OutputDir:    conversion.OutputDir,
		Quality:      req.Body.Quality,
		Total:        tileCount,
		StepsPerTile: stepsPerTile,
		TotalSteps:   totalSteps,
		Succeeded:    succeeded,
		Failed:       failed,
	}, nil
}

func boolPtr(v bool) *bool { return &v }

func tileBoundsFromTiles(tiles []exportAdtTile) tileBounds {
	xs := make([]int, len(tiles))
	ys := make([]int, len(tiles))
	for i, t := range tiles {
		xs[i], ys[i] = t.X, t.Y
	}
	sort.Ints(xs)
	sort.Ints(ys)
	return tileBounds{Min: [2]int{xs[0], ys[0]}, Max: [2]int{xs[len(xs)-1], ys[len(ys)-1]}}
}

func buildMapGenerateStatus(d *Deps, queue *util.JobQueue[mapGenerateJobRequest, mapGenerateJobResult], jobID string) *mapGenerateJobStatus {
	base := queue.GetJobStatus(jobID)
	job := queue.GetJob(jobID)
	if base == nil || job == nil {
		return nil
	}
	pending, _ := queue.GetQueueSnapshot()
	status := &mapGenerateJobStatus{
		ID: base.ID, Status: base.Status, Position: base.Position,
		MapSaveName: job.Request.Body.MapSaveName, MapDir: job.Request.MapDir,
		QueuePending: &pending, Error: base.Error, SubmittedAt: base.SubmittedAt,
		StartedAt: base.StartedAt, FinishedAt: base.FinishedAt, Result: base.Result,
	}
	if base.Status == util.JobProcessing {
		if snap := getMapGenerateProgress(jobID); snap != nil {
			status.Progress = progressViewFromSnapshot(snap)
		} else if adtSnap, _ := d.Client.GetExportProgress(context.Background(), jobID); adtSnap != nil {
			percent := 0
			if adtSnap.TotalSteps > 0 {
				percent = minInt(100, (adtSnap.CompletedSteps*100)/adtSnap.TotalSteps)
			}
			var current *tileCoord
			if adtSnap.CurrentTile != nil {
				current = &tileCoord{X: adtSnap.CurrentTile.X, Y: adtSnap.CurrentTile.Y}
			}
			status.Progress = &mapGenerateProgressView{
				CompletedSteps: adtSnap.CompletedSteps, TotalSteps: adtSnap.TotalSteps, Phase: "adt",
				TileIndex: &adtSnap.TileIndex, TileCount: &adtSnap.TileCount, StepsPerTile: &adtSnap.StepsPerTile,
				CurrentTile: current, TaskName: adtSnap.TaskName, Percent: percent,
			}
		}
	} else if base.Status == util.JobDone && base.Result != nil {
		status.Progress = &mapGenerateProgressView{
			CompletedSteps: base.Result.TotalSteps, TotalSteps: base.Result.TotalSteps,
			Phase: "convert", TaskName: "Complete", Percent: 100,
		}
		clearMapGenerateProgress(jobID)
	} else if base.Status == util.JobFailed {
		clearMapGenerateProgress(jobID)
	}
	return status
}

func progressViewFromSnapshot(snap *mapGenerateSnapshot) *mapGenerateProgressView {
	return &mapGenerateProgressView{
		CompletedSteps: snap.CompletedSteps, TotalSteps: snap.TotalSteps, Phase: string(snap.Phase),
		TileIndex: snap.TileIndex, TileCount: snap.TileCount, StepsPerTile: snap.StepsPerTile,
		CurrentTile: snap.CurrentTile, TaskName: snap.TaskName,
		CreatureCompleted: snap.CreatureCompleted, CreatureTotal: snap.CreatureTotal,
		Percent: toProgressPercent(snap),
	}
}
