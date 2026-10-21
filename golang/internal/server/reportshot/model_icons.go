package reportshot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/converter/icon"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
)

type modelIconJob struct {
	model, key, sequence string
	camera               cameraReference
}

// RenderModelIcons renders one shared 1.7x portrait per map model. Individual
// failures are logged and omitted; callers retain their inherited unit icon.
// Cancellation is propagated, including while loading assets or encoding BLP.
func RenderModelIcons(ctx context.Context, root string, models []string, force bool) (map[string]string, error) {
	result := map[string]string{}
	var jobs []modelIconJob
	cache := map[string]string{}
	cachePath := filepath.Join(root, "npcs", "icons.json")
	if raw, err := os.ReadFile(cachePath); err == nil {
		_ = json.Unmarshal(raw, &cache)
	}
	if cache == nil {
		cache = map[string]string{}
	}
	dependencies := map[string]string{}
	sort.Strings(models)
	for _, model := range models {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		job, err := prepareModelIcon(ctx, root, model, dependencies)
		if err != nil {
			log.Printf("NPC icon %s: %v (using Footman icon)", model, err)
			continue
		}
		active, disabled := modelIconPaths(model)
		if !force && cache[model] == job.key && validIconBLP(root, active) && validIconBLP(root, disabled) {
			result[model] = active
			continue
		}
		jobs = append(jobs, job)
	}
	if len(jobs) == 0 {
		if ctx.Err() == nil {
			saveModelIconCache(root, cache, models)
		}
		return result, ctx.Err()
	}
	base := os.Getenv("WOW_CONVERTER_URL")
	if base == "" {
		port := os.Getenv("PORT")
		if port == "" {
			port = "3001"
		}
		base = "http://127.0.0.1:" + port
	}
	target, err := url.Parse(base)
	if err != nil {
		return result, err
	}
	// Read the generated map assets in place. The running UI supplies the exact
	// production viewer, with no asset copies or extra browser dependencies.
	mux := http.NewServeMux()
	mux.Handle("/api/assets/", http.StripPrefix("/api/assets/", http.FileServer(http.Dir(root))))
	mux.Handle("/", httputil.NewSingleHostReverseProxy(target))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return result, err
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go server.Serve(listener)
	defer server.Close()
	renderBase := "http://" + listener.Addr().String()
	// Avoid launching a browser when a CLI/headless-only export has no UI server.
	check, err := http.NewRequestWithContext(ctx, "GET", base+"/viewer", nil)
	if err != nil {
		return result, err
	}
	response, err := (&http.Client{Timeout: 3 * time.Second}).Do(check)
	if err != nil {
		return result, err
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		return result, fmt.Errorf("icon viewer unavailable: HTTP %d", response.StatusCode)
	}
	owner, err := openBrowser(ctx)
	if err != nil {
		return result, err
	}
	defer owner.close()
	tabs := []*browser{owner}
	for len(tabs) < min(3, len(jobs)) {
		tab, err := owner.newTab(ctx)
		if err != nil {
			break
		}
		tabs = append(tabs, tab)
		defer tab.close()
	}
	for _, tab := range tabs {
		if err = tab.setCacheDisabled(ctx, true); err != nil {
			return result, err
		}
		if _, err = tab.call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{"width": 384, "height": 384, "deviceScaleFactor": 1, "mobile": false}); err != nil {
			return result, err
		}
		if _, err = tab.call(ctx, "Emulation.setFocusEmulationEnabled", map[string]bool{"enabled": true}); err != nil {
			return result, err
		}
	}
	pending := make(chan modelIconJob, len(jobs))
	for _, job := range jobs {
		pending <- job
	}
	close(pending)
	var wg sync.WaitGroup
	var mu sync.Mutex
	completed := 0
	for _, tab := range tabs {
		wg.Add(1)
		go func(b *browser) {
			defer wg.Done()
			for job := range pending {
				if ctx.Err() != nil {
					return
				}
				jobCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
				request := Request{BaseURL: renderBase, Model: job.model, Sequence: job.sequence, ModelScale: 1, cameras: make([]chan cameraReference, len(views))}
				request.cameras[0] = make(chan cameraReference, 1)
				request.cameras[0] <- job.camera
				frames, _, err := captureConverter(jobCtx, b, request, []int{0})
				if err == nil && len(frames) == 1 {
					err = writeModelIcon(jobCtx, root, job.model, frames[0])
				} else if err == nil {
					err = fmt.Errorf("empty capture")
				}
				cancel()
				mu.Lock()
				completed++
				if err == nil {
					active, _ := modelIconPaths(job.model)
					result[job.model] = active
					cache[job.model] = job.key
				} else {
					log.Printf("NPC icon %s: %v (using Footman icon)", job.model, err)
				}
				log.Printf("NPC icons %d/%d", completed, len(jobs))
				mu.Unlock()
			}
		}(tab)
	}
	wg.Wait()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	saveModelIconCache(root, cache, models)
	return result, nil
}

func saveModelIconCache(root string, cache map[string]string, models []string) {
	used := map[string]bool{}
	for _, model := range models {
		used[model] = true
	}
	for model := range cache {
		if !used[model] {
			if filepath.ToSlash(filepath.Clean(model)) == model && filepath.Dir(model) == "npcs" {
				active, disabled := modelIconPaths(model)
				for _, path := range []string{active, disabled} {
					if err := os.Remove(filepath.Join(root, filepath.FromSlash(path))); err != nil && !os.IsNotExist(err) {
						log.Printf("NPC icon cleanup: %v", err)
					}
				}
			}
			delete(cache, model)
		}
	}
	if raw, err := json.MarshalIndent(cache, "", "  "); err == nil {
		if err = os.WriteFile(filepath.Join(root, "npcs", "icons.json"), raw, 0644); err != nil {
			log.Printf("NPC icon cache: %v", err)
		}
	}
}

func prepareModelIcon(ctx context.Context, root, model string, dependencies map[string]string) (modelIconJob, error) {
	var out modelIconJob
	clean := filepath.ToSlash(filepath.Clean(model))
	if clean != model || !strings.HasPrefix(model, "npcs/") {
		return out, fmt.Errorf("invalid NPC model path")
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(model)))
	if err != nil {
		return out, err
	}
	if len(raw) < 4 || string(raw[:4]) != "MDLX" {
		return out, fmt.Errorf("expected MDX model")
	}
	digest := sha256.New()
	digest.Write([]byte("npc-icon-v1-384-128-classichd20-stand-midpoint-zoom1.7"))
	digest.Write(raw)
	var cameras []*mdx.Camera
	var sequences []*mdx.Sequence
	for offset := 4; offset < len(raw); {
		if err = ctx.Err(); err != nil {
			return out, err
		}
		if len(raw)-offset < 8 {
			return out, io.ErrUnexpectedEOF
		}
		tag := string(raw[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		offset += 8
		if size > len(raw)-offset {
			return out, io.ErrUnexpectedEOF
		}
		if tag == "CAMS" || tag == "SEQS" || tag == "TEXS" {
			stream := mdx.NewBinaryStream(raw[offset : offset+size])
			for stream.Index() < size {
				switch tag {
				case "CAMS":
					c := mdx.NewCamera()
					err = c.ReadMdx(stream, 1000)
					cameras = append(cameras, c)
				case "SEQS":
					s := mdx.NewSequence()
					err = s.ReadMdx(stream)
					sequences = append(sequences, s)
				case "TEXS":
					t := mdx.NewTexture()
					err = t.ReadMdx(stream)
					if err == nil && t.Path != "" {
						path := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(t.Path, "\\", "/")))
						if filepath.IsAbs(path) || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
							return out, fmt.Errorf("invalid texture path")
						}
						key, ok := dependencies[path]
						if !ok {
							data, readErr := os.ReadFile(filepath.Join(root, path))
							if readErr != nil {
								return out, readErr
							}
							sum := sha256.Sum256(data)
							key = hex.EncodeToString(sum[:])
							dependencies[path] = key
						}
						digest.Write([]byte(key))
					}
				}
				if err != nil {
					return out, err
				}
			}
		}
		offset += size
	}
	if len(cameras) == 0 || len(sequences) == 0 {
		return out, fmt.Errorf("model has no camera or animation")
	}
	camera := cameras[0]
	for _, c := range cameras {
		if c.Name == "Portrait_Camera" {
			camera = c
			break
		}
	}
	out.camera = cameraReference{Up: [3]float64{0, 0, 1}, ModelMatrix: [16]float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}, Height: 1, ModelScale: 1}
	factor := math.Tan(float64(camera.FieldOfView)/2) / math.Tan(math.Pi/8) / 1.7
	for i := range 3 {
		out.camera.Target[i] = float64(camera.TargetPosition[i])
		out.camera.Eye[i] = out.camera.Target[i] + (float64(camera.Position[i])-out.camera.Target[i])*factor
	}
	out.sequence = sequences[0].Name
	for _, s := range sequences {
		if strings.HasPrefix(strings.ToLower(s.Name), "stand") {
			out.sequence = s.Name
			break
		}
	}
	out.model = model
	out.key = hex.EncodeToString(digest.Sum(nil))
	return out, nil
}

func modelIconPaths(model string) (string, string) {
	name := strings.TrimSuffix(filepath.Base(model), filepath.Ext(model))
	return "ReplaceableTextures/CommandButtons/BTN_" + name + ".blp", "ReplaceableTextures/CommandButtonsDisabled/DISBTN_" + name + ".blp"
}

func validIconBLP(root, path string) bool {
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [4]byte
	_, err = io.ReadFull(f, magic[:])
	return err == nil && string(magic[:]) == "BLP1"
}

func writeModelIcon(ctx context.Context, root, model string, raw []byte) error {
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	if img.Bounds().Dx() != 384 || img.Bounds().Dy() != 384 {
		return fmt.Errorf("unexpected icon capture size %v", img.Bounds())
	}
	foreground := 0
	for y := 0; y < 384; y++ {
		for x := 0; x < 384; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if math.Abs(float64(r>>8)-38)+math.Abs(float64(g>>8)-38)+math.Abs(float64(b>>8)-38) > 24 {
				foreground++
			}
		}
	}
	if foreground < 300 {
		return fmt.Errorf("blank portrait")
	}
	active, disabled := modelIconPaths(model)
	for i, path := range []string{active, disabled} {
		if err = ctx.Err(); err != nil {
			return err
		}
		frame := icon.FrameBtn
		if i == 1 {
			frame = icon.FrameDisBtn
		}
		data, err := icon.ProcessIconImage(raw, icon.MergeOptions(icon.ConversionOptions{Size: icon.Size128, Style: icon.StyleClassicHD20, Frame: frame}))
		if err != nil {
			return err
		}
		destination := filepath.Join(root, filepath.FromSlash(path))
		if err = os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		temporary, err := os.CreateTemp(filepath.Dir(destination), "npc-icon-*.blp")
		if err != nil {
			return err
		}
		tempPath := temporary.Name()
		temporary.Close()
		err = blp.ConvertTextureToBlp(blp.EncodeInput{PNG: data}, tempPath)
		if err == nil {
			err = ctx.Err()
		}
		if err == nil {
			err = os.Rename(tempPath, destination)
		}
		os.Remove(tempPath)
		if err != nil {
			return err
		}
	}
	return nil
}
