package wmo

import (
	"fmt"
	"math"

	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/liquid"
)

// LiquidSurfaces returns a group's masked liquid in native WMO local XYZ.
func (l *Loader) LiquidSurfaces(rootFlags uint32) ([]liquid.Surface, error) {
	g := l.Liquid
	if g == nil {
		return nil, nil
	}
	if len(g.Corner) < 3 || g.VertX != g.TileX+1 || g.VertY != g.TileY+1 || uint64(g.VertX)*uint64(g.VertY) != uint64(len(g.Vertices)) || uint64(g.TileX)*uint64(g.TileY) != uint64(len(g.Tiles)) {
		return nil, fmt.Errorf("invalid WMO liquid grid")
	}
	typ := uint16(l.LiquidType)
	if rootFlags&4 == 0 {
		typ = (typ + 1)
		if l.LiquidType == 15 {
			typ = 0
		}
	}
	surfaces := map[uint16]int{}
	var out []liquid.Surface
	for y := 0; y < int(g.TileY); y++ {
		for x := 0; x < int(g.TileX); x++ {
			flag := g.Tiles[y*int(g.TileX)+x] & 15
			if flag == 15 {
				continue
			}
			id := typ
			if id == 0 {
				id = uint16(flag) + 1
			}
			if rootFlags&4 == 0 {
				// Legacy groups encode a material category, not a DB2 row.
				// Match the WMO extractor's water/ocean/magma/slime IDs.
				switch (id - 1) % 4 {
				case 0:
					id = 13
					if l.Flags&0x80000 != 0 {
						id = 14
					}
				case 1:
					id = 14
				case 2:
					id = 19
				case 3:
					id = 20
				}
			}
			index, ok := surfaces[id]
			if !ok {
				index = len(out)
				surfaces[id] = index
				out = append(out, liquid.Surface{TypeID: id})
			}
			var cell [4]imath.Vector3
			for k, d := range [][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
				vx, vy := x+d[0], y+d[1]
				z := float64(g.Vertices[vy*int(g.VertX)+vx].Height)
				if math.IsNaN(z) || math.IsInf(z, 0) {
					return nil, fmt.Errorf("nonfinite WMO liquid height")
				}
				const unit = 533.3333333333333 / 128
				cell[k] = imath.Vector3{float64(g.Corner[0]) + float64(vx)*unit, float64(g.Corner[1]) + float64(vy)*unit, z}
			}
			out[index].Cells = append(out[index].Cells, cell)
		}
	}
	return out, nil
}
