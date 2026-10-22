package terrainfit

import (
	"image"
	"math"
)

// A scaled material spans several native cells. Allocate its circular source
// phase in the same clustering budget as paint and tint, rather than repeating
// the whole texture in every cell or allocating scale*scale extra tile IDs.
type materialPhases struct {
	scales []int
	span   [2]float64
}

func newMaterialPhases(catalog []terrainMaterial, images map[uint32]*image.NRGBA, heightBlend bool) materialPhases {
	p := materialPhases{span: [2]float64{1, 1}}
	seen := map[int]bool{}
	for _, m := range catalog {
		if m.Scale <= 1 || seen[m.Scale] {
			continue
		}
		varies := false
		for _, id := range []uint32{m.ID, m.HeightID} {
			img := images[id]
			if img == nil || len(img.Pix) < 4 {
				continue
			}
			for i := 4; i < len(img.Pix) && !varies; i += 4 {
				for k := 0; k < 4; k++ {
					if (k < 3 && id == m.ID || k == 3 && heightBlend && m.HeightScale != 0) && img.Pix[i+k] != img.Pix[k] {
						varies = true
					}
				}
			}
		}
		if varies {
			seen[m.Scale] = true
			p.scales = append(p.scales, m.Scale)
		}
	}
	return p
}

func (p *materialPhases) allocate(features []float32, catalog []terrainMaterial, w, h int, grid *TilesetGrid) {
	dim := len(catalog) + 4*len(p.scales) + 3
	if grid != nil {
		for k := range p.span {
			p.span[k] = (math.Abs(grid.StepX[k]) + math.Abs(grid.StepY[k])) * cellsPerADT
		}
	}
	for y := 0; y <= h; y++ {
		for x := 0; x <= w; x++ {
			pos := [2]float64{float64(x), float64(y)}
			if grid != nil {
				for k := range pos {
					// Bake a source-aligned square, then use the existing RGB rotation.
					pos[k] = (grid.Origin[k] + float64(x)*grid.StepX[k] + float64(y)*grid.StepY[k] + min(0, grid.StepX[k]) + min(0, grid.StepY[k])) * cellsPerADT
				}
			}
			point := features[(y*(w+1)+x)*dim : (y*(w+1)+x+1)*dim]
			for si, scale := range p.scales {
				strength := float32(0)
				for i, m := range catalog {
					if m.Scale == scale {
						strength += point[i]
					}
				}
				for k, value := range pos {
					// Source bake UVs restart at each ADT (128 base repeats).
					angle := 2 * math.Pi * (value - math.Floor(value/cellsPerADT)*cellsPerADT) / float64(scale)
					at := len(catalog) + si*4 + k*2
					point[at] = .35 * strength * float32(math.Cos(angle))
					point[at+1] = .35 * strength * float32(math.Sin(angle))
				}
			}
		}
	}
}

func (p materialPhases) uv(center []float32, materials, scale int, u, v float64) (float64, float64) {
	pos := [2]float64{u * p.span[0], v * p.span[1]}
	for si, s := range p.scales {
		if s == scale {
			for k := range pos {
				at := materials + si*4 + k*2
				angle := math.Atan2(float64(center[at+1]), float64(center[at]))
				pos[k] += (angle/(2*math.Pi) - math.Floor(angle/(2*math.Pi))) * float64(scale)
			}
		}
	}
	return pos[0] / float64(scale), pos[1] / float64(scale)
}
