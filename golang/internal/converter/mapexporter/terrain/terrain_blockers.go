package terrain

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	mdlx "github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wc3"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

const pathBlockerModel = "empty.mdl"

type blockerSquare struct {
	x, y, size int
	flags      byte
}

// Each instance is one completely solid square of a single blocking class.
// ponytail: compare four disjoint greedy scan directions with a largest-first
// overlapping cover. Neither heuristic proves a minimum-square cover.
func packBlockers(cells []byte, width, height int) []blockerSquare {
	var best []blockerSquare
	for direction := 0; direction < 4; direction++ {
		used := make([]bool, len(cells))
		index := func(x, y int) int {
			if direction&1 != 0 {
				x = width - 1 - x
			}
			if direction&2 != 0 {
				y = height - 1 - y
			}
			return y*width + x
		}
		var squares []blockerSquare
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				i := index(x, y)
				flags := cells[i]
				if used[i] || flags == 0 {
					continue
				}
				size := 1
				limit := min(256, width-x, height-y)
				for side := 2; side <= limit; side++ {
					valid := true
					for j := 0; j < side; j++ {
						a, b := index(x+j, y+side-1), index(x+side-1, y+j)
						if used[a] || used[b] || cells[a] != flags || cells[b] != flags {
							valid = false
							break
						}
					}
					if !valid {
						break
					}
					size = side
				}
				for yy := y; yy < y+size; yy++ {
					for xx := x; xx < x+size; xx++ {
						used[index(xx, yy)] = true
					}
				}
				x0, y0 := x, y
				if direction&1 != 0 {
					x0 = width - x - size
				}
				if direction&2 != 0 {
					y0 = height - y - size
				}
				squares = append(squares, blockerSquare{x: x0, y: y0, size: size, flags: flags})
			}
		}
		if best == nil || len(squares) < len(best) {
			best = squares
		}
	}
	if overlapping := packLargestSquares(cells, width, height); len(overlapping) < len(best) {
		best = overlapping
	}
	return best
}

// Same-class overlap can cover irregular borders with fewer solid squares.
// DP finds each anchor's largest homogeneous square; process large ones first
// and retain an instance only when it covers at least one previously open cell.
func packLargestSquares(cells []byte, width, height int) []blockerSquare {
	sizes := make([]int, len(cells))
	var buckets [257][]blockerSquare
	for y := height - 1; y >= 0; y-- {
		for x := width - 1; x >= 0; x-- {
			i := y*width + x
			if cells[i] == 0 {
				continue
			}
			size := 1
			if x+1 < width && y+1 < height && cells[i+1] == cells[i] && cells[i+width] == cells[i] && cells[i+width+1] == cells[i] {
				size = min(256, 1+min(sizes[i+1], sizes[i+width], sizes[i+width+1]))
			}
			sizes[i] = size
			buckets[size] = append(buckets[size], blockerSquare{x: x, y: y, size: size, flags: cells[i]})
		}
	}
	covered := make([]bool, len(cells))
	var result []blockerSquare
	for size := 256; size > 0; size-- {
		for _, square := range buckets[size] {
			fresh := false
			for y := square.y; y < square.y+size && !fresh; y++ {
				for x := square.x; x < square.x+size; x++ {
					if !covered[y*width+x] {
						fresh = true
						break
					}
				}
			}
			if !fresh {
				continue
			}
			result = append(result, square)
			for y := square.y; y < square.y+size; y++ {
				for x := square.x; x < square.x+size; x++ {
					covered[y*width+x] = true
				}
			}
		}
	}
	return result
}

func (r *TerrainNavigation) PlaceBlockers(mm *extra.MapManager, dir string) error {
	if err := writePathBlockerModel(dir); err != nil {
		return err
	}
	// World Editor snaps pathing doodads to a 64-unit grid on load/save.
	// Native readback also uses ceil(size/2) for odd footprints. Arbitrary
	// isolated 32-unit cells cannot survive those rules as solid squares.
	// Conservatively combine 2x2 pathing cells, then pack that native grid.
	coarseWidth, coarseHeight := r.Width/2, r.Height/2
	coarse := make([]byte, coarseWidth*coarseHeight)
	for y := 0; y < r.Height; y++ {
		for x := 0; x < r.Width; x++ {
			coarse[(y/2)*coarseWidth+x/2] |= r.Pathing[y*r.Width+x]
		}
	}
	r.Reachable, r.GroundBlocked, r.AirBlocked, r.WaterBlocked = 0, 0, 0, 0
	for y := 0; y < r.Height; y++ {
		for x := 0; x < r.Width; x++ {
			flags := coarse[(y/2)*coarseWidth+x/2]
			if r.Pathing[y*r.Width+x] == 0 && flags != 0 {
				r.BlockerPadding++
			}
			r.Pathing[y*r.Width+x] = flags
			switch {
			case flags == 0 && len(r.waterBlocked) > 0 && r.waterBlocked[y*r.Width+x]:
				r.WaterBlocked++
			case flags == 0:
				r.Reachable++
			case flags&4 != 0:
				r.AirBlocked++
			default:
				r.GroundBlocked++
			}
		}
	}
	squares := packBlockers(coarse, coarseWidth, coarseHeight)
	for i := range squares {
		squares[i].x *= 2
		squares[i].y *= 2
		squares[i].size *= 2
	}
	types := map[string]*extra.DoodadType{}
	terrain := mm.GetTerrain()
	for _, square := range squares {
		key := fmt.Sprintf("%dx%d-%d", square.size, square.size, square.flags)
		typ := types[key]
		if typ == nil {
			texture := blockerTexture(square)
			if texture == "" {
				texture = filepath.ToSlash(filepath.Join("pathing", "square-"+key+".tga"))
				if err := os.MkdirAll(filepath.Join(dir, "pathing"), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(texture)), blockerTGA(square), 0o644); err != nil {
					return err
				}
			}
			kind := "Ground"
			if square.flags&4 != 0 {
				kind = "Ground + Air"
			}
			typ = mm.AddDoodadType([]data.Modification{
				{ID: "dnam", Type: data.ModificationString, Value: fmt.Sprintf("~Path Blocker %s %dx%d", kind, square.size, square.size)},
				{ID: "dfil", Type: data.ModificationString, Value: pathBlockerModel},
				{ID: "dptx", Type: data.ModificationString, Value: filepath.ToSlash(texture)},
				{ID: "dvar", Type: data.ModificationInt, Value: 1},
				{ID: "dfxr", Type: data.ModificationUnreal, Value: float32(270)},
				{ID: "dmis", Type: data.ModificationUnreal, Value: float32(1)},
				{ID: "dmas", Type: data.ModificationUnreal, Value: float32(1)},
				{ID: "dshd", Type: data.ModificationInt, Value: 0},
				{ID: "duch", Type: data.ModificationInt, Value: 1},
				{ID: "dsmm", Type: data.ModificationInt, Value: 0},
			}, false)
			types[key] = typ
		}
		// Even sizes and even origin cells produce centers stable under
		// native 64-unit snapping (offset 32 for sizes congruent to 2 mod 4).
		x := float64(terrain.Map.Offset.X) + float64(square.x+square.size/2)*32
		y := float64(terrain.Map.Offset.Y) + float64(square.y+square.size/2)*32
		mm.AddDoodad(typ, data.Doodad{Angle: wc3.Angle(270), Position: [3]float32{float32(x), float32(y), 0}, Scale: [3]float32{1, 1, 1}, SkinID: typ.Code,
			Flags: data.DoodadFlag{Visible: true, Solid: true}, Life: 100, RandomItemSetPtr: -1})
	}
	r.BlockerInstances = len(squares)
	r.BlockerTypes = len(types)
	return r.Write(dir, true)
}

func writePathBlockerModel(dir string) error {
	template, err := workspace.ResolveTemplateEmptyDir()
	if err != nil {
		return err
	}
	text, err := os.ReadFile(filepath.Join(template, pathBlockerModel))
	if err != nil {
		return err
	}
	out, err := mdlx.ConvertMdlToMdx(string(text))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, pathBlockerModel), text, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "empty.mdx"), out, 0o644)
}

func blockerTexture(r blockerSquare) string {
	if r.size == 2 || r.size == 4 {
		suffix := "Default"
		if r.flags&4 != 0 {
			suffix = "Unflyable"
		}
		return fmt.Sprintf("PathTextures\\%dx%d%s.tga", r.size, r.size, suffix)
	}
	if r.size == 8 && r.flags&4 == 0 {
		return "PathTextures\\8x8SimpleSolid.tga"
	}
	return ""
}

func blockerTGA(r blockerSquare) []byte {
	// Every pixel is occupied: one solid square, without masks or padding.
	b := make([]byte, 18+r.size*r.size*3)
	b[2] = 2
	binary.LittleEndian.PutUint16(b[12:], uint16(r.size))
	binary.LittleEndian.PutUint16(b[14:], uint16(r.size))
	b[16] = 24
	for i := 18; i < len(b); i += 3 {
		b[i], b[i+2] = 255, 255
		if r.flags&4 != 0 {
			b[i+1] = 255
		}
	}
	return b
}
