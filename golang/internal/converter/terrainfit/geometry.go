package terrainfit

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

const wowTileSize = 1600.0 / 3
const modelScale = 128 * cellsPerADT / wowTileSize
const nativeHeightSpan = 3840 / modelScale

type geometryMetrics struct {
	MAE                float64    `json:"maeWowUnits"`
	RMSE               float64    `json:"rmseWowUnits"`
	P95                float64    `json:"p95WowUnits"`
	Max                float64    `json:"maxWowUnits"`
	ClampedFraction    float64    `json:"clampedVertexFraction"`
	Window             [2]float64 `json:"windowWowUnits"`
	SeamMax            float64    `json:"modelBoundaryMaxWowUnits,omitempty"`
	BoundaryAdjustment float64    `json:"modelBoundaryAdjustmentMaxWowUnits,omitempty"`
}

// Refresh geometry independently of the expensive albedo fit. The mask and
// deterministic atlas placements stay unchanged, so encoded texture scores remain
// valid. Reclip source triangles before welding; never weld a previous weld.
func refreshRegionGeometry(ctx context.Context, out, source string, r *regionReport) error {
	fmt.Println("REFRESH GEOMETRY", r.Name)
	dir := filepath.Join(out, r.Name)
	stageDir := filepath.Join(dir, "clamp-hybrid")
	data, err := os.ReadFile(filepath.Join(stageDir, "meshes.json"))
	if err != nil {
		return err
	}
	var models []patchTile
	if err = json.Unmarshal(data, &models); err != nil {
		return err
	}
	if len(models) != len(r.Tiles) {
		return fmt.Errorf("incomplete model geometry: %s", r.Name)
	}
	meshes := make([]Mesh, len(r.Tiles))
	tiles := make([]tileData, len(r.Tiles))
	for i, t := range r.Tiles {
		if err = ctx.Err(); err != nil {
			return err
		}
		data, err = os.ReadFile(filepath.Join(source, t.Name, "mesh.json"))
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, &meshes[i]); err != nil {
			return err
		}
		if models[i].Name != t.Name || len(models[i].Cells) != cellsPerADT*cellsPerADT {
			return fmt.Errorf("invalid patch mask: %s", t.Name)
		}
		tiles[i] = tileData{name: t.Name, mesh: meshes[i]}
	}
	native, geometry := nativeMesh(r.Region, meshes)
	if geometry.Window != r.Geometry.Window {
		return fmt.Errorf("geometry refresh changed the fitted clamp: %s", r.Name)
	}
	w, h := r.Cells[0], r.Cells[1]
	nx := r.X1 - r.X0 + 1
	mask := make([]bool, w*h)
	for ti, t := range models {
		for y := 0; y < cellsPerADT; y++ {
			copy(mask[(ti/nx*cellsPerADT+y)*w+ti%nx*cellsPerADT:], t.Cells[y*cellsPerADT:(y+1)*cellsPerADT])
		}
	}
	before, after := 0.0, 0.0
	triangles := 0
	for ti := range models {
		if err = ctx.Err(); err != nil {
			return err
		}
		m := &models[ti]
		patchMesh, groups := clipPatches(meshes[ti], m.Cells, placePatches(m.Cells), patchAtlasSize, 2)
		b, a := weldPatchBoundary(&patchMesh, groups, native, w, h, ti%nx, ti/nx, mask)
		before, after = max(before, b), max(after, a)
		triangles += len(patchMesh.Indices) / 3
		for i := range groups {
			groups[i].File = m.Name + "-" + groups[i].File
		}
		if err = writeJSON(filepath.Join(stageDir, m.Name+"-model-atlases.json"), groups); err != nil {
			return err
		}
	}
	if err = writeJSON(filepath.Join(stageDir, "meshes.json"), models); err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(dir, "native-mesh.json"), native); err != nil {
		return err
	}
	r.Geometry = geometry
	for i := range r.Stages {
		if r.Stages[i].Name == "clamp-hybrid" {
			g := hybridGeometry(native, w, h, tiles, nx, mask, geometry.Window)
			g.BoundaryAdjustment, g.SeamMax = before, after
			r.Stages[i].Geometry = &g
			r.Stages[i].PatchTriangles = triangles
		}
	}
	r.GeometryVersion = 2
	return nil
}

func markHeightFallback(m Mesh, tx, ty, width, height int, window [2]float64, mask []bool) {
	for _, p := range m.Positions {
		if p[2] >= window[0] && p[2] <= window[1] {
			continue
		}
		gx, gy := (float64(tx)+p[0])*cellsPerADT, (float64(ty)+p[1])*cellsPerADT
		xs, ys := []int{int(math.Floor(gx))}, []int{int(math.Floor(gy))}
		// Source float32 world positions introduce tiny UV jitter at grid edges.
		if math.Abs(gx-math.Round(gx)) < .001 {
			xs = []int{int(math.Round(gx)) - 1, int(math.Round(gx))}
		}
		if math.Abs(gy-math.Round(gy)) < .001 {
			ys = []int{int(math.Round(gy)) - 1, int(math.Round(gy))}
		}
		for _, x := range xs {
			for _, y := range ys {
				if x >= 0 && x < width && y >= 0 && y < height {
					mask[y*width+x] = true
				}
			}
		}
	}
}

// Stitch only the perimeter of a source patch to native ground. Interior source
// heights, including all out-of-range mountains, are unchanged.
func weldPatchBoundary(m *Mesh, groups []patchAtlas, native Mesh, width, height, tx, ty int, mask []bool) (float64, float64) {
	before, after := 0.0, 0.0
	adjust := func(p [3]float64) [3]float64 {
		gx, gy := (float64(tx)+p[0])*cellsPerADT, (float64(ty)+p[1])*cellsPerADT
		edgeX, edgeY := math.Abs(gx-math.Round(gx)) < .001, math.Abs(gy-math.Round(gy)) < .001
		if !edgeX && !edgeY {
			return p
		}
		xs, ys := []int{int(math.Floor(gx))}, []int{int(math.Floor(gy))}
		if edgeX {
			xs = []int{int(math.Round(gx)) - 1, int(math.Round(gx))}
		}
		if edgeY {
			ys = []int{int(math.Round(gy)) - 1, int(math.Round(gy))}
		}
		for _, x := range xs {
			for _, y := range ys {
				if x < 0 || x >= width || y < 0 || y >= height || mask[y*width+x] {
					continue
				}
				u, v := clamp(gx-float64(x), 0, 1), clamp(gy-float64(y), 0, 1)
				z := nativeHeightAt(native, width, x, y, u, v)
				before = max(before, math.Abs(z-p[2]))
				p[2] = z
				after = max(after, math.Abs(z-p[2]))
				return p
			}
		}
		return p
	}
	for i, p := range m.Positions {
		m.Positions[i] = adjust(p)
	}
	for j := range groups {
		for i, p := range groups[j].Mesh.Positions {
			groups[j].Mesh.Positions[i] = adjust(p)
		}
	}
	return before, after
}

func hybridGeometry(native Mesh, width, height int, tiles []tileData, nx int, patches []bool, window [2]float64) geometryMetrics {
	g := geometryMetrics{Window: window}
	errors := []float64{}
	clamped := 0
	stride := width + 1
	sourceCorners := make([]float64, (width+1)*(height+1))
	counts := make([]int, len(sourceCorners))
	for ti, t := range tiles {
		tx, ty := ti%nx, ti/nx
		z, ok := rasterHeights(t.mesh, cellsPerADT+1, false)
		for y := 0; y <= cellsPerADT; y++ {
			for x := 0; x <= cellsPerADT; x++ {
				at := y*(cellsPerADT+1) + x
				if ok[at] {
					j := (ty*cellsPerADT+y)*stride + tx*cellsPerADT + x
					sourceCorners[j] += z[at]
					counts[j]++
				}
			}
		}
		for _, p := range t.mesh.Positions {
			gx, gy := (float64(tx)+p[0])*cellsPerADT, (float64(ty)+p[1])*cellsPerADT
			x, y := min(width-1, int(gx)), min(height-1, int(gy))
			u, v := gx-float64(x), gy-float64(y)
			d := 0.0
			if !patches[y*width+x] {
				d = math.Abs(nativeHeightAt(native, width, x, y, u, v) - p[2])
				if p[2] < window[0] || p[2] > window[1] {
					clamped++
				}
			}
			g.MAE += d
			g.RMSE += d * d
			g.Max = max(g.Max, d)
			errors = append(errors, d)
		}
	}
	gap := func(at int) {
		if counts[at] > 0 {
			g.SeamMax = max(g.SeamMax, math.Abs(sourceCorners[at]/float64(counts[at])-native.Positions[at][2]))
		}
	}
	for y := 0; y < height; y++ {
		for x := 1; x < width; x++ {
			if patches[y*width+x] != patches[y*width+x-1] {
				gap(y*stride + x)
				gap((y+1)*stride + x)
			}
		}
	}
	for y := 1; y < height; y++ {
		for x := 0; x < width; x++ {
			if patches[y*width+x] != patches[(y-1)*width+x] {
				gap(y*stride + x)
				gap(y*stride + x + 1)
			}
		}
	}
	sort.Float64s(errors)
	g.MAE /= float64(len(errors))
	g.RMSE = math.Sqrt(g.RMSE / float64(len(errors)))
	g.P95 = errors[int(float64(len(errors)-1)*.95)]
	g.ClampedFraction = float64(clamped) / float64(len(errors))
	return g
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
		// ADT corner/center coordinates lie on a 256-step grid. Float32 world
		// subtraction leaves tiny gaps at chunk edges; sample their intended
		// coordinates instead of treating those gaps as missing terrain.
		a, b, c = snapTerrainUV(a), snapTerrainUV(b), snapTerrainUV(c)
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

func snapTerrainUV(p [3]float64) [3]float64 {
	for axis := 0; axis < 2; axis++ {
		snapped := math.Round(p[axis]*256) / 256
		if math.Abs(snapped-p[axis]) < 1e-5 {
			p[axis] = snapped
		}
	}
	return p
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
