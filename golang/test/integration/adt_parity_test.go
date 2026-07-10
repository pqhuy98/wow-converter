//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/test/internal/snapshot"
)

const (
	northrendMapID   = 571
	northrendMapDir  = "northrend"
	northrendTileX   = 21
	northrendTileY   = 27
	northrendQuality = 1024
)

func TestADTExportParityNorthrend2127(t *testing.T) {
	env := testServer(t)
	golden := loadGoldenADTManifest(t)

	requestBody := map[string]any{
		"mapID":              northrendMapID,
		"mapDir":             northrendMapDir,
		"tileX":              northrendTileX,
		"tileY":              northrendTileY,
		"quality":            northrendQuality,
		"includeM2":          false,
		"includeWMO":         false,
		"includeWMOSets":     false,
		"includeGameObjects": false,
		"includeLiquid":      true,
		"includeFoliage":     false,
		"includeHoles":       true,
	}

	goExport, err := exportADT(t, env, requestBody)
	if err != nil {
		t.Fatal(err)
	}
	tileID := fmt.Sprintf("%d_%d", northrendTileX, northrendTileY)
	t.Logf("Go/server export at %s (main=%s)", goExport.ExportPath, goExport.MainFile)

	if refURL := strings.TrimRight(os.Getenv("WOW_TS_REFERENCE_URL"), "/"); refURL != "" {
		refEnv := &serverEnv{
			BaseURL:    refURL,
			HTTPClient: env.HTTPClient,
		}
		if !refEnv.ping(t) {
			t.Skipf("TS reference server unavailable at %s", refURL)
		}
		if !refEnv.cascLoaded(t) {
			t.Skip("CASC not loaded on TS reference server")
		}

		tsExport, err := exportADT(t, refEnv, requestBody)
		if err != nil {
			t.Fatalf("TS reference export: %v", err)
		}
		t.Logf("TS reference export at %s", tsExport.ExportPath)

		manifest, err := snapshot.Create(tsExport.ExportPath)
		if err != nil {
			t.Fatalf("snapshot TS export: %v", err)
		}
		summary, err := snapshot.CompareManifestToDir(manifest, goExport.ExportPath, snapshot.CompareOptions{
			ToleranceRegex: regexp.MustCompile(`(?i)\.png$`),
			MaxDelta:       2,
			BaselineDir:    tsExport.ExportPath,
		})
		if err != nil {
			t.Fatalf("compare exports: %v", err)
		}
		if !summary.Pass() {
			var buf bytes.Buffer
			snapshot.PrintSummary(&buf, summary)
			t.Fatalf("Go export differs from TS reference:\n%s", buf.String())
		}
		return
	}

	if len(golden.Files) == 0 || allPlaceholders(golden) {
		t.Skip("golden ADT manifest has placeholder hashes; run snapshot after TS export or set WOW_TS_REFERENCE_URL")
	}

	mapDir := goExport.ExportPath
	summary, err := snapshot.CompareManifestToDir(golden, mapDir, snapshot.CompareOptions{
		ToleranceRegex: regexp.MustCompile(`(?i)tex_.*\.png$`),
		MaxDelta:       2,
		BaselineDir:    filepath.Join(goldenRoot(t), "adt", "northrend_21_27", "data"),
	})
	if err != nil {
		t.Fatalf("compare golden: %v", err)
	}

	filtered := filterTileSummary(summary, tileID)
	if !filtered.Pass() {
		var buf bytes.Buffer
		snapshot.PrintSummary(&buf, filtered)
		t.Fatalf("ADT export differs from golden manifest:\n%s", buf.String())
	}
}

type exportResult struct {
	ExportPath string
	MainFile   string
}

func exportADT(t *testing.T, env *serverEnv, body map[string]any) (*exportResult, error) {
	t.Helper()

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, env.BaseURL+"/rest/exportADT", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("exportADT HTTP %d: %s", resp.StatusCode, truncate(string(data), 300))
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out["id"] != "EXPORT_RESULT" {
		return nil, fmt.Errorf("unexpected export response: %v", out)
	}

	exportPath, _ := out["exportPath"].(string)
	mainFile, _ := out["mainFile"].(string)
	if exportPath == "" {
		return nil, fmt.Errorf("missing exportPath in response: %v", out)
	}
	return &exportResult{ExportPath: exportPath, MainFile: mainFile}, nil
}

func loadGoldenADTManifest(t *testing.T) *snapshot.Manifest {
	t.Helper()
	path := filepath.Join(goldenRoot(t), "adt", "northrend_21_27", "manifest.json")
	manifest, err := snapshot.LoadManifest(path)
	if err != nil {
		t.Fatalf("load golden ADT manifest: %v", err)
	}
	return manifest
}

func allPlaceholders(m *snapshot.Manifest) bool {
	for _, rec := range m.Files {
		if !snapshot.IsPlaceholderSHA(rec.SHA256) {
			return false
		}
	}
	return true
}

func filterTileSummary(summary *snapshot.Summary, tileID string) *snapshot.Summary {
	filtered := &snapshot.Summary{
		MaxDeltaAllowed: summary.MaxDeltaAllowed,
		ToleranceRegex:  summary.ToleranceRegex,
	}
	containsTile := func(path string) bool {
		return strings.Contains(path, tileID)
	}
	for _, f := range summary.Identical {
		if containsTile(f) {
			filtered.Identical = append(filtered.Identical, f)
		}
	}
	for _, f := range summary.WithinTolerance {
		if containsTile(f.File) {
			filtered.WithinTolerance = append(filtered.WithinTolerance, f)
		}
	}
	for _, f := range summary.Different {
		if containsTile(f.File) {
			filtered.Different = append(filtered.Different, f)
		}
	}
	for _, f := range summary.Missing {
		if containsTile(f) {
			filtered.Missing = append(filtered.Missing, f)
		}
	}
	for _, f := range summary.Extra {
		if containsTile(f) {
			filtered.Extra = append(filtered.Extra, f)
		}
	}
	return filtered
}
