package icon

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	pngfmt "github.com/pqhuy98/wow-converter/internal/formats/png"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

var errUpscalerUnavailable = errors.New("upscayl runtime unavailable")

// AIResizer runs the same upscayl-ncnn binary bundled with upscayl-node.
type AIResizer struct {
	mu   sync.Mutex
	jobs map[string]*resizeJob
}

type resizeResult struct {
	path string
	err  error
}

type resizeJob struct {
	done   chan struct{}
	result resizeResult
}

func (j *resizeJob) finish(res resizeResult) {
	j.result = res
	close(j.done)
}

func (j *resizeJob) wait() (string, error) {
	<-j.done
	return j.result.path, j.result.err
}

var (
	defaultAIResizer  = &AIResizer{jobs: make(map[string]*resizeJob)}
	upscalerProcessMu sync.Mutex
)

// UpscalerAvailable reports whether bin/upscayl is installed.
func UpscalerAvailable() bool {
	_, err := upscaylBinaryPath()
	return err == nil
}

func aiForcedOff() bool {
	switch strings.TrimSpace(os.Getenv("ICON_AI")) {
	case "0", "false", "off":
		return true
	default:
		return false
	}
}

func upscaylRoot() string {
	return filepath.Join(workspace.FindRepoRoot(), "bin", "upscayl")
}

func upscaylBinaryPath() (string, error) {
	root := upscaylRoot()
	switch runtime.GOOS {
	case "windows":
		path := filepath.Join(root, "win", "upscayl-bin.exe")
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path, nil
		}
	case "linux":
		path := filepath.Join(root, "linux", "upscayl-bin")
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path, nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		name := "upscayl-bin"
		if runtime.GOOS == "windows" {
			name = "upscayl-bin.exe"
		}
		for _, candidate := range []string{
			filepath.Join(dir, name),
			filepath.Join(dir, "bin", "upscayl", "win", name),
			filepath.Join(dir, "bin", "upscayl", "linux", name),
		} {
			if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
				return candidate, nil
			}
		}
	}
	return "", errUpscalerUnavailable
}

func upscaylModelsDir() string {
	root := upscaylRoot()
	if st, err := os.Stat(filepath.Join(root, "models")); err == nil && st.IsDir() {
		return filepath.Join(root, "models")
	}
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "bin", "upscayl", "models")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate
		}
	}
	return filepath.Join(root, "models")
}

func defaultUpscaleModel() (string, error) {
	dir := upscaylModelsDir()
	preferred := filepath.Join(dir, "realesrgan-x4plus")
	if _, err := os.Stat(preferred + ".param"); err == nil {
		return preferred, nil
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "realesrgan-*"))
	if len(matches) > 0 {
		return matches[len(matches)-1], nil
	}
	return "", fmt.Errorf("no upscayl models in %s", dir)
}

func modelScale(modelName string) int {
	lower := strings.ToLower(modelName)
	switch {
	case strings.Contains(lower, "x2"), strings.Contains(lower, "2x"):
		return 2
	case strings.Contains(lower, "x3"), strings.Contains(lower, "3x"):
		return 3
	default:
		return 4
	}
}

func aiScaleForTarget(inputWidth int, targetSize Size) int {
	scale := 2
	switch targetSize {
	case Size256:
		switch inputWidth {
		case 64:
			scale = 4
		case 128:
			scale = 2
		}
	case Size128:
		if inputWidth == 64 {
			scale = 2
		}
	}
	return scale
}

func nearestStandardUp(maxDim int) int {
	switch {
	case maxDim <= 64:
		return 64
	case maxDim <= 128:
		return 128
	case maxDim <= 256:
		return 256
	default:
		return maxDim
	}
}

func isStandardSize(width, height int) bool {
	for _, s := range []int{64, 128, 256} {
		if width == s && height == s {
			return true
		}
	}
	return false
}

func buildUpscaleArgs(inputPath, outputPath, modelPath string, scale int) []string {
	modelName := filepath.Base(modelPath)
	modelDir := filepath.Dir(modelPath)
	args := []string{
		"-i", inputPath,
		"-o", outputPath,
		"-m", modelDir,
		"-n", modelName,
		"-f", "png",
		"-c", "0",
	}
	if modelScale(modelName) != scale {
		args = append(args, "-s", strconv.Itoa(scale))
	}
	return args
}

func runUpscaler(inputPath, outputPath string, scale int) error {
	bin, err := upscaylBinaryPath()
	if err != nil {
		return err
	}
	model, err := defaultUpscaleModel()
	if err != nil {
		return err
	}

	inputPath, err = filepath.Abs(inputPath)
	if err != nil {
		return err
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}

	args := buildUpscaleArgs(inputPath, outputPath, model, scale)
	cmd := exec.Command(bin, args...)
	if runtime.GOOS == "windows" {
		cmd.Dir = filepath.Dir(bin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	upscalerProcessMu.Lock()
	err = cmd.Run()
	upscalerProcessMu.Unlock()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("upscayl: %s", msg)
	}
	if _, err := os.Stat(outputPath); err != nil {
		return fmt.Errorf("upscayl produced no output at %s", outputPath)
	}
	return nil
}

// ResizePng upscales a PNG file to the target icon size using upscayl.
func (r *AIResizer) ResizePng(pngPath string, targetSize Size, outputPath string) (string, error) {
	if aiForcedOff() || !UpscalerAvailable() {
		return "", errUpscalerUnavailable
	}

	absPng, err := filepath.Abs(pngPath)
	if err != nil {
		return "", err
	}
	absOut, err := filepath.Abs(outputPath)
	if err != nil {
		return "", err
	}

	jobKey := fmt.Sprintf("%s-%s-%s", absPng, targetSize, absOut)
	r.mu.Lock()
	if job, ok := r.jobs[jobKey]; ok {
		r.mu.Unlock()
		return job.wait()
	}
	job := &resizeJob{done: make(chan struct{})}
	r.jobs[jobKey] = job
	r.mu.Unlock()

	path, err := r.performResize(absPng, targetSize, absOut)
	job.finish(resizeResult{path: path, err: err})

	r.mu.Lock()
	delete(r.jobs, jobKey)
	r.mu.Unlock()
	return path, err
}

func (r *AIResizer) performResize(pngPath string, targetSize Size, outputPath string) (string, error) {
	dims, err := pngfmt.GetPngDimensionsFromFile(pngPath)
	if err != nil {
		return "", err
	}

	inputPath := pngPath
	inputWidth := dims.Width
	inputHeight := dims.Height
	var tempPreResize string

	if !isStandardSize(inputWidth, inputHeight) {
		maxDim := inputWidth
		if inputHeight < maxDim {
			maxDim = inputHeight
		}
		nearest := nearestStandardUp(maxDim)
		if nearest > maxDim {
			data, err := os.ReadFile(pngPath)
			if err != nil {
				return "", err
			}
			resized, err := pngfmt.ResizePngFill(data, nearest, nearest)
			if err != nil {
				return "", err
			}
			f, err := os.CreateTemp("", "pre-resize-*.png")
			if err != nil {
				return "", err
			}
			tempPreResize = f.Name()
			if _, err := f.Write(resized); err != nil {
				_ = f.Close()
				_ = os.Remove(tempPreResize)
				return "", err
			}
			_ = f.Close()
			inputPath = tempPreResize
			inputWidth = nearest
			inputHeight = nearest
		}
	}
	defer func() {
		if tempPreResize != "" {
			_ = os.Remove(tempPreResize)
		}
	}()

	scale := aiScaleForTarget(inputWidth, targetSize)
	if err := runUpscaler(inputPath, outputPath, scale); err != nil {
		return "", err
	}
	return outputPath, nil
}

func resizeAiPngWithCache(pngPath string, targetSize Size) ([]byte, error) {
	if aiForcedOff() || !UpscalerAvailable() {
		return nil, errUpscalerUnavailable
	}

	absPng, err := filepath.Abs(pngPath)
	if err != nil {
		return nil, err
	}

	sizeNum := sizeToPixels(targetSize)
	cachePath := fmt.Sprintf("%s__ai%d.png", absPng, sizeNum)
	if data, err := os.ReadFile(cachePath); err == nil {
		return data, nil
	}

	outPath, err := defaultAIResizer.ResizePng(absPng, targetSize, cachePath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(outPath)
}
