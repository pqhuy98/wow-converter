package directm2

import (
	"math"
	"sort"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

type nativeMaskDraw struct {
	geoset *components.Geoset
	mask   [4]float64
}

func nativeMaskCompatible(p bakeProgram) bool {
	if p.shader.pixel != 6 || p.count != 2 || (p.blend != 2 && p.blend != 4) || p.shader.coords[0] != coordT1M0 || p.shader.coords[1] != coordT2M1 {
		return false
	}
	if p.transforms[0] != nil {
		for _, track := range []*components.Animation{p.transforms[0].Translation, p.transforms[0].Rotation, p.transforms[0].Scaling} {
			if track != nil && !constantBakeTrack(track) {
				return false
			}
		}
	}
	return p.transforms[1] == nil || p.transforms[1].Rotation == nil && p.transforms[1].Scaling == nil
}

func sampleNativeMask(v *components.GeosetVertex, p bakeProgram, transform [6]float64) [4]float64 {
	mask := sampleBakeTexture(p.images[0], transformBakeUV(v.TexPosition, transform), p.flags[0])
	if p.alphaOnly {
		return [4]float64{0, 0, 0, mask[3]}
	}
	for k := range 3 {
		mask[k] *= mask[3]
	}
	mask[3] = 1
	return mask
}

// A static, smooth first sample can be represented by Classic's per-geoset
// colour/alpha. Keep the detailed second sample and its UV animation native.
// ponytail: bounded colour quantization and subdivision approximate the mask;
// complex masks fall back to the exact fragment baker, rather than losing art.
func nativeModulateMask(g *components.Geoset, p bakeProgram) []nativeMaskDraw {
	if !nativeMaskCompatible(p) {
		return nil
	}
	transform := bakeTransform(p.transforms[0], 0)
	maskAt := func(v *components.GeosetVertex) [4]float64 {
		return sampleNativeMask(v, p, transform)
	}
	groups := map[[4]uint8]int{}
	draws := []nativeMaskDraw{}
	colorLevels, colorError := 32.0, 1.0/16
	if p.opaqueMask || p.blend == 2 {
		colorLevels, colorError = 128, 1.0/255
	}
	alphaLevels := 16.0
	if p.alphaOnly {
		alphaLevels = 128
	}
	centerMask := func(face components.Face) [4]float64 {
		center := *face.Vertices[0]
		for axis := range 2 {
			center.TexPosition[axis] = (face.Vertices[0].TexPosition[axis] + face.Vertices[1].TexPosition[axis] + face.Vertices[2].TexPosition[axis]) / 3
		}
		return maskAt(&center)
	}
	faceLimit := 65536
	if p.alphaOnly {
		faceLimit *= 2 // Opacity uses at most 129 scalar groups.
	}
	faces := refineMaskFaces(g, maskAt, centerMask, func(face components.Face, want [4]float64) bool {
		return nativeMaskFootprintFits(face, p, transform, want)
	}, faceLimit)
	if faces == nil {
		return nil
	}
	for _, face := range faces {
		mask := centerMask(face)
		if mask[3] < 1.0/255 {
			continue
		}
		key := [4]uint8{}
		for k := range 3 {
			// Preserve faint radiance: uniform RGB buckets round weak stars and
			// dark scrolling detail to black. Square-root buckets devote more
			// precision to these small contributions.
			key[k] = uint8(math.Round(math.Sqrt(mask[k]) * colorLevels))
		}
		key[3] = uint8(math.Round(mask[3] * alphaLevels))
		if !p.opaqueMask && key[0] == 0 && key[1] == 0 && key[2] == 0 && (!p.alphaOnly || key[3] == 0) {
			continue
		}
		if !p.opaqueMask && p.blend != 2 && !p.alphaOnly {
			for c := range 3 {
				mask[c] = math.Pow(float64(key[c])/colorLevels, 2)
			}
		}
		index, ok := groups[key]
		if ok && !p.alphaOnly {
			for c := range 3 {
				if math.Abs(draws[index].mask[c]-mask[c]) > colorError {
					ok = false
				}
			}
		}
		if !ok {
			// Neighbouring RGB buckets often lie along the same tint gradient.
			// Share their draw while retaining a bounded absolute error and faint
			// nonzero channels, rather than emitting hundreds of near-identical
			// Classic draw calls for combinations of three independent buckets.
			if !p.alphaOnly {
				best := colorError
				for i, draw := range draws {
					distance := 0.0
					for c := range 3 {
						if (draw.mask[c] == 0) != (key[c] == 0) {
							distance = 1
							break
						}
						distance = max(distance, math.Abs(draw.mask[c]-mask[c]))
					}
					if distance <= best {
						index, ok, best = i, true, distance
					}
				}
			}
		}
		if !ok {
			if len(draws) >= 512 {
				return nil
			}
			index = len(draws)
			groups[key] = index
			copyG := *g
			copyG.Vertices = nil
			copyG.Faces = nil
			mask[3] = float64(key[3]) / alphaLevels
			draws = append(draws, nativeMaskDraw{geoset: &copyG, mask: mask})
		}
		groups[key] = index
		draws[index].geoset.Faces = append(draws[index].geoset.Faces, face)
	}
	for _, draw := range draws {
		vertices := map[*components.GeosetVertex]*components.GeosetVertex{}
		for fi := range draw.geoset.Faces {
			for vi, v := range draw.geoset.Faces[fi].Vertices {
				copyV := vertices[v]
				if copyV == nil {
					copyVertex := *v
					if v.TexPosition2 != nil {
						copyVertex.TexPosition = *v.TexPosition2
					}
					copyVertex.TexPosition2 = nil
					copyV = &copyVertex
					vertices[v] = copyV
					draw.geoset.Vertices = append(draw.geoset.Vertices, copyV)
				}
				draw.geoset.Faces[fi].Vertices[vi] = copyV
			}
		}
	}
	return draws
}

// Bound the complete source footprint, including bilinear neighbours. Sparse
// vertex/midpoint samples alone can overlook a bright feature inside a face.
func nativeMaskFootprintFits(face components.Face, p bakeProgram, transform [6]float64, want [4]float64) bool {
	lo, hi := imath.Vector2{math.Inf(1), math.Inf(1)}, imath.Vector2{math.Inf(-1), math.Inf(-1)}
	for _, v := range face.Vertices {
		uv := transformBakeUV(v.TexPosition, transform)
		if !finiteBakeUV(uv) {
			return false
		}
		for c := range 2 {
			lo[c], hi[c] = min(lo[c], uv[c]), max(hi[c], uv[c])
		}
	}
	img := p.images[0]
	coordinates := [2][]float64{}
	for axis, n := range [2]int{img.Bounds().Dx(), img.Bounds().Dy()} {
		if p.flags[0]&(1<<axis) == 0 {
			lo[axis], hi[axis] = max(.5/float64(n), min(1-.5/float64(n), lo[axis])), max(.5/float64(n), min(1-.5/float64(n), hi[axis]))
		} else if hi[axis]-lo[axis] >= 1 {
			lo[axis], hi[axis] = 0, 1
		}
		coordinates[axis] = []float64{lo[axis], hi[axis]}
		for i := int(math.Ceil(lo[axis]*float64(n) - .5)); float64(i)+.5 < hi[axis]*float64(n); i++ {
			coordinates[axis] = append(coordinates[axis], (float64(i)+.5)/float64(n))
			if len(coordinates[axis]) > 4096 {
				return false
			}
		}
	}
	// Bilinear samples attain extrema at the UV rectangle boundary or texel
	// centres. Include fractional boundaries so a steep texel edge can still
	// converge when its triangle footprint becomes sufficiently small.
	if len(coordinates[0])*len(coordinates[1]) > 4096 {
		return false
	}
	low, high := [4]float64{1, 1, 1, 1}, [4]float64{}
	for _, y := range coordinates[1] {
		for _, x := range coordinates[0] {
			value := sampleBakeTexture(img, imath.Vector2{x, y}, p.flags[0])
			for c := range 4 {
				low[c], high[c] = min(low[c], value[c]), max(high[c], value[c])
			}
		}
	}
	if p.alphaOnly {
		return want[3]-low[3] <= 1.0/64 && high[3]-want[3] <= 1.0/64
	}
	colorError := 1.0 / 16
	if p.opaqueMask {
		colorError = 1.0 / 255
	}
	for c := range 3 {
		if want[c]-low[c]*low[3] > colorError || high[c]*high[3]-want[c] > colorError {
			return false
		}
	}
	return true
}

func refineMaskFaces(g *components.Geoset, sample func(*components.GeosetVertex) [4]float64, center func(components.Face) [4]float64, fits func(components.Face, [4]float64) bool, faceLimit int) []components.Face {
	ids := map[*components.GeosetVertex]int{}
	for _, v := range g.Vertices {
		if _, ok := ids[v]; !ok {
			ids[v] = len(ids)
		}
	}
	edge := func(a, b *components.GeosetVertex) [2]int {
		x, y := ids[a], ids[b]
		return [2]int{min(x, y), max(x, y)}
	}
	midpoints := map[[2]int]*components.GeosetVertex{}
	midpoint := func(a, b *components.GeosetVertex) *components.GeosetVertex {
		key := edge(a, b)
		v := midpoints[key]
		if v == nil {
			v = interpolateMaskVertex(a, b)
			ids[v] = len(ids)
			midpoints[key] = v
		}
		return v
	}
	faces := append([]components.Face(nil), g.Faces...)
	for depth := 0; depth <= 10; depth++ {
		marked := map[[2]int]bool{}
		for _, f := range faces {
			want := center(f)
			error := 0.0
			for k, a := range f.Vertices {
				b := f.Vertices[(k+1)%3]
				for _, got := range [3][4]float64{sample(a), sample(b), sample(midpoint(a, b))} {
					for c := range 4 {
						error = max(error, math.Abs(got[c]-want[c]))
					}
				}
			}
			if error > 1.0/16 || !fits(f, want) {
				if depth == 10 {
					return nil
				}
				for k, a := range f.Vertices {
					marked[edge(a, f.Vertices[(k+1)%3])] = true
				}
			}
		}
		if len(marked) == 0 {
			return faces
		}
		next := make([]components.Face, 0, len(faces)*2)
		add := func(a, b, c *components.GeosetVertex) {
			next = append(next, components.Face{Vertices: [3]*components.GeosetVertex{a, b, c}})
		}
		// Propagate each split to its neighbour. Shared midpoint vertices keep
		// skinned edges watertight, including different refinement depths.
		for _, f := range faces {
			v, m := f.Vertices, [3]*components.GeosetVertex{}
			count, single, missing := 0, 0, 0
			for k, a := range v {
				if marked[edge(a, v[(k+1)%3])] {
					m[k] = midpoint(a, v[(k+1)%3])
					count++
					single = k
				} else {
					missing = k
				}
			}
			switch count {
			case 0:
				next = append(next, f)
			case 1:
				k := single
				add(v[k], m[k], v[(k+2)%3])
				add(m[k], v[(k+1)%3], v[(k+2)%3])
			case 2:
				k := (missing + 1) % 3
				add(v[(k+1)%3], m[(k+1)%3], m[k])
				add(v[k], m[k], v[(k+2)%3])
				add(m[k], m[(k+1)%3], v[(k+2)%3])
			case 3:
				for k := range 3 {
					add(v[k], m[k], m[(k+2)%3])
				}
				add(m[0], m[1], m[2])
			}
			if len(next) > faceLimit {
				return nil
			}
		}
		faces = next
	}
	return nil
}

func interpolateMaskVertex(a, b *components.GeosetVertex) *components.GeosetVertex {
	fraction := .5
	v := *a
	for k := range 3 {
		v.Position[k] = a.Position[k] + (b.Position[k]-a.Position[k])*fraction
		v.Normal[k] = a.Normal[k] + (b.Normal[k]-a.Normal[k])*fraction
	}
	length := math.Sqrt(v.Normal[0]*v.Normal[0] + v.Normal[1]*v.Normal[1] + v.Normal[2]*v.Normal[2])
	if length > 0 {
		for k := range 3 {
			v.Normal[k] /= length
		}
	}
	for k := range 2 {
		v.TexPosition[k] = a.TexPosition[k] + (b.TexPosition[k]-a.TexPosition[k])*fraction
	}
	if a.TexPosition2 != nil && b.TexPosition2 != nil {
		uv := imath.Vector2{(*a.TexPosition2)[0] + ((*b.TexPosition2)[0]-(*a.TexPosition2)[0])*fraction, (*a.TexPosition2)[1] + ((*b.TexPosition2)[1]-(*a.TexPosition2)[1])*fraction}
		v.TexPosition2 = &uv
	}
	v.SkinWeights = nil
	weights := map[*components.Bone]float64{}
	order := []*components.Bone{}
	for i, input := range []*components.GeosetVertex{a, b} {
		for _, sw := range input.SkinWeights {
			if _, ok := weights[sw.Bone]; !ok {
				order = append(order, sw.Bone)
			}
			weights[sw.Bone] += float64(sw.Weight) * [2]float64{1 - fraction, fraction}[i]
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return weights[order[i]] > weights[order[j]] })
	if len(order) > 4 {
		order = order[:4]
	}
	total := 0.0
	for _, bone := range order {
		total += weights[bone]
	}
	for _, bone := range order {
		v.SkinWeights = append(v.SkinWeights, components.SkinWeight{Bone: bone, Weight: int(math.Round(float64(weights[bone]) * 255 / float64(max(1, total))))})
	}
	if len(v.SkinWeights) > 0 {
		assigned := 0
		for _, weight := range v.SkinWeights {
			assigned += weight.Weight
		}
		v.SkinWeights[0].Weight += 255 - assigned
	}
	return &v
}

func tintMaskAnimation(ga components.GeosetAnim, mask [4]float64) components.GeosetAnim {
	tint := imath.Vector3{mask[2], mask[1], mask[0]} // MDL uses BGR.
	color := components.AnimatedOrStatic[imath.Vector3]{Static: true, Value: imath.Vector3{1, 1, 1}}
	if ga.Color != nil {
		color = *ga.Color
	}
	if color.Static {
		for k := range 3 {
			color.Value[k] *= tint[k]
		}
	} else if color.Anim != nil {
		anim := *color.Anim
		anim.KeyFrames = map[int]any{}
		for t, value := range color.Anim.KeyFrames {
			if v, ok := value.(imath.Vector3); ok {
				for k := range 3 {
					v[k] *= tint[k]
				}
				anim.KeyFrames[t] = v
			}
		}
		if color.Anim.InOutTans != nil {
			anim.InOutTans = map[int]components.InOutTan{}
			for t, tan := range color.Anim.InOutTans {
				for k := range 3 {
					tan.InTan[k] *= tint[k]
					tan.OutTan[k] *= tint[k]
				}
				anim.InOutTans[t] = tan
			}
		}
		color.Anim = &anim
	}
	ga.Color = &color
	alpha := components.AnimatedOrStatic[float64]{Static: true, Value: 1}
	if ga.Alpha != nil {
		alpha = *ga.Alpha
	}
	alpha = scaleBakeRate(alpha, mask[3])
	ga.Alpha = &alpha
	return ga
}
