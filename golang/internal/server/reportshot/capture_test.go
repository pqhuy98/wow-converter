package reportshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSixViewSheetDimensionsAndBlankDetection(t *testing.T) {
	makeFrame := func(c color.Color) []byte {
		img := image.NewRGBA(image.Rect(0, 0, 1440, 900))
		draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
		var data bytes.Buffer
		if err := png.Encode(&data, img); err != nil {
			t.Fatal(err)
		}
		return data.Bytes()
	}
	frame := makeFrame(color.RGBA{R: 200, G: 100, B: 60, A: 255})
	sheet, err := makeSheet([][]byte{frame, frame, frame, frame, frame, frame})
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(sheet))
	if err != nil || cfg.Width != 1920 || cfg.Height != 800 {
		t.Fatalf("sheet: %v %+v", err, cfg)
	}
	blank := makeFrame(color.RGBA{R: 38, G: 38, B: 38, A: 255})
	if _, err = makeSheet([][]byte{blank, blank, blank, blank, blank, blank}); err == nil {
		t.Fatal("blank screenshots accepted")
	}
	if _, err = makeSheet([][]byte{frame}); err == nil {
		t.Fatal("missing views accepted")
	}
	labeled, err := composeSheet([][]byte{frame, frame, frame, frame, frame, frame}, true)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := composeSheet([][]byte{frame, frame, frame, frame, frame, frame}, false)
	if err != nil {
		t.Fatal(err)
	}
	labeledImg, err := png.Decode(bytes.NewReader(labeled))
	if err != nil {
		t.Fatal(err)
	}
	plainImg, err := png.Decode(bytes.NewReader(plain))
	if err != nil {
		t.Fatal(err)
	}
	if !hasWhiteCaption(labeledImg) {
		t.Fatal("labeled sheet missing caption")
	}
	if hasWhiteCaption(plainImg) {
		t.Fatal("unlabeled sheet still captioned")
	}
}

func hasWhiteCaption(img image.Image) bool {
	for y := 0; y < 26; y++ {
		for x := 0; x < 64; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r > 60000 && g > 60000 && b > 60000 {
				return true
			}
		}
	}
	return false
}

// Opt-in real-browser check: local export plus Wowhead, without submitting to production.
func TestLocalLichKingCapture(t *testing.T) {
	base := os.Getenv("WOW_CONVERTER_URL")
	if base == "" {
		base = "http://127.0.0.1:3001"
	}
	resp, err := httpGetOK(base + "/api/get-config")
	if err != nil {
		t.Skip(err)
	}
	resp.Body.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	exportStarted := time.Now()
	request, err := exportLichKing(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("fresh export %s: %s / %s", time.Since(exportStarted).Round(time.Millisecond), request.Model, request.Sequence)
	started := time.Now()
	files, err := Capture(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(os.TempDir(), "wow-report-shot")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"output-wowhead.png", "output-converter.png"} {
		path := filepath.Join(out, name)
		if writeErr := os.WriteFile(path, files[i], 0644); writeErr != nil {
			path = filepath.Join(out, []string{"output-wowhead-next.png", "output-converter-next.png"}[i])
			if err = os.WriteFile(path, files[i], 0644); err != nil {
				t.Fatal(err)
			}
		}
		t.Log(path)
	}
	t.Logf("capture %s", time.Since(started).Round(time.Millisecond))
	logSheetTiles(t, "wowhead", files[0])
	logSheetTiles(t, "converter", files[1])
	assertSheetMasksAlign(t, out, files[0], files[1])
}

func exportLichKing(ctx context.Context, base string) (Request, error) {
	request := Request{BaseURL: base, WowheadURL: "https://www.wowhead.com/wotlk/npc=36597/the-lich-king"}
	// Match the accepted model's settings. Every run submits a new export job.
	payload := `{"character":{"base":{"type":"wowhead","value":"` + request.WowheadURL + `"},"attackTag":"Auto","size":"hero","inGameMovespeed":270,"portraitCameraSequenceName":"Stand"},"outputFileName":"the-lich-king","optimization":{},"format":"mdx","formatVersion":"1000","isBrowse":false}`
	var job struct {
		ID, Status, Error string
		Result            struct {
			ExportedModels []struct{ Path string }
			ReportMetadata struct {
				Models []struct {
					Path       string
					ModelScale float64
					Sequences  []struct {
						Name, WowName string
						WowVariant    int
					}
				}
			}
		}
	}
	readJob := func(method, path, body string) error {
		req, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Lich King export %s: %s", path, resp.Status)
		}
		return json.NewDecoder(resp.Body).Decode(&job)
	}
	if err := readJob("POST", "/api/export/character", payload); err != nil {
		return request, err
	}
	if job.ID == "" {
		return request, fmt.Errorf("Lich King export returned no job ID")
	}
	if err := waitFor(ctx, 6*time.Minute, func() (bool, error) {
		switch job.Status {
		case "done":
			return true, nil
		case "failed", "cancelled":
			return false, fmt.Errorf("Lich King export %s: %s", job.Status, job.Error)
		}
		return false, readJob("GET", "/api/export/character/status/"+job.ID, "")
	}); err != nil {
		return request, err
	}
	for _, asset := range job.Result.ExportedModels {
		for _, model := range job.Result.ReportMetadata.Models {
			if asset.Path != model.Path {
				continue
			}
			for _, seq := range model.Sequences {
				if seq.WowName == "Stand" && seq.WowVariant == 0 {
					request.Model, request.Sequence = asset.Path, seq.Name
					request.ModelScale = model.ModelScale
					request.WowSequence, request.WowVariant = seq.WowName, seq.WowVariant
					return request, nil
				}
			}
		}
	}
	return request, fmt.Errorf("fresh Lich King export has no model with a mapped Stand animation")
}

func TestLichKingCaptureAlwaysExportsAndUsesReturnedMetadata(t *testing.T) {
	posts := make(chan struct{}, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && r.URL.Path == "/api/export/character" {
			posts <- struct{}{}
			fmt.Fprint(w, `{"id":"fresh","status":"pending"}`)
			return
		}
		if r.URL.Path != "/api/export/character/status/fresh" {
			t.Errorf("unexpected export request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"id":"fresh","status":"done","result":{"exportedModels":[{"path":"fresh-lich-king.mdx"}],"reportMetadata":{"models":[{"path":"fresh-lich-king.mdx","modelScale":32.5,"sequences":[{"name":"Stand 7","wowName":"Stand","wowVariant":0}]}]}}}`)
	}))
	defer server.Close()
	for i := 0; i < 2; i++ {
		request, err := exportLichKing(context.Background(), server.URL)
		if err != nil || request.Model != "fresh-lich-king.mdx" || request.Sequence != "Stand 7" || request.WowSequence != "Stand" || request.ModelScale != 32.5 {
			t.Fatalf("fresh request: %+v %v", request, err)
		}
	}
	if len(posts) != 2 {
		t.Fatalf("exported %d times, want 2", len(posts))
	}
}

func logSheetTiles(t *testing.T, name string, sheet []byte) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(sheet))
	if err != nil {
		t.Fatal(err)
	}
	for i, view := range views {
		tile := image.NewRGBA(image.Rect(0, 0, 640, 400))
		draw.Draw(tile, tile.Bounds(), img, image.Pt((i%3)*640, (i/3)*400), draw.Src)
		// Sheet captions are foreground too; exclude them from model bounds.
		draw.Draw(tile, image.Rect(0, 0, 64, 26), image.NewUniform(color.RGBA{38, 38, 38, 255}), image.Point{}, draw.Src)
		var data bytes.Buffer
		if err = png.Encode(&data, tile); err != nil {
			t.Fatal(err)
		}
		aim, err := measureFrame(data.Bytes())
		if err != nil {
			t.Logf("%s %s: %v", name, view, err)
			continue
		}
		t.Logf("%s %s cx=%.3f cy=%.3f %.3fx%.3f", name, view, aim.CX, aim.CY, aim.W, aim.H)
	}
}

func TestMeasureSavedLichKingSheets(t *testing.T) {
	out := filepath.Join(os.TempDir(), "wow-report-shot")
	var sheets [2][]byte
	for i, name := range []string{"output-wowhead.png", "output-converter.png"} {
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Skip(err)
		}
		sheets[i] = data
		logSheetTiles(t, name, data)
	}
	assertSheetMasksAlign(t, out, sheets[0], sheets[1])
}

func TestForegroundMaskMismatch(t *testing.T) {
	bg := image.NewRGBA(image.Rect(0, 0, 64, 40))
	draw.Draw(bg, bg.Bounds(), image.NewUniform(color.RGBA{38, 38, 38, 255}), image.Point{}, draw.Src)
	same := tileMask(bg, image.Point{}, 64, 40)
	if maskMismatch(same, same) != 0 {
		t.Fatal("identical masks differ")
	}
	fg := image.NewRGBA(image.Rect(0, 0, 64, 40))
	draw.Draw(fg, fg.Bounds(), image.NewUniform(color.RGBA{38, 38, 38, 255}), image.Point{}, draw.Src)
	draw.Draw(fg, image.Rect(10, 10, 30, 30), image.NewUniform(color.RGBA{200, 80, 40, 255}), image.Point{}, draw.Src)
	if maskMismatch(same, tileMask(fg, image.Point{}, 64, 40)) <= 0 {
		t.Fatal("foreground change not detected")
	}
}

func assertSheetMasksAlign(t *testing.T, dir string, wowheadSheet, converterSheet []byte) {
	t.Helper()
	wowhead, err := png.Decode(bytes.NewReader(wowheadSheet))
	if err != nil {
		t.Fatal(err)
	}
	converter, err := png.Decode(bytes.NewReader(converterSheet))
	if err != nil {
		t.Fatal(err)
	}
	wowMask := image.NewGray(image.Rect(0, 0, 1920, 800))
	convMask := image.NewGray(image.Rect(0, 0, 1920, 800))
	diffImg := image.NewGray(image.Rect(0, 0, 1920, 800))
	diffs := make([]float64, len(views))
	for i, view := range views {
		origin := image.Pt((i%3)*640, (i/3)*400)
		a := tileMask(wowhead, origin, 640, 400)
		b := tileMask(converter, origin, 640, 400)
		diffs[i] = maskMismatch(a, b)
		for y := 0; y < 400; y++ {
			for x := 0; x < 640; x++ {
				p := (origin.Y+y)*1920 + origin.X + x
				wowMask.Pix[p] = a[y*640+x] * 255
				convMask.Pix[p] = b[y*640+x] * 255
				if a[y*640+x] != b[y*640+x] {
					diffImg.Pix[p] = 255
				}
			}
		}
		t.Logf("mask %s mismatch=%.3f", view, diffs[i])
	}
	for _, item := range []struct {
		name string
		img  image.Image
	}{{"mask-wowhead.png", wowMask}, {"mask-converter.png", convMask}, {"mask-diff.png", diffImg}} {
		raw, err := encodePNG(item.img)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, item.name), raw, 0644); err != nil {
			t.Log(err)
		}
	}
	// Allow texture/shading differences, while detecting a camera shift in any tile.
	for i, diff := range diffs {
		if diff > 0.03 {
			t.Errorf("%s mask mismatch %.3f > 0.03", views[i], diff)
		}
	}
}

func httpGetOK(url string) (*http.Response, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, fmt.Errorf("viewer %s", resp.Status)
	}
	return resp, nil
}

func TestLiveComparisonCapture(t *testing.T) {
	model := os.Getenv("REPORT_SHOT_MODEL")
	if model == "" {
		t.Skip("Set REPORT_SHOT_MODEL to an exported MDX for the live comparison check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
	defer cancel()
	sequence := os.Getenv("REPORT_SHOT_SEQUENCE")
	if sequence == "" {
		sequence = "Stand 1"
	}
	wowheadURL := os.Getenv("REPORT_SHOT_WOWHEAD_URL")
	if wowheadURL == "" {
		wowheadURL = "https://www.wowhead.com/npc=36597/the-lich-king"
	}
	variant, _ := strconv.Atoi(os.Getenv("REPORT_SHOT_VARIANT"))
	request := Request{BaseURL: "http://127.0.0.1:3001", Model: model, Sequence: sequence, WowheadURL: wowheadURL, WowSequence: "Stand", WowVariant: variant}
	var files [2][]byte
	var err error
	if os.Getenv("REPORT_SHOT_WOWHEAD_ONLY") == "1" {
		b, failure := openBrowser(ctx)
		if failure != nil {
			t.Fatal(failure)
		}
		defer b.close()
		err = prepareViewport(ctx, b)
		if err == nil {
			files[0], err = captureWowhead(ctx, b, request)
		}
	} else {
		files, err = Capture(ctx, request)
	}
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join("..", "..", "..", "..", "docs", "screenshots", "bug-reports")
	if err = os.MkdirAll(out, 0755); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"wowhead.png", "converter.png"} {
		if files[i] == nil {
			continue
		}
		if err = os.WriteFile(filepath.Join(out, name), files[i], 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConverterSilhouetteAimMatchesSkillMeasurement(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1440, 900))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{38, 38, 38, 255}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(200, 100, 1000, 700), image.NewUniform(color.RGBA{180, 80, 40, 255}), image.Point{}, draw.Src)
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	aim, err := measureFrame(data.Bytes())
	if err != nil || aim.CX != float64(1198)/2880 || aim.CY != float64(798)/1800 || aim.W != float64(798)/1440 || aim.H != float64(598)/900 {
		t.Fatalf("aim %+v: %v", aim, err)
	}
}
