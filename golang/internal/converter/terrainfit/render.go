package terrainfit

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/pqhuy98/wow-converter/internal/formats/blp"
)

func (f *fit) renderTile(tx, ty, size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		gy := (float64(y) + .5) * cellsPerADT / float64(size)
		cy := int(gy)
		v := gy - float64(cy)
		last := -1
		var l layers
		var variant int
		for x := 0; x < size; x++ {
			gx := (float64(x) + .5) * cellsPerADT / float64(size)
			cx := int(gx)
			u := gx - float64(cx)
			if cx != last {
				l = f.cellLayers(tx*cellsPerADT+cx, ty*cellsPerADT+cy)
				variant = int(f.variants[(ty*cellsPerADT+cy)*f.width+tx*cellsPerADT+cx])
				last = cx
			}
			var colors [4][4]float64
			for j := 0; j < l.count; j++ {
				layer := int(l.masks[j])
				if j == 0 {
					layer = 16 + variant
				}
				colors[j] = sampleSheet(&f.sheets[l.ids[j]], layer, u, v)
			}
			c := compositeLayers(colors, l.count, f.sequential)
			at := y*img.Stride + x*4
			for k := 0; k < 3; k++ {
				img.Pix[at+k] = byte(clamp(c[k], 0, 255) + .5)
			}
			img.Pix[at+3] = 255
		}
	}
	return img
}

func compositeLayers(colors [4][4]float64, count int, sequential bool) [4]float64 {
	if sequential {
		c := colors[0]
		for j := 1; j < count; j++ {
			a := colors[j][3]
			for k := range c {
				c[k] = c[k]*(1-a) + colors[j][k]*a
			}
		}
		return c
	}
	// Retain the historical HiveWE calculation for rejected cached stages.
	c := colors[count-1]
	for j := count - 2; j >= 0; j-- {
		a := c[3]
		for k := range c {
			c[k] = colors[j][k]*(1-a) + c[k]*a
		}
	}
	return c
}

func sampleSheet(s *sheet, layer int, u, v float64) [4]float64 {
	x, y := clamp(u*subtile-.5, 0, subtile-1), clamp(v*subtile-.5, 0, subtile-1)
	ix, iy := int(x), int(y)
	tx, ty := x-float64(ix), y-float64(iy)
	var out [4]float64
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			at := (layer*subtile*subtile + min(subtile-1, iy+j)*subtile + min(subtile-1, ix+i)) * 4
			w := [2]float64{1 - tx, tx}[i] * [2]float64{1 - ty, ty}[j]
			for k := range out {
				out[k] += float64(s.pixels[at+k]) * w
			}
		}
	}
	out[3] /= 255
	return out
}

func (f *fit) saveSheets(dir string) (int64, error) {
	if e := os.MkdirAll(dir, 0755); e != nil {
		return 0, e
	}
	var total int64
	for id := range f.sheets {
		img := image.NewNRGBA(image.Rect(0, 0, subtile*8, subtile*4))
		for layer := 0; layer < 32; layer++ {
			// HiveWE extracts all16 left-half masks, followed by16 right-half variants.
			x, y := (layer%4)*subtile, (layer%16/4)*subtile
			if layer >= 16 {
				x += subtile * 4
			}
			for row := 0; row < subtile; row++ {
				copy(img.Pix[(y+row)*img.Stride+x*4:], f.sheets[id].pixels[(layer*subtile*subtile+row*subtile)*4:(layer*subtile*subtile+(row+1)*subtile)*4])
			}
		}
		var b bytes.Buffer
		if e := png.Encode(&b, img); e != nil {
			return 0, e
		}
		name := filepath.Join(dir, fmt.Sprintf("tile-%02d", id))
		if e := os.WriteFile(name+".png", b.Bytes(), 0644); e != nil {
			return 0, e
		}
		if e := blp.ConvertTextureToBlp(blp.EncodeInput{PNG: b.Bytes(), PreserveAlpha: true}, name+".blp"); e != nil {
			return 0, e
		}
		raw, e := os.ReadFile(name + ".blp")
		if e != nil {
			return 0, e
		}
		total += int64(len(raw))
		decoded, e := blp.DecodePalettedBLP1(raw, 0)
		if e != nil {
			return 0, e
		}
		rgba := decoded.Pix
		for layer := 0; layer < 32; layer++ {
			x, y := (layer%4)*subtile, (layer%16/4)*subtile
			if layer >= 16 {
				x += subtile * 4
			}
			for row := 0; row < subtile; row++ {
				copy(f.sheets[id].pixels[(layer*subtile*subtile+row*subtile)*4:(layer*subtile*subtile+(row+1)*subtile)*4], rgba[((y+row)*subtile*8+x)*4:((y+row)*subtile*8+x+subtile)*4])
			}
		}
		refreshLow(&f.sheets[id])
	}
	return total, nil
}

type metrics struct {
	Pixels  int64   `json:"pixels"`
	MAE     float64 `json:"mae"`
	RMSE    float64 `json:"rmse"`
	PSNR    float64 `json:"psnr"`
	SSIM    float64 `json:"ssim"`
	P95     int     `json:"p95"`
	EdgeMAE float64 `json:"edgeMAE"`
}
type measurement struct {
	metrics
	abs, squared, edge   float64
	edgeN                int64
	ssimSum              float64
	ssimN                int64
	hist                 [256]int64
	cellSquared, cellAbs []float64
	cellPixels           []int64
}

func measure(ref, result *image.NRGBA, coverage []bool) measurement {
	w, h := ref.Rect.Dx(), ref.Rect.Dy()
	m := measurement{cellSquared: make([]float64, cellsPerADT*cellsPerADT), cellAbs: make([]float64, cellsPerADT*cellsPerADT), cellPixels: make([]int64, cellsPerADT*cellsPerADT)}
	// Non-overlapping8x8 luminance SSIM windows at the actual8K resolution.
	for by := 0; by < h; by += 8 {
		for bx := 0; bx < w; bx += 8 {
			var sx, sy, sxx, syy, sxy float64
			n := 0
			for y := by; y < min(h, by+8); y++ {
				for x := bx; x < min(w, bx+8); x++ {
					if !coverage[(y*128/h)*128+x*128/w] {
						continue
					}
					at := y*ref.Stride + x*4
					ci := (y*cellsPerADT/h)*cellsPerADT + x*cellsPerADT/w
					a, b := 0.0, 0.0
					for k := 0; k < 3; k++ {
						v1, v2 := float64(ref.Pix[at+k]), float64(result.Pix[at+k])
						d := v1 - v2
						ad := math.Abs(d)
						m.abs += ad
						m.squared += d * d
						m.hist[int(ad)]++
						m.cellSquared[ci] += d * d
						m.cellAbs[ci] += ad
						a += v1 * []float64{.2126, .7152, .0722}[k]
						b += v2 * []float64{.2126, .7152, .0722}[k]
					}
					sx += a
					sy += b
					sxx += a * a
					syy += b * b
					sxy += a * b
					n++
					m.Pixels++
					m.cellPixels[ci]++
					if x > 0 && y > 0 && coverage[(y*128/h)*128+(x-1)*128/w] && coverage[((y-1)*128/h)*128+x*128/w] {
						for _, off := range []int{4, ref.Stride} {
							for k := 0; k < 3; k++ {
								g1 := int(ref.Pix[at+k]) - int(ref.Pix[at+k-off])
								g2 := int(result.Pix[at+k]) - int(result.Pix[at+k-off])
								m.edge += math.Abs(float64(g1 - g2))
								m.edgeN++
							}
						}
					}
				}
			}
			if n > 1 {
				nn := float64(n)
				mx, my := sx/nn, sy/nn
				vx, vy := max(0, sxx/nn-mx*mx), max(0, syy/nn-my*my)
				cov := sxy/nn - mx*my
				c1, c2 := 6.5025, 58.5225
				m.ssimSum += ((2*mx*my + c1) * (2*cov + c2)) / ((mx*mx + my*my + c1) * (vx + vy + c2))
				m.ssimN++
			}
		}
	}
	m.finish()
	return m
}
func (m *measurement) finish() {
	if m.Pixels == 0 {
		return
	}
	n := float64(m.Pixels * 3)
	m.MAE = m.abs / n
	m.RMSE = math.Sqrt(m.squared / n)
	m.PSNR = 100
	if m.RMSE > 0 {
		m.PSNR = 20 * math.Log10(255/m.RMSE)
	}
	if m.edgeN > 0 {
		m.EdgeMAE = m.edge / float64(m.edgeN)
	}
	if m.ssimN > 0 {
		m.SSIM = m.ssimSum / float64(m.ssimN)
	}
	var sum int64
	for i, v := range m.hist {
		sum += v
		if float64(sum) >= n*.95 {
			m.P95 = i
			break
		}
	}
}
func (m *measurement) add(o measurement) {
	m.Pixels += o.Pixels
	m.abs += o.abs
	m.squared += o.squared
	m.edge += o.edge
	m.edgeN += o.edgeN
	m.ssimSum += o.ssimSum
	m.ssimN += o.ssimN
	for i, v := range o.hist {
		m.hist[i] += v
	}
	m.finish()
}

func loadPNG(path string) (*image.NRGBA, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	img, e := png.Decode(f)
	if e != nil {
		return nil, e
	}
	if n, ok := img.(*image.NRGBA); ok {
		return n, nil
	}
	out := image.NewNRGBA(img.Bounds())
	draw.Draw(out, out.Bounds(), img, image.Point{}, draw.Src)
	return out, nil
}
func savePNG(path string, img image.Image) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	return encoder.Encode(f, img)
}
