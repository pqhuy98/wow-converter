package terrainfit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	bake "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

// MaterialBenchmark uses source paint mixtures and vertex tint, never fitted
// world-image fragments. Its model exceptions are complete source texture blocks.
func MaterialBenchmark(ctx context.Context, out, source, onlyRegion string, paintFamilies int) error {
	if paintFamilies < 0 || paintFamilies > 63 {
		return fmt.Errorf("invalid export paint budget %d", paintFamilies)
	}
	data, err := os.ReadFile(filepath.Join(out, "report.json"))
	if err != nil {
		return err
	}
	var report benchmarkReport
	if err = json.Unmarshal(data, &report); err != nil {
		return err
	}
	report.Renderer = "Shared-corner labels and equal-ID masks. Authored-material candidates use sequential overlay alpha and overlapping mask coverage; rejected historical stages retain HiveWE recursive RGBA mixing. Encoded BLP1 textures; native WC3 runtime not verified."
	found := false
	for i := range report.Regions {
		r := &report.Regions[i]
		if onlyRegion != "" && r.Name != onlyRegion {
			continue
		}
		found = true
		if err = fitMaterialRegion(ctx, out, source, r, paintFamilies); err != nil {
			return err
		}
		if err = writeJSON(filepath.Join(out, r.Name, "report.json"), r); err != nil {
			return err
		}
		if err = writeJSON(filepath.Join(out, "report.json"), report); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("unknown material region: %s", onlyRegion)
	}
	return writeViewer(out)
}

func fitMaterialRegion(ctx context.Context, out, source string, r *regionReport, paintFamilies int) error {
	start := time.Now()
	fmt.Println("MATERIAL FIT", r.Name)
	w, h, nx, ny := r.Cells[0], r.Cells[1], r.X1-r.X0+1, r.Y1-r.Y0+1
	captures := make([]materialCapture, len(r.Tiles))
	for ti, t := range r.Tiles {
		data, err := os.ReadFile(filepath.Join(source, t.Name, "layers.json"))
		if err != nil {
			return err
		}
		if err = json.Unmarshal(data, &captures[ti]); err != nil {
			return err
		}
		c := &captures[ti]
		if c.Version != 2 || c.Build != r.Build || len(c.Chunks) != 256 {
			return fmt.Errorf("material capture mismatch: %s", t.Name)
		}
	}
	images := map[uint32]*image.NRGBA{}
	for _, c := range captures {
		for _, m := range c.Materials {
			for _, id := range []uint32{m.ID, m.HeightID} {
				if id == 0 || images[id] != nil {
					continue
				}
				img, err := loadPNG(filepath.Join(source, "materials", r.Build, fmt.Sprintf("%d.png", id)))
				if err != nil {
					return err
				}
				images[id] = img
			}
		}
	}
	var grid *TilesetGrid
	stage := "material-blend"
	if paintFamilies > 0 {
		grid = &TilesetGrid{Width: w, Height: h, StepX: [2]float64{1.0 / cellsPerADT, 0}, StepY: [2]float64{0, 1.0 / cellsPerADT}, PaintFamilies: paintFamilies}
		stage = fmt.Sprintf("export-%d", paintFamilies)
	}
	f, catalog, centers, err := fitCaptures(captures, nx, ny, images, grid)
	if err != nil {
		return err
	}
	dir := filepath.Join(out, r.Name, stage)
	var textureBytes int64
	if grid != nil {
		f.prepareExportSheets(centers, *grid)
		_, textureBytes, err = f.saveExportSheets(filepath.Join(dir, "tileset"))
	} else {
		textureBytes, err = f.saveSheets(filepath.Join(dir, "tileset"))
	}
	if err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(dir, "palette.json"), struct {
		Materials []terrainMaterial
		Mixtures  [][]float32
	}{catalog, centers}); err != nil {
		return err
	}
	if err = writeJSON(filepath.Join(dir, "assignments.json"), assignments{Width: w, Height: h, Labels: f.labels, BaseVariations: f.variants, Compositing: "sequential"}); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(out, r.Name, "native-mesh.json"))
	if err != nil {
		return err
	}
	var native Mesh
	if err = json.Unmarshal(data, &native); err != nil {
		return err
	}
	tiles, mask := make([]tileData, len(r.Tiles)), make([]bool, w*h)
	for ti, t := range r.Tiles {
		data, err = os.ReadFile(filepath.Join(source, t.Name, "mesh.json"))
		if err != nil {
			return err
		}
		m := &tiles[ti]
		m.name, m.dir = t.Name, filepath.Join(source, t.Name)
		if err = json.Unmarshal(data, &m.mesh); err != nil {
			return err
		}
		_, m.coverage = rasterHeights(m.mesh, cellsPerADT, true)
		markHeightFallback(m.mesh, ti%nx, ti/nx, w, h, r.Geometry.Window, mask)
		for y := 0; y < cellsPerADT; y++ {
			for x := 0; x < cellsPerADT; x++ {
				if !m.coverage[y*cellsPerADT+x] {
					mask[(ti/nx*cellsPerADT+y)*w+ti%nx*cellsPerADT+x] = true
				}
			}
		}
	}
	var aggregate measurement
	var seams seamMeasurement
	var modelTiles []patchTile
	var modelPixels, nativePixels int64
	var nativeAbs, adjustment, gap float64
	var patchBytes int64
	triangles := 0
	for ti, t := range tiles {
		if err = ctx.Err(); err != nil {
			return err
		}
		fmt.Println("  MATERIAL RENDER", t.name)
		ref, e := loadPNG(filepath.Join(t.dir, "wow.png"))
		if e != nil {
			return e
		}
		result := f.renderTile(ti%nx, ti/nx, 8192)
		local := make([]bool, cellsPerADT*cellsPerADT)
		for y := 0; y < cellsPerADT; y++ {
			copy(local[y*cellsPerADT:], mask[(ti/nx*cellsPerADT+y)*w+ti%nx*cellsPerADT:(ti/nx*cellsPerADT+y)*w+(ti%nx+1)*cellsPerADT])
		}
		placements, files, size, e := packSourceBlocks(dir, t.name, ref, result, local, t.coverage)
		if e != nil {
			return e
		}
		patchBytes += size
		mesh, groups := clipPatches(t.mesh, local, placements, 512, 0)
		before, after := weldPatchBoundary(&mesh, groups, native, w, h, ti%nx, ti/nx, mask)
		adjustment, gap = max(adjustment, before), max(gap, after)
		triangles += len(mesh.Indices) / 3
		nonempty := groups[:0]
		for i := range groups {
			if len(groups[i].Mesh.Indices) == 0 {
				continue
			}
			groups[i].File = files[i]
			nonempty = append(nonempty, groups[i])
		}
		modelTiles = append(modelTiles, patchTile{t.name, local})
		if err = writeJSON(filepath.Join(dir, t.name+"-model-atlases.json"), nonempty); err != nil {
			return err
		}
		m := measure(ref, result, t.coverage)
		aggregate.add(m)
		for ci, model := range local {
			if model {
				modelPixels += m.cellPixels[ci]
			} else {
				nativePixels += m.cellPixels[ci]
				nativeAbs += m.cellAbs[ci]
			}
		}
		seams.add(downsample(ref, 1024), downsample(result, 1024), local, t.coverage)
		if err = savePNG(filepath.Join(dir, t.name+"-8k.png"), result); err != nil {
			return err
		}
		if err = savePNG(filepath.Join(dir, t.name+".png"), downsample(result, 2048)); err != nil {
			return err
		}
		if err = saveError(filepath.Join(dir, t.name+"-error.png"), ref, result); err != nil {
			return err
		}
	}
	if err = writeJSON(filepath.Join(dir, "meshes.json"), modelTiles); err != nil {
		return err
	}
	area := float64(modelPixels) / float64(nativePixels+modelPixels)
	geometry := hybridGeometry(native, w, h, tiles, nx, mask, r.Geometry.Window)
	geometry.BoundaryAdjustment, geometry.SeamMax = adjustment, gap
	continuity := seams.metrics()
	s := stageReport{Name: stage, Palette: len(f.sheets), TextureBytes: textureBytes + patchBytes, Metrics: aggregate.metrics, Seconds: time.Since(start).Seconds(), ModelArea: area, PatchTextureBytes: patchBytes, PatchTriangles: triangles, NativeMAE: nativeAbs / float64(nativePixels*3), Geometry: &geometry, MandatoryModelArea: area, NativeMajority: area < .5, Seams: &continuity, VisualStatus: "pending"}
	for i, old := range r.Stages {
		if old.Name == s.Name {
			r.Stages[i] = s
			return nil
		}
	}
	r.Stages = append(r.Stages, s)
	fmt.Printf("MATERIAL SCORE %s MAE %.3f SSIM %.4f native %.2f%% jump %.3f\n", r.Name, s.Metrics.MAE, s.Metrics.SSIM, 100*(1-area), continuity.Native.ResultStep)
	return nil
}

func sampleMaterialAlpha(data []byte, u, v float64) float64 {
	x, y := u*64-.5, v*64-.5
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	tx, ty, value := x-math.Floor(x), y-math.Floor(y), 0.0
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			value += float64(data[max(0, min(63, iy+j))*64+max(0, min(63, ix+i))]) * [2]float64{1 - tx, tx}[i] * [2]float64{1 - ty, ty}[j]
		}
	}
	return value / 255
}

func nearestMaterialFeature(point []float32, centers [][]float32) int {
	best, cost := 0, float32(math.MaxFloat32)
	for i, center := range centers {
		value := materialFeatureDistance(point, center)
		if value < cost {
			best, cost = i, value
		}
	}
	return best
}

func materialFeatureDistance(a, b []float32) float32 {
	var out float32
	for k := range a {
		d := a[k] - b[k]
		out += d * d
	}
	return out
}

func clusterMaterialFeatures(points []float32, dim, maximum int) [][]float32 {
	count := len(points) / dim
	centers := [][]float32{append([]float32(nil), points[(count/2)*dim:(count/2+1)*dim]...)}
	distances := make([]float32, count)
	for i := range distances {
		distances[i] = math.MaxFloat32
	}
	for len(centers) < maximum {
		best := 0
		for i := 0; i < count; i += 3 {
			distances[i] = min(distances[i], materialFeatureDistance(points[i*dim:(i+1)*dim], centers[len(centers)-1]))
			if distances[i] > distances[best] {
				best = i
			}
		}
		if distances[best] < 1e-6 {
			break
		}
		centers = append(centers, append([]float32(nil), points[best*dim:(best+1)*dim]...))
	}
	for pass := 0; pass < 6; pass++ {
		sums, sizes := make([][]float32, len(centers)), make([]int, len(centers))
		for i := range sums {
			sums[i] = make([]float32, dim)
		}
		for i := 0; i < count; i += 3 {
			point := points[i*dim : (i+1)*dim]
			at := nearestMaterialFeature(point, centers)
			sizes[at]++
			for k, value := range point {
				sums[at][k] += value
			}
		}
		for i := range centers {
			if sizes[i] > 0 {
				for k := range centers[i] {
					centers[i][k] = sums[i][k] / float32(sizes[i])
				}
			}
		}
	}
	return centers
}

func fitCaptures(captures []materialCapture, nx, ny int, images map[uint32]*image.NRGBA, grid ...*TilesetGrid) (*fit, []terrainMaterial, [][]float32, error) {
	if nx <= 0 || ny <= 0 || len(captures) != nx*ny {
		return nil, nil, nil, fmt.Errorf("tileset capture grid is %dx%d with %d tiles", nx, ny, len(captures))
	}
	w, h := nx*cellsPerADT, ny*cellsPerADT
	catalog := []terrainMaterial{}
	lookup := map[terrainMaterial]int{}
	heightBlend := false
	for ti, c := range captures {
		if len(c.Chunks) != 256 {
			continue
		}
		if c.Version != 2 {
			return nil, nil, nil, fmt.Errorf("material capture mismatch at tile %d", ti)
		}
		heightBlend = heightBlend || c.HeightTexturing
		for _, m := range c.Materials {
			if m.ID == 0 {
				continue
			}
			if m.Scale < 1 || m.Scale > 1<<15 || m.Scale&(m.Scale-1) != 0 {
				return nil, nil, nil, fmt.Errorf("invalid terrain material scale %d", m.Scale)
			}
			if _, ok := lookup[m]; !ok {
				lookup[m] = len(catalog)
				catalog = append(catalog, m)
			}
		}
	}
	if len(catalog) == 0 {
		return nil, nil, nil, fmt.Errorf("no terrain materials in selected tiles")
	}
	phases := newMaterialPhases(catalog, images, heightBlend)
	dim := len(catalog) + 4*len(phases.scales) + 3
	features, counts := make([]float32, (w+1)*(h+1)*dim), make([]int, (w+1)*(h+1))
	for ti, c := range captures {
		if len(c.Chunks) != 256 {
			continue
		}
		for y := 0; y <= cellsPerADT; y++ {
			for x := 0; x <= cellsPerADT; x++ {
				cx, cy := min(15, x/8), min(15, y/8)
				chunk := c.Chunks[cy*16+cx]
				u, v := float64(x-cx*8)/8, float64(y-cy*8)/8
				at := (ti/nx*cellsPerADT+y)*(w+1) + ti%nx*cellsPerADT + x
				var alpha [8]float64
				alpha[0] = 1
				for j := 1; j < min(8, len(chunk.Layers)); j++ {
					if j < len(chunk.Alpha) && len(chunk.Alpha[j]) == 4096 {
						alpha[j] = sampleMaterialAlpha(chunk.Alpha[j], u, v)
						alpha[0] -= alpha[j]
					}
				}
				alpha[0] = max(0, alpha[0])
				sum := 0.0
				for _, a := range alpha {
					sum += a
				}
				for j, index := range chunk.Layers {
					if j < 8 && c.Materials[index].ID != 0 {
						features[at*dim+lookup[c.Materials[index]]] += float32(alpha[j] / max(sum, 1e-9))
					}
				}
				tint := rgb{1, 1, 1}
				if len(chunk.Tint) == 145 {
					tint = chunk.Tint[(y-cy*8)*17+x-cx*8]
				}
				for k := 0; k < 3; k++ {
					features[at*dim+dim-3+k] += tint[k] * .5
				}
				counts[at]++
			}
		}
	}
	for at, count := range counts {
		for k := 0; k < dim; k++ {
			features[at*dim+k] /= float32(max(1, count))
		}
	}
	maximum := 64
	if len(grid) > 0 && grid[0] != nil {
		g := grid[0]
		if g.Width < 1 || g.Height < 1 || g.Width > 480 || g.Height > 480 {
			return nil, nil, nil, fmt.Errorf("invalid native tileset grid %dx%d", g.Width, g.Height)
		}
		features = resampleMaterialFeatures(features, dim, w, h, *g)
		w, h = g.Width, g.Height
		if g.PaintFamilies < 1 || g.PaintFamilies > 63 {
			return nil, nil, nil, fmt.Errorf("invalid native paint family budget %d", g.PaintFamilies)
		}
		maximum = g.PaintFamilies // The separate alpha entry also consumes a palette slot.
	}
	if len(phases.scales) > 0 {
		var g *TilesetGrid
		if len(grid) > 0 {
			g = grid[0]
		}
		phases.allocate(features, catalog, w, h, g)
	}
	centers := clusterMaterialFeatures(features, dim, maximum)
	f := &fit{width: w, height: h, labels: make([]uint8, (w+1)*(h+1)), variants: make([]uint8, w*h), sequential: true}
	for at := range f.labels {
		f.labels[at] = uint8(nearestMaterialFeature(features[at*dim:(at+1)*dim], centers))
	}
	if err := f.makeMaterialSheetsFrom(catalog, centers, heightBlend, images, phases); err != nil {
		return nil, nil, nil, err
	}
	if len(grid) > 0 && grid[0] != nil {
		sortExportPalette(f, centers)
		f.sheets = append(f.sheets, sheet{pixels: make([]byte, 32*subtile*subtile*4)})
	}
	return f, catalog, centers, nil
}

// Order the editor swatches along a nearest-neighbor walk through material
// mixtures and average color. Keep texture relatives together as tint changes.
func sortExportPalette(f *fit, centers [][]float32) {
	n := len(f.sheets)
	colors := make([]rgb, n)
	for i, s := range f.sheets {
		for _, c := range s.low[16] {
			for k := range c {
				colors[i][k] += c[k] / sampleCount / 255
			}
		}
	}
	start := 0
	for i := range colors {
		if .2126*colors[i][0]+.7152*colors[i][1]+.0722*colors[i][2] < .2126*colors[start][0]+.7152*colors[start][1]+.0722*colors[start][2] {
			start = i
		}
	}
	used := make([]bool, n)
	order := make([]int, 0, n)
	at := start
	for len(order) < n {
		order = append(order, at)
		used[at] = true
		next := -1
		best := float64(math.Inf(1))
		for j := 0; j < n; j++ {
			if used[j] {
				continue
			}
			d := 0.0
			for k := range colors[j] {
				delta := float64(colors[j][k] - colors[at][k])
				d += delta * delta * 4
			}
			for k := 0; k < len(centers[j])-3; k++ {
				delta := float64(centers[j][k] - centers[at][k])
				d += delta * delta * .25
			}
			if d < best {
				next, best = j, d
			}
		}
		at = next
	}
	sorted := make([]sheet, n)
	remap := make([]uint8, n)
	for new, old := range order {
		sorted[new] = f.sheets[old]
		remap[old] = uint8(new)
	}
	for i, id := range f.labels {
		f.labels[i] = remap[id]
	}
	sortedCenters := make([][]float32, n)
	for new, old := range order {
		sortedCenters[new] = centers[old]
	}
	copy(centers, sortedCenters)
	f.sheets = sorted
}

// Interpolate paint mixtures before quantization. Resizing already-quantized
// labels by nearest neighbor stamps repeated square blocks into the final map.
func resampleMaterialFeatures(src []float32, dim, width, height int, grid TilesetGrid) []float32 {
	out := make([]float32, (grid.Width+1)*(grid.Height+1)*dim)
	for y := 0; y <= grid.Height; y++ {
		for x := 0; x <= grid.Width; x++ {
			u := clamp((grid.Origin[0]+float64(x)*grid.StepX[0]+float64(y)*grid.StepY[0])*cellsPerADT, 0, float64(width))
			v := clamp((grid.Origin[1]+float64(x)*grid.StepX[1]+float64(y)*grid.StepY[1])*cellsPerADT, 0, float64(height))
			ix, iy := int(u), int(v)
			for j := 0; j < 2; j++ {
				for i := 0; i < 2; i++ {
					from := (min(height, iy+j)*(width+1) + min(width, ix+i)) * dim
					weight := float32([2]float64{1 - (u - float64(ix)), u - float64(ix)}[i] * [2]float64{1 - (v - float64(iy)), v - float64(iy)}[j])
					to := (y*(grid.Width+1) + x) * dim
					for k := 0; k < dim; k++ {
						out[to+k] += src[from+k] * weight
					}
				}
			}
		}
	}
	return out
}

func (f *fit) makeMaterialSheets(source, build string, catalog []terrainMaterial, centers [][]float32, heightBlend bool) error {
	images := map[uint32]*image.NRGBA{}
	for _, m := range catalog {
		for _, id := range []uint32{m.ID, m.HeightID} {
			if id == 0 || images[id] != nil {
				continue
			}
			img, err := loadPNG(filepath.Join(source, "materials", build, fmt.Sprintf("%d.png", id)))
			if err != nil {
				return err
			}
			images[id] = img
		}
	}
	return f.makeMaterialSheetsFrom(catalog, centers, heightBlend, images)
}

func (f *fit) makeMaterialSheetsFrom(catalog []terrainMaterial, centers [][]float32, heightBlend bool, images map[uint32]*image.NRGBA, phasePlan ...materialPhases) error {
	f.materialCount = len(catalog)
	phases := materialPhases{span: [2]float64{1, 1}}
	if len(phasePlan) > 0 {
		phases = phasePlan[0]
	}
	load := func(id uint32, scale int) (*image.NRGBA, error) {
		img := images[id]
		if img == nil {
			return nil, fmt.Errorf("missing terrain material texture %d", id)
		}
		mips := bake.BuildMipChain(img.Pix, img.Rect.Dx(), img.Rect.Dy())
		level := 0
		target := float64(subtile*scale) / max(phases.span[0], phases.span[1])
		for level+1 < len(mips) && float64(max(mips[level].Width, mips[level].Height)) > target {
			level++
		}
		mip := mips[level]
		return &image.NRGBA{Pix: mip.Data, Stride: mip.Width * 4, Rect: image.Rect(0, 0, mip.Width, mip.Height)}, nil
	}
	diffuse, height := make([]*image.NRGBA, len(catalog)), make([]*image.NRGBA, len(catalog))
	for i, m := range catalog {
		var err error
		if diffuse[i], err = load(m.ID, m.Scale); err != nil {
			return err
		}
		if m.HeightID != 0 {
			if height[i], err = load(m.HeightID, m.Scale); err != nil {
				return err
			}
		}
	}
	f.sheets = make([]sheet, len(centers))
	for ci, center := range centers {
		ids := make([]int, len(catalog))
		for i := range ids {
			ids[i] = i
		}
		sort.SliceStable(ids, func(i, j int) bool { return center[ids[i]] > center[ids[j]] })
		ids = ids[:min(8, len(ids))]
		var paint [8]float64
		sum := 0.0
		for i, id := range ids {
			paint[i] = float64(center[id])
			sum += paint[i]
		}
		for i := range paint {
			paint[i] /= max(sum, 1e-9)
		}
		s := &f.sheets[ci]
		s.pixels = make([]byte, 32*subtile*subtile*4)
		s.detail = make([]float64, subtile*subtile)
		for y := 0; y < subtile; y++ {
			for x := 0; x < subtile; x++ {
				u, v := (float64(x)+.5)/subtile, (float64(y)+.5)/subtile
				var samples [8][4]float64
				var heights [8]float64
				for i, id := range ids {
					tu, tv := phases.uv(center, len(catalog), catalog[id].Scale, u, v)
					samples[i] = sampleMaterialImage(diffuse[id], tu, tv)
					h := samples[i][3]
					if height[id] != nil {
						h = sampleMaterialImage(height[id], tu, tv)[3]
					}
					heights[i] = h/255*catalog[id].HeightScale + catalog[id].HeightOffset
				}
				weights := paint
				if heightBlend {
					weights = bake.TerrainHeightWeights(weights, heights, len(ids))
				}
				var color [3]float64
				for i := range ids {
					s.detail[y*subtile+x] += heights[i] * weights[i]
					for k := range color {
						color[k] += samples[i][k] * weights[i] * float64(center[len(center)-3+k]*2)
					}
				}
				for layer := 0; layer < 32; layer++ {
					at := (layer*subtile*subtile + y*subtile + x) * 4
					for k := range color {
						s.pixels[at+k] = byte(clamp(color[k], 0, 255) + .5)
					}
					s.pixels[at+3] = 255
					if layer < 16 {
						s.pixels[at+3] = byte(materialMaskAlpha(uint8(layer), u, v)*255 + .5)
					}
					// WE can select variations 17..31, which address layer 0.
					// Overlay mask 0 is never drawn, so make that brush fallback full.
					if layer == 0 {
						s.pixels[at+3] = 255
					}
				}
			}
		}
		refreshLow(s)
	}
	return nil
}

// Equal-ID masks need overlapping opaque coverage. On an edge with two labels,
// at least one owns >= half the corner influence, so an absent lower-priority
// material is completely covered. Ordinary bilinear masks leave it visible,
// making the edge depend on the other two corners of each adjacent cell.
func materialMaskAlpha(mask uint8, u, v float64) float64 {
	return min(1, 2*maskAlpha(mask, u, v))
}

func sampleMaterialImage(img *image.NRGBA, u, v float64) [4]float64 {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	x, y := u*float64(w)-.5, v*float64(h)-.5
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	tx, ty := x-math.Floor(x), y-math.Floor(y)
	var out [4]float64
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			at := ((iy+j+h)%h)*img.Stride + ((ix+i+w)%w)*4
			weight := [2]float64{1 - tx, tx}[i] * [2]float64{1 - ty, ty}[j]
			for k := range out {
				out[k] += float64(img.Pix[at+k]) * weight
			}
		}
	}
	return out
}

// Use coherent 512px source blocks for mandatory models. Independent 64px cell
// packing mixed unrelated terrain in distant mip levels and exposed a grid.
func packSourceBlocks(dir, name string, ref, result *image.NRGBA, mask, coverage []bool) ([]patchPlacement, []string, int64, error) {
	var placements []patchPlacement
	var files []string
	var total int64
	for by := 0; by < 16; by++ {
		for bx := 0; bx < 16; bx++ {
			used := false
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					at := (by*8+y)*cellsPerADT + bx*8 + x
					used = used || (mask[at] && coverage[at])
				}
			}
			file := fmt.Sprintf("%s-source-%02d_%02d.blp", name, bx, by)
			if !used {
				if err := os.Remove(filepath.Join(dir, file)); err != nil && !os.IsNotExist(err) {
					return nil, nil, 0, err
				}
				continue
			}
			var encoded bytes.Buffer
			if err := png.Encode(&encoded, ref.SubImage(image.Rect(bx*512, by*512, (bx+1)*512, (by+1)*512))); err != nil {
				return nil, nil, 0, err
			}
			if err := blp.ConvertTextureToBlp(blp.EncodeInput{PNG: encoded.Bytes(), Opaque: true}, filepath.Join(dir, file)); err != nil {
				return nil, nil, 0, err
			}
			data, err := os.ReadFile(filepath.Join(dir, file))
			if err != nil {
				return nil, nil, 0, err
			}
			total += int64(len(data))
			decoded, err := blp.DecodePalettedBLP1(data, 0)
			if err != nil {
				return nil, nil, 0, err
			}
			for y := 0; y < 8; y++ {
				for x := 0; x < 8; x++ {
					cx, cy := bx*8+x, by*8+y
					if !mask[cy*cellsPerADT+cx] || !coverage[cy*cellsPerADT+cx] {
						continue
					}
					placements = append(placements, patchPlacement{x: cx, y: cy, x0: cx * subtile, y0: cy * subtile, x1: (cx + 1) * subtile, y1: (cy + 1) * subtile, atlas: len(files), sx: x * subtile, sy: y * subtile})
					for row := 0; row < subtile; row++ {
						from := (y*subtile+row)*decoded.Stride + x*subtile*4
						to := (cy*subtile+row)*result.Stride + cx*subtile*4
						copy(result.Pix[to:to+subtile*4], decoded.Pix[from:from+subtile*4])
					}
				}
			}
			files = append(files, file)
		}
	}
	return placements, files, total, nil
}
