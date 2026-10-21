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
)

type assignments struct {
	Width, Height          int
	Labels, BaseVariations []uint8
	Compositing            string `json:"Compositing,omitempty"`
}
type patchTile struct {
	Name  string `json:"name"`
	Cells []bool `json:"cells"`
}
type patchPlacement struct{ x, y, x0, y0, x1, y1, atlas, sx, sy int }

const patchAtlasSize = 1024

type patchAtlas struct {
	File string       `json:"file"`
	Mesh Mesh         `json:"mesh"`
	UV   [][2]float64 `json:"uv"`
}

// Refine always preserves source models outside the terrain height clamp.
// Optional visual patches use a25%total-area budget. Mandatory height patches
// take precedence; report when the selected clamp prevents a native majority.
func Refine(ctx context.Context, out string) error {
	b, e := os.ReadFile(filepath.Join(out, "report.json"))
	if e != nil {
		return e
	}
	var report benchmarkReport
	if e = json.Unmarshal(b, &report); e != nil {
		return e
	}
	if report.CellsPerADT != cellsPerADT || len(report.Regions) == 0 {
		return fmt.Errorf("refine requires completed aligned-grid regions")
	}
	// The baseline runner may finish later regions while refinement runs. Its
	// completed per-region reports are authoritative over a partial global index.
	if data, err := os.ReadFile(filepath.Join(out, "regions.json")); err == nil {
		var configured []Region
		if err = json.Unmarshal(data, &configured); err != nil {
			return err
		}
		complete := []regionReport{}
		for _, region := range configured {
			data, err = os.ReadFile(filepath.Join(out, region.Name, "report.json"))
			if err != nil {
				return err
			}
			var result regionReport
			if err = json.Unmarshal(data, &result); err != nil {
				return err
			}
			if len(result.Stages) < 4 || len(result.Tiles) == 0 {
				return fmt.Errorf("baseline is incomplete: %s", region.Name)
			}
			complete = append(complete, result)
		}
		report.Regions = complete
	}
	for i := range report.Regions {
		cached, e := os.ReadFile(filepath.Join(out, report.Regions[i].Name, "report.json"))
		if e != nil {
			return e
		}
		var previous regionReport
		if json.Unmarshal(cached, &previous) == nil && previous.RefinementVersion == 2 && previous.Build == report.Regions[i].Build && previous.Cells == report.Regions[i].Cells {
			report.Regions[i] = previous
			fmt.Println("CACHED REFINEMENT", previous.Name)
			continue
		}
		if e = refineRegion(ctx, out, filepath.Join(out, filepath.FromSlash(report.SourceRoot)), &report.Regions[i]); e != nil {
			return e
		}
		if e = writeJSON(filepath.Join(out, report.Regions[i].Name, "report.json"), report.Regions[i]); e != nil {
			return e
		}
		if e = writeJSON(filepath.Join(out, "report.json"), report); e != nil {
			return e
		}
	}
	for i := range report.Regions {
		if report.Regions[i].GeometryVersion == 2 {
			continue
		}
		if e = refreshRegionGeometry(ctx, out, filepath.Join(out, filepath.FromSlash(report.SourceRoot)), &report.Regions[i]); e != nil {
			return e
		}
		if e = writeJSON(filepath.Join(out, report.Regions[i].Name, "report.json"), report.Regions[i]); e != nil {
			return e
		}
	}
	if e = writeJSON(filepath.Join(out, "report.json"), report); e != nil {
		return e
	}
	return writeViewer(out)
}

func refineRegion(ctx context.Context, out, source string, r *regionReport) error {
	start := time.Now()
	fmt.Println("REFINE CLAMP", r.Name)
	w, h := r.Cells[0], r.Cells[1]
	nx := r.X1 - r.X0 + 1
	dir := filepath.Join(out, r.Name)
	stageDir := filepath.Join(dir, "clamp-hybrid")
	if e := os.MkdirAll(stageDir, 0755); e != nil {
		return e
	}
	b, e := os.ReadFile(filepath.Join(dir, "assignments.json"))
	if e != nil {
		return e
	}
	var assign assignments
	if e = json.Unmarshal(b, &assign); e != nil {
		return e
	}
	low := make([]byte, w*h*sampleCount*3)
	valid := make([]bool, w*h)
	var tiles []tileData
	b, e = os.ReadFile(filepath.Join(dir, "native-mesh.json"))
	if e != nil {
		return e
	}
	var native Mesh
	if e = json.Unmarshal(b, &native); e != nil {
		return e
	}
	geometryError := make([]float64, w*h)
	mandatory := make([]bool, w*h)
	r.SourceHeightBounds = [2]float64{math.Inf(1), math.Inf(-1)}
	for ti, t := range r.Tiles {
		if e = ctx.Err(); e != nil {
			return e
		}
		d := filepath.Join(source, t.Name)
		b, e = os.ReadFile(filepath.Join(d, "mesh.json"))
		if e != nil {
			return e
		}
		var m Mesh
		if e = json.Unmarshal(b, &m); e != nil {
			return e
		}
		_, coverage := rasterHeights(m, 128, true)
		img, e := loadPNG(filepath.Join(d, "wow.png"))
		if e != nil {
			return e
		}
		small := downsample(img, cellsPerADT*samples)
		tx, ty := ti%nx, ti/nx
		for y := 0; y < cellsPerADT*samples; y++ {
			for x := 0; x < cellsPerADT*samples; x++ {
				at := ((ty*cellsPerADT*samples+y)*w*samples + tx*cellsPerADT*samples + x) * 3
				s := y*small.Stride + x*4
				copy(low[at:at+3], small.Pix[s:s+3])
			}
		}
		heights, ok := rasterHeights(m, cellsPerADT*samples, true)
		for y := 0; y < cellsPerADT; y++ {
			for x := 0; x < cellsPerADT; x++ {
				at := (ty*cellsPerADT+y)*w + tx*cellsPerADT + x
				valid[at] = coverage[y*128/cellsPerADT*128+x*128/cellsPerADT]
				for sy := 0; sy < samples; sy++ {
					for sx := 0; sx < samples; sx++ {
						p := (y*samples+sy)*cellsPerADT*samples + x*samples + sx
						if !ok[p] {
							continue
						}
						u, v := (float64(sx)+.5)/samples, (float64(sy)+.5)/samples
						z := nativeHeightAt(native, w, tx*cellsPerADT+x, ty*cellsPerADT+y, u, v)
						delta := z - heights[p]
						geometryError[at] += delta * delta / sampleCount
					}
				}
			}
		}
		tiles = append(tiles, tileData{name: t.Name, dir: d, mesh: m, coverage: coverage})
		for _, p := range m.Positions {
			r.SourceHeightBounds[0] = min(r.SourceHeightBounds[0], p[2])
			r.SourceHeightBounds[1] = max(r.SourceHeightBounds[1], p[2])
		}
		markHeightFallback(m, tx, ty, w, h, r.Geometry.Window, mandatory)
	}
	for i, covered := range valid {
		if !covered {
			mandatory[i] = true
		}
	}
	span := r.SourceHeightBounds[1] - r.SourceHeightBounds[0]
	r.ClampPercent = [2]float64{clamp((r.Geometry.Window[0]-r.SourceHeightBounds[0])/span, 0, 1), clamp((r.Geometry.Window[1]-r.SourceHeightBounds[0])/span, 0, 1)}
	original := &fit{width: w, height: h, reference: low, valid: valid, labels: assign.Labels, variants: assign.BaseVariations}
	original.sheets, e = readSheets(filepath.Join(dir, "source-variations", "tileset"), 64)
	if e != nil {
		return e
	}
	original.colors = make([]rgb, len(original.sheets))
	for id, s := range original.sheets {
		for _, c := range s.low[16] {
			for k := range c {
				original.colors[id][k] += c[k] / sampleCount
			}
		}
	}
	var previousDetail *stageReport
	for i := range r.Stages {
		if r.Stages[i].Name == "aligned-detail" {
			copy := r.Stages[i]
			previousDetail = &copy
		}
	}
	if len(r.Stages) > 4 {
		r.Stages = r.Stages[:4]
	}
	if previousDetail == nil {
		detailStart := time.Now()
		if e = original.learnDetail(ctx, tiles, nx); e != nil {
			return e
		}
		if e = original.optimize(ctx, 1); e != nil {
			return e
		}
		if e = original.learnVariations(ctx, tiles, nx); e != nil {
			return e
		}
		detailDir := filepath.Join(dir, "aligned-detail")
		if e = os.MkdirAll(detailDir, 0755); e != nil {
			return e
		}
		detailBytes, e := original.saveSheets(filepath.Join(detailDir, "tileset"))
		if e != nil {
			return e
		}
		if e = writeJSON(filepath.Join(detailDir, "assignments.json"), assignments{Width: w, Height: h, Labels: original.labels, BaseVariations: original.variants}); e != nil {
			return e
		}
		var detailAggregate measurement
		for ti, t := range tiles {
			if e = ctx.Err(); e != nil {
				return e
			}
			fmt.Println("  ALIGNED DETAIL", t.name)
			ref, e := loadPNG(filepath.Join(t.dir, "wow.png"))
			if e != nil {
				return e
			}
			result := original.renderTile(ti%nx, ti/nx, 8192)
			m := measure(ref, result, t.coverage)
			detailAggregate.add(m)
			if e = savePNG(filepath.Join(detailDir, t.name+"-8k.png"), result); e != nil {
				return e
			}
			if e = savePNG(filepath.Join(detailDir, t.name+".png"), downsample(result, 2048)); e != nil {
				return e
			}
			if e = saveError(filepath.Join(detailDir, t.name+"-error.png"), ref, result); e != nil {
				return e
			}
		}
		r.Stages = append(r.Stages, stageReport{Name: "aligned-detail", Palette: 64, TextureBytes: detailBytes, Metrics: detailAggregate.metrics, Seconds: time.Since(detailStart).Seconds()})
		fmt.Printf("DETAIL SCORE %s MAE %.4f PSNR %.3f SSIM %.4f\n", r.Name, detailAggregate.MAE, detailAggregate.PSNR, detailAggregate.SSIM)
	} else {
		r.Stages = append(r.Stages, *previousDetail)
		saved, e := os.ReadFile(filepath.Join(dir, "aligned-detail", "assignments.json"))
		if e != nil {
			return e
		}
		var detailAssign assignments
		if e = json.Unmarshal(saved, &detailAssign); e != nil {
			return e
		}
		original.labels, original.variants = detailAssign.Labels, detailAssign.BaseVariations
		original.sheets, e = readSheets(filepath.Join(dir, "aligned-detail", "tileset"), 64)
		if e != nil {
			return e
		}
		fmt.Println("  CACHED aligned detail")
	}
	type candidate struct {
		index int
		error float64
	}
	candidates := make([]candidate, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			at := y*w + x
			loss := original.cost(x, y, original.cellLayers(x, y), int(original.variants[at]))/(sampleCount*3*100) + geometryError[at]/4
			if !valid[at] {
				loss = 1e15
			}
			candidates[at] = candidate{at, loss}
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].error > candidates[j].error })
	patchMask := append([]bool(nil), mandatory...)
	nativeValid := append([]bool(nil), valid...)
	count, mandatoryCount := 0, 0
	for _, v := range mandatory {
		if v {
			count++
			mandatoryCount++
		}
	}
	for _, c := range candidates {
		if count >= len(candidates)/4 {
			break
		}
		if patchMask[c.index] {
			continue
		}
		patchMask[c.index] = true
		count++
	}
	for i, v := range patchMask {
		if v {
			nativeValid[i] = false
		}
	}
	fmt.Printf("  required height models %.2f%%; total models %.2f%%\n", 100*float64(mandatoryCount)/float64(w*h), 100*float64(count)/float64(w*h))
	f := newFit(w, h, low, nativeValid, 64)
	if e = f.optimize(ctx, 2); e != nil {
		return e
	}
	if e = f.learnDetail(ctx, tiles, nx); e != nil {
		return e
	}
	if e = f.optimize(ctx, 1); e != nil {
		return e
	}
	if e = f.learnVariations(ctx, tiles, nx); e != nil {
		return e
	}
	textureBytes, e := f.saveSheets(filepath.Join(stageDir, "tileset"))
	if e != nil {
		return e
	}
	var aggregate measurement
	var patchBytes int64
	var modelTriangles int
	var modelTiles []patchTile
	var nativeAbs float64
	var nativePixels int64
	var modelPixels, totalPixels int64
	seamBefore, seamAfter := 0.0, 0.0
	for ti, t := range tiles {
		if e = ctx.Err(); e != nil {
			return e
		}
		fmt.Println("  HYBRID", t.name)
		ref, e := loadPNG(filepath.Join(t.dir, "wow.png"))
		if e != nil {
			return e
		}
		result := f.renderTile(ti%nx, ti/nx, 8192)
		localMask := make([]bool, cellsPerADT*cellsPerADT)
		for y := 0; y < cellsPerADT; y++ {
			for x := 0; x < cellsPerADT; x++ {
				localMask[y*cellsPerADT+x] = patchMask[(ti/nx*cellsPerADT+y)*w+ti%nx*cellsPerADT+x]
			}
		}
		placements, textureBytes, e := packPatches(stageDir, t.name, ref, result, localMask)
		if e != nil {
			return e
		}
		patchBytes += textureBytes
		patchMesh, groups := clipPatches(t.mesh, localMask, placements, patchAtlasSize, 2)
		before, after := weldPatchBoundary(&patchMesh, groups, native, w, h, ti%nx, ti/nx, patchMask)
		seamBefore = max(seamBefore, before)
		seamAfter = max(seamAfter, after)
		modelTriangles += len(patchMesh.Indices) / 3
		for i := range groups {
			groups[i].File = t.name + "-" + groups[i].File
		}
		modelTiles = append(modelTiles, patchTile{t.name, localMask})
		if e = writeJSON(filepath.Join(stageDir, t.name+"-model-atlases.json"), groups); e != nil {
			return e
		}
		m := measure(ref, result, t.coverage)
		aggregate.add(m)
		totalPixels += m.Pixels
		for ci, v := range localMask {
			if !v {
				nativeAbs += m.cellAbs[ci]
				nativePixels += m.cellPixels[ci]
			}
			if v {
				modelPixels += m.cellPixels[ci]
			}
		}
		if e = savePNG(filepath.Join(stageDir, t.name+"-8k.png"), result); e != nil {
			return e
		}
		if e = savePNG(filepath.Join(stageDir, t.name+".png"), downsample(result, 2048)); e != nil {
			return e
		}
		if e = saveError(filepath.Join(stageDir, t.name+"-error.png"), ref, result); e != nil {
			return e
		}
	}
	if e = writeJSON(filepath.Join(stageDir, "meshes.json"), modelTiles); e != nil {
		return e
	}
	if e = writeJSON(filepath.Join(stageDir, "assignments.json"), assignments{Width: w, Height: h, Labels: f.labels, BaseVariations: f.variants}); e != nil {
		return e
	}
	modelArea := float64(modelPixels) / float64(totalPixels)
	sr := stageReport{Name: "clamp-hybrid", Palette: 64, TextureBytes: textureBytes + patchBytes, Metrics: aggregate.metrics, Seconds: time.Since(start).Seconds(), ModelArea: modelArea, PatchTextureBytes: patchBytes, PatchTriangles: modelTriangles, NativeMAE: nativeAbs / float64(nativePixels*3), MandatoryModelArea: float64(mandatoryCount) / float64(w*h), NativeMajority: modelArea < .5, VisualStatus: "rejected"}
	geometry := hybridGeometry(native, w, h, tiles, nx, patchMask, r.Geometry.Window)
	sr.Geometry = &geometry
	geometry.SeamMax = seamAfter
	geometry.BoundaryAdjustment = seamBefore
	r.Stages = append(r.Stages, sr)
	r.RefinementVersion = 2
	fmt.Printf("CLAMP SCORE %s MAE %.4f PSNR %.3f SSIM %.4f native %.2f%% BLP %d\n", r.Name, sr.Metrics.MAE, sr.Metrics.PSNR, sr.Metrics.SSIM, 100*(1-modelArea), sr.TextureBytes)
	return nil
}

func nativeHeightAt(m Mesh, width, x, y int, u, v float64) float64 {
	tl, tr, bl, br := m.Positions[y*(width+1)+x][2], m.Positions[y*(width+1)+x+1][2], m.Positions[(y+1)*(width+1)+x][2], m.Positions[(y+1)*(width+1)+x+1][2]
	if u+v <= 1 {
		return tl*(1-u-v) + tr*u + bl*v
	}
	return br*(u+v-1) + bl*(1-u) + tr*(1-v)
}

func readSheets(dir string, count int) ([]sheet, error) {
	out := make([]sheet, count)
	for id := range out {
		b, e := os.ReadFile(filepath.Join(dir, fmt.Sprintf("tile-%02d.blp", id)))
		if e != nil {
			return nil, e
		}
		img, e := blp.DecodePalettedBLP1(b, 0)
		if e != nil {
			return nil, e
		}
		s := sheet{pixels: make([]byte, 32*subtile*subtile*4)}
		for layer := 0; layer < 32; layer++ {
			x, y := (layer%4)*subtile, (layer%16/4)*subtile
			if layer >= 16 {
				x += subtile * 4
			}
			for row := 0; row < subtile; row++ {
				copy(s.pixels[(layer*subtile*subtile+row*subtile)*4:(layer*subtile*subtile+(row+1)*subtile)*4], img.Pix[(y+row)*img.Stride+x*4:(y+row)*img.Stride+(x+subtile)*4])
			}
		}
		refreshLow(&s)
		out[id] = s
	}
	return out, nil
}

// Keep atlas placement deterministic across texture packing and geometry refresh.
func placePatches(mask []bool) []patchPlacement {
	const size = patchAtlasSize
	slot := int(math.Ceil(8192.0/cellsPerADT)) + 4
	columns := size / slot
	perAtlas := columns * columns
	placements := []patchPlacement{}
	for y := 0; y < cellsPerADT; y++ {
		for x := 0; x < cellsPerADT; x++ {
			if !mask[y*cellsPerADT+x] {
				continue
			}
			i := len(placements)
			x0, y0 := int(math.Ceil(float64(x)*8192/cellsPerADT-.5)), int(math.Ceil(float64(y)*8192/cellsPerADT-.5))
			x1, y1 := int(math.Ceil(float64(x+1)*8192/cellsPerADT-.5)), int(math.Ceil(float64(y+1)*8192/cellsPerADT-.5))
			placements = append(placements, patchPlacement{x, y, x0, y0, x1, y1, i / perAtlas, (i % perAtlas % columns) * slot, (i % perAtlas / columns) * slot})
		}
	}
	return placements
}

// Pack source pixels only for selected cells, with neighboring-source gutters.
// Reconstruct the viewer composite from decoded BLPs, so palette loss is scored.
func packPatches(dir, name string, ref, result *image.NRGBA, mask []bool) ([]patchPlacement, int64, error) {
	const size = patchAtlasSize
	slot := int(math.Ceil(8192.0/cellsPerADT)) + 4
	perAtlas := (size / slot) * (size / slot)
	placements := placePatches(mask)
	var total int64
	for atlas := 0; atlas*perAtlas < len(placements); atlas++ {
		img := image.NewNRGBA(image.Rect(0, 0, size, size))
		for i := 3; i < len(img.Pix); i += 4 {
			img.Pix[i] = 255
		}
		end := min(len(placements), (atlas+1)*perAtlas)
		for _, p := range placements[atlas*perAtlas : end] {
			for y := -2; y < p.y1-p.y0+2; y++ {
				for x := -2; x < p.x1-p.x0+2; x++ {
					src := min(8191, max(0, p.y0+y))*ref.Stride + min(8191, max(0, p.x0+x))*4
					dst := (p.sy+y+2)*img.Stride + (p.sx+x+2)*4
					copy(img.Pix[dst:dst+4], ref.Pix[src:src+4])
				}
			}
		}
		var pngBytes bytes.Buffer
		if e := png.Encode(&pngBytes, img); e != nil {
			return nil, 0, e
		}
		file := filepath.Join(dir, fmt.Sprintf("%s-patch-%03d.blp", name, atlas))
		if e := blp.ConvertTextureToBlp(blp.EncodeInput{PNG: pngBytes.Bytes(), Opaque: true}, file); e != nil {
			return nil, 0, e
		}
		raw, e := os.ReadFile(file)
		if e != nil {
			return nil, 0, e
		}
		total += int64(len(raw))
		decoded, e := blp.DecodePalettedBLP1(raw, 0)
		if e != nil {
			return nil, 0, e
		}
		for _, p := range placements[atlas*perAtlas : end] {
			for y := 0; y < p.y1-p.y0; y++ {
				src := (p.sy+y+2)*decoded.Stride + (p.sx+2)*4
				dst := (p.y0+y)*result.Stride + p.x0*4
				copy(result.Pix[dst:dst+(p.x1-p.x0)*4], decoded.Pix[src:src+(p.x1-p.x0)*4])
			}
		}
	}
	return placements, total, nil
}

// Clip source triangles at native cell boundaries. No flat replacement quads:
// original slopes, fan triangulation and holes survive the model fallback.
func clipPatches(source Mesh, mask []bool, placements []patchPlacement, atlasSize, gutter int) (Mesh, []patchAtlas) {
	byCell := make(map[int]patchPlacement, len(placements))
	groupN := 0
	for _, p := range placements {
		byCell[p.y*cellsPerADT+p.x] = p
		groupN = max(groupN, p.atlas+1)
	}
	groups := make([]patchAtlas, groupN)
	var out Mesh
	add := func(mesh *Mesh, poly [][3]float64) {
		base := uint32(len(mesh.Positions))
		mesh.Positions = append(mesh.Positions, poly...)
		for i := 1; i+1 < len(poly); i++ {
			mesh.Indices = append(mesh.Indices, base, base+uint32(i), base+uint32(i+1))
		}
	}
	for i := 0; i < len(source.Indices); i += 3 {
		tri := [][3]float64{source.Positions[source.Indices[i]], source.Positions[source.Indices[i+1]], source.Positions[source.Indices[i+2]]}
		for j := range tri {
			tri[j] = snapTerrainUV(tri[j])
		}
		x0, x1 := max(0, int(math.Floor(min(tri[0][0], tri[1][0], tri[2][0])*cellsPerADT))), min(cellsPerADT-1, int(math.Floor(max(tri[0][0], tri[1][0], tri[2][0])*cellsPerADT)))
		y0, y1 := max(0, int(math.Floor(min(tri[0][1], tri[1][1], tri[2][1])*cellsPerADT))), min(cellsPerADT-1, int(math.Floor(max(tri[0][1], tri[1][1], tri[2][1])*cellsPerADT)))
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if !mask[y*cellsPerADT+x] {
					continue
				}
				poly := tri
				for _, edge := range []struct {
					axis        int
					bound, sign float64
				}{{0, float64(x) / cellsPerADT, 1}, {0, float64(x+1) / cellsPerADT, -1}, {1, float64(y) / cellsPerADT, 1}, {1, float64(y+1) / cellsPerADT, -1}} {
					poly = clipEdge(poly, edge.axis, edge.bound, edge.sign)
				}
				if len(poly) < 3 {
					continue
				}
				area := 0.0
				for j, p := range poly {
					q := poly[(j+1)%len(poly)]
					area += p[0]*q[1] - q[0]*p[1]
				}
				if math.Abs(area) < 1e-15 {
					continue
				}
				add(&out, poly)
				p := byCell[y*cellsPerADT+x]
				g := &groups[p.atlas]
				g.File = fmt.Sprintf("patch-%03d.blp", p.atlas)
				add(&g.Mesh, poly)
				for _, v := range poly {
					g.UV = append(g.UV, [2]float64{(float64(p.sx+gutter) + v[0]*8192 - float64(p.x0)) / float64(atlasSize), (float64(p.sy+gutter) + v[1]*8192 - float64(p.y0)) / float64(atlasSize)})
				}
			}
		}
	}
	return out, groups
}
func clipEdge(poly [][3]float64, axis int, bound, sign float64) [][3]float64 {
	if len(poly) == 0 {
		return nil
	}
	out := make([][3]float64, 0, len(poly)+1)
	a := poly[len(poly)-1]
	for _, b := range poly {
		insideA, insideB := (a[axis]-bound)*sign >= -1e-12, (b[axis]-bound)*sign >= -1e-12
		if insideA != insideB {
			t := (bound - a[axis]) / (b[axis] - a[axis])
			var v [3]float64
			for k := range v {
				v[k] = a[k] + (b[k]-a[k])*t
			}
			out = append(out, v)
		}
		if insideB {
			out = append(out, b)
		}
		a = b
	}
	return out
}

func saveError(path string, ref, result *image.NRGBA) error {
	img := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			at := y*img.Stride + x*4
			s := (y*8)*ref.Stride + x*8*4
			v := 0
			for k := 0; k < 3; k++ {
				v += int(math.Abs(float64(int(ref.Pix[s+k]) - int(result.Pix[s+k]))))
			}
			v = min(255, v*2)
			img.Pix[at] = byte(v)
			img.Pix[at+1] = byte(v / 4)
			img.Pix[at+3] = 255
		}
	}
	return savePNG(path, img)
}
