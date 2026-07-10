//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/test/internal/snapshot"
)

const defaultServerURL = "http://127.0.0.1:17753"

type serverEnv struct {
	BaseURL    string
	HTTPClient *http.Client
}

func testServer(t *testing.T) *serverEnv {
	t.Helper()

	baseURL := strings.TrimRight(os.Getenv("WOW_DATA_SERVER_URL"), "/")
	if baseURL == "" {
		baseURL = defaultServerURL
	}

	env := &serverEnv{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
	if !env.ping(t) {
		t.Skipf("wow-data-server unavailable at %s", baseURL)
	}
	if !env.cascLoaded(t) {
		t.Skip("CASC not loaded on wow-data-server")
	}
	return env
}

func (e *serverEnv) ping(t *testing.T) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.BaseURL+"/rest/getCascInfo", nil)
	if err != nil {
		return false
	}
	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (e *serverEnv) cascLoaded(t *testing.T) bool {
	t.Helper()
	body, status, err := e.getJSON(t, "/rest/getCascInfo", nil)
	if err != nil || status != http.StatusOK {
		return false
	}
	return body["id"] == "CASC_INFO"
}

func (e *serverEnv) getJSON(t *testing.T, path string, params url.Values) (map[string]any, int, error) {
	t.Helper()
	u := e.BaseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decode json: %w", err)
	}
	return out, resp.StatusCode, nil
}

func (e *serverEnv) resolveFileDataID(t *testing.T, fileName string) uint32 {
	t.Helper()
	body, status, err := e.getJSON(t, "/rest/getFileByName", url.Values{"fileName": {fileName}})
	if err != nil {
		t.Fatalf("getFileByName(%q): %v", fileName, err)
	}
	if status != http.StatusOK || body["id"] != "LISTFILE_RESULT" {
		t.Fatalf("getFileByName(%q): status=%d body=%v", fileName, status, body)
	}
	switch v := body["fileDataID"].(type) {
	case float64:
		return uint32(v)
	case json.Number:
		n, _ := v.Int64()
		return uint32(n)
	default:
		t.Fatalf("unexpected fileDataID type for %q: %T", fileName, body["fileDataID"])
		return 0
	}
}

func (e *serverEnv) fetchCascFile(t *testing.T, fileDataID uint32) []byte {
	t.Helper()
	u := fmt.Sprintf("%s/rest/cascFile?fileDataID=%d", e.BaseURL, fileDataID)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		t.Fatalf("fetch cascFile(%d): %v", fileDataID, err)
	}
	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		t.Fatalf("fetch cascFile(%d): %v", fileDataID, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read cascFile(%d): %v", fileDataID, err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cascFile(%d): HTTP %d: %s", fileDataID, resp.StatusCode, truncate(string(data), 200))
	}
	if len(data) == 0 {
		t.Fatalf("cascFile(%d): empty body", fileDataID)
	}
	return data
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func goldenRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "golden")
}

func loadGoldenCascManifest(t *testing.T) *snapshot.Manifest {
	t.Helper()
	path := filepath.Join(goldenRoot(t), "casc", "manifest.json")
	manifest, err := snapshot.LoadManifest(path)
	if err != nil {
		t.Fatalf("load golden casc manifest: %v", err)
	}
	return manifest
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func assertM2Magic(t *testing.T, data []byte) {
	t.Helper()
	if len(data) < 4 {
		t.Fatal("M2 data too small")
	}
	magicLE := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
	magic := string([]byte{data[0], data[1], data[2], data[3]})
	switch {
	case magic == "MD20", magic == "MD21":
		return
	case magicLE == 0x3032444D, magicLE == 0x3132444D:
		return
	default:
		t.Fatalf("unexpected M2 magic %q (0x%x)", magic, magicLE)
	}
}

func assertBLPMagic(t *testing.T, data []byte) {
	t.Helper()
	if len(data) < 4 {
		t.Fatal("BLP data too small")
	}
	magic := string(data[:4])
	if magic != "BLP1" && magic != "BLP2" {
		t.Fatalf("unexpected BLP magic %q", magic)
	}
}

func assertDB2Magic(t *testing.T, data []byte) {
	t.Helper()
	if len(data) < 4 {
		t.Fatal("DB2 data too small")
	}
	magic := string(data[:4])
	if !strings.HasPrefix(magic, "WDC") {
		t.Fatalf("unexpected DB2 magic %q", magic)
	}
}

func compareAgainstTSReference(t *testing.T, env *serverEnv, fileDataID uint32, data []byte) {
	t.Helper()
	refURL := strings.TrimRight(os.Getenv("WOW_TS_REFERENCE_URL"), "/")
	if refURL == "" {
		return
	}

	u := fmt.Sprintf("%s/rest/cascFile?fileDataID=%d", refURL, fileDataID)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		t.Fatalf("TS reference request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Skipf("TS reference server unavailable at %s: %v", refURL, err)
	}
	defer resp.Body.Close()
	refData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read TS reference: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("TS reference cascFile(%d): HTTP %d", fileDataID, resp.StatusCode)
	}
	if !bytes.Equal(data, refData) {
		t.Fatalf("cascFile(%d): Go server bytes differ from TS reference (%d vs %d bytes, sha %s vs %s)",
			fileDataID, len(data), len(refData), sha256Hex(data)[:12], sha256Hex(refData)[:12])
	}
}
