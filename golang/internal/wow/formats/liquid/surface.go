// Package liquid contains source liquid geometry shared by terrain and WMOs.
package liquid

import "github.com/pqhuy98/wow-converter/internal/math"

// Surface is one masked liquid layer. Cells contain only present quads, with
// corners ordered 00,10,11,01 and the same 00-11 diagonal as WoW. Heights belong
// to the vertices, not the underlying terrain. Coordinates are owned by the
// containing model (world space when returned by an ADT reader).
type Surface struct {
	TypeID uint16            `json:"typeId"`
	Cells  [][4]math.Vector3 `json:"cells"`
}

// Transform returns an owned copy in the caller's coordinate system.
func Transform(surfaces []Surface, transform func(math.Vector3) math.Vector3) []Surface {
	out := make([]Surface, len(surfaces))
	for i, s := range surfaces {
		out[i].TypeID = s.TypeID
		out[i].Cells = make([][4]math.Vector3, len(s.Cells))
		for j, cell := range s.Cells {
			for k, p := range cell {
				out[i].Cells[j][k] = transform(p)
			}
		}
	}
	return out
}
