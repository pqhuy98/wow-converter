package adt

import (
	"fmt"
	"math"

	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/liquid"
)

// LiquidSurfaces returns masked liquid geometry in native WoW world XYZ.
func (a *ADTLoader) LiquidSurfaces() ([]liquid.Surface, error) {
	var out []liquid.Surface
	for i, positionChunk := range a.Chunks {
		chunk := LiquidChunk{}
		if i < len(a.LiquidChunks) {
			chunk = a.LiquidChunks[i]
		}
		if len(chunk.Instances) == 0 && positionChunk.LegacyLiquid != nil {
			chunk.Instances = []LiquidInstance{*positionChunk.LegacyLiquid}
		}
		position := positionChunk.Position
		for _, layer := range chunk.Instances {
			if layer.VertexData.Invalid {
				return nil, fmt.Errorf("unsupported ADT liquid vertex format at chunk %d", i)
			}
			w, h := int(layer.Width), int(layer.Height)
			if w > 8 || h > 8 || int(layer.XOffset)+w > 8 || int(layer.YOffset)+h > 8 {
				return nil, fmt.Errorf("invalid ADT liquid rectangle")
			}
			if len(layer.Bitmap) > 0 && len(layer.Bitmap)*8 < w*h {
				return nil, fmt.Errorf("short ADT liquid bitmap")
			}
			s := liquid.Surface{TypeID: layer.LiquidType}
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					bit := y*w + x
					if len(layer.Bitmap) > 0 && layer.Bitmap[bit/8]&(1<<uint(bit%8)) == 0 {
						continue
					}
					var cell [4]imath.Vector3
					for k, d := range [][2]int{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
						vx, vy := x+d[0], y+d[1]
						z := float64(0) // Depth-only/ocean and absent vertex data use sea level.
						if len(layer.VertexData.Height) > 0 {
							index := vy*(w+1) + vx
							if index >= len(layer.VertexData.Height) {
								return nil, fmt.Errorf("short ADT liquid height array")
							}
							z = float64(layer.VertexData.Height[index])
						}
						if math.IsNaN(z) || math.IsInf(z, 0) {
							return nil, fmt.Errorf("nonfinite ADT liquid height")
						}
						const unit = 533.3333333333333 / 128
						cell[k] = imath.Vector3{float64(position[0]) - float64(int(layer.YOffset)+vy)*unit, float64(position[1]) - float64(int(layer.XOffset)+vx)*unit, z}
					}
					s.Cells = append(s.Cells, cell)
				}
			}
			if len(s.Cells) > 0 {
				out = append(out, s)
			}
		}
	}
	return out, nil
}
