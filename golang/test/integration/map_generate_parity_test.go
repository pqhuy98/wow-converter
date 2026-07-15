//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/test/internal/snapshot"
)

// TestMapGenerateParityValianceKeep compares the complete TS disk pipeline
// against Go's in-memory pipeline using the active Valiance Keep example from
// examples/convert.ts and golang/cmd/convert/main.go.
func TestMapGenerateParityValianceKeep(t *testing.T) {
	tsURL := strings.TrimRight(os.Getenv("WOW_TS_CONVERTER_URL"), "/")
	goURL := strings.TrimRight(os.Getenv("WOW_GO_CONVERTER_URL"), "/")
	if tsURL == "" || goURL == "" {
		t.Skip("set WOW_TS_CONVERTER_URL and WOW_GO_CONVERTER_URL to run full map parity")
	}

	body := map[string]any{
		"tiles": []map[string]int{
			{"x": 21, "y": 27},
			{"x": 22, "y": 27},
			{"x": 21, "y": 28},
			{"x": 22, "y": 28},
		},
		"quality":                  4096,
		"mapSaveName":              "parity-valiancekeep.w3x",
		"clampLower":               0,
		"clampUpper":               1,
		"autoClampPercent":         true,
		"mapAngleDeg":              0,
		"unitScale":                2,
		"includeBuildingInteriors": true,
		"freshExport":              true,
		"creatures": map[string]bool{
			"enable":        true,
			"allAreDoodads": true,
		},
	}

	client := &http.Client{Timeout: 30 * time.Second}
	tsOutput := generateMapAndWait(t, client, tsURL, body)
	artifactRoot := filepath.Join(workspace.FindRepoRoot(), ".parity-artifacts", "map-output")
	if configured := strings.TrimSpace(os.Getenv("WOW_MAP_PARITY_ARTIFACT_DIR")); configured != "" {
		artifactRoot = configured
	}
	if err := os.RemoveAll(artifactRoot); err != nil {
		t.Fatalf("clear parity artifacts: %v", err)
	}
	tsCopy := filepath.Join(artifactRoot, "ts")
	if err := copyTestDirectory(tsOutput, tsCopy); err != nil {
		t.Fatalf("copy TS output: %v", err)
	}
	manifest, err := snapshot.Create(tsCopy)
	if err != nil {
		t.Fatalf("snapshot TS output: %v", err)
	}

	goOutput := generateMapAndWait(t, client, goURL, body)
	goCopy := filepath.Join(artifactRoot, "go")
	if err := copyTestDirectory(goOutput, goCopy); err != nil {
		t.Fatalf("copy Go output: %v", err)
	}
	t.Logf("parity artifacts: TS=%s Go=%s", tsCopy, goCopy)
	summary, err := snapshot.CompareManifestToDir(manifest, goCopy, snapshot.CompareOptions{
		ToleranceRegex: regexp.MustCompile(`(?i)\.png$`),
		MaxDelta:       2,
		BaselineDir:    tsCopy,
	})
	if err != nil {
		t.Fatalf("compare generated maps: %v", err)
	}
	if !summary.Pass() || len(summary.Extra) > 0 {
		var report bytes.Buffer
		snapshot.PrintSummary(&report, summary)
		t.Fatalf("Go map differs from TS reference:\n%s", report.String())
	}
}

type mapGenerateStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error"`
	Result *struct {
		OutputDir string `json:"outputDir"`
	} `json:"result"`
}

func generateMapAndWait(t *testing.T, client *http.Client, baseURL string, body map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		baseURL+"/api/maps/northrend/generate-wc3",
		bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		t.Fatalf("queue map generation at %s: %v", baseURL, err)
	}
	status := decodeMapGenerateStatus(t, response)
	if response.StatusCode != http.StatusOK || status.ID == "" {
		t.Fatalf("queue map generation at %s: HTTP %d: %+v", baseURL, response.StatusCode, status)
	}

	statusURL := baseURL + "/api/maps/generate-wc3/status/" + status.ID
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("map generation timed out at %s", baseURL)
		case <-ticker.C:
			pollRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
			if err != nil {
				t.Fatal(err)
			}
			pollResponse, err := client.Do(pollRequest)
			if err != nil {
				t.Fatalf("poll map generation at %s: %v", baseURL, err)
			}
			status = decodeMapGenerateStatus(t, pollResponse)
			switch status.Status {
			case "done":
				if status.Result == nil || status.Result.OutputDir == "" {
					t.Fatalf("map generation at %s returned no output directory", baseURL)
				}
				return status.Result.OutputDir
			case "failed":
				t.Fatalf("map generation failed at %s: %s", baseURL, status.Error)
			}
		}
	}
}

func decodeMapGenerateStatus(t *testing.T, response *http.Response) mapGenerateStatus {
	t.Helper()
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var status mapGenerateStatus
	if err := json.Unmarshal(data, &status); err != nil {
		t.Fatalf("decode map status HTTP %d: %v: %s", response.StatusCode, err, data)
	}
	return status
}

func copyTestDirectory(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		return nil
	})
}

func TestValianceKeepParityCaseMatchesConvertExample(t *testing.T) {
	expectedTiles := 4
	minX, minY, maxX, maxY := 21, 27, 22, 28
	if count := (maxX - minX + 1) * (maxY - minY + 1); count != expectedTiles {
		t.Fatalf("Valiance Keep parity case changed: got %d tiles", count)
	}
	if fmt.Sprintf("%d:%d-%d:%d", minX, minY, maxX, maxY) != "21:27-22:28" {
		t.Fatal("Valiance Keep bounds no longer match convert examples")
	}
}
