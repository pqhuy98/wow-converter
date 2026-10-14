package reportshot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// RunSourceCLI captures the native WoW renderer embedded in the converter.
func RunSourceCLI() {
	if err := runSourceCLI(os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runSourceCLI(args []string) error {
	flags := flag.NewFlagSet("shot-wow-source", flag.ContinueOnError)
	base := flags.String("base", "http://127.0.0.1:3001", "converter server URL")
	sequence := flags.String("seq", "Stand", "exact WoW animation name")
	view := flags.String("view", "front", "comma-separated views, or all")
	out := flags.String("out", "tmp/shots", "output directory")
	sheet := flags.String("sheet", "", "write a six-view sheet instead of separate frames")
	cameras := flags.String("cameras", "", "existing Wowhead camera map JSON")
	textures := flags.String("textures", "", "comma-separated replacement texture file IDs by M2 texture type")
	model := flags.String("model", "", "exported WC3 model for a paired capture")
	converterSequence := flags.String("converter-seq", "Stand 1", "WC3 sequence for the paired capture")
	modelScale := flags.Float64("model-scale", 56, "exported units per source unit; use export metadata")
	shotTime := flags.Float64("time", -1, "seconds to simulate before capture; default animation midpoint")
	var options, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		options = append(options, arg)
		if arg != "-h" && arg != "--help" && !strings.Contains(arg, "=") && i+1 < len(args) {
			i++
			options = append(options, args[i])
		}
	}
	if err := flags.Parse(options); err != nil {
		return err
	}
	if len(positional) != 1 {
		return errors.New("usage: go run ./cmd/shot-wow-source <file-id-or-m2-path> [--seq Stand] [--time 2] [--cameras map.json]")
	}
	if math.IsNaN(*shotTime) || math.IsInf(*shotTime, 0) || (*shotTime < 0 && *shotTime != -1) || *shotTime > 120 {
		return errors.New("time must be -1 (midpoint) or 0..120 seconds")
	}
	if *model != "" && (math.IsNaN(*modelScale) || math.IsInf(*modelScale, 0) || *modelScale <= 0) {
		return errors.New("model-scale must be finite and positive")
	}
	indices := []int{}
	if *view == "all" || *sheet != "" {
		indices = []int{0, 1, 2, 3, 4, 5}
	} else {
		for _, name := range strings.Split(*view, ",") {
			index := slices.Index(views, strings.TrimSpace(name))
			if index < 0 || slices.Contains(indices, index) {
				return fmt.Errorf("invalid or duplicate view %q", name)
			}
			indices = append(indices, index)
		}
	}
	var byView map[string]cameraReference
	if *cameras != "" {
		data, err := os.ReadFile(workspace.ResolveRepoPath(*cameras))
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, &byView); err != nil {
			return err
		}
		for _, index := range indices {
			if _, ok := byView[views[index]]; !ok {
				return fmt.Errorf("camera missing for %s", views[index])
			}
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(indices))*3*time.Minute)
	defer cancel()
	b, err := openBrowser(ctx)
	if err != nil {
		return err
	}
	defer b.close()
	if err = prepareViewport(ctx, b); err != nil {
		return err
	}
	if err = b.setCacheDisabled(ctx, true); err != nil {
		return err
	}
	q := url.Values{"model": {positional[0]}, "seq": {*sequence}, "textures": {*textures}}
	page := strings.TrimRight(*base, "/") + "/wow-source/index.html?" + q.Encode()
	var frames [][]byte
	var converterFrames [][]byte
	var converterTab *browser
	if *model != "" {
		converterTab, err = b.newTab(ctx)
		if err != nil {
			return err
		}
		defer converterTab.close()
		if err = prepareViewport(ctx, converterTab); err != nil {
			return err
		}
		if err = converterTab.setCacheDisabled(ctx, true); err != nil {
			return err
		}
	}
	for _, index := range indices {
		// Reload each view to reset the native scene, animation and particle RNG.
		if _, err = b.call(ctx, "Page.navigate", map[string]string{"url": page}); err != nil {
			return err
		}
		err = waitFor(ctx, time.Minute, func() (bool, error) {
			var state struct {
				Ready bool   `json:"ready"`
				Error string `json:"error"`
			}
			if err := b.evaluate(ctx, `window.__sourceStatus ? window.__sourceStatus() : {ready:false}`, &state); err != nil {
				return false, err
			}
			if state.Error != "" {
				return false, errors.New(state.Error)
			}
			return state.Ready, nil
		})
		if err != nil {
			var diagnostic string
			_ = b.evaluate(ctx, `JSON.stringify({state:window.__sourceStatus?{...window.__sourceStatus(),info:undefined,selected:undefined}:null,url:location.href,text:document.body?.innerText.slice(0,200)})`, &diagnostic)
			return fmt.Errorf("source load: %w (%s)", err, diagnostic)
		}
		var reference *cameraReference
		if camera, ok := byView[views[index]]; ok {
			reference = &camera
		}
		cameraJSON, _ := json.Marshal(reference)
		nameJSON, _ := json.Marshal(views[index])
		var dataURL string
		// Keep long particle warm-ups outside a single CDP call's deadline.
		expression := fmt.Sprintf(`(() => { window.__sourceCapture = {}; window.__sourceShot(%s,%s,%f).then(data => {window.__sourceCapture.data = data}).catch(error => {window.__sourceCapture.error = String(error)}); return true })()`, nameJSON, cameraJSON, *shotTime)
		if err = b.evaluate(ctx, expression, nil); err != nil {
			return err
		}
		err = waitFor(ctx, 150*time.Second, func() (bool, error) {
			var result struct {
				Data  string
				Error string
			}
			if err := b.evaluate(ctx, `window.__sourceCapture`, &result); err != nil {
				return false, err
			}
			if result.Error != "" {
				return false, errors.New(result.Error)
			}
			dataURL = result.Data
			return dataURL != "", nil
		})
		if err != nil {
			return fmt.Errorf("source %s: %w", views[index], err)
		}
		frame, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(dataURL, "data:image/png;base64,"))
		if err != nil {
			return err
		}
		if _, err = measureFrame(frame); err != nil {
			return fmt.Errorf("source %s: %w", views[index], err)
		}
		frames = append(frames, frame)
		if converterTab != nil {
			if reference == nil {
				var native struct {
					Eye, Target, Up [3]float64
					Height          float64
				}
				if err = b.evaluate(ctx, `({...window.__sourceCamera,height:info.bounds.Max[2]-info.bounds.Min[2]})`, &native); err != nil {
					return err
				}
				reference = &cameraReference{Eye: native.Eye, Target: native.Target, Up: native.Up, Height: math.Max(native.Height, 0.001), ModelMatrix: [16]float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}}
			}
			reference.ModelScale = *modelScale
			request := Request{BaseURL: strings.TrimRight(*base, "/"), Model: assetPath(*model), Sequence: *converterSequence, sequencePrefix: true, cameras: make([]chan cameraReference, len(views))}
			request.cameras[index] = make(chan cameraReference, 1)
			request.cameras[index] <- *reference
			shots, _, err := captureConverter(ctx, converterTab, request, []int{index})
			if err != nil {
				return err
			}
			if *shotTime >= 0 {
				if err = converterTab.evaluate(ctx, fmt.Sprintf(`window.__shotWarmup(%f)`, *shotTime), nil); err != nil {
					return err
				}
				shots[0], err = shootConverterView(ctx, converterTab, views[index], reference)
				if err != nil {
					return err
				}
			}
			converterFrames = append(converterFrames, shots[0])
		}
		if *sheet == "" {
			img, err := renderFrame(frame, 1440, 900, views[index], true)
			if err != nil {
				return err
			}
			data, err := encodePNG(img)
			if err != nil {
				return err
			}
			var pairedData []byte
			if converterTab != nil {
				pairedImage, err := renderFrame(converterFrames[len(converterFrames)-1], 1440, 900, views[index], true)
				if err != nil {
					return err
				}
				pairedData, err = encodePNG(pairedImage)
				if err != nil {
					return err
				}
			}
			dir := workspace.ResolveRepoPath(*out)
			if err = os.MkdirAll(dir, 0755); err != nil {
				return err
			}
			name := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(positional[0], `\`, "/")), ".m2")
			path := filepath.Join(dir, name+"-"+safeSequence(*sequence)+"-"+views[index]+"-wow-source.png")
			if err = os.WriteFile(path, data, 0644); err != nil {
				return err
			}
			fmt.Println(path)
			if converterTab != nil {
				pairedPath := strings.TrimSuffix(path, "-wow-source.png") + "-converter.png"
				if err = os.WriteFile(pairedPath, pairedData, 0644); err != nil {
					return err
				}
				fmt.Println(pairedPath)
			}
		}
	}
	if *sheet != "" {
		if converterTab != nil {
			if err = writeSheet(strings.TrimSuffix(workspace.ResolveRepoPath(*sheet), ".png")+"-converter.png", converterFrames, true); err != nil {
				return err
			}
		}
		return writeSheet(workspace.ResolveRepoPath(*sheet), frames, true)
	}
	return nil
}
