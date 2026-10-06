//go:build integration_tests

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Reuses bun dev, including its loaded CASC data. Never starts another server.
func TestAmaniHubStillTextureBake(t *testing.T) {
	exportWMOStillBake(t, `world\wmo\expansion11\troll\12tr_amani_hub03`, "12tr_amani_hub03-wmo-fixed")
}

func TestVoidPylonStillTextureBake(t *testing.T) {
	exportWMOStillBake(t, `world\wmo\expansion11\void\12vd_void_pylon01`, "12vd_void_pylon01-wmo-fixed")
}

func TestAmaniEagleTempleStillTextureBake(t *testing.T) {
	exportWMOStillBake(t, `world\wmo\expansion11\troll\12tr_amani_eagletemple01`, "12tr_amani_eagletemple01-wmo-fixed")
}

func exportWMOStillBake(t *testing.T, source, output string) {
	t.Helper()
	output += os.Getenv("TEXTURE_BAKE_TEST_SUFFIX")
	base := strings.TrimRight(os.Getenv("WOW_CONVERTER_URL"), "/")
	if base == "" {
		base = "http://127.0.0.1:3001"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 10 * time.Second}
	readJSON := func(method, path string, body []byte, target any) error {
		req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := client.Do(req)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			detail, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
			return fmt.Errorf("%s %s: HTTP %d: %s", method, path, response.StatusCode, detail)
		}
		return json.NewDecoder(response.Body).Decode(target)
	}
	request := []byte(fmt.Sprintf(`{
		"textureBaking":{"enabled":true,"animate":false},
		"character":{"base":{"type":"local","value":%q},"attackTag":"Auto","inGameMovespeed":270,"size":"hero","portraitCameraSequenceName":"Stand"},
		"outputFileName":%q,
		"optimization":{"sortSequences":true,"allMaterialsUnshaded":false,"removeUnusedVertices":true,"removeUnusedNodes":true,"removeUnusedMaterialsTextures":true},
		"format":"mdl","formatVersion":"1000","isBrowse":false,"skinId":""
	}`, source, output))
	var queued struct{ ID string }
	if err := readJSON(http.MethodPost, "/api/export/character", request, &queued); err != nil {
		t.Fatal(err)
	}
	if queued.ID == "" {
		t.Fatal("export job ID missing")
	}
	t.Logf("still-bake export job %s", queued.ID)
	var status struct {
		Status string
		Error  json.RawMessage
		Result struct {
			ExportedModels []struct {
				Path string
				Size int64
			}
			ExportedTextures []struct {
				Path string
				Size int64
			}
			ModelStats struct{ Faces, TextureAnims int }
		}
	}
	started := time.Now()
	var peakHeap, peakInuse, peakSys int64
	for {
		var memory struct {
			Process struct{ HeapAlloc, HeapInuse, Sys int64 }
		}
		if err := readJSON(http.MethodGet, "/api/debugMemory", nil, &memory); err == nil {
			peakHeap = max(peakHeap, memory.Process.HeapAlloc)
			peakInuse = max(peakInuse, memory.Process.HeapInuse)
			peakSys = max(peakSys, memory.Process.Sys)
		}
		if err := readJSON(http.MethodGet, "/api/export/character/status/"+queued.ID, nil, &status); err != nil {
			t.Fatal(err)
		}
		if status.Status == "done" {
			break
		}
		if status.Status == "failed" || status.Status == "cancelled" {
			t.Fatalf("export %s: %s", status.Status, status.Error)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
	}
	if len(status.Result.ExportedModels) != 1 || status.Result.ExportedModels[0].Size <= 0 || status.Result.ModelStats.Faces == 0 {
		t.Fatal("export produced no model geometry")
	}
	baked, textureBytes := 0, int64(0)
	for _, texture := range status.Result.ExportedTextures {
		textureBytes += texture.Size
		if strings.Contains(texture.Path, "baked/uv2/") {
			baked++
		}
	}
	if baked == 0 || status.Result.ModelStats.TextureAnims != 0 {
		t.Fatalf("expected still baked textures without flipbooks, got %d baked textures / %d texture animations", baked, status.Result.ModelStats.TextureAnims)
	}
	t.Logf("export %s: %d faces, %d baked textures, model %d bytes, textures %d bytes; sampled server peak heap %.1f MiB / in-use %.1f MiB / sys %.1f MiB (includes CASC)", time.Since(started), status.Result.ModelStats.Faces, baked, status.Result.ExportedModels[0].Size, textureBytes, float64(peakHeap)/1048576, float64(peakInuse)/1048576, float64(peakSys)/1048576)
	if out := os.Getenv("TEXTURE_BAKE_TEST_OUTPUT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, "export-result.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}
