package terrainfit

import (
	"math"
	"sort"
)

const wowTileSize = 1600.0 / 3
const modelScale = 128 * cellsPerADT / wowTileSize
const nativeHeightSpan = 3840 / modelScale

type geometryMetrics struct {
	MAE             float64    `json:"maeWowUnits"`
	RMSE            float64    `json:"rmseWowUnits"`
	P95             float64    `json:"p95WowUnits"`
	Max             float64    `json:"maxWowUnits"`
	ClampedFraction float64    `json:"clampedVertexFraction"`
	Window          [2]float64 `json:"windowWowUnits"`
}

func rasterHeights(m Mesh, size int, centers bool) ([]float64, []bool) {
	out, valid := make([]float64, size*size), make([]bool, size*size)
	scale := float64(size - 1)
	offset := 0.0
	if centers {
		scale = float64(size)
		offset = .5
	}
	for i := 0; i < len(m.Indices); i += 3 {
		a, b, c := m.Positions[m.Indices[i]], m.Positions[m.Indices[i+1]], m.Positions[m.Indices[i+2]]
		det := (b[1]-c[1])*(a[0]-c[0]) + (c[0]-b[0])*(a[1]-c[1])
		if math.Abs(det) < 1e-15 {
			continue
		}
		x0, x1 := max(0, int(math.Ceil(min(a[0], b[0], c[0])*scale-offset-1e-7))), min(size-1, int(math.Floor(max(a[0], b[0], c[0])*scale-offset+1e-7)))
		y0, y1 := max(0, int(math.Ceil(min(a[1], b[1], c[1])*scale-offset-1e-7))), min(size-1, int(math.Floor(max(a[1], b[1], c[1])*scale-offset+1e-7)))
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				u, v := (float64(x)+offset)/scale, (float64(y)+offset)/scale
				wa := ((b[1]-c[1])*(u-c[0]) + (c[0]-b[0])*(v-c[1])) / det
				wb := ((c[1]-a[1])*(u-c[0]) + (a[0]-c[0])*(v-c[1])) / det
				wc := 1 - wa - wb
				if min(wa, wb, wc) < -1e-6 {
					continue
				}
				at := y*size + x
				out[at] = wa*a[2] + wb*b[2] + wc*c[2]
				valid[at] = true
			}
		}
	}
	return out, valid
}

func nativeMesh(region Region, meshes []Mesh) (Mesh, geometryMetrics) {
	nx, ny := region.X1-region.X0+1, region.Y1-region.Y0+1
	w, h := nx*cellsPerADT, ny*cellsPerADT
	heights := make([]float64, (w+1)*(h+1))
	n := make([]int, len(heights))
	all := []float64{}
	for i, m := range meshes {
		v, ok := rasterHeights(m, cellsPerADT+1, false)
		tx, ty := i%nx, i/nx
		for y := 0; y <= cellsPerADT; y++ {
			for x := 0; x <= cellsPerADT; x++ {
				at := y*(cellsPerADT+1) + x
				if ok[at] {
					j := (ty*cellsPerADT+y)*(w+1) + tx*cellsPerADT + x
					heights[j] += v[at]
					n[j]++
				}
			}
		}
		for _, p := range m.Positions {
			all = append(all, p[2])
		}
	}
	sort.Float64s(all)
	// Choose a fixed Z window that retains the largest number of source vertices.
	// This is an explicit benchmark policy, not the exporter's navigation projection.
	best, lo, j := 0, all[0], 0
	for i, z := range all {
		for all[j] < z-nativeHeightSpan {
			j++
		}
		if i-j+1 > best {
			best = i - j + 1
			lo = all[j]
		}
	}
	hi := lo + nativeHeightSpan
	out := Mesh{Positions: make([][3]float64, len(heights))}
	for y := 0; y <= h; y++ {
		for x := 0; x <= w; x++ {
			at := y*(w+1) + x
			z := lo
			if n[at] > 0 {
				z = heights[at] / float64(n[at])
			} else {
				for radius := 1; radius < 8; radius++ {
					found := false
					for _, d := range [][2]int{{-radius, 0}, {radius, 0}, {0, -radius}, {0, radius}} {
						xx, yy := x+d[0], y+d[1]
						if xx < 0 || xx > w || yy < 0 || yy > h {
							continue
						}
						j := yy*(w+1) + xx
						if n[j] > 0 {
							z = heights[j] / float64(n[j])
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			z = lo + math.Round((clamp(z, lo, hi)-lo)*modelScale*4)/(modelScale*4)
			out.Positions[at] = [3]float64{float64(x) / cellsPerADT, float64(y) / cellsPerADT, z}
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			tl, tr, bl, br := uint32(y*(w+1)+x), uint32(y*(w+1)+x+1), uint32((y+1)*(w+1)+x), uint32((y+1)*(w+1)+x+1)
			out.Indices = append(out.Indices, tl, tr, bl, bl, tr, br)
		}
	}
	var stats geometryMetrics
	stats.Window = [2]float64{lo, hi}
	errors := make([]float64, 0, len(all))
	clamped := 0
	for i, m := range meshes {
		tx, ty := i%nx, i/nx
		for _, p := range m.Positions {
			gx, gy := (float64(tx)+p[0])*cellsPerADT, (float64(ty)+p[1])*cellsPerADT
			x, y := min(w-1, int(gx)), min(h-1, int(gy))
			u, v := gx-float64(x), gy-float64(y)
			tl, tr, bl, br := out.Positions[y*(w+1)+x][2], out.Positions[y*(w+1)+x+1][2], out.Positions[(y+1)*(w+1)+x][2], out.Positions[(y+1)*(w+1)+x+1][2]
			z := tl*(1-u-v) + tr*u + bl*v
			if u+v > 1 {
				z = br*(u+v-1) + bl*(1-u) + tr*(1-v)
			}
			d := math.Abs(z - p[2])
			errors = append(errors, d)
			stats.MAE += d
			stats.RMSE += d * d
			stats.Max = max(stats.Max, d)
			if p[2] < lo || p[2] > hi {
				clamped++
			}
		}
	}
	sort.Float64s(errors)
	stats.MAE /= float64(len(errors))
	stats.RMSE = math.Sqrt(stats.RMSE / float64(len(errors)))
	stats.P95 = errors[int(float64(len(errors)-1)*.95)]
	stats.ClampedFraction = float64(clamped) / float64(len(errors))
	return out, stats
}
