package mapexporter

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

const (
	waterDeepDepth     = 51.2
	waterSeamTolerance = .0625
)

type waterCell struct {
	v      [4]imath.Vector3
	typeID uint16
	bank   int
}

type mapWater struct {
	width, height int
	ox, oy        float64
	cells         []waterCell
	bins          [][]int
	report        waterReport
}

type waterReport struct {
	SourceLayers       int            `json:"sourceLayers"`
	SourceCells        int            `json:"sourceCells"`
	WMOLayers          int            `json:"wmoLayers"`
	NativeCorners      int            `json:"nativeCorners"`
	ApproximatedCells  int            `json:"approximatedCells"`
	UnsupportedCells   int            `json:"unsupportedCells"`
	UnsupportedReasons map[string]int `json:"unsupportedReasons"`
	Types              map[uint16]int `json:"sourceTypes"`
	WaterPathingCells  int            `json:"waterPathingCells"`
	DeepPathingCells   int            `json:"deepPathingCells"`
	MinZ               float64        `json:"minZ"`
	MaxZ               float64        `json:"maxZ"`
}

// collectMapWater uses the same final bounds/rotation/clamp as collision and
// doodad placement. Liquid metadata never participates in choosing map scale.
func collectMapWater(ctx context.Context, m *common.WowObjectManager, t data.Terrain, cfg *MapExportConfig) (*mapWater, error) {
	w := &mapWater{width: t.Map.Width, height: t.Map.Height, ox: float64(t.Map.Offset.X), oy: float64(t.Map.Offset.Y)}
	w.report = waterReport{UnsupportedReasons: map[string]int{}, Types: map[uint16]int{}, MinZ: math.Inf(1), MaxZ: math.Inf(-1)}
	var count int
	m.IterateObjects(func(o *common.WowObject, _ common.ObjectAbsolute) {
		if o.Model != nil {
			count += len(o.Model.Liquids)
		}
	})
	if count == 0 {
		return w, nil
	}
	banks, err := m.AssetManager.LiquidSoundBanks(ctx)
	if err != nil {
		return nil, err
	}
	ext := common.ComputeAbsoluteMinMaxExtents(m.Roots)
	size := imath.V3Sub(ext.Max, ext.Min)
	if size[0] <= 0 || size[1] <= 0 || size[2] <= 0 {
		return nil, fmt.Errorf("invalid liquid map bounds")
	}
	sx, sy := float64(w.width)*128/size[0], float64(w.height)*128/size[1]
	sz := config.MaxGameHeightDiff() / (size[2] * (cfg.Terrain.ClampPercent.Upper - cfg.Terrain.ClampPercent.Lower))
	m.IterateObjects(func(o *common.WowObject, a common.ObjectAbsolute) {
		if o.Model == nil || common.IsWowUnit(o) {
			return
		}
		for _, s := range o.Model.Liquids {
			w.report.SourceLayers++
			if o.Type != common.WowObjectADT {
				w.report.WMOLayers++
			}
			bank, ok := banks[s.TypeID]
			if !ok {
				bank = -1
			}
			for _, src := range s.Cells {
				cell := waterCell{typeID: s.TypeID, bank: bank}
				for j, p := range src {
					p = imath.V3Sum(a.Position, imath.V3Rotate(imath.V3Scale(p, a.ScaleFactor), a.Rotation))
					cell.v[j] = imath.Vector3{w.ox + (p[0]-ext.Min[0])*sx, w.oy + (p[1]-ext.Min[1])*sy, config.DataHeightToGameZ(float64(config.DataHeightMin())) + (p[2]-ext.Min[2]-size[2]*cfg.Terrain.ClampPercent.Lower)*sz}
				}
				w.cells = append(w.cells, cell)
				w.report.Types[s.TypeID]++
				w.report.SourceCells++
			}
		}
	})
	w.index()
	return w, ctx.Err()
}

func (w *mapWater) index() {
	w.bins = make([][]int, w.width*w.height)
	for i, c := range w.cells {
		lo, hi := c.v[0], c.v[0]
		for _, p := range c.v {
			lo = imath.V3Min(lo, p)
			hi = imath.V3Max(hi, p)
			w.report.MinZ = min(w.report.MinZ, p[2])
			w.report.MaxZ = max(w.report.MaxZ, p[2])
		}
		for y := max(0, int(math.Floor((lo[1]-w.oy-waterSeamTolerance)/128))); y <= min(w.height-1, int(math.Floor((hi[1]-w.oy+waterSeamTolerance)/128))); y++ {
			for x := max(0, int(math.Floor((lo[0]-w.ox-waterSeamTolerance)/128))); x <= min(w.width-1, int(math.Floor((hi[0]-w.ox+waterSeamTolerance)/128))); x++ {
				w.bins[y*w.width+x] = append(w.bins[y*w.width+x], i)
			}
		}
	}
	if len(w.cells) == 0 {
		w.report.MinZ, w.report.MaxZ = 0, 0
	}
}

func waterCellHeight(c waterCell, x, y float64) (float64, bool) {
	for _, ids := range [][3]int{{0, 1, 2}, {0, 2, 3}} {
		z, ok := navigationHeight([3]imath.Vector3{c.v[ids[0]], c.v[ids[1]], c.v[ids[2]]}, x, y)
		if ok {
			return z, true
		}
	}
	// Neighboring ADT chunks store independent float32 origins. Close their
	// subpixel seams without filling real holes in the source liquid mask.
	z, distance := waterEdgeHeight(c, x, y)
	return z, distance <= waterSeamTolerance
}

func waterEdgeHeight(c waterCell, x, y float64) (z, distance float64) {
	distance = math.Inf(1)
	for k, a := range c.v {
		b := c.v[(k+1)%4]
		dx, dy := b[0]-a[0], b[1]-a[1]
		u := 0.
		if den := dx*dx + dy*dy; den > 0 {
			u = max(0, min(1, ((x-a[0])*dx+(y-a[1])*dy)/den))
		}
		if d := math.Hypot(x-a[0]-u*dx, y-a[1]-u*dy); d < distance {
			distance, z = d, a[2]+u*(b[2]-a[2])
		}
	}
	return z, distance
}

// sample picks the first liquid above the selected floor. A bridge/upper room
// above a lower liquid stays dry; higher stacked layers are handled separately.
func (w *mapWater) sample(x, y, floor float64, waterOnly bool) (float64, int, bool) {
	if w == nil || len(w.cells) == 0 {
		return 0, 0, false
	}
	xi, yi := int(math.Floor((x-w.ox)/128)), int(math.Floor((y-w.oy)/128))
	if xi == w.width {
		xi--
	}
	if yi == w.height {
		yi--
	}
	if xi < 0 || yi < 0 || xi >= w.width || yi >= w.height {
		return 0, 0, false
	}
	best := math.Inf(1)
	bank := 0
	for _, id := range w.bins[yi*w.width+xi] {
		c := w.cells[id]
		if waterOnly && (c.bank < 0 || c.bank > 1) {
			continue
		}
		if z, ok := waterCellHeight(c, x, y); ok && z > floor+.125 && z < best {
			best, bank = z, c.bank
		}
	}
	return best, bank, !math.IsInf(best, 1)
}

func (w *mapWater) blocksGround(x, y, z float64) bool {
	h, bank, ok := w.sample(x, y, z, false)
	return ok && (bank < 0 || bank > 1 || h-z > waterDeepDepth)
}

// project uses native WC3 water only. Presence follows the source at the WC3
// grid resolution; neighboring corner heights stay coherent across shores.
func (w *mapWater) project(ctx context.Context, t data.Terrain) error {
	w.report.NativeCorners, w.report.ApproximatedCells, w.report.UnsupportedCells = 0, 0, 0
	w.report.UnsupportedReasons = map[string]int{}
	if len(w.cells) == 0 {
		return nil
	}
	assigned := make([]bool, (w.width+1)*(w.height+1))
	for y := 0; y <= w.height; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		for x := 0; x <= w.width; x++ {
			px, py := w.ox+float64(x)*128, w.oy+float64(y)*128
			t.Flags[y][x] &^= uint16(data.TerrainWater)
			ground := config.DataHeightToGameZ(float64(t.GroundHeight[y][x]))
			z, _, visible := w.sample(px, py, ground, true)
			found := visible
			if !found {
				z, _, found = w.sample(px, py, math.Inf(-1), true)
			}
			if !found {
				// Supply every corner used by a native quad with its nearest source edge
				// height. Never interpolate towards an unrelated default water height.
				best := math.Inf(1)
				for yy := max(0, y-1); yy <= min(w.height-1, y); yy++ {
					for xx := max(0, x-1); xx <= min(w.width-1, x); xx++ {
						for _, id := range w.bins[yy*w.width+xx] {
							c := w.cells[id]
							if c.bank < 0 || c.bank > 1 {
								continue
							}
							if h, d := waterEdgeHeight(c, px, py); d < best {
								best, z, found = d, h, true
							}
						}
					}
				}
			}
			if !found {
				continue
			}
			raw := config.GameZToWaterHeight(z)
			if raw < 0 || raw > 0x3fff {
				continue
			}
			assigned[y*(w.width+1)+x] = true
			t.WaterHeight[y][x] = raw
			if visible {
				t.Flags[y][x] |= uint16(data.TerrainWater)
				w.report.NativeCorners++
			}
		}
	}
	// Native water has one surface per 128-unit quad. Keep wet source samples
	// covered instead of punching gaps into the surface and patching them with
	// a second, differently shaded material. Small shores round to this grid.
	cover := func(px, py float64) {
		if _, present := nativeWaterHeight(t, px, py); present {
			return
		}
		x, y := int(math.Floor((px-w.ox)/128)), int(math.Floor((py-w.oy)/128))
		if x == w.width {
			x--
		}
		if y == w.height {
			y--
		}
		if x < 0 || y < 0 || x >= w.width || y >= w.height {
			return
		}
		best, bx, by := math.Inf(1), -1, -1
		for _, d := range [][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
			xx, yy := x+d[0], y+d[1]
			if !assigned[yy*(w.width+1)+xx] {
				continue
			}
			if dist := math.Hypot(w.ox+float64(xx)*128-px, w.oy+float64(yy)*128-py); dist < best {
				best, bx, by = dist, xx, yy
			}
		}
		if bx >= 0 {
			t.Flags[by][bx] |= uint16(data.TerrainWater)
			w.report.NativeCorners++
		}
	}
	for y := 0; y < w.height*4; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		for x := 0; x < w.width*4; x++ {
			px, py := w.ox+(float64(x)+.5)*32, w.oy+(float64(y)+.5)*32
			ground := config.DataHeightToGameZ(GetTerrainHeight(t, (float64(x)+.5)/float64(w.width*4), (float64(y)+.5)/float64(w.height*4)))
			if z, _, wet := w.sample(px, py, ground, true); wet {
				if raw := config.GameZToWaterHeight(z); raw >= 0 && raw <= 0x3fff {
					cover(px, py)
				}
			}
		}
	}
	for _, c := range w.cells {
		reason := ""
		switch {
		case c.bank < 0:
			reason = "unknown-type"
		case c.bank > 1:
			reason = "material"
		}
		for _, p := range c.v {
			if raw := config.GameZToWaterHeight(p[2]); reason == "" && (raw < 0 || raw > 0x3fff) {
				reason = "height-range"
			}
		}
		if reason != "" {
			w.report.UnsupportedCells++
			w.report.UnsupportedReasons[reason]++
			continue
		}
		center := imath.V3Mean(c.v[0], c.v[2])
		u, v := (center[0]-w.ox)/(float64(w.width)*128), (center[1]-w.oy)/(float64(w.height)*128)
		if center[2] > config.DataHeightToGameZ(GetTerrainHeight(t, u, v))+.125 {
			cover(center[0], center[1])
		}
		for _, p := range append(c.v[:], center) {
			if z, ok := nativeWaterHeight(t, p[0], p[1]); !ok || math.Abs(z-p[2]) > 2 {
				w.report.ApproximatedCells++
				break
			}
		}
	}
	return nil
}

func nativeWaterHeight(t data.Terrain, px, py float64) (float64, bool) {
	x, y := (px-float64(t.Map.Offset.X))/128, (py-float64(t.Map.Offset.Y))/128
	xi, yi := int(math.Floor(x)), int(math.Floor(y))
	if xi == t.Map.Width {
		xi--
	}
	if yi == t.Map.Height {
		yi--
	}
	if xi < 0 || yi < 0 || xi >= t.Map.Width || yi >= t.Map.Height {
		return 0, false
	}
	var v [4]imath.Vector3
	present := false
	for k, d := range [][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		xx, yy := xi+d[0], yi+d[1]
		v[k] = imath.Vector3{float64(xx), float64(yy), config.WaterHeightToGameZ(float64(t.WaterHeight[yy][xx]))}
		present = present || t.Flags[yy][xx]&uint16(data.TerrainWater) != 0
	}
	z, _ := waterCellHeight(waterCell{v: v}, x, y)
	return z, present
}

// Cached exports may contain assets from the former liquid-mesh fallback.
// Remove only those generated files, preserving other imported water assets.
func removeLegacyWaterMeshes(dir string) error {
	root := filepath.Join(dir, "water")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		ext := filepath.Ext(name)
		if !entry.IsDir() && strings.HasPrefix(name, "liquid-") && (ext == ".mdl" || ext == ".mdx" || ext == ".tga") {
			if _, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "liquid-"), ext)); err != nil {
				continue
			}
			if err := os.Remove(filepath.Join(root, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// writePathing adds liquid semantics to either the navigation mask or an empty
// base mask. Optional doodad blockers never own swimming/building flags.
func (w *mapWater) write(ctx context.Context, t data.Terrain, nav *terrainNavigation, dir string, includeBlockers bool) error {
	if len(w.cells) == 0 {
		return nil
	}
	w.report.WaterPathingCells, w.report.DeepPathingCells = 0, 0
	width, height := w.width*4, w.height*4
	mask := make([]byte, width*height)
	if nav != nil && includeBlockers {
		copy(mask, nav.pathing)
	}
	preview := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		for x := 0; x < width; x++ {
			px, py := w.ox+(float64(x)+.5)*32, w.oy+(float64(y)+.5)*32
			ground := config.DataHeightToGameZ(GetTerrainHeight(t, (float64(x)+.5)/float64(width), (float64(y)+.5)/float64(height)))
			z, bank, ok := w.sample(px, py, ground, false)
			if !ok {
				continue
			}
			flag := byte(8)
			c := color.NRGBA{40, 150, 220, 255}
			if bank >= 0 && bank <= 1 {
				flag |= 64
				w.report.WaterPathingCells++
				if z-ground > waterDeepDepth {
					flag |= 2
					w.report.DeepPathingCells++
					c = color.NRGBA{25, 65, 155, 255}
				}
			} else {
				flag |= 2
				c = color.NRGBA{210, 85, 35, 255}
			}
			mask[y*width+x] |= flag
			preview.SetNRGBA(x, height-1-y, c)
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b := make([]byte, 16+len(mask))
	copy(b, "MP3W")
	binary.LittleEndian.PutUint32(b[8:], uint32(width))
	binary.LittleEndian.PutUint32(b[12:], uint32(height))
	copy(b[16:], mask)
	if err := os.WriteFile(filepath.Join(dir, "war3map.wpm"), b, 0644); err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(dir, "terrain-water.png"))
	if err != nil {
		return err
	}
	encodeErr := png.Encode(f, preview)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	j, err := json.MarshalIndent(w.report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "terrain-water.json"), j, 0644)
}
