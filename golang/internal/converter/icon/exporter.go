package icon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	pngfmt "github.com/pqhuy98/wow-converter/internal/formats/png"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

// Exporter exports WoW icon textures with optional frame processing.
type Exporter struct {
	client    client.Client
	assetDir  string
	outputDir string
}

// NewExporter creates an icon exporter.
func NewExporter(c client.Client, assetDir, outputDir string) *Exporter {
	return &Exporter{
		client:    c,
		assetDir:  filepath.Clean(assetDir),
		outputDir: outputDir,
	}
}

func (e *Exporter) resolveFileDataID(ctx context.Context, wowTexturePath string) (int, error) {
	file, err := e.client.GetFileByName(ctx, wowTexturePath)
	if err != nil {
		return 0, err
	}
	if file.FileDataID == 0 {
		return 0, fmt.Errorf("texture not found: %s", wowTexturePath)
	}
	return file.FileDataID, nil
}

// ExportPngByPath exports or returns an existing PNG for a WoW texture path.
func (e *Exporter) ExportPngByPath(ctx context.Context, wowTexturePath string) (string, error) {
	fileDataID, err := e.resolveFileDataID(ctx, wowTexturePath)
	if err != nil {
		return "", err
	}
	pngPath := strings.TrimSuffix(wowTexturePath, filepath.Ext(wowTexturePath)) + ".png"
	absPath, err := filepath.Abs(filepath.Join(e.assetDir, filepath.FromSlash(pngPath)))
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(absPath); err == nil {
		return absPath, nil
	}
	if err := ensureTexturePNG(ctx, e.client, fileDataID, absPath); err != nil {
		return "", err
	}
	return absPath, nil
}

func ensureTexturePNG(ctx context.Context, c client.Client, fileDataID int, absPNG string) error {
	raw, err := c.DownloadCascFile(ctx, fileDataID)
	if err != nil {
		return err
	}
	return writeBLPAsPNG(raw, absPNG)
}

func writeBLPAsPNG(raw []byte, absPNG string) error {
	img, err := blp.NewBLPImage(buffer.From(raw))
	if err != nil {
		return err
	}
	pngBuf, err := img.ToPNG(0b1111, 0)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absPNG), 0o755); err != nil {
		return err
	}
	return os.WriteFile(absPNG, pngBuf.Raw(), 0o644)
}

func sizeToPixels(size Size) int {
	switch size {
	case Size64:
		return 64
	case Size128:
		return 128
	case Size256:
		return 256
	default:
		return 0
	}
}

func (e *Exporter) resizePngNormal(pngPath string, targetSize int) ([]byte, error) {
	data, err := os.ReadFile(pngPath)
	if err != nil {
		return nil, err
	}
	oversized, err := pngfmt.ResizePngFill(data, targetSize+1, targetSize+1)
	if err != nil {
		return nil, err
	}
	src, sw, sh, err := pngfmt.DecodeRGBA(oversized)
	if err != nil {
		return nil, err
	}
	if sw <= 1 || sh <= 1 {
		return pngfmt.ResizePngFill(data, targetSize, targetSize)
	}
	cropped := make([]byte, targetSize*targetSize*4)
	for y := 0; y < targetSize; y++ {
		for x := 0; x < targetSize; x++ {
			si := ((y+1)*sw + (x + 1)) * 4
			di := (y*targetSize + x) * 4
			copy(cropped[di:di+4], src[si:si+4])
		}
	}
	return pngfmt.EncodeRGBA(cropped, targetSize, targetSize)
}

// ConvertPngToIconBuffer converts a PNG file to a processed icon buffer.
func (e *Exporter) ConvertPngToIconBuffer(pngPath string, options ConversionOptions) ([]byte, error) {
	merged := MergeOptions(options)
	if merged.Size == SizeOrig {
		data, err := os.ReadFile(pngPath)
		if err != nil {
			return nil, err
		}
		return ProcessIconImage(data, merged)
	}

	targetSize := sizeToPixels(merged.Size)
	dims, err := pngfmt.GetPngDimensionsFromFile(pngPath)
	if err != nil {
		return nil, err
	}
	sourceSize := dims.Width
	if dims.Height < sourceSize {
		sourceSize = dims.Height
	}

	var pngBuffer []byte
	switch {
	case sourceSize == targetSize:
		pngBuffer, err = os.ReadFile(pngPath)
	case merged.ResizeMode == ResizeAI && sourceSize < targetSize:
		pngBuffer, err = resizeAiPngWithCache(pngPath, merged.Size)
		if errors.Is(err, errUpscalerUnavailable) {
			pngBuffer, err = e.resizePngNormal(pngPath, targetSize)
		}
	default:
		pngBuffer, err = e.resizePngNormal(pngPath, targetSize)
	}
	if err != nil {
		return nil, err
	}
	return ProcessIconImage(pngBuffer, merged)
}

// ExportToBlp exports icon items to BLP files.
func (e *Exporter) ExportToBlp(ctx context.Context, items []ExportItem) (int, []string, error) {
	type task struct {
		png     []byte
		blpPath string
	}
	var tasks []task
	seen := map[string]struct{}{}

	for _, item := range items {
		pngPath, err := e.ExportPngByPath(ctx, item.TexturePath)
		if err != nil {
			continue
		}
		var pngBuffer []byte
		if item.Options != nil {
			pngBuffer, err = e.ConvertPngToIconBuffer(pngPath, *item.Options)
		} else {
			pngBuffer, err = os.ReadFile(pngPath)
		}
		if err != nil {
			continue
		}

		frame := FrameNone
		if item.Options != nil {
			frame = MergeOptions(*item.Options).Frame
		}
		blpRel := item.OutputPath
		if blpRel == "" {
			if item.Options != nil && item.Options.Frame != "" {
				blpRel = GetWc3Path(item.TexturePath, frame)
			} else {
				blpRel = filepath.Base(strings.TrimSuffix(item.TexturePath, filepath.Ext(item.TexturePath)) + ".blp")
			}
		} else {
			blpRel, err = validateOutputPath(blpRel, frame)
			if err != nil {
				return 0, nil, err
			}
		}
		if _, ok := seen[blpRel]; ok {
			continue
		}
		seen[blpRel] = struct{}{}
		blpAbs := filepath.Join(e.outputDir, filepath.FromSlash(blpRel))
		if err := os.MkdirAll(filepath.Dir(blpAbs), 0o755); err != nil {
			return 0, nil, err
		}
		tasks = append(tasks, task{png: pngBuffer, blpPath: blpAbs})
	}

	if len(tasks) == 0 {
		return 0, nil, fmt.Errorf("no valid items to export")
	}

	blp.EnsureWorkerPool(0)
	workers := blp.GetWorkerPoolSize()
	if workers <= 0 {
		workers = 1
	}
	if workers > len(tasks) {
		workers = len(tasks)
	}

	paths := make([]string, len(tasks))
	convTasks := make([]func() error, len(tasks))
	for i, t := range tasks {
		i, t := i, t
		convTasks[i] = func() error {
			if err := blp.SubmitBlpTask(blp.TaskInput{Kind: "png", Data: t.png}, t.blpPath); err != nil {
				return err
			}
			rel, err := filepath.Rel(e.outputDir, t.blpPath)
			if err != nil {
				rel = t.blpPath
			}
			paths[i] = filepath.ToSlash(rel)
			return nil
		}
	}
	if err := common.WorkerPool(workers, convTasks); err != nil {
		return 0, nil, err
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p != "" {
			out = append(out, p)
		}
	}
	return len(out), out, nil
}
