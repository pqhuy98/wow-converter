package api

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/server/exportlog"
	"github.com/pqhuy98/wow-converter/internal/server/pathsafe"
	"github.com/pqhuy98/wow-converter/internal/server/util"
	"github.com/pqhuy98/wow-converter/internal/stringsort"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

type exportCharacterRequest struct {
	Character      character.Character `json:"character"`
	OutputFileName string              `json:"outputFileName"`
	Optimization   struct {
		SortSequences                 *bool  `json:"sortSequences"`
		AllMaterialsUnshaded          *bool  `json:"allMaterialsUnshaded"`
		RemoveUnusedVertices          *bool  `json:"removeUnusedVertices"`
		RemoveUnusedNodes             *bool  `json:"removeUnusedNodes"`
		RemoveUnusedMaterialsTextures *bool  `json:"removeUnusedMaterialsTextures"`
		MaxTextureSize                string `json:"maxTextureSize"`
	} `json:"optimization"`
	Format        string `json:"format"`
	FormatVersion string `json:"formatVersion"`
	IsBrowse      bool   `json:"isBrowse"`
	SkinID        string `json:"skinId"`
}

type exportCharacterResponse struct {
	ExportedModels   []exportAssetInfo `json:"exportedModels"`
	ExportedTextures []exportAssetInfo `json:"exportedTextures"`
	ModelStats       map[string]int    `json:"modelStats"`
	OutputDirectory  string            `json:"outputDirectory,omitempty"`
	VersionID        string            `json:"versionId"`
}

type exportAssetInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type exportCharacterJobStatus struct {
	util.JobStatusView[exportCharacterResponse]
	Logs []string `json:"logs,omitempty"`
}

func registerExportCharacter(r Router, d *Deps) {
	exportlog.Install()

	timeout := 60 * time.Second
	if d.Config.IsSharedHosting {
		timeout += 60 * time.Second
	}

	var queue *util.JobQueue[exportCharacterRequest, exportCharacterResponse]
	queue = util.NewJobQueue(util.QueueConfig[exportCharacterRequest, exportCharacterResponse]{
		Concurrency:    1,
		MaxPendingJobs: 100,
		JobTTL:         5 * time.Minute,
		JobTimeout:     timeout,
		OnCompleted: func(_ *util.Job[exportCharacterRequest, exportCharacterResponse]) {
			_ = saveRecentExports(d.Config.RecentExports, queue.RecentCompletedJobs)
		},
	}, func(job *util.Job[exportCharacterRequest, exportCharacterResponse]) (exportCharacterResponse, error) {
		exportlog.Begin()
		defer exportlog.End()
		return runCharacterExport(job.Request, job.ID, d)
	})
	loadRecentExports(queue, d.Config.RecentExports)

	r.Get("/export/character/recent", func(w http.ResponseWriter, _ *http.Request) {
		sendJSON(w, http.StatusOK, queue.RecentCompletedJobs)
	})

	r.Post("/export/character", func(w http.ResponseWriter, req *http.Request) {
		if !d.IsDataServerReady(req.Context()) {
			sendError(w, http.StatusInternalServerError, "wow-data-server is not ready")
			return
		}
		rawBody, err := readJSONMap(req)
		if err != nil {
			if errors.Is(err, errRequestBodyTooLarge) {
				sendError(w, http.StatusRequestEntityTooLarge, "Request body too large")
				return
			}
			sendError(w, http.StatusBadRequest, "Invalid request body")
			return
		}
		body, issues := parseExportCharacterRequest(rawBody)
		if len(issues) > 0 {
			sendJSON(w, http.StatusBadRequest, map[string]any{
				"error":  "Invalid request body",
				"issues": issues,
			})
			return
		}

		var versionID *string
		if d.Config.IsSharedHosting || body.IsBrowse {
			hash := md5.Sum(append(mustJSON(body), []byte(d.Config.ServerDeployTime)...))
			v := hex.EncodeToString(hash[:])
			versionID = &v
			body.OutputFileName = body.OutputFileName + "__" + v
		}

		if versionID != nil {
			if existing := queue.GetJob(*versionID); existing != nil {
				sendJSON(w, http.StatusOK, queue.GetJobStatus(*versionID))
				return
			}
		}

		id := newJobID()
		if versionID != nil {
			id = *versionID
		}
		job := &util.Job[exportCharacterRequest, exportCharacterResponse]{
			ID: id, Request: body, Status: util.JobPending,
			SubmittedAt: time.Now().UnixMilli(), AddToRecent: !body.IsBrowse,
		}
		queue.AddJob(job)
		log.Printf("%s %s %s", ansi.Blue("POST"), "/export/character", ansi.Grayf("Queued job %s", job.ID))
		sendJSON(w, http.StatusOK, queue.GetJobStatus(job.ID))
	})

	r.Get("/export/character/status/{jobId}", func(w http.ResponseWriter, req *http.Request) {
		status := queue.GetJobStatus(chi.URLParam(req, "jobId"))
		if status == nil {
			sendError(w, http.StatusNotFound, "Export request not found")
			return
		}
		resp := exportCharacterJobStatus{JobStatusView: *status}
		if status.Status == util.JobProcessing {
			resp.Logs = exportlog.Snapshot()
		}
		sendJSON(w, http.StatusOK, resp)
	})

	r.Post("/export/character/clean", func(w http.ResponseWriter, _ *http.Request) {
		if d.Config.OutputDir != workspace.ResolveRepoPath(outputDirRel) {
			sendError(w, http.StatusInternalServerError, `Cannot clean exported assets because output directory is not "exported-assets"`)
			return
		}
		if d.Config.IsSharedHosting {
			sendError(w, http.StatusBadRequest, "Cannot clean exported assets because server is in shared hosting mode")
			return
		}
		_ = os.Remove(d.Config.RecentExports)
		queue.RecentCompletedJobs = emptyRecentExports()
		log.Printf("Removed %s", d.Config.RecentExports)
		_ = os.RemoveAll(d.Config.OutputDir)
		_ = os.MkdirAll(d.Config.OutputDir, 0o755)
		log.Printf("Cleared %s", d.Config.OutputDir)
		_ = os.RemoveAll(d.Config.OutputDirBrowse)
		_ = os.MkdirAll(d.Config.OutputDirBrowse, 0o755)
		log.Printf("Cleared %s", d.Config.OutputDirBrowse)
		sendJSON(w, http.StatusOK, map[string]string{"message": "Exported assets cleaned"})
	})

	var demoJobIDs []string
	if d.Config.IsSharedHosting {
		for _, demoReq := range StartupDemoRequests() {
			job := &util.Job[exportCharacterRequest, exportCharacterResponse]{
				ID: newJobID(), Request: demoReq, Status: util.JobPending,
				SubmittedAt: time.Now().UnixMilli(), AddToRecent: false, NoTimeout: true,
			}
			demoJobIDs = append(demoJobIDs, job.ID)
			queue.AddJob(job)
		}
	}

	r.Get("/export/character/demos", func(w http.ResponseWriter, _ *http.Request) {
		out := make([]*exportCharacterJobStatus, 0, len(demoJobIDs))
		for _, id := range demoJobIDs {
			if status := queue.GetJobStatus(id); status != nil && status.Status == util.JobDone {
				jobStatus := exportCharacterJobStatus{JobStatusView: *status}
				out = append(out, &jobStatus)
			}
		}
		sendJSON(w, http.StatusOK, out)
	})

	r.Get("/export/character/check-local-file", func(w http.ResponseWriter, req *http.Request) {
		localPath := req.URL.Query().Get("localPath")
		if localPath == "" {
			sendJSON(w, http.StatusOK, map[string]any{"ok": false, "similarFiles": []string{}})
			return
		}
		normalized := common.NormalizeLocalModelRef(localPath)
		if _, err := pathsafe.ResolveUnderBase(d.ExportAssetDir(), normalized); err != nil {
			sendJSON(w, http.StatusOK, map[string]any{"ok": false, "similarFiles": []string{}})
			return
		}
		ok, similar, err := character.ResolveLocalModelRef(d.Client, localPath)
		if err != nil {
			sendJSON(w, http.StatusOK, map[string]any{"ok": false, "similarFiles": []string{}})
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{"ok": ok, "similarFiles": similar})
	})

	registerExportStatic(r, d)
	if !d.Config.IsSharedHosting {
		_ = os.RemoveAll(d.Config.OutputDirBrowse)
		_ = os.MkdirAll(d.Config.OutputDirBrowse, 0o755)
	}
}

func registerExportStatic(r Router, d *Deps) {
	maxAge := "max-age=5"
	if d.Config.IsSharedHosting {
		maxAge = "max-age=3600"
	}
	assetsFS := http.StripPrefix("/api/assets/", http.FileServer(http.Dir(d.Config.OutputDir)))
	browseFS := http.StripPrefix("/api/browse-assets/", http.FileServer(http.Dir(d.Config.OutputDirBrowse)))
	r.Handle("/assets/*", cacheControlHandler(maxAge, assetsFS))
	r.Handle("/browse-assets/*", cacheControlHandler(maxAge, browseFS))
}

func cacheControlHandler(cacheControl string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "public, "+cacheControl)
		next.ServeHTTP(w, req)
	})
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func loadRecentExports(queue *util.JobQueue[exportCharacterRequest, exportCharacterResponse], path string) {
	queue.RecentCompletedJobs = emptyRecentExports()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(data, &queue.RecentCompletedJobs)
	if queue.RecentCompletedJobs == nil {
		queue.RecentCompletedJobs = emptyRecentExports()
	}
}

func emptyRecentExports() []*util.Job[exportCharacterRequest, exportCharacterResponse] {
	return []*util.Job[exportCharacterRequest, exportCharacterResponse]{}
}

func saveRecentExports(path string, jobs []*util.Job[exportCharacterRequest, exportCharacterResponse]) error {
	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func optimizationBool(v *bool, defaultVal bool) bool {
	if v == nil {
		return defaultVal
	}
	return *v
}

func runCharacterExport(req exportCharacterRequest, jobID string, d *Deps) (exportCharacterResponse, error) {
	startedAt := time.Now()
	requestJSON, _ := json.MarshalIndent(req, "", "  ")
	log.Printf("Start exporting %s: %s", req.OutputFileName, ansi.Gray(string(requestJSON)))

	cfg := config.DefaultConfig()
	cfg.MDX = req.Format == "mdx"
	if req.Optimization.MaxTextureSize != "" {
		switch req.Optimization.MaxTextureSize {
		case "256":
			cfg.MaxTextureSize = 256
		case "512":
			cfg.MaxTextureSize = 512
		case "1024":
			cfg.MaxTextureSize = 1024
		}
	}

	outDir := d.Config.OutputDir
	if req.IsBrowse {
		outDir = d.Config.OutputDirBrowse
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return exportCharacterResponse{}, err
	}

	exporter := character.NewCharacterExporter(cfg, d.Client)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	if err := client.SyncConfig(ctx, d.Client); err != nil {
		log.Printf("syncConfig: %v", err)
	}

	_, err := exporter.ExportCharacter(ctx, req.Character, req.OutputFileName, character.ExportOptions{
		LocalModelSkinID: req.SkinID,
	})
	if err != nil {
		return exportCharacterResponse{}, err
	}
	exporter.OptimizeModelsTextures(character.ExportOptimization{
		SortSequences:                 optimizationBool(req.Optimization.SortSequences, true),
		RemoveUnusedVertices:          optimizationBool(req.Optimization.RemoveUnusedVertices, true),
		RemoveUnusedNodes:             optimizationBool(req.Optimization.RemoveUnusedNodes, true),
		RemoveUnusedMaterialsTextures: optimizationBool(req.Optimization.RemoveUnusedMaterialsTextures, true),
		FormatVersion:                 req.FormatVersion,
		AllMaterialsUnshaded:          req.Optimization.AllMaterialsUnshaded != nil && *req.Optimization.AllMaterialsUnshaded,
		ParticlesDensity:              req.Character.ParticlesDensity,
	})

	var texturePaths, modelPaths []string
	var texErr, modelErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		texturePaths, texErr = exporter.WriteAllTextures(outDir)
	}()
	go func() {
		defer wg.Done()
		modelPaths, modelErr = exporter.WriteAllModels(outDir, req.Format)
	}()
	wg.Wait()
	if texErr != nil {
		return exportCharacterResponse{}, texErr
	}
	if modelErr != nil {
		return exportCharacterResponse{}, modelErr
	}

	resp := newExportCharacterResponse(jobID)
	resp.ModelStats = exporter.AggregateModelStats(req.FormatVersion)
	for _, modelFile := range modelPaths {
		info, err := os.Stat(modelFile)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(outDir, modelFile)
		if err != nil {
			continue
		}
		resp.ExportedModels = append(resp.ExportedModels, exportAssetInfo{Path: filepath.ToSlash(rel), Size: info.Size()})
	}
	for _, tex := range texturePaths {
		info, err := os.Stat(tex)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(outDir, tex)
		if err != nil {
			continue
		}
		resp.ExportedTextures = append(resp.ExportedTextures, exportAssetInfo{Path: filepath.ToSlash(rel), Size: info.Size()})
	}
	sort.Slice(resp.ExportedTextures, func(i, j int) bool {
		return stringsort.Less(resp.ExportedTextures[i].Path, resp.ExportedTextures[j].Path)
	})
	if !d.Config.IsSharedHosting {
		resp.OutputDirectory = outDir
	}
	summary, _ := json.MarshalIndent(map[string]any{
		"exportedModels":  resp.ExportedModels,
		"outputDirectory": resp.OutputDirectory,
		"versionId":       resp.VersionID,
	}, "", "  ")
	log.Printf("Job finished %s %s", ansi.Yellowf("%.2fs", time.Since(startedAt).Seconds()), ansi.Gray(string(summary)))
	return resp, nil
}

func newExportCharacterResponse(jobID string) exportCharacterResponse {
	return exportCharacterResponse{
		ExportedModels:   []exportAssetInfo{},
		ExportedTextures: []exportAssetInfo{},
		VersionID:        jobID,
	}
}
