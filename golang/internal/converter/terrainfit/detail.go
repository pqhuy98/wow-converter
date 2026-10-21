package terrainfit

import (
	"context"
	"fmt"
	"path/filepath"
)

type detailWeights [4][subtile * subtile]float32

// With128cells/ADT, every native64px square matches one8K reference block.
// Learn a sharp periodic RGB function per family, shared by its masks/variants.
// The fixed native alpha masks still determine all mixing. Damped least squares
// fits against the source image; no WoW shader is installed into WC3.
func (f *fit) learnDetail(ctx context.Context, tiles []tileData, nx int) error {
	cache := make(map[int]*detailWeights)
	getWeights := func(l layers) *detailWeights {
		if cached := cache[l.key]; cached != nil {
			return cached
		}
		out := new(detailWeights)
		for p := 0; p < subtile*subtile; p++ {
			u, v := (float64(p%subtile)+.5)/subtile, (float64(p/subtile)+.5)/subtile
			var a [4]float64
			a[0] = 1
			for j := 1; j < l.count; j++ {
				a[j] = float64(byte(maskAlpha(l.masks[j], u, v)*255+.5)) / 255
			}
			w := shaderWeights(a, l.count)
			for j := 0; j < l.count; j++ {
				out[j][p] = float32(w[j])
			}
		}
		cache[l.key] = out
		return out
	}
	colors := make([][]rgb, len(f.sheets))
	for id := range colors {
		colors[id] = make([]rgb, subtile*subtile)
		for p := range colors[id] {
			at := (16*subtile*subtile + p) * 4
			for k := range colors[id][p] {
				colors[id][p][k] = float32(f.sheets[id].pixels[at+k])
			}
		}
	}
	for pass := 0; pass < 5; pass++ {
		numerator := make([][]rgb, len(colors))
		denominator := make([][]float32, len(colors))
		for id := range colors {
			numerator[id] = make([]rgb, subtile*subtile)
			denominator[id] = make([]float32, subtile*subtile)
		}
		for ti, t := range tiles {
			if e := ctx.Err(); e != nil {
				return e
			}
			img, e := loadPNG(filepath.Join(t.dir, "wow.png"))
			if e != nil {
				return e
			}
			for y := 0; y < cellsPerADT; y++ {
				for x := 0; x < cellsPerADT; x++ {
					gx, gy := ti%nx*cellsPerADT+x, ti/nx*cellsPerADT+y
					// Deterministic rotating subsets bound fitting cost and cover5/8cells.
					if (gy*f.width+gx)%8 != pass || !f.valid[gy*f.width+gx] {
						continue
					}
					l := f.cellLayers(gx, gy)
					w := getWeights(l)
					for p := 0; p < subtile*subtile; p++ {
						at := (y*subtile+p/subtile)*img.Stride + (x*subtile+p%subtile)*4
						delta := rgb{float32(img.Pix[at]), float32(img.Pix[at+1]), float32(img.Pix[at+2])}
						for j := 0; j < l.count; j++ {
							c := colors[l.ids[j]][p]
							for k := range delta {
								delta[k] -= w[j][p] * c[k]
							}
						}
						for j := 0; j < l.count; j++ {
							id := l.ids[j]
							weight := w[j][p]
							denominator[id][p] += weight * weight
							for k := range delta {
								numerator[id][p][k] += weight * delta[k]
							}
						}
					}
				}
			}
		}
		for id := range colors {
			for p := range colors[id] {
				d := denominator[id][p]
				if d < 1e-5 {
					continue
				}
				for k := range colors[id][p] {
					colors[id][p][k] = float32(clamp(float64(colors[id][p][k]+.4*numerator[id][p][k]/d), 0, 255))
				}
			}
		}
		fmt.Printf("  periodic detail least-squares pass%d/5\n", pass+1)
	}
	for id := range f.sheets {
		for layer := 0; layer < 32; layer++ {
			for p, c := range colors[id] {
				at := (layer*subtile*subtile + p) * 4
				for k := range c {
					f.sheets[id].pixels[at+k] = byte(c[k] + .5)
				}
			}
		}
		refreshLow(&f.sheets[id])
	}
	for i := range f.variants {
		f.variants[i] = 0
	}
	return nil
}

func (f *fit) learnVariations(ctx context.Context, tiles []tileData, nx int) error {
	selected := f.exemplars()
	for ti, t := range tiles {
		img, e := loadPNG(filepath.Join(t.dir, "wow.png"))
		if e != nil {
			return e
		}
		for id, pool := range selected {
			for vi, ex := range pool {
				if ex.x/cellsPerADT == ti%nx && ex.y/cellsPerADT == ti/nx {
					f.fillExemplar(id, vi+1, ex, img, ti%nx, ti/nx)
				}
			}
		}
	}
	for i := range f.sheets {
		refreshLow(&f.sheets[i])
	}
	return f.chooseVariants(ctx)
}
