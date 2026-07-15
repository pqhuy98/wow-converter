package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"

	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/server/rest"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
	"github.com/pqhuy98/wow-converter/internal/wow/service"
)

// InProcessClient calls wow-data-server REST handlers in-process (bundled mode).
type InProcessClient struct {
	handler *rest.Handler
	casc    cascTracker
}

// NewInProcessClient creates a client backed by the local REST handler.
func NewInProcessClient(handler *rest.Handler) *InProcessClient {
	return &InProcessClient{handler: handler}
}

func (c *InProcessClient) WaitUntilReady(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		info, err := c.GetCASCInfo(ctx)
		if err == nil && info.BuildName != "" {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("wow-data-server not ready: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (c *InProcessClient) GetConfig(ctx context.Context, key string) (ConfigResponse, error) {
	params := url.Values{}
	if key != "" {
		params.Set("key", key)
	}
	body, status, err := c.doGet(ctx, "/rest/getConfig", params)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, unexpectedResponse("getConfig", body)
	}
	switch body["id"] {
	case "CONFIG_SINGLE":
		return ConfigResponse{body["key"].(string): body["value"]}, nil
	case "CONFIG_FULL":
		cfg, _ := body["config"].(map[string]any)
		return ConfigResponse(cfg), nil
	default:
		return nil, unexpectedResponse("getConfig", body)
	}
}

func (c *InProcessClient) SetConfig(ctx context.Context, key string, value any) (ConfigResponse, error) {
	body, status, err := c.doPost(ctx, "/rest/setConfig", map[string]any{"key": key, "value": value})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK || body["id"] != "CONFIG_SET_DONE" {
		return nil, unexpectedResponse("setConfig", body)
	}
	return ConfigResponse{body["key"].(string): body["value"]}, nil
}

func (c *InProcessClient) GetMapList(ctx context.Context) ([]casc.MapEntry, error) {
	body, status, err := c.doGet(ctx, "/rest/getMapList", nil)
	if err != nil {
		return nil, err
	}
	if status == http.StatusConflict && body["id"] == "ERR_NO_CASC" {
		return nil, errNoCASC
	}
	if status != http.StatusOK || body["id"] != "MAP_LIST" {
		return nil, unexpectedResponse("getMapList", body)
	}
	return decodeSlice[casc.MapEntry](body["maps"])
}

func (c *InProcessClient) LoadCASCLocal(ctx context.Context, installDirectory string) ([]casc.Build, error) {
	body, status, err := c.doPost(ctx, "/rest/loadCascLocal", map[string]any{"installDirectory": installDirectory})
	if err != nil {
		return nil, err
	}
	if status == http.StatusConflict && body["id"] == "ERR_CASC_ACTIVE" {
		return nil, errCASCActive
	}
	return parseBuildsResponse(body)
}

func (c *InProcessClient) LoadCASCRemote(ctx context.Context, regionTag string) ([]casc.Build, error) {
	body, status, err := c.doPost(ctx, "/rest/loadCascRemote", map[string]any{"regionTag": regionTag})
	if err != nil {
		return nil, err
	}
	if status == http.StatusConflict && body["id"] == "ERR_CASC_ACTIVE" {
		return nil, errCASCActive
	}
	return parseBuildsResponse(body)
}

func (c *InProcessClient) LoadCASCBuild(ctx context.Context, buildIndex int) (CASCInfo, error) {
	body, _, err := c.doPost(ctx, "/rest/loadCascBuild", map[string]any{"buildIndex": buildIndex})
	if err != nil {
		return CASCInfo{}, err
	}
	info, err := parseCASCInfoResponse(body)
	if err != nil {
		return CASCInfo{}, err
	}
	c.casc.apply(info)
	return info, nil
}

func (c *InProcessClient) GetCASCInfo(ctx context.Context) (CASCInfo, error) {
	body, status, err := c.doGet(ctx, "/rest/getCascInfo", nil)
	if err != nil {
		return CASCInfo{}, err
	}
	if status == http.StatusServiceUnavailable && body["id"] == "CASC_UNAVAILABLE" {
		c.onCascUnavailable()
		return CASCInfo{}, errCASCUnavailable
	}
	if status == http.StatusOK && body["id"] == "CASC_INFO" {
		info, err := parseCASCInfo(body)
		if err != nil {
			return CASCInfo{}, err
		}
		c.casc.apply(info)
		return info, nil
	}
	return CASCInfo{}, unexpectedResponse("getCascInfo", body)
}

func (c *InProcessClient) GetCascLoadProgress(ctx context.Context) (CascLoadProgress, error) {
	body, status, err := c.doGet(ctx, "/rest/getCascLoadProgress", nil)
	if err != nil {
		return CascLoadProgress{}, err
	}
	if status != http.StatusOK || body["id"] != "CASC_LOAD_PROGRESS" {
		return CascLoadProgress{}, unexpectedResponse("getCascLoadProgress", body)
	}
	loading, _ := body["loading"].(bool)
	message, _ := body["message"].(string)
	return CascLoadProgress{Loading: loading, Message: message}, nil
}

func (c *InProcessClient) UnloadCASC(ctx context.Context) error {
	body, status, err := c.doPost(ctx, "/rest/unloadCasc", map[string]any{})
	if err != nil {
		return err
	}
	if status != http.StatusOK || body["id"] != "CASC_UNLOADED" {
		return unexpectedResponse("unloadCasc", body)
	}
	c.casc.clear()
	runtimecache.ClearConverterRuntimeCaches()
	return nil
}

func (c *InProcessClient) SoftRestart(ctx context.Context, reloadEnv bool) (casc.SoftRestartResult, error) {
	body, status, err := c.doPost(ctx, "/rest/softRestart", map[string]any{"reloadEnv": reloadEnv})
	if err != nil {
		return casc.SoftRestartResult{}, err
	}
	if status != http.StatusOK || body["id"] != "SOFT_RESTART_DONE" {
		return casc.SoftRestartResult{}, unexpectedResponse("softRestart", body)
	}
	result := casc.SoftRestartResult{CascLoaded: body["cascLoaded"] == true}
	if v, ok := body["buildName"].(string); ok {
		result.BuildName = v
	}
	if v, ok := body["error"].(string); ok {
		result.Error = v
	}
	c.casc.clear()
	runtimecache.ClearConverterRuntimeCaches()
	return result, nil
}

func (c *InProcessClient) SearchFiles(ctx context.Context, search string, useRegex bool) ([]casc.ListfileEntry, error) {
	params := url.Values{"search": {search}}
	if useRegex {
		params.Set("useRegularExpression", "1")
	} else {
		params.Set("useRegularExpression", "0")
	}
	body, status, err := c.doGet(ctx, "/rest/searchFiles", params)
	if err != nil {
		return nil, err
	}
	if status == http.StatusConflict && body["id"] == "ERR_LISTFILE_NOT_LOADED" {
		return nil, errListfileNotLoaded
	}
	if status != http.StatusOK || body["id"] != "LISTFILE_SEARCH_RESULT" {
		return nil, unexpectedResponse("searchFiles", body)
	}
	return decodeSlice[casc.ListfileEntry](body["entries"])
}

func (c *InProcessClient) CollectBrowseFileIndex(ctx context.Context) ([]casc.ListfileEntry, []casc.ListfileEntry, error) {
	if err := c.WaitUntilReady(ctx); err != nil {
		return nil, nil, err
	}
	if !c.handler.Services.Listfile.IsLoaded() {
		return nil, nil, errListfileNotLoaded
	}
	models, textures := c.handler.Services.Listfile.CollectBrowseFileIndex()
	return models, textures, nil
}

func (c *InProcessClient) CollectMapTileFileIndex(ctx context.Context) ([]casc.ListfileEntry, error) {
	if err := c.WaitUntilReady(ctx); err != nil {
		return nil, err
	}
	if !c.handler.Services.Listfile.IsLoaded() {
		return nil, errListfileNotLoaded
	}
	return c.handler.Services.Listfile.CollectMapTileFileIndex(), nil
}

func (c *InProcessClient) GetFileByID(ctx context.Context, fileDataID int) (casc.ListfileEntry, error) {
	params := url.Values{"fileDataID": {strconv.Itoa(fileDataID)}}
	body, _, err := c.doGet(ctx, "/rest/getFileById", params)
	if err != nil {
		return casc.ListfileEntry{}, err
	}
	return parseListfileResult(body)
}

func (c *InProcessClient) GetFileByName(ctx context.Context, fileName string) (casc.ListfileEntry, error) {
	params := url.Values{"fileName": {fileName}}
	body, _, err := c.doGet(ctx, "/rest/getFileByName", params)
	if err != nil {
		return casc.ListfileEntry{}, err
	}
	return parseListfileResult(body)
}

func (c *InProcessClient) GetModelSkins(ctx context.Context, fileDataID int) ([]casc.ModelSkin, error) {
	params := url.Values{"fileDataID": {strconv.Itoa(fileDataID)}}
	body, status, err := c.doGet(ctx, "/rest/getModelSkins", params)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK || body["id"] != "MODEL_SKINS" {
		return nil, unexpectedResponse("getModelSkins", body)
	}
	return decodeSlice[casc.ModelSkin](body["skins"])
}

func (c *InProcessClient) InitModelCaches(ctx context.Context) error {
	body, status, err := c.doGet(ctx, "/rest/initModelCaches", nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK || body["id"] != "MODEL_CACHES_READY" {
		return unexpectedResponse("initModelCaches", body)
	}
	return nil
}

func (c *InProcessClient) ResolveNpcDisplayMeta(ctx context.Context, displayID int) (casc.NpcDisplayMeta, error) {
	params := url.Values{"displayId": {strconv.Itoa(displayID)}}
	body, status, err := c.doGet(ctx, "/rest/resolveNpcDisplay", params)
	if err != nil {
		return casc.NpcDisplayMeta{}, err
	}
	if status == http.StatusConflict {
		return casc.NpcDisplayMeta{}, errNoCASC
	}
	if status != http.StatusOK || body["id"] != "NPC_DISPLAY_META" {
		return casc.NpcDisplayMeta{}, unexpectedResponse("resolveNpcDisplay", body)
	}
	return decodeObject[casc.NpcDisplayMeta](body)
}

func (c *InProcessClient) DownloadCascFile(ctx context.Context, fileDataID int) ([]byte, error) {
	params := url.Values{"fileDataID": {strconv.Itoa(fileDataID)}}
	req := httptest.NewRequest(http.MethodGet, "/rest/cascFile?"+params.Encode(), nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	c.handler.CascFile(rec, req)
	if rec.Code != http.StatusOK {
		return nil, unexpectedHTTP(rec.Code, rec.Body.Bytes())
	}
	return rec.Body.Bytes(), nil
}

func (c *InProcessClient) DownloadExportFile(ctx context.Context, relativePath string) ([]byte, error) {
	params := url.Values{"path": {relativePath}}
	req := httptest.NewRequest(http.MethodGet, "/rest/download?"+params.Encode(), nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	c.handler.Download(rec, req)
	if rec.Code != http.StatusOK {
		return nil, unexpectedHTTP(rec.Code, rec.Body.Bytes())
	}
	return rec.Body.Bytes(), nil
}

func (c *InProcessClient) GetCharMeta(ctx context.Context, params casc.CharacterMetaParams) (casc.CharacterMetaResponse, error) {
	body, status, err := c.doPost(ctx, "/rest/charMeta", params)
	if err != nil {
		return casc.CharacterMetaResponse{}, err
	}
	if status == http.StatusOK && body["id"] == "CHAR_META" {
		return decodeObject[casc.CharacterMetaResponse](body)
	}
	if status == http.StatusConflict {
		return casc.CharacterMetaResponse{}, errNoCASC
	}
	return casc.CharacterMetaResponse{}, unexpectedResponse("charMeta", body)
}

func (c *InProcessClient) ExportADT(ctx context.Context, params casc.ADTExportParams) (casc.ADTExportResult, error) {
	body, status, err := c.doPost(ctx, "/rest/exportADT", params)
	if err != nil {
		return casc.ADTExportResult{}, err
	}
	if status == http.StatusOK && body["id"] == "EXPORT_RESULT" {
		return decodeObject[casc.ADTExportResult](body)
	}
	if status == http.StatusConflict {
		return casc.ADTExportResult{}, errNoCASC
	}
	return casc.ADTExportResult{}, unexpectedResponse("exportADT", body)
}

func (c *InProcessClient) ExportADTForConversion(ctx context.Context, params casc.ADTExportParams) (*exportadt.ConversionOutput, error) {
	// ponytail: avoid REST JSON round-trip (PNG base64 + map re-marshal) in bundled mode.
	if server.GlobalRuntime.GetCascOptional() == nil {
		return nil, errNoCASC
	}
	exportadt.BeginConversionExport()
	defer exportadt.EndConversionExport()
	return service.ADTExporterService{}.ExportForConversion(ctx, params)
}

func (c *InProcessClient) GetExportProgress(ctx context.Context, progressKey string) (*casc.ExportProgressSnapshot, error) {
	params := url.Values{"key": {progressKey}}
	body, status, err := c.doGet(ctx, "/rest/exportProgress", params)
	if err != nil {
		return nil, err
	}
	if status == http.StatusOK && body["id"] == "EXPORT_PROGRESS" {
		snap, err := decodeObject[casc.ExportProgressSnapshot](body)
		if err != nil {
			return nil, err
		}
		return &snap, nil
	}
	return nil, nil
}

func (c *InProcessClient) FinalizeExportProgress(ctx context.Context, progressKey string) (*casc.ExportProgressSnapshot, error) {
	body, status, err := c.doPost(ctx, "/rest/finalizeExportProgress", map[string]any{"key": progressKey})
	if err != nil {
		return nil, err
	}
	if status == http.StatusOK && body["id"] == "EXPORT_PROGRESS" {
		snap, err := decodeObject[casc.ExportProgressSnapshot](body)
		if err != nil {
			return nil, err
		}
		return &snap, nil
	}
	return nil, nil
}

func (c *InProcessClient) doGet(ctx context.Context, path string, params url.Values) (map[string]any, int, error) {
	target := path
	if params != nil {
		target += "?" + params.Encode()
	}
	req := httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	c.dispatchGET(path, rec, req)
	return decodeBody(rec)
}

func (c *InProcessClient) doPost(ctx context.Context, path string, payload any) (map[string]any, int, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c.dispatchPOST(path, rec, req)
	return decodeBody(rec)
}

func (c *InProcessClient) dispatchGET(path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/rest/getCascInfo":
		c.handler.GetCascInfo(w, r)
	case "/rest/getCascLoadProgress":
		c.handler.GetCascLoadProgress(w, r)
	case "/rest/getConfig":
		c.handler.GetConfig(w, r)
	case "/rest/searchFiles":
		c.handler.SearchFiles(w, r)
	case "/rest/collectBrowseFileIndex":
		c.handler.CollectBrowseFileIndex(w, r)
	case "/rest/collectMapTileFileIndex":
		c.handler.CollectMapTileFileIndex(w, r)
	case "/rest/getFileById":
		c.handler.GetFileByID(w, r)
	case "/rest/getFileByName":
		c.handler.GetFileByName(w, r)
	case "/rest/getModelSkins":
		c.handler.GetModelSkins(w, r)
	case "/rest/initModelCaches":
		c.handler.InitModelCaches(w, r)
	case "/rest/resolveNpcDisplay":
		c.handler.ResolveNpcDisplay(w, r)
	case "/rest/getMapList":
		c.handler.GetMapList(w, r)
	case "/rest/exportProgress":
		c.handler.ExportProgress(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (c *InProcessClient) dispatchPOST(path string, w http.ResponseWriter, r *http.Request) {
	switch path {
	case "/rest/loadCascLocal":
		c.handler.LoadCascLocal(w, r)
	case "/rest/loadCascRemote":
		c.handler.LoadCascRemote(w, r)
	case "/rest/loadCascBuild":
		c.handler.LoadCascBuild(w, r)
	case "/rest/unloadCasc":
		c.handler.UnloadCasc(w, r)
	case "/rest/softRestart":
		c.handler.SoftRestart(w, r)
	case "/rest/setConfig":
		c.handler.SetConfig(w, r)
	case "/rest/charMeta":
		c.handler.CharMeta(w, r)
	case "/rest/exportADT":
		c.handler.ExportADT(w, r)
	case "/rest/exportADTForConversion":
		c.handler.ExportADTForConversion(w, r)
	case "/rest/finalizeExportProgress":
		c.handler.FinalizeExportProgress(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func decodeBody(rec *httptest.ResponseRecorder) (map[string]any, int, error) {
	if rec.Body.Len() == 0 {
		return map[string]any{}, rec.Code, nil
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		return nil, rec.Code, err
	}
	return out, rec.Code, nil
}

func stringify(v any) string {
	s, _ := v.(string)
	return s
}

var (
	errNoCASC            = &clientError{op: "casc", id: "not loaded"}
	errCASCUnavailable   = &clientError{op: "casc", id: "unavailable"}
	errCASCActive        = &clientError{op: "casc", id: "already active"}
	errInvalidInstall    = &clientError{op: "casc", id: "invalid install"}
	errNoCASCSetup       = &clientError{op: "casc", id: "no setup"}
	errInvalidBuild      = &clientError{op: "casc", id: "invalid build"}
	errListfileNotLoaded = &clientError{op: "listfile", id: "not loaded"}
)

// Ensure InProcessClient implements Client.
var _ Client = (*InProcessClient)(nil)

func unexpectedResponse(op string, body map[string]any) error {
	msg := stringify(body["message"])
	if msg != "" {
		return &clientError{op: op, id: stringify(body["id"]), msg: msg}
	}
	return &clientError{op: op, id: stringify(body["id"])}
}

func unexpectedHTTP(status int, body []byte) error {
	return &clientError{op: "http", id: strconv.Itoa(status) + ": " + string(body)}
}

type clientError struct {
	op, id, msg string
}

func (e *clientError) Error() string {
	if e.msg != "" {
		return e.op + ": " + e.id + ": " + e.msg
	}
	return e.op + ": unexpected response " + e.id
}
