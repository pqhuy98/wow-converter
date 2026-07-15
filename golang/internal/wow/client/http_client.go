package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
	"github.com/pqhuy98/wow-converter/internal/wow/transport"
)

const defaultTimeout = 20 * time.Minute

// HTTPClient talks to wow-data-server over HTTP or a unix socket.
type HTTPClient struct {
	baseURL    string
	socketPath string
	httpClient *http.Client
	casc       cascTracker
}

// NewHTTPClient creates a client for the given base URL.
// When baseURL is empty, WOW_DATA_SERVER_URL, unix socket transport, or http://127.0.0.1:<port> is used.
func NewHTTPClient(baseURL string) *HTTPClient {
	socketPath := ""
	if baseURL == "" && transport.UsesSocketTransport() {
		socketPath = transport.DefaultSocketPath()
		baseURL = "http://unix"
	}
	if baseURL == "" {
		baseURL = defaultBaseURL()
	}
	transportCfg := &http.Transport{}
	if socketPath != "" {
		transportCfg.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		}
	}
	return &HTTPClient{
		baseURL:    baseURL,
		socketPath: socketPath,
		httpClient: &http.Client{
			Timeout:   defaultTimeout,
			Transport: transportCfg,
		},
	}
}

func defaultBaseURL() string {
	if v := os.Getenv("WOW_DATA_SERVER_URL"); v != "" {
		return v
	}
	port := os.Getenv("WOW_DATA_SERVER_PORT")
	if port == "" {
		port = "17753"
	}
	return "http://127.0.0.1:" + port
}

func (c *HTTPClient) GetConfig(ctx context.Context, key string) (ConfigResponse, error) {
	params := url.Values{}
	if key != "" {
		params.Set("key", key)
	}
	jsonBody, err := c.getJSON(ctx, "/rest/getConfig", params)
	if err != nil {
		return nil, err
	}
	switch jsonBody["id"] {
	case "CONFIG_SINGLE":
		return ConfigResponse{jsonBody["key"].(string): jsonBody["value"]}, nil
	case "CONFIG_FULL":
		cfg, _ := jsonBody["config"].(map[string]any)
		return ConfigResponse(cfg), nil
	default:
		return nil, fmt.Errorf("unexpected response to getConfig: %v", jsonBody["id"])
	}
}

func (c *HTTPClient) SetConfig(ctx context.Context, key string, value any) (ConfigResponse, error) {
	jsonBody, err := c.postJSON(ctx, "/rest/setConfig", map[string]any{"key": key, "value": value})
	if err != nil {
		return nil, err
	}
	if jsonBody["id"] != "CONFIG_SET_DONE" {
		return nil, errors.New("failed to set configuration")
	}
	return ConfigResponse{jsonBody["key"].(string): jsonBody["value"]}, nil
}

func (c *HTTPClient) GetMapList(ctx context.Context) ([]casc.MapEntry, error) {
	jsonBody, err := c.getJSON(ctx, "/rest/getMapList", nil)
	if err != nil {
		return nil, err
	}
	switch jsonBody["id"] {
	case "MAP_LIST":
		return decodeSlice[casc.MapEntry](jsonBody["maps"])
	case "ERR_NO_CASC":
		return nil, errors.New("no CASC loaded")
	default:
		return nil, errors.New("failed to get map list")
	}
}

func (c *HTTPClient) LoadCASCLocal(ctx context.Context, installDirectory string) ([]casc.Build, error) {
	jsonBody, err := c.postJSON(ctx, "/rest/loadCascLocal", map[string]any{"installDirectory": installDirectory})
	if err != nil {
		return nil, err
	}
	return parseBuildsResponse(jsonBody, true)
}

func (c *HTTPClient) LoadCASCRemote(ctx context.Context, regionTag string) ([]casc.Build, error) {
	jsonBody, err := c.postJSON(ctx, "/rest/loadCascRemote", map[string]any{"regionTag": regionTag})
	if err != nil {
		return nil, err
	}
	return parseBuildsResponse(jsonBody, false)
}

func (c *HTTPClient) LoadCASCBuild(ctx context.Context, buildIndex int) (CASCInfo, error) {
	jsonBody, err := c.postJSON(ctx, "/rest/loadCascBuild", map[string]any{"buildIndex": buildIndex})
	if err != nil {
		return CASCInfo{}, err
	}
	info, err := parseCASCInfoResponse(jsonBody)
	if err != nil {
		return CASCInfo{}, err
	}
	c.casc.apply(info)
	return info, nil
}

func (c *HTTPClient) GetCASCInfo(ctx context.Context) (CASCInfo, error) {
	jsonBody, status, err := c.getJSONAllowError(ctx, "/rest/getCascInfo", nil)
	if err != nil {
		return CASCInfo{}, err
	}
	if status >= 200 && status < 300 && jsonBody["id"] == "CASC_INFO" {
		info, err := parseCASCInfo(jsonBody)
		if err != nil {
			return CASCInfo{}, err
		}
		c.casc.apply(info)
		return info, nil
	}
	if jsonBody["id"] == "CASC_UNAVAILABLE" {
		c.onCascUnavailable()
		return CASCInfo{}, errors.New("CASC not available")
	}
	return CASCInfo{}, errors.New("failed to get CASC info")
}

func (c *HTTPClient) GetCascLoadProgress(ctx context.Context) (CascLoadProgress, error) {
	jsonBody, status, err := c.getJSONAllowError(ctx, "/rest/getCascLoadProgress", nil)
	if err != nil {
		return CascLoadProgress{}, err
	}
	if status < 200 || status >= 300 || jsonBody["id"] != "CASC_LOAD_PROGRESS" {
		return CascLoadProgress{}, errors.New("failed to get CASC load progress")
	}
	loading, _ := jsonBody["loading"].(bool)
	message, _ := jsonBody["message"].(string)
	return CascLoadProgress{Loading: loading, Message: message}, nil
}

func (c *HTTPClient) UnloadCASC(ctx context.Context) error {
	jsonBody, err := c.postJSON(ctx, "/rest/unloadCasc", map[string]any{})
	if err != nil {
		return err
	}
	if jsonBody["id"] != "CASC_UNLOADED" {
		return errors.New("failed to unload CASC")
	}
	c.casc.clear()
	runtimecache.ClearConverterRuntimeCaches()
	return nil
}

func (c *HTTPClient) SoftRestart(ctx context.Context, reloadEnv bool) (casc.SoftRestartResult, error) {
	jsonBody, err := c.postJSON(ctx, "/rest/softRestart", map[string]any{"reloadEnv": reloadEnv})
	if err != nil {
		return casc.SoftRestartResult{}, err
	}
	if jsonBody["id"] != "SOFT_RESTART_DONE" {
		return casc.SoftRestartResult{}, errors.New("soft restart failed")
	}
	result := casc.SoftRestartResult{
		CascLoaded: jsonBody["cascLoaded"] == true,
	}
	if v, ok := jsonBody["buildName"].(string); ok {
		result.BuildName = v
	}
	if v, ok := jsonBody["error"].(string); ok {
		result.Error = v
	}
	c.casc.clear()
	runtimecache.ClearConverterRuntimeCaches()
	return result, nil
}

func (c *HTTPClient) SearchFiles(ctx context.Context, search string, useRegex bool) ([]casc.ListfileEntry, error) {
	params := url.Values{}
	params.Set("search", search)
	if useRegex {
		params.Set("useRegularExpression", "1")
	} else {
		params.Set("useRegularExpression", "0")
	}
	jsonBody, err := c.getJSON(ctx, "/rest/searchFiles", params)
	if err != nil {
		return nil, err
	}
	switch jsonBody["id"] {
	case "LISTFILE_SEARCH_RESULT":
		return decodeSlice[casc.ListfileEntry](jsonBody["entries"])
	case "ERR_LISTFILE_NOT_LOADED":
		return nil, errors.New("listfile not loaded")
	default:
		return nil, errors.New("failed to search files")
	}
}

func (c *HTTPClient) CollectBrowseFileIndex(ctx context.Context) ([]casc.ListfileEntry, []casc.ListfileEntry, error) {
	if err := c.WaitUntilReady(ctx); err != nil {
		return nil, nil, err
	}
	jsonBody, err := c.getJSON(ctx, "/rest/collectBrowseFileIndex", nil)
	if err != nil {
		return nil, nil, err
	}
	switch jsonBody["id"] {
	case "BROWSE_FILE_INDEX":
		models, err := decodeSlice[casc.ListfileEntry](jsonBody["models"])
		if err != nil {
			return nil, nil, err
		}
		textures, err := decodeSlice[casc.ListfileEntry](jsonBody["textures"])
		if err != nil {
			return nil, nil, err
		}
		return models, textures, nil
	case "ERR_LISTFILE_NOT_LOADED":
		return nil, nil, errors.New("listfile not loaded")
	default:
		return nil, nil, errors.New("failed to collect browse file index")
	}
}

func (c *HTTPClient) CollectMapTileFileIndex(ctx context.Context) ([]casc.ListfileEntry, error) {
	if err := c.WaitUntilReady(ctx); err != nil {
		return nil, err
	}
	jsonBody, err := c.getJSON(ctx, "/rest/collectMapTileFileIndex", nil)
	if err != nil {
		return nil, err
	}
	switch jsonBody["id"] {
	case "MAP_TILE_FILE_INDEX":
		return decodeSlice[casc.ListfileEntry](jsonBody["entries"])
	case "ERR_LISTFILE_NOT_LOADED":
		return nil, errors.New("listfile not loaded")
	default:
		return nil, errors.New("failed to collect map tile file index")
	}
}

func (c *HTTPClient) GetFileByID(ctx context.Context, fileDataID int) (casc.ListfileEntry, error) {
	params := url.Values{}
	params.Set("fileDataID", strconv.Itoa(fileDataID))
	jsonBody, err := c.getJSON(ctx, "/rest/getFileById", params)
	if err != nil {
		return casc.ListfileEntry{}, err
	}
	return parseListfileResult(jsonBody)
}

func (c *HTTPClient) GetFileByName(ctx context.Context, fileName string) (casc.ListfileEntry, error) {
	params := url.Values{}
	params.Set("fileName", fileName)
	jsonBody, err := c.getJSON(ctx, "/rest/getFileByName", params)
	if err != nil {
		return casc.ListfileEntry{}, err
	}
	return parseListfileResult(jsonBody)
}

func (c *HTTPClient) GetModelSkins(ctx context.Context, fileDataID int) ([]casc.ModelSkin, error) {
	params := url.Values{}
	params.Set("fileDataID", strconv.Itoa(fileDataID))
	jsonBody, err := c.getJSON(ctx, "/rest/getModelSkins", params)
	if err != nil {
		return nil, err
	}
	if jsonBody["id"] != "MODEL_SKINS" {
		return nil, errors.New("failed to get model skins")
	}
	return decodeSlice[casc.ModelSkin](jsonBody["skins"])
}

func (c *HTTPClient) InitModelCaches(ctx context.Context) error {
	jsonBody, err := c.getJSON(ctx, "/rest/initModelCaches", nil)
	if err != nil {
		return err
	}
	if jsonBody["id"] != "MODEL_CACHES_READY" {
		return errors.New("failed to initialize model caches")
	}
	return nil
}

func (c *HTTPClient) ResolveNpcDisplayMeta(ctx context.Context, displayID int) (casc.NpcDisplayMeta, error) {
	params := url.Values{}
	params.Set("displayId", strconv.Itoa(displayID))
	jsonBody, status, err := c.getJSONAllowError(ctx, "/rest/resolveNpcDisplay", params)
	if err != nil {
		return casc.NpcDisplayMeta{}, err
	}
	if status == http.StatusConflict || jsonBody["id"] == "ERR_NO_CASC" {
		return casc.NpcDisplayMeta{}, errors.New("no CASC loaded")
	}
	if status != http.StatusOK || jsonBody["id"] != "NPC_DISPLAY_META" {
		return casc.NpcDisplayMeta{}, errors.New("failed to resolve NPC display metadata")
	}
	meta := casc.NpcDisplayMeta{
		Found: jsonBody["found"] == true,
	}
	if model, ok := jsonBody["model"].(float64); ok {
		meta.Model = int(model)
	}
	if textures, ok := jsonBody["textures"].(map[string]any); ok {
		meta.Textures = make(map[string]int, len(textures))
		for k, v := range textures {
			if n, ok := v.(float64); ok {
				meta.Textures[k] = int(n)
			}
		}
	}
	if geosets, ok := jsonBody["geosets"].([]any); ok {
		for _, entry := range geosets {
			row, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			geoset := casc.NpcDisplayGeoset{}
			if idx, ok := row["geosetIndex"].(float64); ok {
				geoset.GeosetIndex = int(idx)
			}
			if val, ok := row["geosetValue"].(float64); ok {
				geoset.GeosetValue = int(val)
			}
			meta.Geosets = append(meta.Geosets, geoset)
		}
	}
	return meta, nil
}

func (c *HTTPClient) DownloadCascFile(ctx context.Context, fileDataID int) ([]byte, error) {
	params := url.Values{}
	params.Set("fileDataID", strconv.Itoa(fileDataID))
	req, err := c.newRequest(ctx, http.MethodGet, "/rest/cascFile", params, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		var errBody map[string]any
		if json.Unmarshal(body, &errBody) == nil {
			if errBody["id"] == "ERR_NOT_FOUND" {
				return nil, fmt.Errorf("CASC file not found: %d", fileDataID)
			}
		}
		return nil, fmt.Errorf("failed to download CASC file %d (%d)", fileDataID, res.StatusCode)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("CASC file is empty: %d", fileDataID)
	}
	return body, nil
}

func (c *HTTPClient) DownloadExportFile(ctx context.Context, relativePath string) ([]byte, error) {
	params := url.Values{}
	params.Set("path", relativePath)
	req, err := c.newRequest(ctx, http.MethodGet, "/rest/download", params, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download remote file: %s (%d)", relativePath, res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

func (c *HTTPClient) GetCharMeta(ctx context.Context, params casc.CharacterMetaParams) (casc.CharacterMetaResponse, error) {
	jsonBody, status, err := c.postJSONAllowError(ctx, "/rest/charMeta", params)
	if err != nil {
		return casc.CharacterMetaResponse{}, err
	}
	if status == http.StatusOK && jsonBody["id"] == "CHAR_META" {
		return decodeObject[casc.CharacterMetaResponse](jsonBody)
	}
	return casc.CharacterMetaResponse{}, charMetaRESTError(status, jsonBody)
}

func (c *HTTPClient) ExportADT(ctx context.Context, params casc.ADTExportParams) (casc.ADTExportResult, error) {
	jsonBody, status, err := c.postJSONAllowError(ctx, "/rest/exportADT", params)
	if err != nil {
		return casc.ADTExportResult{}, err
	}
	if status == http.StatusOK && jsonBody["id"] == "EXPORT_RESULT" {
		return decodeObject[casc.ADTExportResult](jsonBody)
	}
	if status == http.StatusConflict || jsonBody["id"] == "ERR_NO_CASC" {
		return casc.ADTExportResult{}, errors.New("no CASC loaded")
	}
	if status == http.StatusBadRequest {
		msg, _ := jsonBody["message"].(string)
		return casc.ADTExportResult{}, fmt.Errorf("invalid parameters for ADT export: %s", msg)
	}
	if status >= 500 {
		msg, _ := jsonBody["message"].(string)
		return casc.ADTExportResult{}, fmt.Errorf("server error during ADT export: %s", msg)
	}
	return casc.ADTExportResult{}, errors.New("unexpected response for ADT export")
}

func (c *HTTPClient) ExportADTForConversion(ctx context.Context, params casc.ADTExportParams) (*exportadt.ConversionOutput, error) {
	req, err := c.newRequest(ctx, http.MethodPost, "/rest/exportADTForConversion", nil, params)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		ID       string                      `json:"id"`
		Snapshot *exportadt.ConversionOutput `json:"snapshot"`
		Message  string                      `json:"message"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusOK && envelope.ID == "ADT_CONVERSION_SNAPSHOT" && envelope.Snapshot != nil {
		return envelope.Snapshot, nil
	}
	if res.StatusCode == http.StatusConflict || envelope.ID == "ERR_NO_CASC" {
		return nil, errors.New("no CASC loaded")
	}
	if res.StatusCode >= 500 {
		return nil, fmt.Errorf("server error during ADT conversion export: %s", envelope.Message)
	}
	return nil, errors.New("unexpected response for ADT conversion export")
}

func (c *HTTPClient) GetExportProgress(ctx context.Context, progressKey string) (*casc.ExportProgressSnapshot, error) {
	params := url.Values{}
	params.Set("key", progressKey)
	jsonBody, status, err := c.getJSONAllowError(ctx, "/rest/exportProgress", params)
	if err != nil {
		return nil, err
	}
	if status >= 200 && status < 300 && jsonBody["id"] == "EXPORT_PROGRESS" {
		snapshot, err := decodeObject[casc.ExportProgressSnapshot](jsonBody)
		if err != nil {
			return nil, err
		}
		return &snapshot, nil
	}
	return nil, nil
}

func (c *HTTPClient) FinalizeExportProgress(ctx context.Context, progressKey string) (*casc.ExportProgressSnapshot, error) {
	jsonBody, status, err := c.postJSONAllowError(ctx, "/rest/finalizeExportProgress", map[string]any{"key": progressKey})
	if err != nil {
		return nil, err
	}
	if status == http.StatusOK && jsonBody["id"] == "EXPORT_PROGRESS" {
		snapshot, err := decodeObject[casc.ExportProgressSnapshot](jsonBody)
		if err != nil {
			return nil, err
		}
		return &snapshot, nil
	}
	return nil, nil
}

func (c *HTTPClient) getJSON(ctx context.Context, path string, params url.Values) (map[string]any, error) {
	body, _, err := c.getJSONAllowError(ctx, path, params)
	return body, err
}

func (c *HTTPClient) getJSONAllowError(ctx context.Context, path string, params url.Values) (map[string]any, int, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path, params, nil)
	if err != nil {
		return nil, 0, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	return decodeJSONMap(res.Body, res.StatusCode)
}

func (c *HTTPClient) postJSON(ctx context.Context, path string, payload any) (map[string]any, error) {
	body, _, err := c.postJSONAllowError(ctx, path, payload)
	return body, err
}

func (c *HTTPClient) postJSONAllowError(ctx context.Context, path string, payload any) (map[string]any, int, error) {
	req, err := c.newRequest(ctx, http.MethodPost, path, nil, payload)
	if err != nil {
		return nil, 0, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()
	return decodeJSONMap(res.Body, res.StatusCode)
}

func (c *HTTPClient) newRequest(ctx context.Context, method, path string, params url.Values, payload any) (*http.Request, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path
	if params != nil {
		u.RawQuery = params.Encode()
	}

	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := os.Getenv("WOW_DATA_SERVER_TOKEN"); token != "" {
		req.Header.Set("X-Wow-Data-Token", token)
	}
	return req, nil
}

func decodeJSONMap(r io.Reader, status int) (map[string]any, int, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, status, err
	}
	if len(data) == 0 {
		return map[string]any{}, status, nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, status, err
	}
	return out, status, nil
}

func parseBuildsResponse(jsonBody map[string]any, local bool) ([]casc.Build, error) {
	switch jsonBody["id"] {
	case "CASC_INSTALL_BUILDS":
		return decodeSlice[casc.Build](jsonBody["builds"])
	default:
		return nil, buildsRESTError(jsonBody, local)
	}
}

func parseCASCInfoResponse(jsonBody map[string]any) (CASCInfo, error) {
	switch jsonBody["id"] {
	case "CASC_INFO":
		return parseCASCInfo(jsonBody)
	case "ERR_NO_CASC_SETUP":
		return CASCInfo{}, errors.New("no CASC setup available")
	case "ERR_INVALID_CASC_BUILD":
		return CASCInfo{}, errors.New("invalid build index")
	case "ERR_CASC_FAILED":
		return CASCInfo{}, errors.New("failed to load CASC build")
	default:
		return CASCInfo{}, errors.New("failed to load CASC build")
	}
}

func parseCASCInfo(jsonBody map[string]any) (CASCInfo, error) {
	info := CASCInfo{
		Type:        stringField(jsonBody, "type"),
		BuildConfig: jsonBody["buildConfig"],
		BuildName:   stringField(jsonBody, "buildName"),
		BuildKey:    stringField(jsonBody, "buildKey"),
	}
	if build, ok := jsonBody["build"].(map[string]any); ok {
		info.Build = casc.BuildInfo{
			Product: stringField(build, "Product"),
			Version: stringField(build, "Version"),
		}
	}
	return info, nil
}

func parseListfileResult(jsonBody map[string]any) (casc.ListfileEntry, error) {
	switch jsonBody["id"] {
	case "LISTFILE_RESULT":
		entry := casc.ListfileEntry{FileName: stringField(jsonBody, "fileName")}
		if v, ok := jsonBody["fileDataID"].(float64); ok {
			entry.FileDataID = int(v)
		}
		return entry, nil
	case "ERR_LISTFILE_NOT_LOADED":
		return casc.ListfileEntry{}, errors.New("listfile not loaded")
	default:
		return casc.ListfileEntry{}, errors.New("failed to get listfile result")
	}
}

func decodeSlice[T any](raw any) ([]T, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var out []T
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeObject[T any](raw map[string]any) (T, error) {
	var out T
	b, err := json.Marshal(raw)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	return out, nil
}

func stringField(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// WaitUntilReady polls getCascInfo until CASC is loaded or ctx expires.
func (c *HTTPClient) WaitUntilReady(ctx context.Context) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		info, err := c.GetCASCInfo(ctx)
		if err == nil && info.BuildName != "" {
			return nil
		}
		logWaitingForDataServerOnce(c.baseURL)
		select {
		case <-ctx.Done():
			return fmt.Errorf("wow-data-server not ready at %s: %w", c.baseURL, ctx.Err())
		case <-ticker.C:
		}
	}
}

var waitForDataServerLogged sync.Map

func logWaitingForDataServerOnce(baseURL string) {
	if _, loaded := waitForDataServerLogged.LoadOrStore(baseURL, struct{}{}); !loaded {
		fmt.Fprintf(os.Stderr, "waiting for wow-data-server %s ...\n", baseURL)
	}
}

var (
	_ Client               = (*HTTPClient)(nil)
	_ DirectListfileClient = (*HTTPClient)(nil)
)
