package reportshot

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestShotArgsPreserveSingleAndMultipleViews(t *testing.T) {
	for _, tc := range []struct {
		source string
		args   []string
		want   []int
	}{
		{"converter", []string{`exported-assets\lich-king.mdx`, "--view", "front", "--seq", "Stand"}, []int{0}},
		{"wowhead", []string{"https://www.wowhead.com/wotlk/npc=36597/the-lich-king", "--seq", "Stand", "--view", "bottom,back"}, []int{5, 2}},
		{"converter", []string{"lich-king.mdx"}, []int{0, 1, 2, 3, 4, 5}},
	} {
		opts, err := parseShotArgs(tc.source, tc.args, io.Discard)
		if err != nil || !reflect.DeepEqual(opts.indices, tc.want) {
			t.Fatalf("%v: indices %v, error %v", tc.args, opts.indices, err)
		}
	}
	for _, view := range []string{"unknown", "front,front"} {
		if _, err := parseShotArgs("converter", []string{"x.mdx", "--view", view}, io.Discard); err == nil {
			t.Fatalf("accepted invalid views %q", view)
		}
	}
	if got := assetPath(`C:\models\exported-assets\folder\king.mdx`); got != "folder/king.mdx" {
		t.Fatalf("asset path %q", got)
	}
	if got := groupViews([]int{5, 0}); !reflect.DeepEqual(got, [][]int{{0}, {5}}) {
		t.Fatalf("capture groups %v", got)
	}
	sheet, err := parseShotArgs("wowhead", []string{"https://www.wowhead.com/npc=1", "--view", "front", "--sheet", "tmp/x.wowhead.png"}, io.Discard)
	if err != nil || !reflect.DeepEqual(sheet.indices, []int{0, 1, 2, 3, 4, 5}) {
		t.Fatalf("sheet views %v, error %v", sheet.indices, err)
	}
	if !strings.HasSuffix(sheet.sheet, "x.wowhead.png") {
		t.Fatalf("sheet path %q", sheet.sheet)
	}
	aimed, err := parseShotArgs("wowhead", []string{"https://www.wowhead.com/npc=1", "--aim-sheet", "tmp/x.expected.png", "--sheet", "tmp/x.wowhead.png", "--cameras", "tmp/x.cameras.json"}, io.Discard)
	if err != nil || !strings.HasSuffix(aimed.aimSheet, "x.expected.png") || !strings.HasSuffix(aimed.cameras, "x.cameras.json") {
		t.Fatalf("aim-sheet %q cameras %q error %v", aimed.aimSheet, aimed.cameras, err)
	}
	converter := [2][][]byte{{[]byte("wowhead")}, {[]byte("converter")}}
	if got := pickSheetFrames("converter", converter); len(got) != 1 || string(got[0]) != "converter" {
		t.Fatalf("converter sheet frames %v", got)
	}
	if got := pickSheetFrames("wowhead", converter); len(got) != 1 || string(got[0]) != "wowhead" {
		t.Fatalf("wowhead sheet frames %v", got)
	}
}

func TestShotArgsModelScale(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want float64
	}{
		{name: "default", args: []string{"model.mdx"}, want: 0},
		{name: "explicit", args: []string{"model.mdx", "--model-scale", "84.5"}, want: 84.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseShotArgs("converter", tc.args, io.Discard)
			if err != nil || opts.request.ModelScale != tc.want {
				t.Fatalf("model scale %v, error %v; want %v", opts.request.ModelScale, err, tc.want)
			}
		})
	}
	for _, value := range []string{"-1", "NaN", "+Inf", "-Inf"} {
		t.Run(value, func(t *testing.T) {
			if _, err := parseShotArgs("converter", []string{"model.mdx", "--model-scale", value}, io.Discard); err == nil {
				t.Fatalf("accepted invalid model scale %q", value)
			}
		})
	}
}

// Uses a local page to prove that a CLI single-view shot does not shoot the other five.
func TestShotCLISingleViewUsesServerCaptureAndLabel(t *testing.T) {
	if os.Getenv("REPORT_SHOT_CLI_TEST") != "1" {
		t.Skip("Set REPORT_SHOT_CLI_TEST=1 to exercise the CLI with the installed browser")
	}
	captures := make(chan string, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/view" {
			captures <- r.URL.Query().Get("name")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, `<!doctype html><style>body{margin:0;background:rgb(38,38,38)}nextjs-portal{position:fixed;inset:0;background:lime}</style><canvas width="1440" height="900"></canvas><nextjs-portal></nextjs-portal><script>
document.documentElement.dataset.viewerReady='1';document.documentElement.dataset.viewerSequence='Stand 1';
window.__shotView=async name=>{const c=document.querySelector('canvas').getContext('2d');c.fillStyle='red';c.fillRect(400,200,400,500);await fetch('/view?name='+name);return name;};
</script>`)
	}))
	defer server.Close()
	out := t.TempDir()
	if err := runCLI("converter", []string{"fake.mdx", "--seq", "Stand", "--view", "front", "--base", server.URL, "--out", out}); err != nil {
		t.Fatal(err)
	}
	if len(captures) != 1 || <-captures != "front" {
		t.Fatal("single-view capture shot an unexpected view")
	}
	files, err := os.ReadDir(out)
	if err != nil || len(files) != 1 || files[0].Name() != "fake-Stand 1-front.png" {
		t.Fatalf("output %v, error %v", files, err)
	}
	data, err := os.ReadFile(filepath.Join(out, files[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds() != image.Rect(0, 0, 1440, 900) {
		t.Fatalf("invalid shot: %v", err)
	}
	r, g, b, _ := img.At(500, 300).RGBA()
	if r != 65535 || g != 0 || b != 0 {
		t.Fatal("development overlay covered the model")
	}
	label := tileMask(img, image.Point{}, 64, 26)
	if !strings.Contains(string(label), string([]byte{1})) {
		t.Fatal("single-view shot has no label")
	}
}

func TestShotCamerasJSONRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cameras.json")
	want := cameraReference{Target: [3]float64{1, 2, 3}, Eye: [3]float64{4, 5, 6}, Up: [3]float64{0, 0, 1}, Height: 7, ModelScale: 56}
	if err := writeShotCameras(path, []int{0}, []cameraReference{want}); err != nil {
		t.Fatal(err)
	}
	opts := shotOptions{cameras: path, indices: []int{0}}
	if err := loadShotCameras(&opts); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-opts.request.cameras[0]:
		if got != want {
			t.Fatalf("camera %+v", got)
		}
	default:
		t.Fatal("missing loaded camera")
	}
}

func TestShotCamerasJSONUsesRequestedModelScale(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cameras.json")
	camera := cameraReference{Target: [3]float64{1, 2, 3}, Eye: [3]float64{4, 5, 6}, Up: [3]float64{0, 0, 1}, Height: 7, ModelScale: 56}
	if err := writeShotCameras(path, []int{0}, []cameraReference{camera}); err != nil {
		t.Fatal(err)
	}
	opts, err := parseShotArgs("converter", []string{"model.mdx", "--view", "front", "--model-scale", "84", "--cameras", path}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if err := loadShotCameras(&opts); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-opts.request.cameras[0]:
		if got.ModelScale != 84 {
			t.Fatalf("camera model scale %v, want CLI override 84", got.ModelScale)
		}
	default:
		t.Fatal("missing loaded camera")
	}
}

func TestLoadAimsFromSheetUsesSideTiles(t *testing.T) {
	sheet := image.NewRGBA(image.Rect(0, 0, 1920, 800))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.RGBA{38, 38, 38, 255}), image.Point{}, draw.Src)
	draw.Draw(sheet, image.Rect(200, 100, 400, 300), image.NewUniform(color.RGBA{255, 0, 0, 255}), image.Point{}, draw.Src)
	path := filepath.Join(t.TempDir(), "expected.png")
	data, err := encodePNG(sheet)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	aims, err := loadAimsFromSheet(path, []int{0, 4, 5})
	if err != nil {
		t.Fatal(err)
	}
	if aims[4] != nil || aims[5] != nil {
		t.Fatal("top/bottom must not consume sheet aims")
	}
	select {
	case aim := <-aims[0]:
		if aim.CX < 0.45 || aim.CX > 0.5 || aim.CY < 0.48 || aim.CY > 0.51 {
			t.Fatalf("front sheet aim %+v", aim)
		}
	default:
		t.Fatal("missing front sheet aim")
	}
}

func TestStandaloneWowheadMissingAimDoesNotWait(t *testing.T) {
	opts := shotOptions{request: Request{WowheadURL: "https://www.wowhead.com/npc=36597/the-lich-king", WowSequence: "Stand"}, indices: []int{0}, out: t.TempDir()}
	if err := loadConverterAims(&opts); err != nil || opts.request.aims[0] != nil {
		t.Fatalf("missing aim must remain nil: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 640, 400))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{38, 38, 38, 255}), image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(200, 100, 400, 300), image.NewUniform(color.RGBA{255, 0, 0, 255}), image.Point{}, draw.Src)
	data, _ := encodePNG(img)
	if err := os.WriteFile(filepath.Join(opts.out, "the-lich-king-Stand-01-front-converter.png"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := loadConverterAims(&opts); err != nil {
		t.Fatal(err)
	}
	select {
	case aim := <-opts.request.aims[0]:
		if aim.CX < 0.45 || aim.CX > 0.5 || aim.CY < 0.48 || aim.CY > 0.51 {
			t.Fatalf("legacy converter aim %+v", aim)
		}
	default:
		t.Fatal("missing legacy aim")
	}
}

func TestFreshLichKingExportFailureDoesNotReuseOldAssets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"id": "failed", "status": "failed", "error": "export failed"})
	}))
	defer server.Close()
	request, err := exportLichKing(context.Background(), server.URL)
	if err == nil || request.Model != "" {
		t.Fatalf("reused a model after failed export: %+v %v", request, err)
	}
}
