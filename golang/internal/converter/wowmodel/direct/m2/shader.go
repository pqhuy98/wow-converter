package directm2

import (
	"fmt"
	"math"

	imath "github.com/pqhuy98/wow-converter/internal/math"
)

// M2's explicit effect table selects a pixel combiner AND vertex UV routing.
// The numeric pixel IDs follow commonM2Material.glsl in WebWowViewerCpp.
// Routing for crossfades includes all four samplers, as in WhiteoutFlakes.
type m2Shader struct {
	pixel  int
	coords [4]m2Coord
	edge   bool
}

type m2Coord uint8

const (
	coordT1M0 m2Coord = iota
	coordT1M1
	coordT2M0
	coordT2M1
	coordEnv
	coordT1
	coordT2
)

var m2VertexCoords = [18][4]m2Coord{
	{coordT1M0, coordT1M0, coordT1, coordT1},
	{coordEnv, coordEnv, coordT1, coordT1},
	{coordT1M0, coordT2M1, coordT1, coordT2},
	{coordT1M0, coordEnv, coordT1, coordT1},
	{coordEnv, coordT1M1, coordT1, coordT1},
	{coordEnv, coordEnv, coordT1, coordT1},
	{coordT1M0, coordEnv, coordT1, coordT1},
	{coordT1M0, coordT1M1, coordT1, coordT1},
	{coordT1M0, coordT1M0, coordT1M0, coordT1M0},
	{coordT1M0, coordT1M0, coordT1, coordT1},
	{coordT2M0, coordT2M0, coordT2, coordT2},
	{coordT1M0, coordEnv, coordT2, coordT2},
	{coordT1M0, coordT2M1, coordT1, coordT2},
	{coordT1M0, coordT1M0, coordT1M0, coordT2M1},
	{coordEnv, coordEnv, coordT1, coordT1},
	{coordT1M0, coordT2M1, coordT1, coordT1},
	{coordT1M0, coordT2M1, coordT1, coordT2},
	{coordT2M1, coordT1, coordT1, coordT2},
}

var m2Effects = [36][2]int{
	{12, 3}, {13, 3}, {14, 3}, {15, 6}, {16, 3}, {13, 7},
	{16, 7}, {17, 3}, {18, 3}, {19, 6}, {20, 7}, {21, 3},
	{22, 3}, {23, 3}, {23, 7}, {20, 2}, {24, 3}, {25, 6},
	{26, 8}, {33, 9}, {27, 11}, {6, 12}, {28, 13}, {29, 7},
	{25, 11}, {33, 14}, {30, 15}, {31, 2}, {32, 15}, {34, 7},
	{35, 16}, {35, 17}, {0, 0}, {7, 12}, {1, 9}, {36, 12},
}

var m2SamplerCounts = [37]int{1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 3, 2, 2, 2, 3, 2, 2, 2, 2, 2, 3, 3, 3, 4, 2, 3, 2, 3, 1, 2, 3, 2}

func decodeM2Shader(id uint16, count int) (m2Shader, error) {
	vertex, pixel := 0, 0
	if id&0x8000 != 0 {
		index := int(id & 0x7fff)
		if index >= len(m2Effects) {
			return m2Shader{}, fmt.Errorf("invalid M2 effect %#x", id)
		}
		pixel, vertex = m2Effects[index][0], m2Effects[index][1]
	} else {
		if count == 1 {
			if id&0x70 != 0 {
				pixel = 1
			}
			if id&0x80 != 0 {
				vertex = 1
			} else if id&0x4000 != 0 {
				vertex = 10
			}
		} else {
			if id&0x70 != 0 {
				pixel = [8]int{11, 6, 6, 8, 7, 6, 9, 10}[id&7]
			} else {
				pixel = [8]int{5, 2, 2, 13, 3, 2, 4, 13}[id&7]
			}
			if id&0x80 != 0 {
				vertex = 4
				if id&8 != 0 {
					vertex = 5
				}
			} else if id&8 != 0 {
				vertex = 3
			} else if id&0x4000 != 0 {
				vertex = 2
			} else {
				vertex = 7
			}
		}
	}
	return m2Shader{pixel: pixel, coords: m2VertexCoords[vertex], edge: vertex == 9 || vertex == 12 || vertex == 14}, nil
}

type m2Fragment struct {
	diffuse, emission [3]float64
	alpha             float64
}

// Evaluate before lighting/framebuffer blending. Keep the emissive term apart:
// adding it to a lit diffuse texture would darken Eranog's hands and chest.
// Weights are texture-unit weights, not texture alpha. Guild tint constants are
// neutral here; character texture compositing supplies the chosen tabard art.
func evaluateM2Combiner(pixel int, t [4][4]float64, weight [4]float64) m2Fragment {
	a, b, c, d := t[0], t[1], t[2], t[3]
	f := m2Fragment{alpha: 1}
	for k := range 3 {
		switch pixel {
		case 0, 1, 33:
			f.diffuse[k] = a[k]
		case 2, 5, 6, 11, 36:
			f.diffuse[k] = a[k] * b[k]
		case 3, 4, 7, 9:
			f.diffuse[k] = a[k] * b[k] * 2
		case 8, 10:
			f.diffuse[k] = a[k]
			f.emission[k] = b[k]
		case 12:
			f.diffuse[k] = a[k] * (b[k]*2*(1-a[3]) + a[3])
		case 13, 16:
			f.diffuse[k] = a[k]
			f.emission[k] = b[k] * b[3]
		case 14, 17:
			f.diffuse[k] = a[k]
			f.emission[k] = b[k] * b[3] * (1 - a[3])
		case 15:
			f.diffuse[k] = a[k] * (b[k]*2*(1-a[3]) + a[3])
			f.emission[k] = c[k] * c[3] * weight[2]
		case 18:
			f.diffuse[k] = (a[k]*(1-b[3])+b[k]*b[3])*(1-a[3]) + a[k]*a[3]
		case 19:
			f.diffuse[k] = a[k]*b[k]*2*(1-c[3]) + c[k]*c[3]
		case 20, 23:
			f.diffuse[k] = a[k]
			f.emission[k] = b[k] * b[3] * weight[1]
		case 21:
			f.diffuse[k] = a[k]
			f.emission[k] = b[k] * (1 - a[3])
		case 22:
			f.diffuse[k] = a[k] * (b[k]*(1-a[3]) + a[3])
		case 24:
			f.diffuse[k] = a[k]*(1-b[3]) + b[k]*b[3]
			f.emission[k] = a[k] * a[3] * weight[0]
		case 25:
			glow := clampM2(c[3] * weight[2])
			f.diffuse[k] = a[k] * (b[k]*2*(1-a[3]) + a[3]) * (1 - glow)
			f.emission[k] = c[k] * glow
		case 26, 28:
			f.diffuse[k] = (a[k]*(1-clampM2(weight[1]))+b[k]*clampM2(weight[1]))*(1-clampM2(weight[2])) + c[k]*clampM2(weight[2])
		case 27:
			f.diffuse[k] = (a[k]*b[k]*2*(1-c[3])+c[k]*c[3])*(1-a[3]) + a[k]*a[3]
		case 29:
			f.diffuse[k] = a[k]*(1-b[3]) + b[k]*b[3]
		case 30, 32:
			f.diffuse[k] = a[k]*(1-b[3]) + a[k]*b[k]*b[3]
			f.diffuse[k] = f.diffuse[k]*(1-c[3]) + c[k]*c[3]
		case 31:
			f.diffuse[k] = a[k]*(1-b[3]) + a[k]*b[k]*b[3]
		case 34:
			f.diffuse[k] = a[k]
			f.emission[k] = a[k] * b[k] * weight[1]
		case 35:
			f.diffuse[k] = a[k] * b[k] * c[k]
		}
	}
	switch pixel {
	case 1, 9, 10, 11, 16, 23, 33, 34:
		f.alpha = a[3]
	case 2:
		f.alpha = b[3]
	case 3:
		f.alpha = b[3] * 2
	case 6, 36:
		f.alpha = a[3] * b[3]
	case 7:
		f.alpha = a[3] * b[3] * 2
	case 8, 21:
		f.alpha = a[3] + b[3]
	case 17:
		f.alpha = a[3] + b[3]*(.3*b[0]+.59*b[1]+.11*b[2])
	case 26, 28:
		f.alpha = (a[3]*(1-clampM2(weight[1]))+b[3]*clampM2(weight[1]))*(1-clampM2(weight[2])) + c[3]*clampM2(weight[2])
		if pixel == 28 {
			f.alpha *= d[3]
		}
	case 30, 31:
		f.alpha = a[3]
	case 35:
		f.alpha = a[3] * b[3] * c[3]
	}
	return f
}

func clampM2(v float64) float64 { return min(1, max(0, v)) }

// The renderer receives view-space positions/normals. This function is exact;
// the exporter uses a declared reference view when native WC3 sphere mapping
// cannot express a nonlinear multi-texture combiner.
func m2SphereCoord(position, normal imath.Vector3) imath.Vector2 {
	normalize := func(v imath.Vector3) imath.Vector3 {
		l := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
		if l > 1e-12 {
			for k := range 3 {
				v[k] /= l
			}
		}
		return v
	}
	v, n := normalize(position), normalize(normal)
	dot := v[0]*n[0] + v[1]*n[1] + v[2]*n[2]
	r := normalize(imath.Vector3{v[0] - 2*dot*n[0], v[1] - 2*dot*n[1], v[2] - 2*dot*n[2] + 1})
	return imath.Vector2{.5 - .5*r[0], .5 - .5*r[1]}
}
