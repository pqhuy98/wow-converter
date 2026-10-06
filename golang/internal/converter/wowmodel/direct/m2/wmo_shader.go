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

// WMOAdditionalGeosetCount reports the geosets BakeWMOMaterial appends after
// baking the batch's original geoset. It lets the WMO orchestrator preserve
// the serial per-material pixel budget when independent batches are baked in
// parallel.
func WMOAdditionalGeosetCount(cfg config.Config, shader, blendMode uint32, hasEnvironment bool) (int, error) {
	if shader >= uint32(len(wmoEffects)) {
		return 0, fmt.Errorf("unknown WMO shader %d", shader)
	}
	rawBlend, err := wmoRawBlendMode(blendMode)
	if err != nil {
		return 0, err
	}
	effect := wmoEffects[shader]
	hasEmission := wmoShaderHasEmission(effect[0], blendMode, hasEnvironment)
	staticEmissionGeoset := !cfg.TextureBaking.Flipbook() && hasEmission && rawBlend == 0
	extra := 0
	if staticEmissionGeoset {
		extra++
	}
	// BakeWMOMaterial wraps pixel combiner zero. Its generated atlas adds an
	// emission layer only when its source shader reports emission, and adds a
	// radiance layer for additive or screen blend modes. The wrapper's post-bake
	// pass loop turns each such layer into an additional geoset.
	if hasEmission && !staticEmissionGeoset {
		extra++
	}
	if rawBlend == 7 || blendMode == 12 {
		extra++
	}
	return extra, nil
}

func wmoRawBlendMode(blendMode uint32) (uint16, error) {
	switch blendMode {
	case 0, 1, 2:
		return uint16(blendMode), nil
	case 3, 7:
		return 4, nil
	case 4:
		return 5, nil
	case 5, 6:
		return 6, nil
	case 8, 9:
		return 0, nil
	case 10:
		return 3, nil
	case 11:
		return 2, nil
	case 12:
		return 5, nil
	case 13:
		return 7, nil
	default:
		return 0, fmt.Errorf("unknown WMO framebuffer blend %d", blendMode)
	}
}

func wmoShaderHasEmission(pixel int, blendMode uint32, hasEnvironment bool) bool {
	hasEmission := false
	switch pixel {
	case 3, 5, 7, 9, 11, 13, 19, 20:
		hasEmission = blendMode != 6 && blendMode != 12
	}
	return hasEmission && (pixel != 20 || hasEnvironment)
}

type wmoVarying struct {
	uv                 [4]imath.Vector2
	color              [2][4]float64 // BGRA: second MOCV, then MOC2 (never lighting MOCV).
	position, normal   imath.Vector3
	tangent, bitangent imath.Vector3
}

type wmoPreparedVertex struct {
	position, normal               imath.Vector3
	positionPresent, normalPresent [3]bool
	uv                             [4]imath.Vector2
	uvPresent                      [4]bool
	color                          [2]uint32
	colorPresent                   [2]bool
}

type wmoPreparedFace struct {
	indices            [3]uint16
	tangent, bitangent imath.Vector3
}

type wmoPreparedVaryings struct {
	group       *wmo.Loader
	batch       wmo.RenderBatch
	vertices    []wmoPreparedVertex
	vertexReady []bool
	faces       []wmoPreparedFace
	faceReady   []bool
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
	// Match the renderer's aColor2/aColorSecond bindings and missing-stream defaults.
	streams := [2][]uint32{nil, group.BlendColours}
	if len(group.VertexColours) > 1 {
		streams[0] = group.VertexColours[1]
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
				// The WMO loader flips V for OBJ; sampling raw BLP rows needs WoW UVs.
				v.uv[layer][0] += weight * float64(group.UVs[layer][index*2])
				v.uv[layer][1] += weight * (1 - float64(group.UVs[layer][index*2+1]))
			} else {
				v.uv[layer][0] += weight
				v.uv[layer][1] += weight
			}
		}
		for layer := range 2 {
			if index < len(streams[layer]) {
				word := streams[layer][index]
				for channel := range 4 {
					v.color[layer][channel] += weight * float64((word>>uint(channel*8))&255) / 255
				}
			} else {
				v.color[layer][3] += weight
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
			coords[corner] = imath.Vector2{float64(group.UVs[1][index*2]), 1 - float64(group.UVs[1][index*2+1])}
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

func prepareWMOVaryings(group *wmo.Loader, batch wmo.RenderBatch, faceCount int) func(int, [3]float64) wmoVarying {
	prepared := &wmoPreparedVaryings{
		group:       group,
		batch:       batch,
		vertices:    make([]wmoPreparedVertex, (len(group.Vertices)+2)/3),
		vertexReady: make([]bool, (len(group.Vertices)+2)/3),
		faces:       make([]wmoPreparedFace, faceCount),
		faceReady:   make([]bool, faceCount),
	}
	return prepared.at
}

func (p *wmoPreparedVaryings) vertex(index int) *wmoPreparedVertex {
	if !p.vertexReady[index] {
		v := &p.vertices[index]
		group := p.group
		for axis := range 3 {
			if index*3+axis < len(group.Vertices) {
				v.position[axis] = float64(group.Vertices[index*3+axis])
				v.positionPresent[axis] = true
			}
			if index*3+axis < len(group.Normals) {
				v.normal[axis] = float64(group.Normals[index*3+axis])
				v.normalPresent[axis] = true
			}
		}
		for layer := range 4 {
			v.uv[layer] = imath.Vector2{1, 1}
			if layer < len(group.UVs) && index*2+1 < len(group.UVs[layer]) {
				v.uv[layer] = imath.Vector2{float64(group.UVs[layer][index*2]), 1 - float64(group.UVs[layer][index*2+1])}
				v.uvPresent[layer] = true
			}
		}
		streams := [2][]uint32{nil, group.BlendColours}
		if len(group.VertexColours) > 1 {
			streams[0] = group.VertexColours[1]
		}
		for layer := range 2 {
			if index < len(streams[layer]) {
				v.color[layer] = streams[layer][index]
				v.colorPresent[layer] = true
			}
		}
		p.vertexReady[index] = true
	}
	return &p.vertices[index]
}

func (p *wmoPreparedVaryings) face(face int) *wmoPreparedFace {
	if !p.faceReady[face] {
		indices := [3]uint16{}
		var positions [3]imath.Vector3
		var coords [3]imath.Vector2
		for corner := range 3 {
			index := int(p.group.Indices[int(p.batch.FirstFace)+face*3+corner])
			indices[corner] = uint16(index)
			v := p.vertex(index)
			positions[corner] = v.position
			if v.uvPresent[1] {
				coords[corner] = v.uv[1]
			}
		}
		prepared := &p.faces[face]
		prepared.indices = indices
		du1, dv1 := coords[1][0]-coords[0][0], coords[1][1]-coords[0][1]
		du2, dv2 := coords[2][0]-coords[0][0], coords[2][1]-coords[0][1]
		det := du1*dv2 - du2*dv1
		if math.Abs(det) > 1e-12 {
			for axis := range 3 {
				e1, e2 := positions[1][axis]-positions[0][axis], positions[2][axis]-positions[0][axis]
				prepared.tangent[axis] = (e1*dv2 - e2*dv1) / det
				prepared.bitangent[axis] = (e2*du1 - e1*du2) / det
			}
		}
		p.faceReady[face] = true
	}
	return &p.faces[face]
}

func (p *wmoPreparedVaryings) at(face int, bary [3]float64) wmoVarying {
	prepared := p.face(face)
	v := wmoVarying{tangent: prepared.tangent, bitangent: prepared.bitangent}
	for corner, rawIndex := range prepared.indices {
		source := p.vertex(int(rawIndex))
		weight := bary[corner]
		for axis := range 3 {
			if source.positionPresent[axis] {
				v.position[axis] += weight * source.position[axis]
			}
			if source.normalPresent[axis] {
				v.normal[axis] += weight * source.normal[axis]
			}
		}
		for layer := range 4 {
			if source.uvPresent[layer] {
				v.uv[layer][0] += weight * source.uv[layer][0]
				v.uv[layer][1] += weight * source.uv[layer][1]
			} else {
				v.uv[layer][0] += weight
				v.uv[layer][1] += weight
			}
		}
		for layer := range 2 {
			if source.colorPresent[layer] {
				word := source.color[layer]
				for channel := range 4 {
					v.color[layer][channel] += weight * float64((word>>uint(channel*8))&255) / 255
				}
			} else {
				v.color[layer][3] += weight
			}
		}
	}
	return v
}

func wmoGenericSamplerUsed(effect [3]int, sampler int) bool {
	if sampler < 0 || sampler >= effect[2] {
		return false
	}
	// Shader 19 replaces every generic sample after slot zero with explicit
	// samples at its normal/reflection/parallax coordinates below.
	return effect[0] != 19 || sampler == 0
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
		if !wmoGenericSamplerUsed(effect, i) {
			continue
		}
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
	beforeGeosets := len(result.MDL.Geosets)
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
	hasEnvironment := images[0] != nil
	additionalGeosets, err := WMOAdditionalGeosetCount(cfg, material.Shader, material.BlendMode, hasEnvironment)
	if err != nil {
		return err
	}
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
	rawBlend, err := wmoRawBlendMode(material.BlendMode)
	if err != nil {
		return err
	}
	layer.FilterMode = m2BakeBlend(rawBlend)
	g.Material = &components.Material{TwoSided: layer.TwoSided, Layers: []components.Layer{layer}}
	hasEmission := wmoShaderHasEmission(effect[0], material.BlendMode, hasEnvironment)
	program := bakeProgram{shader: m2Shader{pixel: 0}, count: 1, blend: rawBlend, images: [4]*image.NRGBA{images[0]}, hasEmission: hasEmission, screen: material.BlendMode == 12, pixelBudget: bakeMaterialPixels / max(1, len(result.MDL.Geosets))}
	if !cfg.TextureBaking.Flipbook() {
		// Static architecture needs surface detail independent of batch count.
		// Keep the shared sparse working-memory cap and a larger surface budget,
		// capped at one megapixel per material rather than an effect flipbook.
		program.pixelBudget = min(1024*1024, bakeMaterialPixels*4/max(1, len(result.MDL.Geosets)))
	}
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
	varyingAt := prepareWMOVaryings(group, batch, len(g.Faces))
	for matrix := range 2 {
		if !cfg.TextureBaking.Flipbook() {
			break
		}
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
		v := varyingAt(p.face, p.bary)
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
		if !cfg.TextureBaking.Flipbook() {
			// Only the painted result matters for a still frame. Masked-out UVs
			// and normals used by zero emission must not duplicate identical texels.
			x, y := program.shade(a, bakeMoment{}), program.shade(b, bakeMoment{})
			if rawBlend == 0 {
				return !sameWMOStillRGB(x.diffuse, y.diffuse) || !sameWMOStillRGB(x.emission, y.emission)
			}
			if math.Abs(x.alpha-y.alpha) > 1e-9 {
				return true
			}
			for channel := range 3 {
				if math.Abs(x.diffuse[channel]-y.diffuse[channel]) > 1e-9 || math.Abs(x.emission[channel]-y.emission[channel]) > 1e-9 {
					return true
				}
			}
			return false
		}
		x, y := varyingAt(a.face, a.bary), varyingAt(b.face, b.bary)
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
	// Reflections can differ on surfaces with identical diffuse UVs. Packing
	// both outputs in one chart duplicates the albedo for every normal and
	// spends the surface budget on gutters. Static passes can use independent
	// layouts, with most of the shared budget reserved for the diffuse detail.
	var emission *components.Geoset
	var emissionProgram bakeProgram
	staticEmissionGeoset := !cfg.TextureBaking.Flipbook() && program.hasEmission && rawBlend == 0
	if staticEmissionGeoset {
		emission = cloneBakeGeoset(g)
		emission.Name += "_ShaderEmission"
		emissiveLayer := layer
		emissiveLayer.FilterMode, emissiveLayer.Unshaded, emissiveLayer.NoDepthSet = components.BlendAddAlpha, true, true
		emission.Material = &components.Material{TwoSided: g.Material.TwoSided, Layers: []components.Layer{emissiveLayer}}
		shade := program.shade
		emissionProgram = program
		emissionProgram.hasEmission, emissionProgram.blend = false, 4
		emissionProgram.pixelBudget = max(1, program.pixelBudget/4)
		emissionProgram.shade = func(p bakePixel, moment bakeMoment) m2Fragment {
			f := shade(p, moment)
			return m2Fragment{diffuse: f.emission, alpha: 1}
		}
		emissionProgram.conflict = func(a, b bakePixel) bool {
			x, y := emissionProgram.shade(a, bakeMoment{}), emissionProgram.shade(b, bakeMoment{})
			return !sameWMOStillRGB(x.diffuse, y.diffuse)
		}
		program.hasEmission = false
		program.pixelBudget -= emissionProgram.pixelBudget
		program.shade = func(p bakePixel, moment bakeMoment) m2Fragment {
			f := shade(p, moment)
			f.emission = [3]float64{}
			return f
		}
	}
	if !cfg.TextureBaking.Flipbook() && rawBlend == 0 {
		// Chart comparisons reuse each candidate's painted result. A sample may
		// collide with thousands of charts; shading it again for each comparison
		// changes no pixels and used most of the still-bake CPU time.
		program.prepare = func(p *bakePixel) {
			f := program.shade(*p, bakeMoment{})
			for k := range 3 {
				p.color[k] = uint8(math.Round(clampM2(f.diffuse[k]) * 255))
			}
		}
		program.conflict = func(a, b bakePixel) bool { return a.color != b.color }
		if emission != nil {
			emissionProgram.prepare = func(p *bakePixel) {
				f := emissionProgram.shade(*p, bakeMoment{})
				for k := range 3 {
					p.color[k] = uint8(math.Round(clampM2(f.diffuse[k]) * 255))
				}
			}
			emissionProgram.conflict = func(a, b bakePixel) bool { return a.color != b.color }
		} else if program.hasEmission {
			program.prepare = func(p *bakePixel) {
				f := program.shade(*p, bakeMoment{})
				for k := range 3 {
					p.color[k] = uint8(math.Round(clampM2(f.diffuse[k]) * 255))
					p.color[k+3] = uint8(math.Round(clampM2(f.emission[k]) * 255))
				}
			}
		}
	}
	switch effect[0] {
	case 6, 7, 8, 12, 13, 15, 17, 20:
		wmoBlendRaster(g, group, batch, effect[0], images)
		if emission != nil {
			wmoBlendRaster(emission, group, batch, effect[0], images)
		}
	}
	if err := bakeM2Geoset(ctx, cfg, result, g, program); err != nil {
		return err
	}
	if emission != nil {
		if err := bakeM2Geoset(ctx, cfg, result, emission, emissionProgram); err != nil {
			return err
		}
		result.MDL.Geosets = append(result.MDL.Geosets, emission)
	}
	for i, extra := range g.Material.Layers[1:] {
		copyG := cloneBakeGeoset(g)
		copyG.Name += fmt.Sprintf("_ShaderPass%d", i+1)
		copyG.Material = &components.Material{Layers: []components.Layer{extra}}
		result.MDL.Geosets = append(result.MDL.Geosets, copyG)
		result.MDL.Materials = append(result.MDL.Materials, copyG.Material)
	}
	g.Material.Layers = g.Material.Layers[:1]
	if got := len(result.MDL.Geosets) - beforeGeosets; got != additionalGeosets {
		return fmt.Errorf("WMO shader %d blend %d appended %d geosets, expected %d", material.Shader, material.BlendMode, got, additionalGeosets)
	}
	return nil
}

// Opaque still textures store clamped eight-bit RGB. Sub-byte differences in
// masked UVs or reflection normals must not duplicate identical painted texels.
func sameWMOStillRGB(a, b [3]float64) bool {
	for channel := range 3 {
		if math.Round(clampM2(a[channel])*255) != math.Round(clampM2(b[channel])*255) {
			return false
		}
	}
	return true
}

// Raster coordinates are independent of the shader's source UVs. A height-
// blended WMO can collapse UV1 on faces textured through UV2/3/4, or leave huge
// placeholder coordinates in an unused layer. Rasterize each face through its
// most detailed active layer; wmoVaryingAt still samples all original inputs.
func wmoBlendRaster(g *components.Geoset, group *wmo.Loader, batch wmo.RenderBatch, pixel int, images [9]*image.NRGBA) {
	varyingAt := prepareWMOVaryings(group, batch, len(g.Faces))
	type rasterVertex struct {
		vertex *components.GeosetVertex
		uv     imath.Vector2
	}
	vertices := map[rasterVertex]*components.GeosetVertex{}
	g.Vertices = nil
	for fi := range g.Faces {
		var source [3]wmoVarying
		for corner := range 3 {
			bary := [3]float64{}
			bary[corner] = 1
			source[corner] = varyingAt(fi, bary)
		}
		best, score := -1, 0.0
		primaryValid := false
		layers := 2
		if pixel == 7 || pixel == 15 || pixel == 17 {
			layers = 3
		}
		if pixel == 20 {
			layers = 4
		}
		for layer := range layers {
			activity := 0.0
			for _, v := range source {
				if pixel == 20 {
					if layer < 3 {
						activity += v.color[1][2-layer]
					} else {
						activity += 1 - clampM2(v.color[1][0]+v.color[1][1]+v.color[1][2])
					}
				} else {
					alpha := v.color[0][3]
					if pixel == 6 || pixel == 15 {
						alpha = 1 - sampleBakeTexture(images[1], v.uv[1], 3)[3]*(1-alpha)
					} else if pixel == 17 {
						alpha = 1 - sampleBakeTexture(images[1], v.uv[1], 3)[3]*(1-sampleBakeTexture(images[2], v.uv[2], 3)[3])
					}
					switch layer {
					case 0:
						activity += alpha
					case 1:
						activity += 1 - alpha
					case 2:
						activity += 1
					}
				}
			}
			a, b, c := source[0].uv[layer], source[1].uv[layer], source[2].uv[layer]
			area := math.Abs((b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0]))
			if layer == 0 {
				primaryValid = activity > 0 && area > 1e-12
			}
			sampler := layer
			if pixel == 20 {
				sampler++
			}
			detail := area * activity * float64(images[sampler].Bounds().Dx()*images[sampler].Bounds().Dy())
			if detail > score {
				best, score = layer, detail
			}
		}
		// Preserve a usable active primary unwrap and its shared edges. Changing
		// every triangle's domain fragments ordinary surfaces into tiny charts.
		if primaryValid {
			best = 0
		}
		var uv [3]imath.Vector2
		if best >= 0 {
			for corner := range 3 {
				uv[corner] = source[corner].uv[best]
			}
		} else {
			// All source UVs may be swatches while blend weights still vary.
			// Project on the largest-area geometric plane for this triangle.
			bestArea := 0.0
			for _, axes := range [][2]int{{0, 1}, {0, 2}, {1, 2}} {
				var p [3]imath.Vector2
				for corner, v := range g.Faces[fi].Vertices {
					p[corner] = imath.Vector2{v.Position[axes[0]], v.Position[axes[1]]}
				}
				area := math.Abs((p[1][0]-p[0][0])*(p[2][1]-p[0][1]) - (p[1][1]-p[0][1])*(p[2][0]-p[0][0]))
				if area > bestArea {
					uv, bestArea = p, area
				}
			}
			lo := imath.Vector2{min(uv[0][0], uv[1][0], uv[2][0]), min(uv[0][1], uv[1][1], uv[2][1])}
			scale := max(max(uv[0][0], uv[1][0], uv[2][0])-lo[0], max(uv[0][1], uv[1][1], uv[2][1])-lo[1], 1e-9)
			for corner := range 3 {
				uv[corner] = imath.Vector2{(uv[corner][0] - lo[0]) / scale, (uv[corner][1] - lo[1]) / scale}
			}
		}
		for corner, original := range g.Faces[fi].Vertices {
			key := rasterVertex{original, uv[corner]}
			v := vertices[key]
			if v == nil {
				copyV := *original
				copyV.TexPosition, copyV.TexPosition2 = uv[corner], nil
				v = &copyV
				vertices[key] = v
				g.Vertices = append(g.Vertices, v)
			}
			g.Faces[fi].Vertices[corner] = v
		}
	}
	// Source tile numbers and repeats have no role in the raster domain.
	// Bound each connected island independently: one extreme tiled triangle
	// must not stretch the common grid and reduce every other surface to swatches.
	// Shader samples still use the original source UVs, including all repeats.
	parents := map[*components.GeosetVertex]*components.GeosetVertex{}
	find := func(v *components.GeosetVertex) *components.GeosetVertex {
		for parents[v] != v {
			parents[v] = parents[parents[v]]
			v = parents[v]
		}
		return v
	}
	for _, v := range g.Vertices {
		parents[v] = v
	}
	for _, face := range g.Faces {
		root := find(face.Vertices[0])
		for _, v := range face.Vertices[1:] {
			parents[find(v)] = root
		}
	}
	minima := map[*components.GeosetVertex]imath.Vector2{}
	maxima := map[*components.GeosetVertex]imath.Vector2{}
	for _, v := range g.Vertices {
		root := find(v)
		lo, ok := minima[root]
		if !ok {
			lo = v.TexPosition
			maxima[root] = v.TexPosition
		}
		minima[root] = imath.Vector2{min(lo[0], v.TexPosition[0]), min(lo[1], v.TexPosition[1])}
		hi := maxima[root]
		maxima[root] = imath.Vector2{max(hi[0], v.TexPosition[0]), max(hi[1], v.TexPosition[1])}
	}
	for _, v := range g.Vertices {
		root := find(v)
		lo, hi := minima[root], maxima[root]
		scale := max(1, hi[0]-lo[0], hi[1]-lo[1])
		for axis := range 2 {
			v.TexPosition[axis] = (v.TexPosition[axis] - math.Floor(lo[axis])) / scale
		}
	}
}
