package reportshot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// CaptureTerrainBenchmark renders the generated HTML with the installed browser,
// scores identical cameras and saves both canvas images and a UI screenshot.
func CaptureTerrainBenchmark(ctx context.Context, out, onlyRegion string) error {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return e
	}
	srv := &http.Server{Handler: http.FileServer(http.Dir(filepath.Dir(out))), ReadHeaderTimeout: 10 * time.Second}
	defer srv.Close()
	go srv.Serve(listener)
	b, e := openBrowser(ctx)
	if e != nil {
		return e
	}
	defer b.close()
	if e = prepareViewport(ctx, b); e != nil {
		return e
	}
	if _, e = b.call(ctx, "Page.navigate", map[string]string{"url": "http://" + listener.Addr().String() + "/" + filepath.Base(out) + "/"}); e != nil {
		return e
	}
	readyDeadline := time.Now().Add(60 * time.Second)
	for {
		var ready bool
		var failure string
		if e = b.evaluate(ctx, "!!window.terrainReady", &ready); e != nil {
			return e
		}
		if ready {
			break
		}
		if e = b.evaluate(ctx, "window.terrainError||''", &failure); e != nil {
			return e
		}
		if failure != "" {
			return fmt.Errorf("terrain viewer: %s", failure)
		}
		if time.Now().After(readyDeadline) {
			return fmt.Errorf("terrain viewer did not become ready")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	frameDir := filepath.Join(out, "frames")
	if e = os.MkdirAll(frameDir, 0755); e != nil {
		return e
	}
	var regions []struct {
		Name   string `json:"name"`
		Stages []struct {
			Name string `json:"name"`
		} `json:"stages"`
	}
	data, e := os.ReadFile(filepath.Join(out, "report.json"))
	if e != nil {
		return e
	}
	var report struct {
		Regions json.RawMessage `json:"regions"`
	}
	if e = json.Unmarshal(data, &report); e != nil {
		return e
	}
	if e = json.Unmarshal(report.Regions, &regions); e != nil {
		return e
	}
	firstRegion := 0
	if onlyRegion != "" {
		firstRegion = -1
		for i, region := range regions {
			if region.Name == onlyRegion {
				firstRegion = i
			}
		}
		if firstRegion < 0 {
			return fmt.Errorf("unknown capture region: %s", onlyRegion)
		}
	}
	records := []map[string]any{}
	checks := []map[string]any{}
	for i, region := range regions {
		if onlyRegion != "" && i != firstRegion {
			continue
		}
		stage := region.Stages[len(region.Stages)-1].Name
		stageJSON, _ := json.Marshal(stage)
		if e = evaluateTerrainPromise(ctx, b, fmt.Sprintf("window.terrainBenchmark.select(%d,%s)", i, stageJSON), nil); e != nil {
			return e
		}
		var check json.RawMessage
		if e = evaluateTerrainPromise(ctx, b, "window.terrainBenchmark.validateNative()", &check); e != nil {
			return e
		}
		checks = append(checks, map[string]any{"region": region.Name, "check": check})
		fmt.Println("NATIVE CPU/GPU CHECK", region.Name, string(check))
		for _, view := range []string{"top", "oblique", "lit", "close"} {
			fmt.Println("CAPTURE CAMERA", region.Name, view)
			stage := region.Stages[len(region.Stages)-1].Name
			stageJSON, _ := json.Marshal(stage)
			viewJSON, _ := json.Marshal(view)
			var capture struct {
				Score json.RawMessage `json:"score"`
				Wow   string          `json:"wow"`
				WC3   string          `json:"wc3"`
			}
			script := fmt.Sprintf("window.terrainBenchmark.capture(%d,%s,%s)", i, stageJSON, viewJSON)
			if e = evaluateTerrainPromise(ctx, b, script, &capture); e != nil {
				return e
			}
			for name, image := range map[string]string{"wow": capture.Wow, "wc3": capture.WC3} {
				comma := strings.IndexByte(image, ',')
				if comma < 0 {
					return fmt.Errorf("missing canvas image")
				}
				png, e := base64.StdEncoding.DecodeString(image[comma+1:])
				if e != nil {
					return e
				}
				if e = os.WriteFile(filepath.Join(frameDir, region.Name+"-"+view+"-"+name+".png"), png, 0644); e != nil {
					return e
				}
			}
			records = append(records, map[string]any{"region": region.Name, "view": view, "stage": stage, "score": capture.Score})
		}
	}
	// Exercise the full-8K source path as well as the lighter region overview.
	stageJSON, _ := json.Marshal(regions[firstRegion].Stages[len(regions[firstRegion].Stages)-1].Name)
	if e = evaluateTerrainPromise(ctx, b, fmt.Sprintf("window.terrainBenchmark.select(%d,%s,'0')", firstRegion, stageJSON), nil); e != nil {
		return e
	}
	var fullCheck json.RawMessage
	if e = evaluateTerrainPromise(ctx, b, "window.terrainBenchmark.validateNative()", &fullCheck); e != nil {
		return e
	}
	checks = append(checks, map[string]any{"region": regions[firstRegion].Name, "focus": "8K ADT", "check": fullCheck})
	fmt.Println("FULL 8K TILE CHECK", string(fullCheck))
	// Exercise the actual keyboard handler and verify the full camera state survives.
	var before, after struct {
		Mode   string          `json:"mode"`
		Camera json.RawMessage `json:"camera"`
		Matrix []float64       `json:"matrix"`
	}
	if e = b.evaluate(ctx, "window.terrainBenchmark.state()", &before); e != nil {
		return e
	}
	if _, e = b.call(ctx, "Input.dispatchKeyEvent", map[string]string{"type": "keyDown", "key": "ArrowRight", "code": "ArrowRight"}); e != nil {
		return e
	}
	if e = b.evaluate(ctx, "window.terrainBenchmark.state()", &after); e != nil {
		return e
	}
	if after.Mode != "wc3" || !reflect.DeepEqual(before.Camera, after.Camera) || !reflect.DeepEqual(before.Matrix, after.Matrix) {
		return fmt.Errorf("arrow toggle changed the camera or failed to switch mesh")
	}
	if _, e = b.call(ctx, "Input.dispatchKeyEvent", map[string]string{"type": "keyDown", "key": "ArrowLeft", "code": "ArrowLeft"}); e != nil {
		return e
	}
	if e = b.evaluate(ctx, "window.terrainBenchmark.state()", &after); e != nil {
		return e
	}
	if after.Mode != "wow" {
		return fmt.Errorf("left arrow did not select reference")
	}
	encoded, e := json.MarshalIndent(records, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(out, "views.json"), encoded, 0644); e != nil {
		return e
	}
	encoded, e = json.MarshalIndent(checks, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(out, "native-checks.json"), encoded, 0644); e != nil {
		return e
	}
	if e = evaluateTerrainPromise(ctx, b, fmt.Sprintf("window.terrainBenchmark.select(%d)", firstRegion), nil); e != nil {
		return e
	}
	if e = b.evaluate(ctx, "document.getElementById('wc3').click()", nil); e != nil {
		return e
	}
	raw, e := b.call(ctx, "Page.captureScreenshot", map[string]string{"format": "png"})
	if e != nil {
		return e
	}
	var shot struct {
		Data string `json:"data"`
	}
	if e = json.Unmarshal(raw, &shot); e != nil {
		return e
	}
	png, e := base64.StdEncoding.DecodeString(shot.Data)
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(out, "viewer.png"), png, 0644)
}

func evaluateTerrainPromise(ctx context.Context, b *browser, expression string, result any) error {
	script := `(()=>{window.__terrainResult=undefined;window.__terrainFailure='';Promise.resolve().then(()=>` + expression + `).then(v=>window.__terrainResult=v).catch(e=>window.__terrainFailure=String(e));return true})()`
	if e := b.evaluate(ctx, script, nil); e != nil {
		return e
	}
	deadline := time.Now().Add(60 * time.Second)
	for {
		var status struct {
			Ready   bool   `json:"ready"`
			Failure string `json:"failure"`
		}
		if e := b.evaluate(ctx, `({ready:window.__terrainResult!==undefined,failure:window.__terrainFailure})`, &status); e != nil {
			return e
		}
		if status.Failure != "" {
			return fmt.Errorf("terrain capture: %s", status.Failure)
		}
		if status.Ready {
			return b.evaluate(ctx, "window.__terrainResult", result)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("terrain capture timed out")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}
