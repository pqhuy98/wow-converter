package terrainfit

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

//go:embed viewer.html
var viewerHTML []byte

//go:embed native-viewer.js
var nativeViewerJS []byte

func writeViewer(out string) error {
	if e := os.WriteFile(filepath.Join(out, "native-viewer.js"), nativeViewerJS, 0644); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(out, "index.html"), viewerHTML, 0644)
}

type tileReport struct {
	Name    string  `json:"name"`
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Metrics metrics `json:"metrics"`
}
type stageReport struct {
	Name               string           `json:"name"`
	Palette            int              `json:"palette"`
	TextureBytes       int64            `json:"blpBytes"`
	Metrics            metrics          `json:"metrics"`
	Seconds            float64          `json:"seconds"`
	ModelArea          float64          `json:"modelArea,omitempty"`
	PatchTextureBytes  int64            `json:"patchBLPBytes,omitempty"`
	PatchTriangles     int              `json:"patchTriangles,omitempty"`
	NativeMAE          float64          `json:"nativeOnlyMAE,omitempty"`
	Geometry           *geometryMetrics `json:"geometry,omitempty"`
	MandatoryModelArea float64          `json:"mandatoryModelArea,omitempty"`
	NativeMajority     bool             `json:"nativeMajority"`
	Seams              *seamMetrics     `json:"seams,omitempty"`
	ModelSourceStage   string           `json:"modelSourceStage,omitempty"`
	VisualStatus       string           `json:"visualStatus,omitempty"`
}
type patchBudget struct {
	Fraction         float64 `json:"fraction"`
	MAE              float64 `json:"mae"`
	PSNR             float64 `json:"psnr"`
	RGBBytesWithMips int64   `json:"rgbBytesWithMips"`
}
type regionReport struct {
	Region
	Cells              [2]int          `json:"cells"`
	Build              string          `json:"build"`
	Stages             []stageReport   `json:"stages"`
	Tiles              []tileReport    `json:"tiles"`
	Geometry           geometryMetrics `json:"geometry"`
	PatchCurve         []patchBudget   `json:"idealResidualCurve"`
	SourcePNGBytes     int64           `json:"sourcePNGBytes"`
	HoleFraction       float64         `json:"holeFraction"`
	RefinementVersion  int             `json:"refinementVersion,omitempty"`
	GeometryVersion    int             `json:"geometryVersion,omitempty"`
	ClampPercent       [2]float64      `json:"clampPercent"`
	SourceHeightBounds [2]float64      `json:"sourceHeightBounds"`
}
type benchmarkReport struct {
	Version       int            `json:"version"`
	Created       string         `json:"created"`
	Reference     string         `json:"reference"`
	Renderer      string         `json:"renderer"`
	CellsPerADT   int            `json:"cellsPerADT"`
	SubtilePixels int            `json:"subtilePixels"`
	Regions       []regionReport `json:"regions"`
	SourceRoot    string         `json:"sourceRoot"`
}
type tileData struct {
	name, dir string
	mesh      Mesh
	coverage  []bool
}

func Benchmark(ctx context.Context, out, sourceRoot string, regions []Region) error {
	report := benchmarkReport{Version: 1, Created: time.Now().UTC().Format(time.RFC3339), Reference: "8192px production Go WoW terrain bake: diffuse/paint/height layers and MCCV; unlit, no objects/water", Renderer: "HiveWE corner sorting, masks, base variation and recursive RGBA mixing; generated BLP1 atlases decoded before evaluation. Browser approximation; native WC3 runtime not verified.", CellsPerADT: cellsPerADT, SubtilePixels: subtile}
	relative, e := filepath.Rel(out, sourceRoot)
	if e != nil {
		return e
	}
	report.SourceRoot = filepath.ToSlash(relative)
	if e := writeViewer(out); e != nil {
		return e
	}
	for _, region := range regions {
		if e := ctx.Err(); e != nil {
			return e
		}
		r, e := benchmarkRegion(ctx, out, sourceRoot, region)
		if e != nil {
			return fmt.Errorf("%s: %w", region.Name, e)
		}
		report.Regions = append(report.Regions, r)
		if e = writeJSON(filepath.Join(out, "report.json"), report); e != nil {
			return e
		}
	}
	return nil
}

func benchmarkRegion(ctx context.Context, out, sourceRoot string, region Region) (regionReport, error) {
	fmt.Println("FIT", region.Name)
	nx, ny := region.X1-region.X0+1, region.Y1-region.Y0+1
	w, h := nx*cellsPerADT, ny*cellsPerADT
	r := regionReport{Region: region, Cells: [2]int{w, h}}
	dir := filepath.Join(out, region.Name)
	if e := os.MkdirAll(dir, 0755); e != nil {
		return r, e
	}
	low := make([]byte, w*h*sampleCount*3)
	valid := make([]bool, w*h)
	var tiles []tileData
	var meshes []Mesh
	holeN := 0
	for ty := 0; ty < ny; ty++ {
		for tx := 0; tx < nx; tx++ {
			name := fmt.Sprintf("%s-%d_%d", region.Map, region.X0+tx, region.Y0+ty)
			d := filepath.Join(sourceRoot, name)
			b, e := os.ReadFile(filepath.Join(d, "capture.json"))
			if e != nil {
				return r, e
			}
			var info captureInfo
			if e = json.Unmarshal(b, &info); e != nil {
				return r, e
			}
			if info.Version != 1 || info.Width != 8192 {
				return r, fmt.Errorf("invalid capture")
			}
			if r.Build != "" && r.Build != info.Build {
				return r, fmt.Errorf("mixed WoW builds")
			}
			r.Build = info.Build
			b, e = os.ReadFile(filepath.Join(d, "mesh.json"))
			if e != nil {
				return r, e
			}
			var mesh Mesh
			if e = json.Unmarshal(b, &mesh); e != nil {
				return r, e
			}
			_, coverage := rasterHeights(mesh, 128, true)
			for _, v := range coverage {
				if !v {
					holeN++
				}
			}
			img, e := loadPNG(filepath.Join(d, "wow.png"))
			if e != nil {
				return r, e
			}
			if img.Rect.Dx() != 8192 || img.Rect.Dy() != 8192 {
				return r, fmt.Errorf("reference changed dimensions")
			}
			stat, e := os.Stat(filepath.Join(d, "wow.png"))
			if e != nil {
				return r, e
			}
			r.SourcePNGBytes += stat.Size()
			small := downsample(img, cellsPerADT*samples)
			if e = savePNG(filepath.Join(d, "preview.png"), downsample(img, 2048)); e != nil {
				return r, e
			}
			for y := 0; y < cellsPerADT*samples; y++ {
				for x := 0; x < cellsPerADT*samples; x++ {
					at := ((ty*cellsPerADT*samples+y)*w*samples + tx*cellsPerADT*samples + x) * 3
					s := y*small.Stride + x*4
					copy(low[at:at+3], small.Pix[s:s+3])
				}
			}
			for y := 0; y < cellsPerADT; y++ {
				for x := 0; x < cellsPerADT; x++ {
					valid[(ty*cellsPerADT+y)*w+tx*cellsPerADT+x] = coverage[min(127, int((float64(y)+.5)*128/cellsPerADT))*128+min(127, int((float64(x)+.5)*128/cellsPerADT))]
				}
			}
			tiles = append(tiles, tileData{name: name, dir: d, mesh: mesh, coverage: coverage})
			meshes = append(meshes, mesh)
		}
	}
	r.HoleFraction = float64(holeN) / float64(len(tiles)*128*128)
	native, g := nativeMesh(region, meshes)
	r.Geometry = g
	if e := writeJSON(filepath.Join(dir, "native-mesh.json"), native); e != nil {
		return r, e
	}
	var f *fit
	var finalCells []patchCell
	for stage, name := range []string{"16-colors", "64-colors", "shared-corners", "source-variations"} {
		start := time.Now()
		stageDir := filepath.Join(dir, name)
		if e := os.MkdirAll(stageDir, 0755); e != nil {
			return r, e
		}
		switch stage {
		case 0:
			f = newFit(w, h, low, valid, 16)
		case 1:
			f = newFit(w, h, low, valid, 64)
		case 2:
			if e := f.optimize(ctx, 2); e != nil {
				return r, e
			}
		case 3:
			selected := f.exemplars()
			for ti, t := range tiles {
				img, e := loadPNG(filepath.Join(t.dir, "wow.png"))
				if e != nil {
					return r, e
				}
				for id, exemplars := range selected {
					for vi, e := range exemplars {
						if (e.y/cellsPerADT)*nx+e.x/cellsPerADT == ti {
							f.fillExemplar(id, vi+1, e, img, ti%nx, ti/nx)
						}
					}
				}
			}
			for i := range f.sheets {
				refreshLow(&f.sheets[i])
			}
			if e := f.chooseVariants(ctx); e != nil {
				return r, e
			}
		}
		blpBytes, e := f.saveSheets(filepath.Join(stageDir, "tileset"))
		if e != nil {
			return r, e
		}
		var aggregate measurement
		for ti, t := range tiles {
			if e := ctx.Err(); e != nil {
				return r, e
			}
			fmt.Printf("  %s %s: 8192px render + diff\n", name, t.name)
			ref, e := loadPNG(filepath.Join(t.dir, "wow.png"))
			if e != nil {
				return r, e
			}
			result := f.renderTile(ti%nx, ti/nx, 8192)
			m := measure(ref, result, t.coverage)
			aggregate.add(m)
			if e = savePNG(filepath.Join(stageDir, t.name+".png"), downsample(result, 2048)); e != nil {
				return r, e
			}
			if stage == 3 {
				if e = savePNG(filepath.Join(stageDir, t.name+"-8k.png"), result); e != nil {
					return r, e
				}
				r.Tiles = append(r.Tiles, tileReport{t.name, ti % nx, ti / nx, m.metrics})
				for ci, pixels := range m.cellPixels {
					if pixels > 0 {
						finalCells = append(finalCells, patchCell{m.cellSquared[ci], m.cellAbs[ci], pixels})
					}
				}
				errImg := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
				for y := 0; y < 1024; y++ {
					for x := 0; x < 1024; x++ {
						at := y*errImg.Stride + x*4
						rx, ry := x*8, y*8
						s := ry*ref.Stride + rx*4
						v := 0
						for k := 0; k < 3; k++ {
							v += int(math.Abs(float64(int(ref.Pix[s+k]) - int(result.Pix[s+k]))))
						}
						v = min(255, v*2)
						errImg.Pix[at] = byte(v)
						errImg.Pix[at+1] = byte(v / 4)
						errImg.Pix[at+3] = 255
					}
				}
				if e = savePNG(filepath.Join(stageDir, t.name+"-error.png"), errImg); e != nil {
					return r, e
				}
			}
		}
		sr := stageReport{Name: name, Palette: len(f.sheets), TextureBytes: blpBytes, Metrics: aggregate.metrics, Seconds: time.Since(start).Seconds()}
		r.Stages = append(r.Stages, sr)
		fmt.Printf("SCORE %s %s MAE %.4f PSNR %.3f SSIM %.4f BLP %d\n", region.Name, name, sr.Metrics.MAE, sr.Metrics.PSNR, sr.Metrics.SSIM, blpBytes)
		if e = writeJSON(filepath.Join(dir, "report.json"), r); e != nil {
			return r, e
		}
	}
	r.PatchCurve = residualCurve(finalCells)
	assignment := struct {
		Width, Height          int
		Labels, BaseVariations []uint8
		Orientation            string
	}{w, h, f.labels, f.variants, "image top-left rows; BaseVariations belong to each cell's lower-left corner; labels and variants are base64 byte arrays"}
	if e := writeJSON(filepath.Join(dir, "assignments.json"), assignment); e != nil {
		return r, e
	}
	return r, writeJSON(filepath.Join(dir, "report.json"), r)
}

type patchCell struct {
	squared, abs float64
	pixels       int64
}

func residualCurve(cells []patchCell) []patchBudget {
	sort.SliceStable(cells, func(i, j int) bool {
		return cells[i].squared/float64(cells[i].pixels) > cells[j].squared/float64(cells[j].pixels)
	})
	totalS, totalA := 0.0, 0.0
	var pixels int64
	for _, c := range cells {
		totalS += c.squared
		totalA += c.abs
		pixels += c.pixels
	}
	out := []patchBudget{}
	removedS, removedA := 0.0, 0.0
	done := 0
	var removedPixels int64
	for _, fraction := range []float64{0, .05, .1, .25, .5} {
		count := int(fraction * float64(len(cells)))
		for done < count {
			removedS += cells[done].squared
			removedA += cells[done].abs
			removedPixels += cells[done].pixels
			done++
		}
		rmse := math.Sqrt(max(0, totalS-removedS) / float64(pixels*3))
		psnr := 100.0
		if rmse > 0 {
			psnr = 20 * math.Log10(255/rmse)
		}
		out = append(out, patchBudget{fraction, (totalA - removedA) / float64(pixels*3), psnr, removedPixels * 4})
	}
	return out
}

// Area resampling derives fitting/viewer images from8K, never from the minimap.
func downsample(img *image.NRGBA, size int) *image.NRGBA {
	out := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := float64(img.Rect.Dx()) / float64(size)
	for y := 0; y < size; y++ {
		y0, y1 := float64(y)*scale, float64(y+1)*scale
		for x := 0; x < size; x++ {
			x0, x1 := float64(x)*scale, float64(x+1)*scale
			var sum [3]float64
			for sy := int(y0); sy < int(math.Ceil(y1)); sy++ {
				wy := min(float64(sy+1), y1) - max(float64(sy), y0)
				for sx := int(x0); sx < int(math.Ceil(x1)); sx++ {
					w := wy * (min(float64(sx+1), x1) - max(float64(sx), x0))
					at := min(img.Rect.Dy()-1, sy)*img.Stride + min(img.Rect.Dx()-1, sx)*4
					for k := range sum {
						sum[k] += w * float64(img.Pix[at+k])
					}
				}
			}
			at := y*out.Stride + x*4
			for k := range sum {
				out.Pix[at+k] = byte(sum[k]/(scale*scale) + .5)
			}
			out.Pix[at+3] = 255
		}
	}
	return out
}
