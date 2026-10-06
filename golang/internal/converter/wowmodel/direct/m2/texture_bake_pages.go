package directm2

import (
	"image"
	"math"

	imath "github.com/pqhuy98/wow-converter/internal/math"
)

type bakePageLayout struct {
	cellW, cellH  int
	width, height int
	cols, rows    int
}

func (p bakePageLayout) capacity() int { return p.cols * p.rows }

func (p bakePageLayout) translation(frame int) imath.Vector3 {
	frame %= p.capacity()
	return imath.Vector3{float64(frame%p.cols*p.cellW) / float64(p.width), float64(frame/p.cols*p.cellH) / float64(p.height), 0}
}

// Keep frame origins aligned through the first three mip levels. Their padded
// chart pixels are unchanged; only unused space beyond the chart bounds goes.
// Uniform page dimensions let one UV track serve every page without scaling.
func planBakePages(charts []bakeChart, frames int) bakePageLayout {
	w, h := 1, 1
	for _, chart := range charts {
		w, h = max(w, chart.x+chart.width), max(h, chart.y+chart.height)
	}
	w, h = (w+7)/8*8, (h+7)/8*8
	minW, minH := bakePowerOfTwo(w), bakePowerOfTwo(h)
	best := bakePageLayout{}
	bestPixels, bestPages := math.MaxInt, math.MaxInt
	for pageW := minW; pageW <= max(2048, minW); pageW *= 2 {
		for pageH := minH; pageH <= max(2048, minH); pageH *= 2 {
			p := bakePageLayout{cellW: w, cellH: h, width: pageW, height: pageH, cols: pageW / w, rows: pageH / h}
			pages := (frames + p.capacity() - 1) / p.capacity()
			pixels := pages * pageW * pageH
			if pixels < bestPixels || pixels == bestPixels && (pages < bestPages || pages == bestPages && max(pageW, pageH) < max(best.width, best.height)) {
				best, bestPixels, bestPages = p, pixels, pages
			}
		}
	}
	return best
}

// Downsample straight-alpha colors without darkening covered edges. Some bake
// outputs intentionally use RGB independently of alpha, including zero-alpha
// radiance; their caller opts out of alpha weighting. UVs remain normalized.
func halfBakeTexture(src *image.NRGBA, independentRGB bool) *image.NRGBA {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, max(1, w/2), max(1, h/2)))
	for y := range dst.Bounds().Dy() {
		for x := range dst.Bounds().Dx() {
			sumAlpha, count := 0, 0
			var sumRGB, sumPremultiplied [3]int
			for sy := y * 2; sy < min(y*2+2, h); sy++ {
				for sx := x * 2; sx < min(x*2+2, w); sx++ {
					at := sy*src.Stride + sx*4
					a := int(src.Pix[at+3])
					sumAlpha += a
					for c := range 3 {
						rgb := int(src.Pix[at+c])
						sumRGB[c] += rgb
						sumPremultiplied[c] += rgb * a
					}
					count++
				}
			}
			at := y*dst.Stride + x*4
			alpha := (sumAlpha + count/2) / count
			dst.Pix[at+3] = uint8(alpha)
			for c := range 3 {
				if independentRGB {
					dst.Pix[at+c] = uint8((sumRGB[c] + count/2) / count)
				} else if sumAlpha > 0 {
					dst.Pix[at+c] = uint8((sumPremultiplied[c] + sumAlpha/2) / sumAlpha)
				}
			}
		}
	}
	return dst
}

// These secondary layers use additive blending and never write depth. With
// zero RGB they contribute zero for every alpha/UV, so a black texel suffices.
// Do not use this for the black attenuation pass: its alpha affects the scene.
func compactBakeRadiance(src *image.NRGBA) *image.NRGBA {
	for y := range src.Bounds().Dy() {
		for x := range src.Bounds().Dx() {
			at := y*src.Stride + x*4
			if src.Pix[at] != 0 || src.Pix[at+1] != 0 || src.Pix[at+2] != 0 {
				return src
			}
		}
	}
	return image.NewNRGBA(image.Rect(0, 0, 1, 1))
}
