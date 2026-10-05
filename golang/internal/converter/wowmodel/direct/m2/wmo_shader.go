package directm2

import (
	"context"
	"fmt"
	"image"
	"log"
	"math"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

// WMO material index -> pixel combiner, vertex routing, sampler count.
// Numeric IDs follow WebWowViewerCpp's commonWMOMaterial.glsl.
var wmoEffects = [24][3]int{
	{0, 0, 1}, {1, 3, 1}, {2, 3, 1}, {3, 1, 2}, {4, 0, 1}, {5, 1, 2},
	{6, 4, 2}, {7, 0, 3}, {8, 6, 2}, {9, 4, 2}, {-1, -1, 2}, {10, 2, 3},
	{11, 2, 3}, {12, 4, 2}, {-1, -1, 2}, {13, 4, 2}, {0, 0, 1}, {14, 2, 3},
	{15, 7, 3}, {16, 4, 2}, {17, 7, 3}, {18, 0, 1}, {19, 8, 6}, {20, 0, 9},
}

func WMOShaderSamplerCount(shader uint32) (int, error) {
	if shader >= uint32(len(wmoEffects)) {
		return 0, fmt.Errorf("unknown WMO shader %d", shader)
	}
	return wmoEffects[shader][2], nil
}

type wmoVarying struct {
	uv                 [4]imath.Vector2
	color              [2][4]float64 // Raw normalized BGRA vertex channels.
	position, normal   imath.Vector3
	tangent, bitangent imath.Vector3
}

func wmoLerp(a, b, t float64) float64 { return a*(1-t) + b*t }

func evaluateWMOCombiner(pixel int, t [9][4]float64, color, second [4]float64) m2Fragment {
	a, b, c := t[0], t[1], t[2]
	f := m2Fragment{alpha: a[3]}
	for k := range 3 {
		switch pixel {
		case -1:
			f.diffuse[k] = a[k] * b[k]
		case 0, 1, 2, 18:
			f.diffuse[k] = a[k]
		case 3:
			f.diffuse[k] = a[k]
			f.emission[k] = b[k] * a[3]
			f.alpha = 1
		case 4:
			f.diffuse[k] = a[k]
			f.alpha = 1
		case 5:
			f.diffuse[k] = a[k]
			f.emission[k] = a[k] * a[3] * b[k]
			f.alpha = 1
		case 6:
			f.diffuse[k] = wmoLerp(wmoLerp(a[k], b[k], b[3]), a[k], color[3])
		case 7:
			mixed := wmoLerp(b[k], a[k], color[3])
			alpha := wmoLerp(b[3], a[3], color[3])
			f.diffuse[k] = mixed
			f.emission[k] = mixed * alpha * c[k]
		case 8, 12:
			f.diffuse[k] = wmoLerp(b[k], a[k], color[3])
			if pixel == 12 {
				f.alpha = 1
			}
		case 9:
			f.diffuse[k] = a[k]
			f.emission[k] = b[k] * b[3] * color[3]
		case 10:
			f.diffuse[k] = wmoLerp(wmoLerp(2*a[k]*b[k], c[k], clampM2(c[3]*color[3])), a[k], a[3])
		case 11:
			f.diffuse[k] = a[k]
			f.emission[k] = a[k]*a[3]*b[k] + c[k]*c[3]*color[3]
		case 13:
			f.diffuse[k] = wmoLerp(b[k]*(1-b[3]), a[k], color[3])
			f.emission[k] = b[k] * b[3] * (1 - color[3])
		case 14:
			f.diffuse[k] = wmoLerp(2*a[k]*b[k]+c[k]*clampM2(c[3]*color[3]), a[k], a[3])
			f.alpha = 1
		case 15:
			f.diffuse[k] = wmoLerp(wmoLerp(a[k], b[k], b[3]), a[k], color[3]) * c[k] * 2
		case 16:
			f.diffuse[k] = wmoLerp(a[k], a[k]*b[k]*2, color[3])
		case 17:
			f.diffuse[k] = wmoLerp(wmoLerp(a[k], b[k], b[3]), a[k], c[3]) * c[k] * 2
		case 19:
			back, mid, mask := t[4], t[3], t[5]
			layer := back[k] + mid[k]*mid[3]
			diffuse := wmoLerp(layer, c[k], mask[1])
			f.diffuse[k] = wmoLerp(diffuse, a[k], color[3])
			reflected := wmoLerp(b[k]*b[3], a[k]*a[3], color[3])
			f.emission[k] = (c[k]*mask[2]+back[k]*back[3]*(1-c[2]))*(1-color[3]) + reflected*b[k]
			f.alpha = 1
		case 20:
			weights := [4]float64{second[2], second[1], second[0], 1 - clampM2(second[0]+second[1]+second[2])}
			largest := 0.0
			for i := range 4 {
				weights[i] *= max(t[5+i][3], .004)
				largest = max(largest, weights[i])
			}
			sum := 0.0
			for i := range 4 {
				weights[i] *= 1 - clampM2(largest-weights[i])
				sum += weights[i]
			}
			if sum < 1e-12 {
				weights = [4]float64{0, 0, 0, 1}
				sum = 1
			}
			mixed, alpha := 0.0, 0.0
			for i := range 4 {
				mixed += t[1+i][k] * weights[i] / sum
				alpha += t[1+i][3] * weights[i] / sum
			}
			f.diffuse[k] = mixed * (1 - second[3])
			f.emission[k] = alpha * a[k] * mixed
			f.alpha = 1
		}
	}
	return f
}

func wmoUVScroll(t float64, speed float32) float64 {
	if speed == 0 {
		return 0
	}
	value := t * float64(speed) / 1000000
	return value - math.Floor(value)
}

func wmoVaryingAt(group *wmo.Loader, batch wmo.RenderBatch, face int, bary [3]float64) wmoVarying {
	v := wmoVarying{}
	if len(group.VertexColours) == 0 {
		v.color[0] = [4]float64{0, 0, 0, 1}
	}
	for corner := range 3 {
		index := int(group.Indices[int(batch.FirstFace)+face*3+corner])
		weight := bary[corner]
		for axis := range 3 {
			if index*3+axis < len(group.Vertices) {
				v.position[axis] += weight * float64(group.Vertices[index*3+axis])
			}
			if index*3+axis < len(group.Normals) {
				v.normal[axis] += weight * float64(group.Normals[index*3+axis])
			}
		}
		for layer := range 4 {
			if layer < len(group.UVs) && index*2+1 < len(group.UVs[layer]) {
				for axis := range 2 {
					v.uv[layer][axis] += weight * float64(group.UVs[layer][index*2+axis])
				}
			} else if layer > 0 {
				for axis := range 2 {
					if len(group.UVs) > 0 && index*2+axis < len(group.UVs[0]) {
						v.uv[layer][axis] += weight * float64(group.UVs[0][index*2+axis])
					}
				}
			}
		}
		for layer := range 2 {
			if layer < len(group.VertexColours) && index < len(group.VertexColours[layer]) {
				word := group.VertexColours[layer][index]
				for channel := range 4 {
					v.color[layer][channel] += weight * float64((word>>uint(channel*8))&255) / 255
				}
			}
		}
	}
	var positions [3]imath.Vector3
	var coords [3]imath.Vector2
	for corner := range 3 {
		index := int(group.Indices[int(batch.FirstFace)+face*3+corner])
		for axis := range 3 {
			positions[corner][axis] = float64(group.Vertices[index*3+axis])
		}
		if len(group.UVs) > 1 && index*2+1 < len(group.UVs[1]) {
			for axis := range 2 {
				coords[corner][axis] = float64(group.UVs[1][index*2+axis])
			}
		}
	}
	du1, dv1 := coords[1][0]-coords[0][0], coords[1][1]-coords[0][1]
	du2, dv2 := coords[2][0]-coords[0][0], coords[2][1]-coords[0][1]
	det := du1*dv2 - du2*dv1
	if math.Abs(det) > 1e-12 {
		for axis := range 3 {
			e1, e2 := positions[1][axis]-positions[0][axis], positions[2][axis]-positions[0][axis]
			v.tangent[axis] = (e1*dv2 - e2*dv1) / det
			v.bitangent[axis] = (e2*du1 - e1*du2) / det
		}
	}
	return v
}

func sampleWMOProgram(effect [3]int, v wmoVarying, images [9]*image.NRGBA, speed [4]float32, t float64) m2Fragment {
	uv := v.uv
	for axis := range 2 {
		uv[0][axis] += wmoUVScroll(t, speed[axis])
		uv[1][axis] += wmoUVScroll(t, speed[axis+2])
		uv[2][axis] += wmoUVScroll(t, speed[axis])
	}
	normal := imath.Vector3{v.normal[1], v.normal[2], v.normal[0]}
	env := m2SphereCoord(imath.Vector3{0, 0, -1}, normal)
	length := math.Sqrt(normal[0]*normal[0] + normal[1]*normal[1] + normal[2]*normal[2])
	reflection := imath.Vector2{}
	if length > 1e-12 {
		reflection = imath.Vector2{2 * normal[2] * normal[0] / (length * length), 2 * normal[2] * normal[1] / (length * length)}
	}
	switch effect[1] {
	case 1:
		uv[1] = reflection
	case 2:
		uv[2] = uv[1]
		uv[1] = env
	case 5:
		uv[2] = reflection
	case 6:
		uv[1] = imath.Vector2{v.position[1] * .24, v.position[2] * .24}
	case 8:
		uv[0] = v.uv[0]
	}
	samples := [9][4]float64{}
	coords := [9]imath.Vector2{uv[0], uv[1], uv[2], uv[1], uv[2], uv[0], uv[1], uv[2], uv[3]}
	if effect[0] == 20 {
		coords = [9]imath.Vector2{env, uv[0], uv[1], uv[2], uv[3], uv[0], uv[1], uv[2], uv[3]}
	}
	for i := range effect[2] {
		if images[i] != nil {
			samples[i] = sampleBakeTexture(images[i], coords[i], 3)
		} else {
			samples[i] = [4]float64{1, 1, 1, 1}
		}
	}
	if effect[0] == 19 {
		samples[2] = sampleBakeTexture(images[2], uv[1], 3)
		samples[5] = sampleBakeTexture(images[5], uv[1], 3)
		samples[1] = sampleBakeTexture(images[2], imath.Vector2{v.color[1][2], v.color[1][1]}, 3)
		normalize := func(p imath.Vector3) imath.Vector3 {
			l := math.Sqrt(p[0]*p[0] + p[1]*p[1] + p[2]*p[2])
			if l > 1e-12 {
				for k := range 3 {
					p[k] /= l
				}
			}
			return p
		}
		tangent, bitangent, n := normalize(v.tangent), normalize(v.bitangent), normalize(v.normal)
		denominator := n[0]
		if math.Abs(denominator) < .01 {
			denominator = math.Copysign(.01, denominator)
		}
		offset := imath.Vector2{tangent[0] / denominator * samples[5][0] * .25, bitangent[0] / denominator * samples[5][0] * .25}
		samples[3] = sampleBakeTexture(images[3], imath.Vector2{uv[1][0] - offset[0], uv[1][1] - offset[1]}, 3)
		samples[4] = sampleBakeTexture(images[4], imath.Vector2{uv[2][0] - offset[0], uv[2][1] - offset[1]}, 3)
	}
	return evaluateWMOCombiner(effect[0], samples, v.color[0], v.color[1])
}

// BakeWMOMaterial accepts source group data so none of its four UV sets or
// two vertex-colour sets are discarded by the intermediate WC3 geometry.
func BakeWMOMaterial(ctx context.Context, cfg config.Config, result *ConvertResult, g *components.Geoset, group *wmo.Loader, batch wmo.RenderBatch, material wmo.Material, images [9]*image.NRGBA, speed [4]float32) error {
	if g.Material == nil || len(g.Material.Layers) == 0 {
		return fmt.Errorf("WMO batch has no WC3 material")
	}
	if uint64(batch.FirstFace)+uint64(batch.NumFaces) > uint64(len(group.Indices)) {
		return fmt.Errorf("WMO batch indices exceed group geometry")
	}
	for _, index := range group.Indices[int(batch.FirstFace) : int(batch.FirstFace)+int(batch.NumFaces)] {
		if int(index)*3+2 >= len(group.Vertices) {
			return fmt.Errorf("WMO batch vertex %d exceeds group geometry", index)
		}
	}
	if material.Shader >= uint32(len(wmoEffects)) {
		return fmt.Errorf("unknown WMO shader %d", material.Shader)
	}
	effect := wmoEffects[material.Shader]
	for i := range 9 {
		if images[i] == nil {
			images[i] = image.NewNRGBA(image.Rect(0, 0, 1, 1))
			if !(effect[0] == 20 && i == 0) {
				for c := range 4 {
					images[i].Pix[c] = 255
				}
			}
		}
	}
	layer := g.Material.Layers[0]
	layer.Unshaded = material.Flags&1 != 0
	layer.Unfogged = material.Flags&2 != 0
	layer.TwoSided = material.Flags&4 != 0
	layer.NoDepthSet = material.BlendMode > 1
	rawBlend := uint16(0)
	switch material.BlendMode {
	case 0, 1, 2:
		rawBlend = uint16(material.BlendMode)
	case 3, 7:
		rawBlend = 4
	case 4:
		rawBlend = 5
	case 5, 6:
		rawBlend = 6
	case 8, 9:
		rawBlend = 0
	case 10:
		rawBlend = 3
	case 11:
		rawBlend = 2
	case 12:
		rawBlend = 5
	case 13:
		rawBlend = 7
	default:
		return fmt.Errorf("unknown WMO framebuffer blend %d", material.BlendMode)
	}
	layer.FilterMode = m2BakeBlend(rawBlend)
	g.Material = &components.Material{TwoSided: layer.TwoSided, Layers: []components.Layer{layer}}
	hasEmission := false
	switch effect[0] {
	case 3, 5, 7, 9, 11, 13, 19, 20:
		hasEmission = material.BlendMode != 6 && material.BlendMode != 12
	}
	program := bakeProgram{shader: m2Shader{pixel: 0}, count: 1, blend: rawBlend, images: [4]*image.NRGBA{images[0]}, hasEmission: hasEmission, screen: material.BlendMode == 12, pixelBudget: bakeMaterialPixels / max(1, len(result.MDL.Geosets))}
	for _, img := range images {
		if img.Bounds().Dx()*img.Bounds().Dy() > program.images[0].Bounds().Dx()*program.images[0].Bounds().Dy() {
			program.images[0] = img
		}
	}
	if program.images[0] == nil {
		program.images[0] = image.NewNRGBA(image.Rect(0, 0, 64, 64))
	}
	for k := range 4 {
		program.weights[k] = components.AnimatedOrStatic[float64]{Static: true, Value: 1}
	}
	if effect[1] == 1 || effect[1] == 2 || effect[1] == 5 || effect[0] == 19 || effect[0] == 20 {
		log.Printf("WMO shader %d: camera-dependent reflection/parallax uses a front reference view", material.Shader)
	}
	for matrix := range 2 {
		sx, sy := speed[matrix*2], speed[matrix*2+1]
		if sx == 0 && sy == 0 {
			continue
		}
		longest := 0.0
		for _, s := range []float32{sx, sy} {
			if s != 0 {
				longest = max(longest, 1000000/math.Abs(float64(s)))
			}
		}
		duration := max(1, int(math.Ceil(min(longest, 60000))))
		seq := components.NewGlobalSequence(len(result.MDL.GlobalSequences), duration)
		result.MDL.GlobalSequences = append(result.MDL.GlobalSequences, &seq)
		program.transforms[matrix] = &components.TextureAnim{Translation: &components.Animation{GlobalSeq: &seq, Type: components.AnimTypeTVertexAnim, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, duration: imath.Vector3{float64(sx) * float64(duration) / 1000000, float64(sy) * float64(duration) / 1000000, 0}}}}
	}
	program.shade = func(p bakePixel, moment bakeMoment) m2Fragment {
		v := wmoVaryingAt(group, batch, p.face, p.bary)
		fragment := sampleWMOProgram(effect, v, images, speed, moment.time)
		switch material.BlendMode {
		case 6:
			for k := range 3 {
				fragment.diffuse[k] += fragment.emission[k] + 1
				fragment.emission[k] = 0
			}
			fragment.alpha = 1
		case 7:
			fragment.alpha = 1 - fragment.alpha
		case 8, 9:
			factor := fragment.alpha
			if material.BlendMode == 8 {
				factor = 1 - factor
			}
			for k := range 3 {
				fragment.diffuse[k] *= factor
				fragment.emission[k] *= factor
			}
			fragment.alpha = 1
		case 11:
			fragment.alpha = 1 // Exported models have no scene constant-alpha state.
		}
		return fragment
	}
	program.conflict = func(a, b bakePixel) bool {
		x, y := wmoVaryingAt(group, batch, a.face, a.bary), wmoVaryingAt(group, batch, b.face, b.bary)
		for i := range 4 {
			for axis := range 2 {
				if math.Abs(x.uv[i][axis]-y.uv[i][axis]) > 1e-4 {
					return true
				}
			}
		}
		for i := range 2 {
			for channel := range 4 {
				if math.Abs(x.color[i][channel]-y.color[i][channel]) > 1e-4 {
					return true
				}
			}
		}
		if effect[1] == 1 || effect[1] == 2 || effect[1] == 5 || effect[0] == 19 || effect[0] == 20 {
			for axis := range 3 {
				if math.Abs(x.normal[axis]-y.normal[axis]) > 1e-4 {
					return true
				}
			}
		}
		return false
	}
	if err := bakeM2Geoset(ctx, cfg, result, g, program); err != nil {
		return err
	}
	for i, extra := range g.Material.Layers[1:] {
		copyG := cloneBakeGeoset(g)
		copyG.Name += fmt.Sprintf("_ShaderPass%d", i+1)
		copyG.Material = &components.Material{Layers: []components.Layer{extra}}
		result.MDL.Geosets = append(result.MDL.Geosets, copyG)
		result.MDL.Materials = append(result.MDL.Materials, copyG.Material)
	}
	g.Material.Layers = g.Material.Layers[:1]
	return nil
}
