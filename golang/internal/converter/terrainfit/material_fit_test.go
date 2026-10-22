package terrainfit

import (
	"bytes"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestMaterialMasksKeepSharedEdgesIndependentOfOtherCorners(t *testing.T) {
	palette := [4][3]float64{{0, 0, 0}, {255, 0, 0}, {0, 255, 0}, {0, 0, 255}}
	evaluate := func(f *fit, x int, u, v float64) [4]float64 {
		l := f.cellLayers(x, 0)
		var colors [4][4]float64
		for j := 0; j < l.count; j++ {
			copy(colors[j][:3], palette[l.ids[j]][:])
			colors[j][3] = 1
			if j > 0 {
				colors[j][3] = materialMaskAlpha(l.masks[j], u, v)
			}
		}
		return compositeLayers(colors, l.count, true)
	}
	// All 4^6 assignments of a two-cell neighborhood, including three/four-way
	// junctions with a different lowest-priority material in each cell.
	f := &fit{width: 2, height: 1, labels: make([]uint8, 6)}
	for code := 0; code < 4096; code++ {
		for i := range f.labels {
			f.labels[i] = uint8((code >> (2 * i)) & 3)
		}
		for j := 0; j <= 16; j++ {
			v := float64(j) / 16
			left, right := evaluate(f, 0, 1, v), evaluate(f, 1, 0, v)
			for k := 0; k < 3; k++ {
				if math.Abs(left[k]-right[k]) > 1e-9 {
					t.Fatalf("shared-edge discontinuity: labels %v, v %g: %v / %v", f.labels, v, left, right)
				}
			}
		}
	}
	var colors [4][4]float64
	colors[0], colors[1], colors[2] = [4]float64{0, 0, 0, 1}, [4]float64{100, 0, 0, .75}, [4]float64{0, 100, 0, .5}
	if got := compositeLayers(colors, 3, true); got[0] != 37.5 || got[1] != 50 {
		t.Fatalf("overlays must use their own alpha: %v", got)
	}
}

func TestMaterialPaletteRepeatsMaterialsWithoutWorldFragments(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "materials", "test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for id, file := range []string{"1.png", "2.png"} {
		img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				at := y*img.Stride + x*4
				color := [3]byte{byte(x * 2), 100, 20}
				if id == 1 {
					color = [3]byte{220, 180, 100}
				}
				copy(img.Pix[at:at+3], color[:])
				img.Pix[at+3] = 255
			}
		}
		if err := savePNG(filepath.Join(dir, file), img); err != nil {
			t.Fatal(err)
		}
	}
	f := &fit{}
	if err := f.makeMaterialSheets(root, "test", []terrainMaterial{{ID: 1, Scale: 1}, {ID: 2, Scale: 1}}, [][]float32{{.5, .5, .5, .5, .5}}, false); err != nil {
		t.Fatal(err)
	}
	for layer := 0; layer < 32; layer++ {
		at := (layer*subtile*subtile + 23*subtile + 17) * 4
		for k, want := range []byte{127, 140, 60} {
			if got := f.sheets[0].pixels[at+k]; got != want {
				t.Fatalf("layer %d channel %d: got %d want %d", layer, k, got, want)
			}
		}
	}
}

func TestMaterialDictionaryPreservesSeparateLayerIdentities(t *testing.T) {
	points := []float32{1, 0, .5, .5, .5, 0, 1, .5, .5, .5, 1, 0, .5, .5, .5, 0, 1, .5, .5, .5}
	centers := clusterMaterialFeatures(points, 5, 64)
	if len(centers) != 2 || nearestMaterialFeature(points[:5], centers) == nearestMaterialFeature(points[5:10], centers) {
		t.Fatalf("material identities merged or duplicate slots allocated: %v", centers)
	}
	alpha := make([]byte, 4096)
	for i := range alpha {
		alpha[i] = 127
	}
	for _, uv := range [][2]float64{{0, 0}, {1, 1}, {.5, .5}} {
		if got := sampleMaterialAlpha(alpha, uv[0], uv[1]); math.Abs(got-127.0/255) > 1e-9 {
			t.Fatalf("authored paint sampling: %g", got)
		}
	}
}

func TestSourceBlockMeshUsesContinuousWorldUV(t *testing.T) {
	m := Mesh{Positions: [][3]float64{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}}, Indices: []uint32{0, 1, 2, 2, 1, 3}}
	mask := make([]bool, cellsPerADT*cellsPerADT)
	mask[17*cellsPerADT+9] = true
	_, groups := clipPatches(m, mask, []patchPlacement{{x: 9, y: 17, x0: 576, y0: 1088, sx: 64, sy: 64}}, 512, 0)
	if len(groups) != 1 || len(groups[0].UV) == 0 {
		t.Fatal("missing source block")
	}
	for i, p := range groups[0].Mesh.Positions {
		want := [2]float64{(p[0]*8192 - 512) / 512, (p[1]*8192 - 1024) / 512}
		if math.Abs(groups[0].UV[i][0]-want[0]) > 1e-9 || math.Abs(groups[0].UV[i][1]-want[1]) > 1e-9 {
			t.Fatal("unrelated cell coordinates mixed into source-block UV")
		}
	}
}

func TestExportInterpolatesMixturesBeforeQuantization(t *testing.T) {
	src := []float32{1, 0, 0, 1, 1, 0, 0, 1}
	grid := TilesetGrid{Width: 2, Height: 1, StepX: [2]float64{.5 / 128, 0}, StepY: [2]float64{0, 1.0 / 128}}
	got := resampleMaterialFeatures(src, 2, 1, 1, grid)
	if got[2] != .5 || got[3] != .5 {
		t.Fatalf("fractional mixture became a square repeated label: %v", got)
	}
}

func TestExportRGBRotationPreservesNativeMasks(t *testing.T) {
	s := sheet{pixels: make([]byte, 32*subtile*subtile*4)}
	for layer := 0; layer < 32; layer++ {
		for y := 0; y < subtile; y++ {
			for x := 0; x < subtile; x++ {
				at := (layer*subtile*subtile + y*subtile + x) * 4
				s.pixels[at] = byte(x)
				s.pixels[at+1] = byte(y)
				s.pixels[at+3] = byte((x + 2*y + layer) % 256)
			}
		}
	}
	before := append([]byte(nil), s.pixels...)
	rotateMaterialRGB(&s, 1)
	at := (10*subtile*subtile + 12*subtile + 7) * 4
	if s.pixels[at] != 51 || s.pixels[at+1] != 7 {
		t.Fatal("RGB orientation wrong")
	}
	for i := 3; i < len(before); i += 4 {
		if s.pixels[i] != before[i] {
			t.Fatal("rotated native alpha mask")
		}
	}
}

func TestEveryBrushVariationIsFilledAndHasSafeDistinctDetail(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			at := y*img.Stride + x*4
			img.Pix[at] = byte(x * 4)
			img.Pix[at+1] = byte(y * 4)
			img.Pix[at+2] = byte((x*7 + y*13) % 256)
			img.Pix[at+3] = 255
		}
	}
	f := &fit{}
	if err := f.makeMaterialSheetsFrom([]terrainMaterial{{ID: 1, Scale: 1}}, [][]float32{{1, .5, .5, .5}}, false, map[uint32]*image.NRGBA{1: img}); err != nil {
		t.Fatal(err)
	}
	makeExportVariants(&f.sheets[0])
	s := &f.sheets[0]
	seen := map[string]bool{}
	for variant := 0; variant < 32; variant++ {
		layer := 0
		if variant <= 15 {
			layer = 16 + variant
		} else if variant == 16 {
			layer = 15
		}
		for at := layer*64*64*4 + 3; at < (layer+1)*64*64*4; at += 4 {
			if s.pixels[at] != 255 {
				t.Fatalf("brush variation %d has an empty pixel", variant)
			}
		}
		if variant < 16 {
			frame := s.pixels[layer*64*64*4 : (layer+1)*64*64*4]
			seen[string(frame)] = true
			for y := 0; y < 64; y++ {
				for x := 0; x < 64; x++ {
					if x < 4 || y < 4 || x >= 60 || y >= 60 {
						at := (y*64 + x) * 4
						base := 16*64*64*4 + at
						for k := 0; k < 3; k++ {
							if frame[at+k] != s.pixels[base+k] {
								t.Fatal("variant introduced a boundary seam")
							}
						}
					}
				}
			}
		}
	}
	if len(seen) != 16 {
		t.Fatalf("only %d distinct full variants", len(seen))
	}
}

func TestExportPaletteGroupsColorsAndRemapsPaint(t *testing.T) {
	f := &fit{labels: []uint8{0, 1, 2}, sheets: make([]sheet, 3)}
	for i, v := range []float32{120, 220, 30} {
		for p := range f.sheets[i].low[16] {
			f.sheets[i].low[16][p] = rgb{v, v, v}
		}
	}
	sortExportPalette(f, [][]float32{{1, .5, .5, .5}, {1, .5, .5, .5}, {1, .5, .5, .5}})
	if f.labels[0] != 1 || f.labels[1] != 2 || f.labels[2] != 0 {
		t.Fatalf("paint did not follow sorted swatches: %v", f.labels)
	}
	if f.sheets[0].low[16][0][0] != 30 || f.sheets[2].low[16][0][0] != 220 {
		t.Fatal("similar shades were not grouped")
	}
}

func TestMaterialContoursFollowSourceDetailAndPreserveEdges(t *testing.T) {
	s := sheet{pixels: make([]byte, 32*subtile*subtile*4), detail: make([]float64, subtile*subtile)}
	for y := 0; y < subtile; y++ {
		for x := 0; x < subtile; x++ {
			u, v := (float64(x)+.5)/subtile, (float64(y)+.5)/subtile
			s.detail[y*subtile+x] = math.Sin(float64(x)*math.Pi*2/subtile) + .2*math.Cos(float64(y)*math.Pi*2/subtile)
			for layer := 0; layer < 32; layer++ {
				at := (layer*subtile*subtile+y*subtile+x)*4 + 3
				s.pixels[at] = 255
				if layer > 0 && layer < 15 {
					s.pixels[at] = byte(materialMaskAlpha(uint8(layer), u, v)*255 + .5)
				}
			}
		}
	}
	before := append([]byte(nil), s.pixels...)
	naturalizeExportMasks(&s)
	low, high := 255, 0
	for x := 8; x < 56; x++ {
		alpha := int(s.pixels[(3*subtile*subtile+16*subtile+x)*4+3])
		low = min(low, alpha)
		high = max(high, alpha)
	}
	if high-low < 20 {
		t.Fatalf("source contour remained straight: alpha range %d..%d", low, high)
	}
	for layer := 0; layer < 32; layer++ {
		for y := 0; y < subtile; y++ {
			for x := 0; x < subtile; x++ {
				if x == 0 || y == 0 || x == subtile-1 || y == subtile-1 || layer == 0 || layer >= 15 {
					at := (layer*subtile*subtile+y*subtile+x)*4 + 3
					if s.pixels[at] != before[at] {
						t.Fatalf("changed shared edge/full variant %d at %d,%d", layer, x, y)
					}
				}
			}
		}
	}
}

// Exercise the real PNG->paletted BLP encoder. Dithering and each mip can
// introduce seams even when the original uncompressed pixels were equal.
func TestExportedVariantsHaveExactOppositeEdgesAtEveryMip(t *testing.T) {
	s := sheet{pixels: make([]byte, 32*subtile*subtile*4)}
	for layer := 0; layer < 32; layer++ {
		for y := 0; y < subtile; y++ {
			for x := 0; x < subtile; x++ {
				at := (layer*subtile*subtile + y*subtile + x) * 4
				s.pixels[at] = byte(x*3 + y)
				s.pixels[at+1] = byte(x + y*3)
				s.pixels[at+2] = byte((x*17 + y*5) % 256)
				s.pixels[at+3] = 255
				if layer > 0 && layer < 15 {
					s.pixels[at+3] = byte(materialMaskAlpha(uint8(layer), (float64(x)+.5)/subtile, (float64(y)+.5)/subtile)*255 + .5)
				}
			}
		}
	}
	periodicExportRGB(&s)
	makeExportVariants(&s)
	f := &fit{sheets: []sheet{s}}
	dir := t.TempDir()
	if _, err := f.saveSheets(dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "tile-00.blp"))
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), raw...)
	if err := sealEncodedTileBorders(raw); err != nil {
		t.Fatal(err)
	}
	for level := 0; level <= 6; level++ {
		img, err := blp.DecodePalettedBLP1(raw, level)
		if err != nil {
			t.Fatal(err)
		}
		prior, err := blp.DecodePalettedBLP1(before, level)
		if err != nil {
			t.Fatal(err)
		}
		for i := 3; i < len(img.Pix); i += 4 {
			if img.Pix[i] != prior.Pix[i] {
				t.Fatalf("changed alpha at mip %d", level)
			}
		}
		side := subtile >> level
		frame := func(layer, x, y int) []byte {
			xx := (layer%4)*side + x
			if layer >= 16 {
				xx += 4 * side
			}
			yy := layer%16/4*side + y
			at := yy*img.Stride + xx*4
			return img.Pix[at : at+3]
		}
		for a := 16; a < 32; a++ {
			for b := 16; b < 32; b++ {
				for i := 0; i < side; i++ {
					if !bytes.Equal(frame(a, side-1, i), frame(b, 0, i)) || !bytes.Equal(frame(a, i, side-1), frame(b, i, 0)) {
						t.Fatalf("mip %d, variants %d/%d have an RGB seam", level, a-16, b-16)
					}
				}
			}
		}
	}
}

func TestNativeFitHonorsAvailablePaintBudgetAndKeepsAlphaLast(t *testing.T) {
	capture := materialCapture{Version: 2, Materials: []terrainMaterial{{ID: 1, Scale: 1}}, Chunks: make([]materialChunk, 256)}
	for i := range capture.Chunks {
		capture.Chunks[i] = materialChunk{Layers: []int{0}, Tint: make([]rgb, 145)}
		for j := range capture.Chunks[i].Tint {
			capture.Chunks[i].Tint[j] = rgb{float32(i%16) / 15, float32(i/16) / 15, 1}
		}
	}
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	grid := TilesetGrid{Width: 8, Height: 8, StepX: [2]float64{.125, 0}, StepY: [2]float64{0, .125}, PaintFamilies: 3}
	f, _, centers, err := fitCaptures([]materialCapture{capture}, 1, 1, map[uint32]*image.NRGBA{1: img}, &grid)
	if err != nil {
		t.Fatal(err)
	}
	if len(centers) != 3 || len(f.sheets) != 4 {
		t.Fatalf("budget ignored or alpha missing: centers=%d sheets=%d", len(centers), len(f.sheets))
	}
	for _, id := range f.labels {
		if id >= 3 {
			t.Fatalf("paint references alpha/unavailable tile %d", id)
		}
	}
	for _, v := range f.sheets[3].pixels {
		if v != 0 {
			t.Fatal("last sheet must be alpha")
		}
	}
	grid.PaintFamilies = 64
	if _, _, _, err := fitCaptures([]materialCapture{capture}, 1, 1, map[uint32]*image.NRGBA{1: img}, &grid); err == nil {
		t.Fatal("accepted palette without space for alpha")
	}
}

// The actual Northrend regression was white snow with 13..20 RGB levels of
// darker variant interior, fenced by unchanged white edge pixels.
func TestVariantsDoNotInventBoxesOnUniformSnow(t *testing.T) {
	makeSheet := func(color [3]byte) sheet {
		s := sheet{pixels: make([]byte, 32*subtile*subtile*4)}
		for layer := 0; layer < 32; layer++ {
			for i := 0; i < subtile*subtile; i++ {
				at := (layer*subtile*subtile + i) * 4
				copy(s.pixels[at:at+3], color[:])
				s.pixels[at+3] = 255
			}
		}
		return s
	}
	snow := makeSheet([3]byte{241, 249, 253})
	darker := makeSheet([3]byte{170, 183, 191})
	makeExportVariants(&snow, &darker)
	for layer := 16; layer < 32; layer++ {
		for i := 0; i < subtile*subtile; i++ {
			for k, want := range [3]byte{241, 249, 253} {
				if got := snow.pixels[(layer*subtile*subtile+i)*4+k]; got != want {
					t.Fatalf("invented snow shading: variant=%d pixel=%d channel=%d got=%d want=%d", layer-16, i, k, got, want)
				}
			}
		}
	}
}

func TestVariantInteriorPreservesSnowBrightnessWithSourceDetail(t *testing.T) {
	s := sheet{pixels: make([]byte, 32*subtile*subtile*4)}
	alt := sheet{pixels: make([]byte, len(s.pixels))}
	for layer := 0; layer < 32; layer++ {
		for y := 0; y < subtile; y++ {
			for x := 0; x < subtile; x++ {
				at := (layer*subtile*subtile + y*subtile + x) * 4
				for k := 0; k < 3; k++ {
					s.pixels[at+k] = byte(235 + k*3 + (x*7+y*13)%9)
					alt.pixels[at+k] = byte(160 + k*5 + (x*3+y*11)%35)
				}
				s.pixels[at+3] = 255
				alt.pixels[at+3] = 255
			}
		}
	}
	makeExportVariants(&s, &alt)
	var base [3]float64
	for i := 0; i < subtile*subtile; i++ {
		for k := range base {
			base[k] += float64(s.pixels[(16*subtile*subtile+i)*4+k]) / (subtile * subtile)
		}
	}
	for layer := 17; layer < 32; layer++ {
		var mean [3]float64
		for i := 0; i < subtile*subtile; i++ {
			for k := range mean {
				mean[k] += float64(s.pixels[(layer*subtile*subtile+i)*4+k]) / (subtile * subtile)
			}
		}
		for k := range mean {
			if math.Abs(mean[k]-base[k]) > .1 {
				t.Fatalf("variant %d changed tint %v -> %v", layer-16, base, mean)
			}
		}
	}
}

func TestExportPreservesAuthoredDetailPhaseAcrossCells(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, subtile, subtile))
	for y := 0; y < subtile; y++ {
		for x := 0; x < subtile; x++ {
			at := y*img.Stride + x*4
			img.Pix[at], img.Pix[at+1], img.Pix[at+2], img.Pix[at+3] = byte(x*4), byte(y*4), byte((x*7+y*13)%256), 255
		}
	}
	f := &fit{width: 128, height: 128, labels: make([]uint8, 129*129), variants: make([]uint8, 128*128), sequential: true}
	centers := [][]float32{{1, .5, .5, .5}}
	if err := f.makeMaterialSheetsFrom([]terrainMaterial{{ID: 1, Scale: 1}}, centers, false, map[uint32]*image.NRGBA{1: img}); err != nil {
		t.Fatal(err)
	}
	f.sheets = append(f.sheets, sheet{pixels: make([]byte, 32*subtile*subtile*4)})
	f.prepareExportSheets(centers, TilesetGrid{StepX: [2]float64{1, 0}})
	tileset := Tileset{Width: 128, Height: 128, Variants: f.variants, sampler: f}
	want := tileset.Sample(.37, .63)
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			got := tileset.Sample(float64(x)+.37, float64(y)+.63)
			for k := 0; k < 3; k++ {
				if math.Abs(got[k]-want[k]) > 1e-9 {
					t.Fatalf("source detail shifted at cell %d,%d: %v -> %v", x, y, want, got)
				}
			}
		}
	}
}
