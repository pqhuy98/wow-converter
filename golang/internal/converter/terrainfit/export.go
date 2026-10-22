package terrainfit

import (
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
)

// Tileset is a fitted native ground palette plus a final alpha entry.
type Tileset struct {
	MinTileX, MinTileY int
	Width, Height      int
	Labels             []uint8
	Variants           []uint8
	Sheets             [][]byte
	sampler            *fit
}

// TilesetGrid maps the exported, top-down native corner grid into absolute ADT
// image coordinates (tile index plus fraction). It includes the map rotation.
type TilesetGrid struct {
	Width, Height        int
	Origin, StepX, StepY [2]float64
	// Available stock ground IDs (alpha is a separate final entry).
	PaintFamilies int
}

// FitTileset clusters authored paint mixtures for the selected ADTs.
func FitTileset(ctx context.Context, source casc.Source, mapName string, tiles [][2]int, grid TilesetGrid) (*Tileset, error) {
	if source == nil {
		return nil, fmt.Errorf("WoW data is required for terrain as WC3 tilesets")
	}
	if len(tiles) == 0 {
		return nil, fmt.Errorf("no ADT tiles for WC3 tileset export")
	}
	minX, minY := tiles[0][0], tiles[0][1]
	maxX, maxY := minX, minY
	present := map[[2]int]bool{}
	for _, tile := range tiles {
		present[tile] = true
		minX = min(minX, tile[0])
		maxX = max(maxX, tile[0])
		minY = min(minY, tile[1])
		maxY = max(maxY, tile[1])
	}
	nx, ny := maxX-minX+1, maxY-minY+1
	captures := make([]materialCapture, nx*ny)
	images := map[uint32]*image.NRGBA{}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !present[[2]int{x, y}] {
				continue
			}
			capture, err := loadMaterialTile(ctx, source, mapName, x, y, images)
			if err != nil {
				return nil, err
			}
			captures[(y-minY)*nx+(x-minX)] = capture
		}
	}
	grid.Origin[0] -= float64(minX)
	grid.Origin[1] -= float64(minY)
	f, _, centers, err := fitCaptures(captures, nx, ny, images, &grid)
	if err != nil {
		return nil, err
	}
	f.prepareExportSheets(centers, grid)
	dir, err := os.MkdirTemp("", "wc3-tileset-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	sheets, _, err := f.saveExportSheets(dir)
	if err != nil {
		return nil, err
	}

	return &Tileset{
		MinTileX: minX, MinTileY: minY,
		Width: f.width, Height: f.height,
		Labels: f.labels, Variants: f.variants, Sheets: sheets, sampler: f,
	}, nil
}

// Sample returns the generated native albedo at top-down cell coordinates.
// It is used to match model-side boundary textures to the exported paint.
func (t *Tileset) Sample(x, y float64) [4]float64 {
	x = clamp(x, 0, float64(t.Width)-1e-7)
	y = clamp(y, 0, float64(t.Height)-1e-7)
	cx, cy := int(x), int(y)
	layers := t.sampler.cellLayers(cx, cy)
	var colors [4][4]float64
	for j := 0; j < layers.count; j++ {
		layer := int(layers.masks[j])
		if j == 0 {
			layer = 16 + int(t.Variants[cy*t.Width+cx])
		}
		colors[j] = sampleSheet(&t.sampler.sheets[layers.ids[j]], layer, x-float64(cx), y-float64(cy))
	}
	return compositeLayers(colors, layers.count, true)
}

// Shared by map export and the original full-8K benchmark.
func (f *fit) prepareExportSheets(centers [][]float32, grid TilesetGrid) {
	// Masks stay in native cell orientation. Only the periodic source RGB rotates.
	turns := int(math.Round(math.Atan2(grid.StepX[1], grid.StepX[0])/(math.Pi/2))) & 3
	for i := range f.sheets {
		if i < len(f.sheets)-1 {
			periodicExportRGB(&f.sheets[i])
		}
		rotateMaterialRGB(&f.sheets[i], turns)
		if i < len(f.sheets)-1 {
			naturalizeExportMasks(&f.sheets[i])
		}
	}
	for i := range f.sheets {
		if i < len(f.sheets)-1 {
			alternatives := []*sheet{}
			dominant := func(c []float32) int {
				at := 0
				n := f.materialCount
				if n == 0 {
					n = len(c) - 3
				}
				for k := 1; k < n; k++ {
					if c[k] > c[at] {
						at = k
					}
				}
				return at
			}
			for j := range centers {
				if j != i && dominant(centers[i]) == dominant(centers[j]) {
					distance := 0.0
					for k := range centers[i] {
						d := float64(centers[i][k] - centers[j][k])
						distance += d * d
					}
					if distance < .08 {
						alternatives = append(alternatives, &f.sheets[j])
					}
				}
			}
			makeExportVariants(&f.sheets[i], alternatives...)
		}
	}
	// Source textures repeat at this phase on the aligned ADT grid. Keep that
	// authored detail in exported maps; the other variants remain paintable in WE.
	clear(f.variants)
}

func (f *fit) saveExportSheets(dir string) ([][]byte, int64, error) {
	total, err := f.saveSheets(dir)
	if err != nil {
		return nil, 0, err
	}
	sheets := make([][]byte, len(f.sheets))
	for id := range f.sheets {
		sheets[id], err = os.ReadFile(filepath.Join(dir, fmt.Sprintf("tile-%02d.blp", id)))
		if err != nil {
			return nil, 0, err
		}
		if id < len(f.sheets)-1 {
			if err := sealEncodedTileBorders(sheets[id]); err != nil {
				return nil, 0, err
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("tile-%02d.blp", id)), sheets[id], 0644); err != nil {
				return nil, 0, err
			}
			decoded, err := blp.DecodePalettedBLP1(sheets[id], 0)
			if err != nil {
				return nil, 0, err
			}
			for layer := 0; layer < 32; layer++ {
				x := (layer % 4) * subtile
				if layer >= 16 {
					x += 4 * subtile
				}
				y := (layer % 16 / 4) * subtile
				for row := 0; row < subtile; row++ {
					copy(f.sheets[id].pixels[(layer*subtile*subtile+row*subtile)*4:], decoded.Pix[(y+row)*decoded.Stride+x*4:(y+row)*decoded.Stride+(x+subtile)*4])
				}
			}
			refreshLow(&f.sheets[id])
		}
	}
	return sheets, total, nil
}

func periodicExportRGB(s *sheet) {
	base := append([]byte(nil), s.pixels[16*subtile*subtile*4:17*subtile*subtile*4]...)
	const band = 8
	for axis := 0; axis < 2; axis++ {
		original := append([]byte(nil), base...)
		for j := 0; j < subtile; j++ {
			for i := 0; i < band; i++ {
				a, b := j*subtile+i, j*subtile+subtile-1-i
				if axis == 1 {
					a, b = i*subtile+j, (subtile-1-i)*subtile+j
				}
				t := float64(i) / band
				weight := .5 * (1 - t*t*(3-2*t))
				for k := 0; k < 3; k++ {
					base[a*4+k] = byte(float64(original[a*4+k])*(1-weight) + float64(original[b*4+k])*weight + .5)
					base[b*4+k] = byte(float64(original[b*4+k])*(1-weight) + float64(original[a*4+k])*weight + .5)
				}
			}
		}
	}
	for layer := 0; layer < 32; layer++ {
		for i := 0; i < subtile*subtile; i++ {
			copy(s.pixels[(layer*subtile*subtile+i)*4:(layer*subtile*subtile+i)*4+3], base[i*4:i*4+3])
		}
	}
}

// Error-diffusion quantization can give equal RGB edge pixels different palette
// indices. Seal RGB indices across every frame, at every complete atlas mip.
// Keep the independent transition alpha plane untouched.
func sealEncodedTileBorders(raw []byte) error {
	if len(raw) < 1180 || string(raw[:4]) != "BLP1" || binary.LittleEndian.Uint32(raw[4:]) != 1 {
		return fmt.Errorf("tileset requires paletted BLP1")
	}
	width, height := int(binary.LittleEndian.Uint32(raw[12:])), int(binary.LittleEndian.Uint32(raw[16:]))
	if width != 512 || height != 256 {
		return fmt.Errorf("unexpected tileset dimensions %dx%d", width, height)
	}
	for level := 0; level <= 6; level++ {
		w, h := width>>level, height>>level
		side := subtile >> level
		offset := int(binary.LittleEndian.Uint32(raw[28+level*4:]))
		if offset < 1180 || offset+w*h > len(raw) {
			return fmt.Errorf("truncated tileset mip %d", level)
		}
		indices := raw[offset : offset+w*h]
		rx, ry := 4*side, 0 // Full variant 0.
		for row := 0; row < side; row++ {
			value := indices[(ry+row)*w+rx]
			for layer := 0; layer < 32; layer++ {
				x := (layer % 4) * side
				if layer >= 16 {
					x += 4 * side
				}
				y := (layer % 16 / 4) * side
				indices[(y+row)*w+x] = value
				indices[(y+row)*w+x+side-1] = value
			}
		}
		for col := 0; col < side; col++ {
			value := indices[ry*w+rx+col]
			for layer := 0; layer < 32; layer++ {
				x := (layer % 4) * side
				if layer >= 16 {
					x += 4 * side
				}
				y := (layer % 16 / 4) * side
				indices[y*w+x+col] = value
				indices[(y+side-1)*w+x+col] = value
			}
		}
	}
	return nil
}

// Source height gives grass clumps and rock crevices their own contours. When
// height is flat, use diffuse detail. The displacement vanishes on shared edges,
// preserving the same coverage there for every neighboring material/mask.
func naturalizeExportMasks(s *sheet) {
	field := append([]float64(nil), s.detail...)
	if len(field) != subtile*subtile {
		return
	}
	stats := func(v []float64) (float64, float64) {
		mean := 0.0
		for _, x := range v {
			mean += x / float64(len(v))
		}
		variance := 0.0
		for _, x := range v {
			variance += (x - mean) * (x - mean) / float64(len(v))
		}
		return mean, math.Sqrt(variance)
	}
	_, std := stats(field)
	if std < 1e-6 {
		for i := range field {
			at := (16*subtile*subtile + i) * 4
			field[i] = .2126*float64(s.pixels[at]) + .7152*float64(s.pixels[at+1]) + .0722*float64(s.pixels[at+2])
		}
	}
	for pass := 0; pass < 2; pass++ {
		smoothed := make([]float64, len(field))
		for y := 0; y < subtile; y++ {
			for x := 0; x < subtile; x++ {
				for j := -1; j <= 1; j++ {
					for i := -1; i <= 1; i++ {
						weight := float64([3]int{1, 2, 1}[i+1]*[3]int{1, 2, 1}[j+1]) / 16
						smoothed[y*subtile+x] += field[((y+j+subtile)%subtile)*subtile+(x+i+subtile)%subtile] * weight
					}
				}
			}
		}
		field = smoothed
	}
	mean, std := stats(field)
	if std < 1e-6 {
		return
	}
	for i := range field {
		field[i] = clamp((field[i]-mean)/(2*std), -1, 1)
	}
	for y := 0; y < subtile; y++ {
		for x := 0; x < subtile; x++ {
			u, v := (float64(x)+.5)/subtile, (float64(y)+.5)/subtile
			edge := math.Sin(math.Pi*u) * math.Sin(math.Pi*v)
			// Leave the outer texels exact too: WC3 samples/clamps texel centres.
			if x == 0 || y == 0 || x == subtile-1 || y == subtile-1 {
				edge = 0
			}
			du := .14 * edge * field[y*subtile+x]
			dv := .14 * edge * field[((y+23)%subtile)*subtile+(x+11)%subtile]
			for layer := 1; layer < 15; layer++ {
				s.pixels[(layer*subtile*subtile+y*subtile+x)*4+3] = byte(materialMaskAlpha(uint8(layer), clamp(u+du, 0, 1), clamp(v+dv, 0, 1))*255 + .5)
			}
		}
	}
	refreshLow(s)
}

// Vary source detail, not the material's tint or illumination. A differently
// colored interior surrounded by fixed edge pixels forms a visible box on snow.
func makeExportVariants(s *sheet, alternatives ...*sheet) {
	base := append([]byte(nil), s.pixels[16*subtile*subtile*4:17*subtile*subtile*4]...)
	moments := func(pixels []byte) (mean, deviation [3]float64) {
		for i := 0; i < subtile*subtile; i++ {
			for k := 0; k < 3; k++ {
				mean[k] += float64(pixels[i*4+k]) / (subtile * subtile)
			}
		}
		for i := 0; i < subtile*subtile; i++ {
			for k := 0; k < 3; k++ {
				d := float64(pixels[i*4+k]) - mean[k]
				deviation[k] += d * d / (subtile * subtile)
			}
		}
		for k := range deviation {
			deviation[k] = math.Sqrt(deviation[k])
		}
		return
	}
	mean, deviation := moments(base)
	for variant := 1; variant < 16; variant++ {
		var alternate []byte
		var altMean, altDeviation [3]float64
		if len(alternatives) > 0 {
			alternate = alternatives[(variant-1)%len(alternatives)].pixels[16*subtile*subtile*4 : 17*subtile*subtile*4]
			altMean, altDeviation = moments(alternate)
		}
		delta := make([][3]float64, subtile*subtile)
		weights := make([]float64, subtile*subtile)
		var correction [3]float64
		totalWeight := 0.0
		for y := 0; y < subtile; y++ {
			for x := 0; x < subtile; x++ {
				i := y*subtile + x
				d := min(x, y, subtile-1-x, subtile-1-y)
				weight := clamp(float64(d-4)/8, 0, 1)
				weight = weight * weight * (3 - 2*weight)
				weights[i] = weight
				totalWeight += weight
				from := (((y+(variant/4)*16)%subtile)*subtile + (x+(variant%4)*16)%subtile) * 4
				blend := .2 + .65*variantNoise(float64(x)/17, float64(y)/17, uint32(variant))
				for k := 0; k < 3; k++ {
					detail := float64(base[from+k])
					if alternate != nil {
						scale := 0.0
						if altDeviation[k] > 1e-6 {
							scale = deviation[k] / altDeviation[k]
						}
						other := mean[k] + (float64(alternate[from+k])-altMean[k])*scale
						detail = detail*(1-blend) + other*blend
					}
					delta[i][k] = detail - float64(base[i*4+k])
					correction[k] += delta[i][k] * weight
				}
			}
		}
		// A phase shift under a fixed edge envelope can still bias the interior.
		// Remove that DC change before clamping/rounding, with zero change at edges.
		for i := range delta {
			for k := 0; k < 3; k++ {
				value := float64(base[i*4+k]) + (delta[i][k]-correction[k]/totalWeight)*weights[i]
				s.pixels[((16+variant)*subtile*subtile+i)*4+k] = byte(clamp(value, 0, 255) + .5)
			}
		}
	}
	refreshLow(s)
}

func rotateMaterialRGB(s *sheet, turns int) {
	if turns == 0 {
		return
	}
	original := append([]byte(nil), s.pixels...)
	detail := append([]float64(nil), s.detail...)
	for layer := 0; layer < 32; layer++ {
		for y := 0; y < subtile; y++ {
			for x := 0; x < subtile; x++ {
				sx, sy := x, y
				switch turns {
				case 1:
					sx, sy = subtile-1-y, x
				case 2:
					sx, sy = subtile-1-x, subtile-1-y
				case 3:
					sx, sy = y, subtile-1-x
				}
				to, from := (layer*subtile*subtile+y*subtile+x)*4, (layer*subtile*subtile+sy*subtile+sx)*4
				copy(s.pixels[to:to+3], original[from:from+3])
				if layer == 0 && len(detail) == subtile*subtile {
					s.detail[y*subtile+x] = detail[sy*subtile+sx]
				}
			}
		}
	}
	refreshLow(s)
}

// Smooth deterministic clumps, shared in texture space rather than a cell grid.
func variantNoise(x, y float64, seed uint32) float64 {
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	u, v := x-float64(ix), y-float64(iy)
	u = u * u * (3 - 2*u)
	v = v * v * (3 - 2*v)
	value := func(x, y int) float64 {
		h := uint32(x)*0x9e3779b9 ^ uint32(y)*0x85ebca6b ^ seed*0xc2b2ae35
		h ^= h >> 16
		h *= 0x7feb352d
		h ^= h >> 15
		return float64(h&65535) / 65535
	}
	return (value(ix, iy)*(1-u)+value(ix+1, iy)*u)*(1-v) + (value(ix, iy+1)*(1-u)+value(ix+1, iy+1)*u)*v
}
