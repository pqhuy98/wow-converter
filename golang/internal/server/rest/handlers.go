package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/pqhuy98/wow-converter/internal/server/httplimit"
	"github.com/pqhuy98/wow-converter/internal/server/pathsafe"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
	"github.com/pqhuy98/wow-converter/internal/wow/service"
)

// Services bundles CASC-layer dependencies for REST handlers.
type Services struct {
	Loader     casc.Loader
	Listfile   casc.Listfile
	ModelCache casc.ModelCache
	MapList    casc.MapList
	Character  casc.CharacterMeta
	Export     casc.ADTExporter
	Progress   casc.ExportProgress
	Memory     casc.MemoryDiagnosticsCollector
}

// Handler implements wow-data-server REST endpoints.
type Handler struct {
	Runtime       *server.RuntimeState
	Services      Services
	responseCache *responseCache
	exportID      atomic.Int32
}

// NewHandler creates a handler with default noop CASC services.
func NewHandler(runtime *server.RuntimeState) *Handler {
	return &Handler{
		Runtime: runtime,
		Services: Services{
			Loader:     casc.NoopLoader{},
			Listfile:   casc.NoopListfile{},
			ModelCache: casc.NoopModelCache{},
			MapList:    casc.NoopMapList{},
			Character:  casc.NoopCharacterMeta{},
			Export:     casc.NoopADTExporter{},
			Progress:   casc.NoopExportProgress{},
			Memory:     casc.NoopMemoryDiagnostics{},
		},
		responseCache: newResponseCache(),
	}
}

func (h *Handler) nextExportID() int {
	id := h.exportID.Add(1)
	if id <= 0 {
		h.exportID.Store(1)
		return 1
	}
	return int(id)
}

// GET /rest/getCascInfo
func (h *Handler) GetCascInfo(w http.ResponseWriter, _ *http.Request) {
	cascSource := h.Runtime.GetCascOptional()
	if cascSource == nil || !cascSource.IsLoaded() {
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"id": "CASC_UNAVAILABLE"})
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"id":          "CASC_INFO",
		"type":        cascSource.TypeName(),
		"build":       cascSource.Build(),
		"buildConfig": cascSource.BuildConfig(),
		"buildName":   cascSource.GetBuildName(),
		"buildKey":    cascSource.GetBuildKey(),
	})
}

// GET /rest/getCascLoadProgress
func (h *Handler) GetCascLoadProgress(w http.ResponseWriter, _ *http.Request) {
	loading := h.Services.Loader.IsLoading() || log.IsLoadingProgressActive()
	sendJSON(w, http.StatusOK, map[string]any{
		"id":      "CASC_LOAD_PROGRESS",
		"loading": loading,
		"message": log.LatestLoadingMessage(),
	})
}

// GET /rest/getConfig
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if key := r.URL.Query().Get("key"); key != "" {
		value, ok := server.GetConfigValue(key)
		if !ok {
			sendJSON(w, http.StatusBadRequest, map[string]any{
				"id":      "ERR_INVALID_PARAMETERS",
				"message": "unknown config key",
			})
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{
			"id":    "CONFIG_SINGLE",
			"key":   key,
			"value": value,
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"id":     "CONFIG_FULL",
		"config": server.GetConfig(),
	})
}

// GET /rest/searchFiles
func (h *Handler) SearchFiles(w http.ResponseWriter, r *http.Request) {
	if !h.Services.Listfile.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_LISTFILE_NOT_LOADED"})
		return
	}

	search := r.URL.Query().Get("search")
	useRegex := r.URL.Query().Get("useRegularExpression") == "1"
	if useRegex {
		if safe, ok := SafeRegexPattern(search); !ok {
			search = ""
			useRegex = false
		} else {
			search = safe
		}
	}
	entries := h.Services.Listfile.GetFilteredEntries(search, useRegex)
	sendJSON(w, http.StatusOK, map[string]any{
		"id":      "LISTFILE_SEARCH_RESULT",
		"entries": entries,
	})
}

// GET /rest/collectBrowseFileIndex
func (h *Handler) CollectBrowseFileIndex(w http.ResponseWriter, r *http.Request) {
	if !h.Services.Listfile.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_LISTFILE_NOT_LOADED"})
		return
	}
	models, textures := h.Services.Listfile.CollectBrowseFileIndex()
	sendJSON(w, http.StatusOK, map[string]any{
		"id":       "BROWSE_FILE_INDEX",
		"models":   models,
		"textures": textures,
	})
}

// GET /rest/collectMapTileFileIndex
func (h *Handler) CollectMapTileFileIndex(w http.ResponseWriter, r *http.Request) {
	if !h.Services.Listfile.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_LISTFILE_NOT_LOADED"})
		return
	}
	entries := h.Services.Listfile.CollectMapTileFileIndex()
	sendJSON(w, http.StatusOK, map[string]any{
		"id":      "MAP_TILE_FILE_INDEX",
		"entries": entries,
	})
}

// GET /rest/getFileById
func (h *Handler) GetFileByID(w http.ResponseWriter, r *http.Request) {
	if !h.Services.Listfile.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_LISTFILE_NOT_LOADED"})
		return
	}

	fileDataID, err := parseRequiredIntQuery(r, "fileDataID")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"fileDataID": "number"},
		})
		return
	}

	fileName := h.Services.Listfile.GetByID(fileDataID)
	sendJSON(w, http.StatusOK, map[string]any{
		"id":         "LISTFILE_RESULT",
		"fileDataID": fileDataID,
		"fileName":   fileName,
	})
}

// GET /rest/getFileByName
func (h *Handler) GetFileByName(w http.ResponseWriter, r *http.Request) {
	if !h.Services.Listfile.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_LISTFILE_NOT_LOADED"})
		return
	}

	fileName := r.URL.Query().Get("fileName")
	if fileName == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"fileName": "string"},
		})
		return
	}

	fileDataID := h.Services.Listfile.GetByFilename(fileName)
	sendJSON(w, http.StatusOK, map[string]any{
		"id":         "LISTFILE_RESULT",
		"fileDataID": fileDataID,
		"fileName":   fileName,
	})
}

// GET /rest/getModelSkins
func (h *Handler) GetModelSkins(w http.ResponseWriter, r *http.Request) {
	fileDataID, err := parseRequiredIntQuery(r, "fileDataID")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"fileDataID": "number"},
		})
		return
	}

	ctx := r.Context()
	if err := h.Services.ModelCache.EnsureInitialized(ctx); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{
			"id":      "ERR_INTERNAL",
			"message": err.Error(),
		})
		return
	}

	skins, err := h.Services.ModelCache.GetAllSkinsForModel(fileDataID)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{
			"id":      "ERR_INTERNAL",
			"message": err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"id":         "MODEL_SKINS",
		"fileDataID": fileDataID,
		"skins":      skins,
	})
}

// GET /rest/initModelCaches
func (h *Handler) InitModelCaches(w http.ResponseWriter, r *http.Request) {
	if err := h.Services.ModelCache.EnsureInitialized(r.Context()); err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{
			"id":      "ERR_INTERNAL",
			"message": err.Error(),
		})
		return
	}
	sendJSON(w, http.StatusOK, map[string]any{"id": "MODEL_CACHES_READY"})
}

// GET /rest/resolveNpcDisplay
func (h *Handler) ResolveNpcDisplay(w http.ResponseWriter, r *http.Request) {
	cascSource := h.Runtime.GetCascOptional()
	if cascSource == nil || !cascSource.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_NO_CASC"})
		return
	}

	displayID, err := parseRequiredIntQuery(r, "displayId")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"displayId": "number"},
		})
		return
	}

	meta, err := service.ResolveNpcDisplayMeta(r.Context(), displayID)
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{
			"id":      "ERR_INTERNAL",
			"message": err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"id":       "NPC_DISPLAY_META",
		"found":    meta.Found,
		"model":    meta.Model,
		"textures": meta.Textures,
		"geosets":  meta.Geosets,
	})
}

// GET /rest/cascFile
func (h *Handler) CascFile(w http.ResponseWriter, r *http.Request) {
	cascSource := h.Runtime.GetCascOptional()
	if cascSource == nil || !cascSource.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_NO_CASC"})
		return
	}

	fileDataID, err := parsePositiveIntQuery(r, "fileDataID")
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"fileDataID": "number"},
		})
		return
	}

	data, err := cascSource.GetFile(r.Context(), fileDataID)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return
		}
		if strings.Contains(err.Error(), "does not exist in root") {
			sendJSON(w, http.StatusNotFound, map[string]any{"id": "ERR_NOT_FOUND", "fileDataID": fileDataID})
			return
		}
		if strings.Contains(err.Error(), "No root entry found for locale") {
			sendJSON(w, http.StatusNotFound, map[string]any{
				"id":         "ERR_NOT_FOUND",
				"fileDataID": fileDataID,
				"message":    err.Error(),
			})
			return
		}
		sendJSON(w, http.StatusInternalServerError, map[string]any{"id": "ERR_INTERNAL", "message": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// GET /rest/download
func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	exportDir := server.GetConfig().ExportDirectory
	if exportDir == "" {
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"id": "ERR_EXPORT_DIR_UNAVAILABLE"})
		return
	}

	requested := r.URL.Query().Get("path")
	if requested == "" || strings.ContainsRune(requested, 0) {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"path": "string (relative)"},
		})
		return
	}

	base := filepath.Clean(exportDir)
	abs := filepath.Clean(filepath.Join(base, requested))
	if abs != base && !strings.HasPrefix(abs, base+string(filepath.Separator)) {
		sendJSON(w, http.StatusForbidden, map[string]any{"id": "ERR_FORBIDDEN"})
		return
	}

	ext := strings.ToLower(filepath.Ext(abs))
	allowedExts := []string{".png", ".json", ".obj", ".mtl", ".csv"}
	if !containsString(allowedExts, ext) {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":          "ERR_INVALID_FILE_TYPE",
			"ext":         ext,
			"allowedExts": allowedExts,
		})
		return
	}

	f, err := pathsafe.OpenRegularFileUnderBase(exportDir, requested)
	if err != nil {
		if errors.Is(err, pathsafe.ErrInvalidPath) {
			sendJSON(w, http.StatusForbidden, map[string]any{"id": "ERR_FORBIDDEN"})
			return
		}
		if os.IsNotExist(err) {
			sendJSON(w, http.StatusNotFound, map[string]any{"id": "ERR_NOT_FOUND"})
			return
		}
		sendJSON(w, http.StatusInternalServerError, map[string]any{"id": "ERR_INTERNAL", "message": err.Error()})
		return
	}
	defer f.Close()

	w.Header().Set("Content-Type", contentTypeForExt(ext))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, f)
}

// GET /rest/debugMemory
func (h *Handler) DebugMemory(w http.ResponseWriter, _ *http.Request) {
	diag := h.Services.Memory.Collect()
	sendJSON(w, http.StatusOK, map[string]any{
		"id":                   "DEBUG_MEMORY",
		"summary":              diag.Summary,
		"responseCacheEntries": h.responseCache.size(),
		"process":              diag.Process,
		"casc":                 diag.Casc,
		"listfile":             diag.Listfile,
		"indexes":              diag.Indexes,
		"dbCaches":             diag.DBCaches,
		"exportCaches":         diag.ExportCaches,
		"converter":            diag.Converter,
	})
}

// GET /rest/getMapList
func (h *Handler) GetMapList(w http.ResponseWriter, r *http.Request) {
	cascSource := h.Runtime.GetCascOptional()
	if cascSource == nil || !cascSource.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_NO_CASC"})
		return
	}

	maps, err := h.Services.MapList.GetMaps(r.Context())
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{
			"id":      "ERR_INTERNAL",
			"message": err.Error(),
		})
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{"id": "MAP_LIST", "maps": maps})
}

// GET /rest/exportProgress
func (h *Handler) ExportProgress(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":      "ERR_INVALID_PARAMETERS",
			"message": "key is required",
		})
		return
	}

	snapshot, ok := h.Services.Progress.GetSnapshot(key)
	if !ok {
		sendJSON(w, http.StatusNotFound, map[string]any{"id": "ERR_NOT_FOUND"})
		return
	}

	sendJSON(w, http.StatusOK, mergeID("EXPORT_PROGRESS", snapshot))
}

// POST /rest/loadCascLocal
func (h *Handler) LoadCascLocal(w http.ResponseWriter, r *http.Request) {
	if h.Services.Loader.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_CASC_ACTIVE"})
		return
	}
	if h.Services.Loader.IsLoading() {
		_ = h.Services.Loader.AwaitLoad(r.Context())
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_CASC_ACTIVE"})
		return
	}

	body, err := readJSONBody(r)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_JSON"})
		return
	}

	installDirectory, _ := body["installDirectory"].(string)
	if installDirectory == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"installDirectory": "string"},
		})
		return
	}

	builds, err := h.Services.Loader.LoadLocal(r.Context(), installDirectory)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":      "ERR_INVALID_INSTALL",
			"message": err.Error(),
		})
		return
	}

	h.Services.Loader.SetPendingFromLocal(installDirectory, builds)
	sendJSON(w, http.StatusOK, map[string]any{"id": "CASC_INSTALL_BUILDS", "builds": builds})
}

// POST /rest/loadCascRemote
func (h *Handler) LoadCascRemote(w http.ResponseWriter, r *http.Request) {
	if h.Services.Loader.IsLoaded() {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_CASC_ACTIVE"})
		return
	}
	if h.Services.Loader.IsLoading() {
		_ = h.Services.Loader.AwaitLoad(r.Context())
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_CASC_ACTIVE"})
		return
	}

	body, err := readJSONBody(r)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_JSON"})
		return
	}

	regionTag, _ := body["regionTag"].(string)
	if regionTag == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"regionTag": "string"},
		})
		return
	}

	builds, err := h.Services.Loader.LoadRemote(r.Context(), regionTag)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_INSTALL"})
		return
	}

	h.Services.Loader.SetPendingFromRemote(regionTag, builds)
	sendJSON(w, http.StatusOK, map[string]any{"id": "CASC_INSTALL_BUILDS", "builds": builds})
}

// POST /rest/loadCascBuild
func (h *Handler) LoadCascBuild(w http.ResponseWriter, r *http.Request) {
	if h.Services.Loader.IsLoaded() {
		h.GetCascInfo(w, r)
		return
	}
	if h.Services.Loader.IsLoading() {
		if err := h.Services.Loader.AwaitLoad(r.Context()); err != nil {
			sendJSON(w, http.StatusInternalServerError, map[string]any{"id": "ERR_CASC_FAILED"})
			return
		}
		h.Services.Loader.ClearPending()
		h.GetCascInfo(w, r)
		return
	}

	body, err := readJSONBody(r)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_JSON"})
		return
	}

	buildIndex, ok := body["buildIndex"].(float64)
	if !ok {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"buildIndex": "number"},
		})
		return
	}

	pending := h.Services.Loader.PendingBuilds()
	if len(pending) == 0 {
		sendJSON(w, http.StatusConflict, map[string]any{"id": "ERR_NO_CASC_SETUP"})
		return
	}
	if int(buildIndex) < 0 || int(buildIndex) >= len(pending) {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_CASC_BUILD"})
		return
	}

	source, err := h.Services.Loader.LoadBuild(r.Context(), int(buildIndex))
	if err != nil {
		sendJSON(w, http.StatusInternalServerError, map[string]any{"id": "ERR_CASC_FAILED"})
		return
	}

	h.Services.Loader.ClearPending()
	h.Runtime.SetCasc(source)
	h.GetCascInfo(w, r)
}

// POST /rest/unloadCasc
func (h *Handler) UnloadCasc(w http.ResponseWriter, r *http.Request) {
	if err := h.Services.Loader.Unload(r.Context()); err != nil {
		sendJSON(w, http.StatusConflict, map[string]any{
			"id":      "ERR_CASC_LOADING",
			"message": err.Error(),
		})
		return
	}

	h.Services.Loader.ClearPending()
	h.Runtime.SetCasc(nil)
	h.responseCache.clear()
	sendJSON(w, http.StatusOK, map[string]any{"id": "CASC_UNLOADED"})
}

// POST /rest/softRestart
func (h *Handler) SoftRestart(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONBody(r)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_JSON"})
		return
	}

	reloadEnv, _ := body["reloadEnv"].(bool)
	result, err := h.Services.Loader.SoftRestart(r.Context(), reloadEnv)
	if err != nil {
		sendJSON(w, http.StatusConflict, map[string]any{
			"id":      "ERR_CASC_LOADING",
			"message": err.Error(),
		})
		return
	}

	h.Services.Loader.ClearPending()
	h.Runtime.SetCasc(nil)
	h.responseCache.clear()

	if reloadEnv {
		response := map[string]any{
			"id":         "SOFT_RESTART_DONE",
			"cascLoaded": result.CascLoaded,
		}
		if result.BuildName != "" {
			response["buildName"] = result.BuildName
		}
		if result.Error != "" {
			response["error"] = result.Error
		}
		sendJSON(w, http.StatusOK, response)
		return
	}

	sendJSON(w, http.StatusOK, map[string]any{
		"id":         "SOFT_RESTART_DONE",
		"cascLoaded": false,
	})
}

// POST /rest/setConfig
func (h *Handler) SetConfig(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONBody(r)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_JSON"})
		return
	}

	key, _ := body["key"].(string)
	if key == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":       "ERR_INVALID_PARAMETERS",
			"required": map[string]string{"key": "string", "value": "any"},
		})
		return
	}

	if !server.SetConfigValue(key, body["value"]) {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":      "ERR_FORBIDDEN_CONFIG_KEY",
			"message": "config key is not writable over HTTP",
		})
		return
	}

	value, _ := server.GetConfigValue(key)
	sendJSON(w, http.StatusOK, map[string]any{
		"id":    "CONFIG_SET_DONE",
		"key":   key,
		"value": value,
	})
}

// POST /rest/charMeta
func (h *Handler) CharMeta(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONBody(r)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_JSON"})
		return
	}

	cascSource := h.Runtime.GetCascOptional()
	buildKey := ""
	if cascSource != nil {
		buildKey = cascSource.GetBuildKey()
	}
	cacheKey := stableCacheKey("/rest/charMeta|"+buildKey, body)
	if cached, ok := h.responseCache.get(cacheKey); ok {
		sendJSON(w, cached.status, cached.body)
		return
	}

	if cascSource == nil || !cascSource.IsLoaded() {
		h.responseCache.sendAndCache(w, cacheKey, http.StatusConflict, map[string]any{"id": "ERR_NO_CASC"})
		return
	}

	params, valid := parseCharacterMetaParams(body)
	if !valid {
		h.responseCache.sendAndCache(w, cacheKey, http.StatusBadRequest, map[string]any{
			"id": "ERR_INVALID_PARAMETERS",
			"required": map[string]string{
				"race":           "number",
				"gender":         "number",
				"customizations": "object",
			},
		})
		return
	}

	meta, err := h.Services.Character.GetCharacterMeta(r.Context(), params)
	if err != nil {
		h.responseCache.sendAndCache(w, cacheKey, http.StatusInternalServerError, map[string]any{
			"id":      "ERR_INTERNAL",
			"message": err.Error(),
		})
		return
	}

	response := map[string]any{"id": "CHAR_META"}
	for k, v := range structToMap(meta) {
		response[k] = v
	}
	h.responseCache.sendAndCache(w, cacheKey, http.StatusOK, response)
}

// POST /rest/exportADT
func (h *Handler) ExportADT(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONBody(r)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_JSON"})
		return
	}

	progressKey, _ := body["progressKey"].(string)
	cascSource := h.Runtime.GetCascOptional()
	buildKey := ""
	if cascSource != nil {
		buildKey = cascSource.GetBuildKey()
	}

	cacheBody := body
	if progressKey != "" {
		cacheBody = copyMapWithout(body, "progressKey")
	}
	cacheKey := stableCacheKey("/rest/exportADT|"+buildKey, cacheBody)

	if progressKey == "" {
		if cached, ok := h.responseCache.get(cacheKey); ok {
			sendJSON(w, cached.status, cached.body)
			return
		}
	}

	if cascSource == nil || !cascSource.IsLoaded() {
		h.sendExportError(w, cacheKey, progressKey, http.StatusConflict, map[string]any{"id": "ERR_NO_CASC"})
		return
	}

	params, valid, tileErr := parseADTExportParams(body)
	if !valid {
		h.sendExportError(w, cacheKey, progressKey, http.StatusBadRequest, map[string]any{
			"id": "ERR_INVALID_PARAMETERS",
			"required": map[string]string{
				"mapID":  "number",
				"mapDir": "string",
				"tileX":  "number (0-63)",
				"tileY":  "number (0-63)",
			},
		})
		return
	}
	if tileErr {
		h.sendExportError(w, cacheKey, progressKey, http.StatusBadRequest, map[string]any{
			"id":      "ERR_INVALID_TILE_COORDS",
			"message": "Tile coordinates must be 0-63",
		})
		return
	}

	exportID := h.nextExportID()
	result, err := h.Services.Export.Export(r.Context(), params, exportID)
	if err != nil {
		h.sendExportError(w, cacheKey, progressKey, http.StatusInternalServerError, map[string]any{
			"id":      "ERR_INTERNAL",
			"message": err.Error(),
		})
		return
	}

	responseObj := map[string]any{
		"id":         "EXPORT_RESULT",
		"type":       "ADT",
		"exportID":   result.ExportID,
		"mapID":      result.MapID,
		"mapDir":     result.MapDir,
		"tileX":      result.TileX,
		"tileY":      result.TileY,
		"tileIndex":  result.TileIndex,
		"exportPath": result.ExportPath,
		"exportType": result.ExportType,
		"mainFile":   result.MainFile,
	}

	if progressKey != "" {
		sendJSON(w, http.StatusOK, responseObj)
		return
	}
	h.responseCache.sendAndCache(w, cacheKey, http.StatusOK, responseObj)
}

// POST /rest/finalizeExportProgress
func (h *Handler) FinalizeExportProgress(w http.ResponseWriter, r *http.Request) {
	body, err := readJSONBody(r)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"id": "ERR_INVALID_JSON"})
		return
	}

	key, _ := body["key"].(string)
	if key == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"id":      "ERR_INVALID_PARAMETERS",
			"message": "key is required",
		})
		return
	}

	snapshot, ok := h.Services.Progress.Finalize(key)
	if !ok {
		sendJSON(w, http.StatusNotFound, map[string]any{"id": "ERR_NOT_FOUND"})
		return
	}

	exportadt.ReleaseAdtExportBatchMemory()

	sendJSON(w, http.StatusOK, mergeID("EXPORT_PROGRESS", snapshot))
}

func (h *Handler) sendExportError(w http.ResponseWriter, cacheKey, progressKey string, status int, obj map[string]any) {
	if progressKey != "" {
		sendJSON(w, status, obj)
		return
	}
	h.responseCache.sendAndCache(w, cacheKey, status, obj)
}

func readJSONBody(r *http.Request) (map[string]any, error) {
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, httplimit.MaxRequestBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > httplimit.MaxRequestBodyBytes {
		return nil, errRequestBodyTooLarge
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, err
	}
	return body, nil
}

var errRequestBodyTooLarge = errors.New("request body too large")

func parseRequiredIntQuery(r *http.Request, name string) (int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return 0, errors.New("missing")
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func parsePositiveIntQuery(r *http.Request, name string) (int, error) {
	n, err := parseRequiredIntQuery(r, name)
	if err != nil || n <= 0 {
		return 0, errors.New("invalid")
	}
	return n, nil
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func parseCharacterMetaParams(body map[string]any) (casc.CharacterMetaParams, bool) {
	race, raceOK := body["race"].(float64)
	gender, genderOK := body["gender"].(float64)
	customizations, custOK := body["customizations"].(map[string]any)
	if !raceOK || !genderOK || !custOK {
		return casc.CharacterMetaParams{}, false
	}

	out := casc.CharacterMetaParams{
		Race:           int(race),
		Gender:         int(gender),
		Customizations: map[string]int{},
	}
	for k, v := range customizations {
		if n, ok := v.(float64); ok {
			out.Customizations[k] = int(n)
		}
	}
	if override, ok := body["fileDataIdOverride"].(float64); ok {
		v := int(override)
		out.FileDataIDOverride = &v
	}
	return out, true
}

func parseADTExportParams(body map[string]any) (casc.ADTExportParams, bool, bool) {
	mapID, mapOK := body["mapID"].(float64)
	mapDir, dirOK := body["mapDir"].(string)
	tileX, xOK := body["tileX"].(float64)
	tileY, yOK := body["tileY"].(float64)
	if !mapOK || !dirOK || !xOK || !yOK {
		return casc.ADTExportParams{}, false, false
	}
	if int(tileX) < 0 || int(tileX) > 63 || int(tileY) < 0 || int(tileY) > 63 {
		return casc.ADTExportParams{}, true, true
	}

	params := casc.ADTExportParams{
		MapID:       int(mapID),
		MapDir:      mapDir,
		TileX:       int(tileX),
		TileY:       int(tileY),
		ProgressKey: stringField(body, "progressKey"),
	}
	params.Quality = floatPtr(body, "quality")
	params.IncludeM2 = boolPtr(body, "includeM2")
	params.IncludeWMO = boolPtr(body, "includeWMO")
	params.IncludeWMOSets = boolPtr(body, "includeWMOSets")
	params.IncludeGameObjects = boolPtr(body, "includeGameObjects")
	params.IncludeLiquid = boolPtr(body, "includeLiquid")
	params.IncludeFoliage = boolPtr(body, "includeFoliage")
	params.IncludeHoles = boolPtr(body, "includeHoles")
	params.SplitAlphaMaps = boolPtr(body, "splitAlphaMaps")
	params.SplitLargeTerrainBakes = boolPtr(body, "splitLargeTerrainBakes")
	params.TileIndex = floatPtr(body, "tileIndex")
	params.TileCount = floatPtr(body, "tileCount")
	params.StepsPerTile = floatPtr(body, "stepsPerTile")
	if raw, ok := body["gameObjects"].([]any); ok {
		params.GameObjects = raw
	}
	return params, true, false
}

func stringField(body map[string]any, key string) string {
	v, _ := body[key].(string)
	return v
}

func boolPtr(body map[string]any, key string) *bool {
	v, ok := body[key].(bool)
	if !ok {
		return nil
	}
	return &v
}

func floatPtr(body map[string]any, key string) *int {
	v, ok := body[key].(float64)
	if !ok {
		return nil
	}
	n := int(v)
	return &n
}

func copyMapWithout(src map[string]any, key string) map[string]any {
	out := make(map[string]any, len(src))
	for k, v := range src {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func structToMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func mergeID(id string, snapshot *casc.ExportProgressSnapshot) map[string]any {
	out := structToMap(snapshot)
	out["id"] = id
	return out
}
