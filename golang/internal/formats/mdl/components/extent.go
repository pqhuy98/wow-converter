package components

import "github.com/pqhuy98/wow-converter/internal/math"

type Bound struct {
	MinimumExtent math.Vector3
	MaximumExtent math.Vector3
	BoundsRadius  float64
}
