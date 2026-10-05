package directm2

import (
	"image"
	"math"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

// Multiplication can use two native UV draws wherever source coverage is
// guaranteed. Both retain the original triangles and skinning. A modulation
// draw must never cover a cutout hole: it would multiply the background there.
func nativeOpaqueProduct(g *components.Geoset, p bakeProgram) (*components.Geoset, *components.Geoset, *components.Geoset) {
	if p.shader.pixel != 6 || p.count != 2 || p.blend > 1 || p.shader.edge || p.shader.coords[0] != coordT1M0 || p.shader.coords[1] != coordT2M1 {
		return nil, nil, nil
	}
	offset, animated, ok := nativeOpacityDomain(p.transforms[1])
	if !ok {
		return nil, nil, nil
	}
	if _, _, ok := nativeOpacityDomain(p.transforms[0]); !ok || (p.blend == 1 && p.transforms[0] != nil && !constantBakeTrack(p.transforms[0].Translation)) {
		return nil, nil, nil
	}
	first, second := newOpaqueAlphaRect(p.images[0]), newAlphaCoverageRect(p.images[1], 129)
	transform := bakeTransform(p.transforms[0], 0)
	rest, base, factor := cloneBakeGeoset(g), cloneBakeGeoset(g), cloneBakeGeoset(g)
	restFaces, baseFaces, factorFaces := rest.Faces, base.Faces, factor.Faces
	rest.Faces, base.Faces, factor.Faces = nil, nil, nil
	for i, f := range g.Faces {
		lo, hi := imath.Vector2{math.Inf(1), math.Inf(1)}, imath.Vector2{math.Inf(-1), math.Inf(-1)}
		lo2, hi2 := lo, hi
		for _, v := range f.Vertices {
			uv, uv2 := transformBakeUV(v.TexPosition, transform), v.TexPosition
			if v.TexPosition2 != nil {
				uv2 = *v.TexPosition2
			}
			for k := range 2 {
				lo[k], hi[k] = min(lo[k], uv[k]), max(hi[k], uv[k])
				lo2[k], hi2[k] = min(lo2[k], uv2[k]+offset[k]), max(hi2[k], uv2[k]+offset[k])
			}
		}
		if p.blend == 0 || (first.contains(lo, hi, [2]bool{}, p.flags[0]) && second.contains(lo2, hi2, animated, p.flags[1])) {
			base.Faces = append(base.Faces, baseFaces[i])
			factor.Faces = append(factor.Faces, factorFaces[i])
		} else {
			rest.Faces = append(rest.Faces, restFaces[i])
		}
	}
	if len(base.Faces) == 0 {
		return nil, nil, nil
	}
	material := *g.Material
	material.Layers = append([]components.Layer(nil), g.Material.Layers...)
	rest.Material = &material
	for _, v := range base.Vertices {
		if v.TexPosition2 != nil {
			v.TexPosition = *v.TexPosition2
		}
		v.TexPosition2 = nil
	}
	return rest, base, factor
}

// Uniform opaque mask faces can retain the detailed second sample in a single
// Classic draw. Keep original triangles so they fit the baked remainder exactly.
func nativeOpaqueMasks(g *components.Geoset, p bakeProgram) (*components.Geoset, []nativeMaskDraw) {
	if p.shader.pixel != 6 || p.count != 2 || p.blend > 1 || p.shader.edge || p.shader.coords[0] != coordT1M0 || p.shader.coords[1] != coordT2M1 {
		return nil, nil
	}
	if p.transforms[0] != nil {
		for _, a := range []*components.Animation{p.transforms[0].Translation, p.transforms[0].Rotation, p.transforms[0].Scaling} {
			if a != nil && !constantBakeTrack(a) {
				return nil, nil
			}
		}
	}
	if _, _, ok := nativeOpacityDomain(p.transforms[1]); !ok {
		return nil, nil
	}
	transform := bakeTransform(p.transforms[0], 0)
	proof := newOpaqueAlphaRect(p.images[0])
	rest, native := cloneBakeGeoset(g), cloneBakeGeoset(g)
	restFaces, nativeFaces := rest.Faces, native.Faces
	rest.Faces, native.Faces = nil, nil
	p.blend, p.opaqueMask = 4, true
	for i, f := range g.Faces {
		lo, hi := imath.Vector2{math.Inf(1), math.Inf(1)}, imath.Vector2{math.Inf(-1), math.Inf(-1)}
		center := imath.Vector2{}
		for _, v := range f.Vertices {
			uv := transformBakeUV(v.TexPosition, transform)
			for k := range 2 {
				lo[k], hi[k] = min(lo[k], uv[k]), max(hi[k], uv[k])
				center[k] += uv[k] / 3
			}
		}
		mask := sampleBakeTexture(p.images[0], center, p.flags[0])
		if proof.contains(lo, hi, [2]bool{}, p.flags[0]) && nativeMaskFootprintFits(f, p, transform, mask) {
			native.Faces = append(native.Faces, nativeFaces[i])
		} else {
			rest.Faces = append(rest.Faces, restFaces[i])
		}
	}
	if len(native.Faces) == 0 {
		return nil, nil
	}
	draws := nativeModulateMask(native, p)
	if len(draws) == 0 {
		return nil, nil
	}
	// Eligibility already bounded every face; subdivision would break the
	// interface with the original triangles of the baked remainder.
	faces := 0
	for _, draw := range draws {
		faces += len(draw.geoset.Faces)
	}
	if faces != len(native.Faces) {
		return nil, nil
	}
	material := *g.Material
	material.Layers = append([]components.Layer(nil), g.Material.Layers...)
	rest.Material = &material
	return rest, draws
}

func nativeCutoutAlpha(img *image.NRGBA) *image.NRGBA {
	copyImage := image.NewNRGBA(img.Bounds())
	copy(copyImage.Pix, img.Pix)
	for i := 3; i < len(copyImage.Pix); i += 4 {
		// Classic tests .75; WoW tests about .5. Map [0,1] into [.5,1]
		// without clipping, preserving linear filtering within one alpha byte.
		copyImage.Pix[i] = uint8((int(img.Pix[i]) + 256) / 2)
	}
	return copyImage
}

func nativeModulateColor(img *image.NRGBA) *image.NRGBA {
	out := image.NewNRGBA(img.Bounds())
	copy(out.Pix, img.Pix)
	for i := 3; i < len(out.Pix); i += 4 {
		// Classic's Modulate shader discards near-zero alpha. Source alpha is
		// already accounted for by the coverage proof or ignored by Opaque.
		out.Pix[i] = 255
	}
	return out
}

// When the animated detail is opaque over its entire swept UV domain, Blend
// factorizes into a static black alpha draw and additive modulated radiance.
// This keeps the alpha mask exact without tessellating its opacity gradient.
func nativeDetailOpaque(g *components.Geoset, p bakeProgram) bool {
	lo, hi := imath.Vector2{math.Inf(1), math.Inf(1)}, imath.Vector2{math.Inf(-1), math.Inf(-1)}
	for _, v := range g.Vertices {
		uv := v.TexPosition
		if v.TexPosition2 != nil {
			uv = *v.TexPosition2
		}
		for axis := range 2 {
			lo[axis], hi[axis] = min(lo[axis], uv[axis]), max(hi[axis], uv[axis])
		}
	}
	offset, animated, ok := nativeOpacityDomain(p.transforms[1])
	if !ok {
		return false
	}
	for axis := range 2 {
		lo[axis], hi[axis] = lo[axis]+offset[axis], hi[axis]+offset[axis]
	}
	return newOpaqueAlphaRect(p.images[1]).contains(lo, hi, animated, p.flags[1])
}

func nativeOpacityDomain(ta *components.TextureAnim) (imath.Vector2, [2]bool, bool) {
	var offset imath.Vector2
	var animated [2]bool
	if ta == nil {
		return offset, animated, true
	}
	if ta.Rotation != nil || ta.Scaling != nil {
		return offset, animated, false
	}
	if a := ta.Translation; a != nil {
		if a.Interpolation != components.InterpLinear && a.Interpolation != components.InterpDontInterp {
			return offset, animated, false
		}
		first := true
		for _, key := range a.KeyFrames {
			v, ok := key.(imath.Vector3)
			if !ok {
				return offset, animated, false
			}
			if first {
				offset, first = imath.Vector2{v[0], v[1]}, false
			} else {
				for axis := range 2 {
					animated[axis] = animated[axis] || math.Abs(v[axis]-offset[axis]) > 1e-9
				}
			}
		}
	}
	return offset, animated, true
}

func nativeMaskAlpha(g *components.Geoset, p bakeProgram) (*components.Geoset, *image.NRGBA) {
	g = cloneBakeGeoset(g)
	w, h := p.images[0].Bounds().Dx(), p.images[0].Bounds().Dy()
	lo, hi := imath.Vector2{math.Inf(1), math.Inf(1)}, imath.Vector2{math.Inf(-1), math.Inf(-1)}
	transform := bakeTransform(p.transforms[0], 0)
	for _, v := range g.Vertices {
		v.TexPosition = transformBakeUV(v.TexPosition, transform)
		v.TexPosition2 = nil
		for k := range 2 {
			lo[k], hi[k] = min(lo[k], v.TexPosition[k]), max(hi[k], v.TexPosition[k])
		}
	}
	x0, y0, cw, ch := 0, 0, w, h
	if lo[0] >= 0 && lo[1] >= 0 && hi[0] <= 1 && hi[1] <= 1 {
		x0, y0 = max(0, int(math.Floor(lo[0]*float64(w)))-bakePadding), max(0, int(math.Floor(lo[1]*float64(h)))-bakePadding)
		cw = bakePowerOfTwo(int(math.Ceil(hi[0]*float64(w))) + bakePadding - x0)
		ch = bakePowerOfTwo(int(math.Ceil(hi[1]*float64(h))) + bakePadding - y0)
	}
	img := image.NewNRGBA(image.Rect(0, 0, cw, ch))
	for y := range ch {
		for x := range cw {
			sx, sy := min(w-1, x+x0), min(h-1, y+y0)
			img.Pix[y*img.Stride+x*4+3] = p.images[0].Pix[sy*p.images[0].Stride+sx*4+3]
		}
	}
	for _, v := range g.Vertices {
		v.TexPosition[0] = (v.TexPosition[0]*float64(w) - float64(x0)) / float64(cw)
		v.TexPosition[1] = (v.TexPosition[1]*float64(h) - float64(y0)) / float64(ch)
	}
	return g, img
}

type opaqueAlphaRect struct {
	w, h int
	bad  []uint32
}

func newOpaqueAlphaRect(img *image.NRGBA) opaqueAlphaRect {
	return newAlphaCoverageRect(img, 255)
}

func newAlphaCoverageRect(img *image.NRGBA, minimum uint8) opaqueAlphaRect {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	p := opaqueAlphaRect{w: w, h: h, bad: make([]uint32, (w+1)*(h+1))}
	for y := range h {
		row := uint32(0)
		for x := range w {
			if img.Pix[y*img.Stride+x*4+3] < minimum {
				row++
			}
			p.bad[(y+1)*(w+1)+x+1] = p.bad[y*(w+1)+x+1] + row
		}
	}
	return p
}

func (p opaqueAlphaRect) contains(lo, hi imath.Vector2, animated [2]bool, flags uint32) bool {
	intervals := [2][][2]int{}
	for axis, n := range [2]int{p.w, p.h} {
		if !finiteBakeUV(lo) || !finiteBakeUV(hi) {
			return false
		}
		// GLSL linear sampling reads texels around uv*size-.5.
		a, b := int(math.Floor(lo[axis]*float64(n)-.5)), int(math.Floor(hi[axis]*float64(n)-.5))+1
		if animated[axis] || b-a+1 >= n {
			intervals[axis] = [][2]int{{0, n}}
		} else if flags&(1<<axis) == 0 {
			intervals[axis] = [][2]int{{max(0, min(n-1, a)), max(0, min(n-1, b)) + 1}}
		} else {
			a = ((a % n) + n) % n
			length := b - int(math.Floor(lo[axis]*float64(n)-.5)) + 1
			if a+length <= n {
				intervals[axis] = [][2]int{{a, a + length}}
			} else {
				intervals[axis] = [][2]int{{a, n}, {0, a + length - n}}
			}
		}
	}
	for _, x := range intervals[0] {
		for _, y := range intervals[1] {
			stride := p.w + 1
			if p.bad[y[1]*stride+x[1]]+p.bad[y[0]*stride+x[0]]-p.bad[y[0]*stride+x[1]]-p.bad[y[1]*stride+x[0]] != 0 {
				return false
			}
		}
	}
	return true
}
