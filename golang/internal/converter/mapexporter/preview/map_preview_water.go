package preview

import (
	"fmt"
	"math"

	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/liquid"
)

// Keep source liquid masks and heights intact: decimating these small grids can
// bridge dry holes or flatten sloped water. No textures or model conversion.
func (b *previewBuilder) addLiquids(asset *PreviewAsset, surfaces []liquid.Surface) error {
	indices := map[uint16]int{}
	var parts []PreviewPart
	var vertices [][]float32
	for _, s := range surfaces {
		if err := b.ctx.Err(); err != nil {
			return err
		}
		if len(s.Cells) == 0 {
			continue
		}
		i, found := indices[s.TypeID]
		if !found {
			i = len(parts)
			indices[s.TypeID] = i
			parts = append(parts, PreviewPart{Mesh: fmt.Sprintf("%s-water-%d.bin", asset.ID, s.TypeID), Liquid: &PreviewLiquid{TypeID: s.TypeID, Min: imath.V3All(math.Inf(1)), Max: imath.V3All(math.Inf(-1))}})
			vertices = append(vertices, nil)
		}
		for _, cell := range s.Cells {
			for _, p := range cell {
				parts[i].Liquid.Min = imath.V3Min(parts[i].Liquid.Min, p)
				parts[i].Liquid.Max = imath.V3Max(parts[i].Liquid.Max, p)
			}
			for _, k := range []int{0, 1, 2, 0, 2, 3} {
				p := cell[k]
				vertices[i] = append(vertices[i], float32(p[0]), float32(p[1]), float32(p[2]), 0, 0, 1, 0, 0)
			}
		}
	}
	for i := range parts {
		parts[i].Vertices = len(vertices[i]) / 8
		if err := b.write(parts[i].Mesh, previewVertexBytes(vertices[i])); err != nil {
			return err
		}
		asset.Parts = append(asset.Parts, parts[i])
	}
	return nil
}
