package terrainfit

import (
	"context"
	"fmt"
	"image"
	"math"
	"sort"
)

// Align native squares to WoW's128 base texture repeats per ADT. This avoids
// spending variants on arbitrary subtexel phases;3x2 selections use384x256cells.
const cellsPerADT = 128
const samples = 8
const subtile = 64
const sampleCount = samples * samples

type rgb [3]float32
type sheet struct {
	pixels []byte
	low    [32][sampleCount]rgb
}
type fit struct {
	width, height int
	reference     []byte // samples x samples per cell; contiguous regional image
	valid         []bool
	labels        []uint8 // shared corners, image rows run top to bottom
	variants      []uint8 // cell base variation; serialized at its lower-left corner
	sheets        []sheet
	colors        []rgb
	sequential    bool // newer candidates use each overlay's own alpha
}

type layers struct {
	ids        [4]uint8
	masks      [4]uint8
	count, key int
}
type coefficients [4][sampleCount]float32

var weights = makeWeights()

func (f *fit) cellLayers(x, y int) layers {
	stride := f.width + 1
	corners := [4]uint8{f.labels[(y+1)*stride+x], f.labels[(y+1)*stride+x+1], f.labels[y*stride+x+1], f.labels[y*stride+x]}
	ids := corners
	for i := 1; i < 4; i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	l := layers{}
	for _, id := range ids {
		if l.count == 0 || l.ids[l.count-1] != id {
			l.ids[l.count] = id
			l.count++
		}
	}
	for c, id := range corners {
		for i := 0; i < l.count; i++ {
			if l.ids[i] == id {
				l.key |= i << (c * 2)
				l.masks[i] |= []uint8{2, 1, 4, 8}[c]
				break
			}
		}
	}
	return l
}

func makeWeights() [256]coefficients {
	var out [256]coefficients
	for key := range out {
		var masks [4]uint8
		count := 0
		for c := 0; c < 4; c++ {
			rank := (key >> (2 * c)) & 3
			count = max(count, rank+1)
			masks[rank] |= []uint8{2, 1, 4, 8}[c]
		}
		for p := 0; p < sampleCount; p++ {
			u, v := (float64(p%samples)+.5)/samples, (float64(p/samples)+.5)/samples
			var a [4]float64
			a[0] = 1
			for j := 1; j < count; j++ {
				a[j] = maskAlpha(masks[j], u, v)
			}
			coeff := shaderWeights(a, count)
			for j := 0; j < count; j++ {
				out[key][j][p] = float32(coeff[j])
			}
		}
	}
	return out
}

// UV here is top-left image UV. Bits follow HiveWE's bottom-right, bottom-left,
// top-right, top-left order. Generated masks are bilinear corner influence.
func maskAlpha(mask uint8, u, v float64) float64 {
	a := 0.0
	for i, w := range [4]float64{u * v, (1 - u) * v, u * (1 - v), (1 - u) * (1 - v)} {
		if mask&(1<<i) != 0 {
			a += w
		}
	}
	return a
}

// Match terrain.frag's recursive mix(..., color, color.a), INCLUDING alpha.
// Ordinary premultiplied-over produces different coefficients for 3/4 layers.
func shaderWeights(alpha [4]float64, count int) [4]float64 {
	var w [4]float64
	w[count-1] = 1
	a := alpha[count-1]
	for j := count - 2; j >= 0; j-- {
		for k := j + 1; k < count; k++ {
			w[k] *= a
		}
		w[j] = 1 - a
		a = alpha[j]*(1-a) + a*a
	}
	return w
}

func newFit(width, height int, reference []byte, valid []bool, count int) *fit {
	f := &fit{width: width, height: height, reference: reference, valid: valid, labels: make([]uint8, (width+1)*(height+1)), variants: make([]uint8, width*height)}
	points := make([]rgb, 0, (width+1)*(height+1)/3)
	for y := 0; y <= height; y++ {
		for x := 0; x <= width; x++ {
			if (y*(width+1)+x)%3 == 0 {
				points = append(points, f.cornerColor(x, y))
			}
		}
	}
	f.colors = clusterColors(points, count)
	f.sheets = make([]sheet, count)
	for i, c := range f.colors {
		s := sheet{pixels: make([]byte, 32*subtile*subtile*4)}
		for layer := 0; layer < 32; layer++ {
			for p := 0; p < subtile*subtile; p++ {
				at := (layer*subtile*subtile + p) * 4
				for k := 0; k < 3; k++ {
					s.pixels[at+k] = byte(clamp(float64(c[k]), 0, 255) + .5)
				}
				a := 1.0
				if layer < 16 {
					a = maskAlpha(uint8(layer), (float64(p%subtile)+.5)/subtile, (float64(p/subtile)+.5)/subtile)
				}
				s.pixels[at+3] = byte(a*255 + .5)
			}
		}
		refreshLow(&s)
		f.sheets[i] = s
	}
	for y := 0; y <= height; y++ {
		for x := 0; x <= width; x++ {
			f.labels[y*(width+1)+x] = uint8(nearest(f.cornerColor(x, y), f.colors))
		}
	}
	return f
}

func (f *fit) cornerColor(x, y int) rgb {
	var c rgb
	n := float32(0)
	w := f.width * samples
	h := f.height * samples
	for j := -2; j <= 1; j++ {
		for i := -2; i <= 1; i++ {
			px, py := min(w-1, max(0, x*samples+i)), min(h-1, max(0, y*samples+j))
			if !f.valid[(py/samples)*f.width+px/samples] {
				continue
			}
			at := (py*w + px) * 3
			for k := 0; k < 3; k++ {
				c[k] += float32(f.reference[at+k])
			}
			n++
		}
	}
	if n > 0 {
		for k := range c {
			c[k] /= n
		}
	}
	return c
}

func clusterColors(points []rgb, count int) []rgb {
	out := []rgb{points[len(points)/2]}
	dist := make([]float32, len(points))
	for i := range dist {
		dist[i] = math.MaxFloat32
	}
	for len(out) < count {
		best := 0
		for i, p := range points {
			dist[i] = min(dist[i], distance(p, out[len(out)-1]))
			if dist[i] > dist[best] {
				best = i
			}
		}
		out = append(out, points[best])
	}
	for pass := 0; pass < 6; pass++ {
		sum := make([]rgb, count)
		n := make([]int, count)
		for _, p := range points {
			j := nearest(p, out)
			n[j]++
			for k := range p {
				sum[j][k] += p[k]
			}
		}
		for j := range out {
			if n[j] > 0 {
				for k := range out[j] {
					out[j][k] = sum[j][k] / float32(n[j])
				}
			}
		}
	}
	// Priority is stable and meaningful: darkest/lower luma base first.
	sort.SliceStable(out, func(i, j int) bool { return out[i][0]+out[i][1]+out[i][2] < out[j][0]+out[j][1]+out[j][2] })
	return out
}
func nearest(p rgb, colors []rgb) int {
	best := 0
	for i := 1; i < len(colors); i++ {
		if distance(p, colors[i]) < distance(p, colors[best]) {
			best = i
		}
	}
	return best
}
func distance(a, b rgb) float32 {
	v := float32(0)
	for k := range a {
		d := a[k] - b[k]
		v += d * d
	}
	return v
}

func (f *fit) target(x, y, p int) rgb {
	at := (((y*samples + p/samples) * f.width * samples) + x*samples + p%samples) * 3
	return rgb{float32(f.reference[at]), float32(f.reference[at+1]), float32(f.reference[at+2])}
}

func (f *fit) cost(x, y int, l layers, variant int) float64 {
	if !f.valid[y*f.width+x] {
		return 0
	}
	w := &weights[l.key]
	s := float64(0)
	for p := 0; p < sampleCount; p++ {
		var c rgb
		for j := 0; j < l.count; j++ {
			layer := int(l.masks[j])
			if j == 0 {
				layer = 16 + variant
			}
			t := f.sheets[l.ids[j]].low[layer][p]
			for k := range c {
				c[k] += w[j][p] * t[k]
			}
		}
		s += float64(distance(c, f.target(x, y, p)))
	}
	return s
}

func (f *fit) optimize(ctx context.Context, passes int) error {
	for pass := 0; pass < passes; pass++ {
		changed := 0
		for yy := 0; yy <= f.height; yy++ {
			if e := ctx.Err(); e != nil {
				return e
			}
			y := yy
			if pass%2 == 1 {
				y = f.height - yy
			}
			for xx := 0; xx <= f.width; xx++ {
				x := xx
				if pass%2 == 1 {
					x = f.width - xx
				}
				at := y*(f.width+1) + x
				old := f.labels[at]
				var candidates [12]uint8
				n := 1
				candidates[0] = old
				add := func(v uint8) {
					for _, c := range candidates[:n] {
						if c == v {
							return
						}
					}
					if n < len(candidates) {
						candidates[n] = v
						n++
					}
				}
				c := f.cornerColor(x, y)
				d := make([]float32, len(f.colors))
				for i := range d {
					d[i] = distance(c, f.colors[i])
				}
				for j := 0; j < 4; j++ {
					best := 0
					for i := 1; i < len(d); i++ {
						if d[i] < d[best] {
							best = i
						}
					}
					add(uint8(best))
					d[best] = math.MaxFloat32
				}
				for _, p := range [][2]int{{x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}} {
					if p[0] >= 0 && p[0] <= f.width && p[1] >= 0 && p[1] <= f.height {
						add(f.labels[p[1]*(f.width+1)+p[0]])
					}
				}
				score := func() float64 {
					s := 0.0
					for cy := max(0, y-1); cy < min(f.height, y+1); cy++ {
						for cx := max(0, x-1); cx < min(f.width, x+1); cx++ {
							s += f.cost(cx, cy, f.cellLayers(cx, cy), int(f.variants[cy*f.width+cx]))
						}
					}
					return s
				}
				best, bestCost := old, score()
				for _, candidate := range candidates[1:n] {
					f.labels[at] = candidate
					s := score()
					if s < bestCost-1e-5 {
						best, bestCost = candidate, s
					}
				}
				f.labels[at] = best
				if best != old {
					changed++
				}
			}
		}
		fmt.Printf("  shared-corner sweep %d: %d changes\n", pass+1, changed)
	}
	return nil
}

type exemplar struct {
	x, y     int
	features [sampleCount]rgb
}

// Keep variation0 as the constant-color baseline. The other15 variations are
// representative source residual patches, addressable only on the base family.
// This is deliberately bounded vector quantization, not a unique atlas per cell.
func (f *fit) exemplars() [][]exemplar {
	pools := make([][]exemplar, len(f.sheets))
	for y := 0; y < f.height; y++ {
		for x := 0; x < f.width; x++ {
			if !f.valid[y*f.width+x] || (y*f.width+x)%7 != 0 {
				continue
			}
			l := f.cellLayers(x, y)
			base := int(l.ids[0])
			w := &weights[l.key]
			e := exemplar{x: x, y: y}
			for p := 0; p < sampleCount; p++ {
				c := f.target(x, y, p)
				for j := 1; j < l.count; j++ {
					for k := range c {
						c[k] -= w[j][p] * f.sheets[l.ids[j]].low[l.masks[j]][p][k]
					}
				}
				for k := range c {
					if w[0][p] > .15 {
						c[k] = float32(clamp(float64(c[k]/w[0][p]), 0, 255))
					} else {
						c[k] = f.colors[base][k]
					}
				}
				e.features[p] = c
			}
			pools[base] = append(pools[base], e)
		}
	}
	selected := make([][]exemplar, len(pools))
	for id, pool := range pools {
		if len(pool) == 0 {
			continue
		}
		dist := make([]float64, len(pool))
		for i, e := range pool {
			for p, c := range e.features {
				dist[i] += float64(distance(c, f.sheets[id].low[16][p]))
			}
		}
		for len(selected[id]) < 15 && len(selected[id]) < len(pool) {
			best := 0
			for i := range pool {
				if dist[i] > dist[best] {
					best = i
				}
			}
			e := pool[best]
			selected[id] = append(selected[id], e)
			for i, p := range pool {
				d := 0.0
				for j := range p.features {
					d += float64(distance(p.features[j], e.features[j]))
				}
				dist[i] = min(dist[i], d)
			}
		}
	}
	return selected
}

func (f *fit) fillExemplar(id, variant int, e exemplar, img *image.NRGBA, tileX, tileY int) {
	l := f.cellLayers(e.x, e.y)
	for y := 0; y < subtile; y++ {
		for x := 0; x < subtile; x++ {
			u, v := (float64(x)+.5)/subtile, (float64(y)+.5)/subtile
			var a [4]float64
			a[0] = 1
			for j := 1; j < l.count; j++ {
				a[j] = maskAlpha(l.masks[j], u, v)
			}
			w := shaderWeights(a, l.count)
			px := ((float64(e.x-tileX*cellsPerADT)+u)/cellsPerADT)*float64(img.Rect.Dx()) - .5
			py := ((float64(e.y-tileY*cellsPerADT)+v)/cellsPerADT)*float64(img.Rect.Dy()) - .5
			t := sampleImage(img, px, py)
			at := ((16+variant)*subtile*subtile + y*subtile + x) * 4
			var overlay [4][4]float64
			for j := 1; j < l.count; j++ {
				overlay[j] = sampleSheet(&f.sheets[l.ids[j]], int(l.masks[j]), u, v)
			}
			for k := 0; k < 3; k++ {
				c := float64(t[k])
				for j := 1; j < l.count; j++ {
					c -= w[j] * overlay[j][k]
				}
				if w[0] > .15 {
					c /= w[0]
				} else {
					c = float64(f.colors[id][k])
				}
				f.sheets[id].pixels[at+k] = byte(clamp(c, 0, 255) + .5)
			}
		}
	}
}

func (f *fit) chooseVariants(ctx context.Context) error {
	changed := 0
	for y := 0; y < f.height; y++ {
		if e := ctx.Err(); e != nil {
			return e
		}
		for x := 0; x < f.width; x++ {
			l := f.cellLayers(x, y)
			best, bestCost := 0, f.cost(x, y, l, 0)
			for v := 1; v < 16; v++ {
				s := f.cost(x, y, l, v)
				if s < bestCost {
					best, bestCost = v, s
				}
			}
			f.variants[y*f.width+x] = uint8(best)
			if best > 0 {
				changed++
			}
		}
	}
	fmt.Printf("  selected full variations on %d cells\n", changed)
	return nil
}

func refreshLow(s *sheet) {
	for layer := 0; layer < 32; layer++ {
		for y := 0; y < samples; y++ {
			for x := 0; x < samples; x++ {
				c := rgb{}
				const step = subtile / samples
				for j := 0; j < step; j++ {
					for i := 0; i < step; i++ {
						at := (layer*subtile*subtile + (y*step+j)*subtile + x*step + i) * 4
						for k := range c {
							c[k] += float32(s.pixels[at+k])
						}
					}
				}
				for k := range c {
					c[k] /= step * step
				}
				s.low[layer][y*samples+x] = c
			}
		}
	}
}

func sampleImage(img *image.NRGBA, x, y float64) rgb {
	x = clamp(x, 0, float64(img.Rect.Dx()-1))
	y = clamp(y, 0, float64(img.Rect.Dy()-1))
	ix, iy := int(x), int(y)
	tx, ty := float32(x-float64(ix)), float32(y-float64(iy))
	var out rgb
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			at := min(img.Rect.Dy()-1, iy+j)*img.Stride + min(img.Rect.Dx()-1, ix+i)*4
			w := []float32{1 - tx, tx}[i] * []float32{1 - ty, ty}[j]
			for k := range out {
				out[k] += w * float32(img.Pix[at+k])
			}
		}
	}
	return out
}
func clamp(v, lo, hi float64) float64 { return min(hi, max(lo, v)) }
