package reportshot

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	appconfig "github.com/pqhuy98/wow-converter/internal/config"

	pngtools "github.com/pqhuy98/wow-converter/internal/formats/png"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Embedded in both the desktop server and the screenshot commands.
//
//go:embed wowhead-init.js
var wowheadInit string

//go:embed wowhead-sample.js
var wowheadSample string

var views = []string{"front", "left", "back", "right", "top", "bottom"}

// Request identifies the exact output sequence and its source animation.
type Request struct {
	BaseURL        string
	Model          string
	Sequence       string
	WowheadURL     string
	WowSequence    string
	WowVariant     int
	ModelScale     float64
	aims           []chan frameAim
	cameras        []chan cameraReference
	sequencePrefix bool
}

type cameraReference struct {
	Target      [3]float64  `json:"target"`
	Eye         [3]float64  `json:"eye"`
	Up          [3]float64  `json:"up"`
	ModelMatrix [16]float64 `json:"modelMatrix"`
	Height      float64     `json:"height"`
	ModelScale  float64     `json:"modelScale,omitempty"`
}

type frameAim struct {
	CX float64 `json:"cx"`
	CY float64 `json:"cy"`
	W  float64 `json:"w"`
	H  float64 `json:"h"`
}

// Capture returns Wowhead first, converter second. Both sheets are 1920×800.
func Capture(ctx context.Context, request Request) ([2][]byte, error) {
	var sheets [2][]byte
	frames, _, err := captureFrames(ctx, request, []int{0, 1, 2, 3, 4, 5})
	if err != nil {
		return sheets, err
	}
	for i := range sheets {
		sheets[i], err = makeSheet(frames[i])
		if err != nil {
			return sheets, err
		}
	}
	return sheets, nil
}

func captureFrames(ctx context.Context, request Request, indices []int) ([2][][]byte, string, error) {
	var result [2][][]byte
	var sequence string
	groups := groupViews(indices)
	b, err := openBrowser(ctx)
	if err != nil {
		return result, sequence, err
	}
	defer b.close()
	if err = prepareViewport(ctx, b); err != nil {
		return result, sequence, err
	}
	if err = b.setCacheDisabled(ctx, true); err != nil {
		return result, sequence, err
	}
	wow0, err := b.newTab(ctx)
	if err != nil {
		return result, sequence, err
	}
	defer wow0.close()
	if err = prepareViewport(ctx, wow0); err != nil {
		return result, sequence, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	failures := make(chan error, 4)
	request.aims = make([]chan frameAim, len(views))
	request.cameras = make([]chan cameraReference, len(views))
	for i := range request.aims {
		request.aims[i] = make(chan frameAim, 1)
		request.cameras[i] = make(chan cameraReference, 1)
	}
	wowheadFrames := make([][]byte, len(views))
	storeWowhead := func(indices []int, frames [][]byte) {
		for j, index := range indices {
			wowheadFrames[index] = frames[j]
		}
	}
	loaded := make(chan struct{})
	wg.Add(2)
	go func() {
		defer wg.Done()
		frames, selected, err := captureConverter(ctx, b, request, indices)
		if err != nil {
			failures <- err
			cancel()
			return
		}
		result[1], sequence = frames, selected
	}()
	go func() {
		defer wg.Done()
		if err := prepareWowheadPage(ctx, wow0, request); err != nil {
			failures <- err
			cancel()
			return
		}
		close(loaded)
		frames, _, err := shootWowheadViews(ctx, wow0, request, groups[0])
		if err != nil {
			failures <- err
			cancel()
			return
		}
		storeWowhead(groups[0], frames)
	}()
	select {
	case <-loaded:
	case err := <-failures:
		cancel()
		wg.Wait()
		return result, sequence, err
	case <-ctx.Done():
		wg.Wait()
		select {
		case err := <-failures:
			return result, sequence, err
		default:
			return result, sequence, context.Cause(ctx)
		}
	}
	for _, indices := range groups[1:] {
		tab, err := b.newTab(ctx)
		if err != nil {
			cancel()
			wg.Wait()
			return result, sequence, err
		}
		defer tab.close()
		if err = prepareViewport(ctx, tab); err != nil {
			cancel()
			wg.Wait()
			return result, sequence, err
		}
		wg.Add(1)
		go func(tab *browser, indices []int) {
			defer wg.Done()
			if err := prepareWowheadPage(ctx, tab, request); err != nil {
				failures <- err
				cancel()
				return
			}
			frames, _, err := shootWowheadViews(ctx, tab, request, indices)
			if err != nil {
				failures <- err
				cancel()
				return
			}
			storeWowhead(indices, frames)
		}(tab, indices)
	}
	wg.Wait()
	select {
	case err := <-failures:
		return result, sequence, err
	default:
		for _, index := range indices {
			result[0] = append(result[0], wowheadFrames[index])
		}
		return result, sequence, nil
	}
}

// Keep the report's three Wowhead workers, skipping tabs for unrequested views.
func groupViews(indices []int) [][]int {
	var groups [][]int
	for worker := 0; worker < 3; worker++ {
		var group []int
		for _, index := range indices {
			if index%3 == worker {
				group = append(group, index)
			}
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
	}
	return groups
}

func captureConverter(ctx context.Context, b *browser, request Request, indices []int) ([][]byte, string, error) {
	var result [][]byte
	var sequence string
	viewer, err := url.Parse(request.BaseURL + "/viewer")
	if err != nil {
		return result, sequence, err
	}
	q := viewer.Query()
	q.Set("model", request.Model)
	q.Set("source", "export")
	q.Set("shot", "1")
	q.Set("seq", request.Sequence)
	viewer.RawQuery = q.Encode()
	if _, err = b.call(ctx, "Page.navigate", map[string]string{"url": viewer.String()}); err != nil {
		return result, sequence, err
	}
	err = waitFor(ctx, 60*time.Second, func() (bool, error) {
		var state struct {
			Ready    string `json:"ready"`
			Error    string `json:"error"`
			Sequence string `json:"sequence"`
		}
		err := b.evaluate(ctx, `({ready:document.documentElement?.dataset.viewerReady||'',error:document.documentElement?.dataset.viewerError||'',sequence:document.documentElement?.dataset.viewerSequence||''})`, &state)
		if err != nil {
			return false, err
		}
		if state.Error != "" {
			log.Printf("report screenshot: converter viewer error: %s", state.Error)
			return false, errors.New("The exported model could not be loaded for screenshots. Export it again and retry.")
		}
		if state.Ready == "1" && state.Sequence != request.Sequence && !(request.sequencePrefix && strings.HasPrefix(strings.ToLower(state.Sequence), strings.ToLower(request.Sequence))) {
			return false, errors.New("The selected animation no longer matches this export. Please export again.")
		}
		sequence = state.Sequence
		return state.Ready == "1", nil
	})
	if err != nil {
		return result, sequence, fmt.Errorf("Converter screenshot: %w", err)
	}
	_ = b.evaluate(ctx, `document.head.insertAdjacentHTML('beforeend','<style>nextjs-portal{display:none!important}</style>')`, nil)
	// Preserve Wowhead's existing side fit, then transfer its final cameras back.
	// These preliminary frames only supply aims; the sheet uses reference cameras.
	if request.cameras != nil && request.aims != nil {
		for _, index := range indices {
			if index >= 4 {
				continue
			}
			view := views[index]
			frame, err := shootConverterView(ctx, b, view, nil)
			if err != nil {
				return nil, sequence, err
			}
			aim, err := measureFrame(frame)
			if err != nil {
				return nil, sequence, err
			}
			select {
			case request.aims[index] <- aim:
			case <-ctx.Done():
				return nil, sequence, ctx.Err()
			}
		}
	}
	for _, index := range indices {
		view := views[index]
		var reference *cameraReference
		if request.cameras != nil {
			select {
			case camera := <-request.cameras[index]:
				reference = &camera
			case <-ctx.Done():
				return nil, sequence, ctx.Err()
			}
		}
		frame, err := shootConverterView(ctx, b, view, reference)
		if err != nil {
			return result, sequence, err
		}
		result = append(result, frame)
	}
	return result, sequence, nil
}

func shootConverterView(ctx context.Context, b *browser, view string, reference *cameraReference) ([]byte, error) {
	log.Printf("report screenshot: converter %s reference=%t", view, reference != nil)
	name, _ := json.Marshal(view)
	cameraJSON, _ := json.Marshal(reference)
	var switched string
	if err := b.evaluate(ctx, `window.__shotView && window.__shotView(`+string(name)+`,`+string(cameraJSON)+`)`, &switched); err != nil {
		return nil, err
	}
	if switched != view {
		return nil, errors.New("The converter viewer could not prepare every view. Please retake screenshots.")
	}
	if err := b.evaluate(ctx, `new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>r(true))))`, nil); err != nil {
		return nil, err
	}
	raw, err := b.call(ctx, "Page.captureScreenshot", map[string]string{"format": "png"})
	if err != nil {
		return nil, err
	}
	var shot struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(raw, &shot); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(shot.Data)
}

func captureWowhead(ctx context.Context, b *browser, request Request) ([]byte, error) {
	if err := prepareWowheadPage(ctx, b, request); err != nil {
		return nil, err
	}
	frames, _, err := shootWowheadViews(ctx, b, request, []int{0, 1, 2, 3, 4, 5})
	if err != nil {
		return nil, err
	}
	return makeSheet(frames)
}

func prepareViewport(ctx context.Context, b *browser) error {
	if _, err := b.call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false}); err != nil {
		return err
	}
	_, err := b.call(ctx, "Emulation.setFocusEmulationEnabled", map[string]bool{"enabled": true})
	return err
}

func prepareWowheadPage(ctx context.Context, b *browser, request Request) error {
	if _, err := b.call(ctx, "Page.addScriptToEvaluateOnNewDocument", map[string]string{"source": wowheadInit}); err != nil {
		return err
	}
	_, err := b.call(ctx, "Emulation.setUserAgentOverride", map[string]any{"userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36", "platform": "Win32",
		"userAgentMetadata": map[string]any{"brands": []map[string]string{{"brand": "Google Chrome", "version": "131"}, {"brand": "Chromium", "version": "131"}, {"brand": "Not_A Brand", "version": "24"}}, "fullVersion": "131.0.6778.205", "platform": "Windows", "platformVersion": "15.0.0", "architecture": "x86", "model": "", "mobile": false}})
	if err != nil {
		return err
	}
	page, err := url.Parse(request.WowheadURL)
	if err != nil {
		return err
	}
	if page.Fragment == "" {
		page.Fragment = "modelviewer"
	}
	if _, err = b.call(ctx, "Page.navigate", map[string]string{"url": page.String()}); err != nil {
		return err
	}
	if err = waitFor(ctx, 30*time.Second, func() (bool, error) {
		var loaded bool
		err := b.evaluate(ctx, `location.hostname.endsWith('wowhead.com') && document.readyState!=='loading'`, &loaded)
		return loaded, err
	}); err != nil {
		return fmt.Errorf("Wowhead page could not load: %w", err)
	}
	if err = waitFor(ctx, 60*time.Second, func() (bool, error) {
		if err := wowheadBlocked(ctx, b); err != nil {
			return false, err
		}
		var ready bool
		err := b.evaluate(ctx, `!!(window.__whViewer&&window.__whViewer.renderer)`, &ready)
		return ready && b.assetsReady(), err
	}); err != nil {
		return fmt.Errorf("Wowhead viewer could not load: %w", err)
	}
	return nil
}

func shootWowheadViews(ctx context.Context, b *browser, request Request, indices []int) ([][]byte, []cameraReference, error) {
	var wowheadFrames [][]byte
	var cameras []cameraReference
	for _, index := range indices {
		view := views[index]
		log.Printf("report screenshot: Wowhead %s", view)
		var aim *frameAim
		// Top/bottom keep Wowhead's fixed camera and send it to the converter.
		// Only side cameras refine against a converter silhouette.
		if index < 4 && request.aims != nil && request.aims[index] != nil {
			select {
			case measured := <-request.aims[index]:
				aim = &measured
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}
		}
		config, _ := json.Marshal(map[string]any{"view": view, "sequence": request.WowSequence, "variant": request.WowVariant, "aim": aim})
		expr := `(()=>{const c=` + string(config) + `;window.__whAim=c.aim;window.__whWantView=c.view;window.__whWantSeq=c.sequence;window.__whWantVariant=c.variant;window.__whPng='';window.__whShot='';window.__whPending=false;})()`
		if err := b.evaluate(ctx, expr, nil); err != nil {
			return nil, nil, err
		}
		var shot string
		type sample struct {
			Phase   string  `json:"phase"`
			Seq     int     `json:"seq"`
			Colored float64 `json:"colored"`
			BoxH    float64 `json:"boxH"`
			BoxW    float64 `json:"boxW"`
			FitN    int     `json:"fitN"`
		}
		var lastState sample
		err := waitFor(ctx, 60*time.Second, func() (bool, error) {
			if err := wowheadBlocked(ctx, b); err != nil {
				return false, err
			}
			var state sample
			if err := b.evaluate(ctx, wowheadSample, &state); err != nil {
				return false, err
			}
			lastState = state
			if state.Phase == "bad-anim" {
				return false, errors.New("Wowhead does not provide this animation variant. Choose another sequence and prepare screenshots again.")
			}
			if err := b.evaluate(ctx, `window.__whPng||''`, &shot); err != nil {
				return false, err
			}
			return shot != "", nil
		})
		if err != nil {
			log.Printf("report screenshot: Wowhead %s failed: phase=%s frame=%d fit=%d size=%.2fx%.2f pixels=%.0f", view, lastState.Phase, lastState.Seq, lastState.FitN, lastState.BoxW, lastState.BoxH, lastState.Colored)
			return nil, nil, fmt.Errorf("Wowhead screenshot: %w", err)
		}
		var reference cameraReference
		if err := b.evaluate(ctx, `(() => {
			const actor = window.__whViewer.renderer.actors[0];
			const classic = !!(actor.b && actor.b.aq);
			const model = classic ? actor.b : actor.a;
			const vertices = classic ? model.aq.l : model.bf.Q;
			const matrix = classic ? model.al : model.j;
			if (!vertices || !vertices.length || !matrix || matrix.length !== 16) throw new Error('Missing camera geometry');
			let minZ = Infinity, maxZ = -Infinity;
			for (const vertex of vertices) {
				const z = (classic ? vertex.f : vertex.a)[2];
				minZ = Math.min(minZ, z); maxZ = Math.max(maxZ, z);
			}
			const camera = window.__whPlaced;
			const eye = Array.from(window.__whViewer.renderer.eye);
			const up = camera.view === 'top' || camera.view === 'bottom' ? [0, 1, 0] : [0, 0, 1];
			return {target: camera.target, eye, up, modelMatrix: Array.from(matrix), height: maxZ - minZ};
		})()`, &reference); err != nil {
			return nil, nil, err
		}
		if reference.Height <= 0 || reference.Eye == reference.Target {
			return nil, nil, errors.New("Wowhead did not provide a valid reference camera.")
		}
		reference.ModelScale = request.ModelScale
		if reference.ModelScale == 0 {
			reference.ModelScale = appconfig.DefaultConfig().RawModelScaleUp
		}
		if request.cameras != nil {
			request.cameras[index] <- reference
		}
		var cam string
		_ = b.evaluate(ctx, `JSON.stringify({bind:window.__whBind,placed:window.__whPlaced,scale:window.__whScale||1})`, &cam)
		log.Printf("report screenshot: Wowhead %s ready: frames=%d fit=%d size=%.2fx%.2f cam=%s", view, lastState.Seq, lastState.FitN, lastState.BoxW, lastState.BoxH, cam)
		frame, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(shot, "data:image/png;base64,"))
		if err != nil {
			return nil, nil, err
		}
		wowheadFrames = append(wowheadFrames, frame)
		cameras = append(cameras, reference)
	}
	return wowheadFrames, cameras, nil
}

func wowheadBlocked(ctx context.Context, b *browser) error {
	var title string
	if err := b.evaluate(ctx, "document.title", &title); err != nil {
		return err
	}
	if strings.HasPrefix(title, "403") || strings.Contains(strings.ToLower(title), "request could not be satisfied") {
		return errors.New("Wowhead blocked screenshot capture. Please retake screenshots later.")
	}
	return nil
}

// Match the TypeScript skill's silhouette center/size measurements, before sheet labels/resizing.
func measureFrame(frame []byte) (frameAim, error) {
	img, err := png.Decode(bytes.NewReader(frame))
	if err != nil {
		return frameAim{}, err
	}
	bounds := img.Bounds()
	minX, minY, maxX, maxY := bounds.Max.X, bounds.Max.Y, -1, -1
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 2 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 2 {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 >= 32 && r>>8 <= 44 && g>>8 >= 32 && g>>8 <= 44 && b>>8 >= 32 && b>>8 <= 44 {
				continue
			}
			minX = min(minX, x)
			minY = min(minY, y)
			maxX = max(maxX, x)
			maxY = max(maxY, y)
		}
	}
	if maxY < 0 {
		return frameAim{}, errors.New("The converter screenshot appears blank. Please retake it.")
	}
	return frameAim{CX: float64(minX+maxX) / 2 / float64(bounds.Dx()), CY: float64(minY+maxY) / 2 / float64(bounds.Dy()), W: float64(maxX-minX) / float64(bounds.Dx()), H: float64(maxY-minY) / float64(bounds.Dy())}, nil
}

func waitFor(ctx context.Context, duration time.Duration, probe func() (bool, error)) error {
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	for {
		ready, err := probe()
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("The model viewer timed out. Please retake screenshots.")
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func isShotBackground(r, g, b uint32) bool {
	return r>>8 >= 32 && r>>8 <= 44 && g>>8 >= 32 && g>>8 <= 44 && b>>8 >= 32 && b>>8 <= 44
}

func tileMask(img image.Image, origin image.Point, width, height int) []uint8 {
	mask := make([]uint8, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(origin.X+x, origin.Y+y).RGBA()
			if !isShotBackground(r, g, b) {
				mask[y*width+x] = 1
			}
		}
	}
	return mask
}

func maskMismatch(a, b []uint8) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 1
	}
	diff := 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
	}
	return float64(diff) / float64(len(a))
}

func maskImage(mask []uint8, width, height int) *image.Gray {
	out := image.NewGray(image.Rect(0, 0, width, height))
	for i, bit := range mask {
		if bit == 1 {
			out.Pix[i] = 255
		}
	}
	return out
}

func encodePNG(img image.Image) ([]byte, error) {
	var out bytes.Buffer
	err := png.Encode(&out, img)
	return out.Bytes(), err
}

func makeSheet(frames [][]byte) ([]byte, error) {
	return composeSheet(frames, true)
}

func composeSheet(frames [][]byte, captions bool) ([]byte, error) {
	if len(frames) != len(views) {
		return nil, errors.New("All six views are required.")
	}
	sheet := image.NewRGBA(image.Rect(0, 0, 1920, 800))
	for i, frame := range frames {
		caption := ""
		if captions {
			caption = views[i]
		}
		img, err := renderFrame(frame, 640, 400, caption)
		if err != nil {
			return nil, err
		}
		point := image.Pt((i%3)*640, (i/3)*400)
		draw.Draw(sheet, image.Rectangle{Min: point, Max: point.Add(image.Pt(640, 400))}, img, image.Point{}, draw.Src)
	}
	return encodePNG(sheet)
}

// Both individual skill shots and report tiles use the same validation and caption.
func renderFrame(frame []byte, width, height int, view string) (image.Image, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(frame))
	if err != nil || cfg.Width != 1440 || cfg.Height != 900 {
		return nil, errors.New("The browser produced an invalid screenshot.")
	}
	if width != cfg.Width || height != cfg.Height {
		frame, err = pngtools.ResizePngBytes(frame, width, height)
		if err != nil {
			return nil, err
		}
	}
	img, err := png.Decode(bytes.NewReader(frame))
	if err != nil {
		return nil, err
	}
	colored := 0
	for y := 0; y < height; y += 4 {
		for x := 0; x < width; x += 4 {
			r, g, b, _ := img.At(x, y).RGBA()
			if max(r, g, b)-min(r, g, b) > 6*257 || r > 65*257 {
				colored++
			}
		}
	}
	if colored < 10 {
		return nil, errors.New("A screenshot appears blank. Please retake both screenshots.")
	}
	out := image.NewRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
	if view != "" {
		label := font.Drawer{Dst: out, Src: image.NewUniform(color.White), Face: basicfont.Face7x13, Dot: fixed.P(10, 20)}
		label.DrawString(view)
	}
	return out, nil
}
