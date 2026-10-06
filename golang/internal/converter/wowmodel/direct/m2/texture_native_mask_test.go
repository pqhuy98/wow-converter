package directm2

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestNativeMaskKeepsDetailedUVsAndBoundsTheMaskError(t *testing.T) {
	mask := image.NewNRGBA(image.Rect(0, 0, 256, 1))
	for x := range 256 {
		mask.SetNRGBA(x, 0, color.NRGBA{uint8(x), uint8(x / 2), 128, 255})
	}
	bone := &components.Bone{}
	g := &components.Geoset{}
	for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		uv2 := imath.Vector2{uv[0]*3 + .2, uv[1]*2 + .3}
		g.Vertices = append(g.Vertices, &components.GeosetVertex{Position: imath.Vector3{uv[0], uv[1], 0}, Normal: imath.Vector3{0, 0, 1}, TexPosition: uv, TexPosition2: &uv2, SkinWeights: []components.SkinWeight{{Bone: bone, Weight: 255}}})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	p := bakeProgram{shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, blend: 4, count: 2, images: [4]*image.NRGBA{mask, mask}}
	draws := nativeModulateMask(g, p)
	if len(draws) < 2 {
		t.Fatal("smooth mask was not represented by native detail groups")
	}
	area := 0.0
	for _, draw := range draws {
		for _, face := range draw.geoset.Faces {
			a, b, c := face.Vertices[0].Position, face.Vertices[1].Position, face.Vertices[2].Position
			area += math.Abs((b[0]-a[0])*(c[1]-a[1])-(b[1]-a[1])*(c[0]-a[0])) / 2
			for _, v := range face.Vertices {
				if v.TexPosition2 != nil || math.Abs(v.TexPosition[0]-(v.Position[0]*3+.2)) > 1e-9 || math.Abs(v.TexPosition[1]-(v.Position[1]*2+.3)) > 1e-9 {
					t.Fatal("source detail UVs changed")
				}
				if len(v.SkinWeights) != 1 || v.SkinWeights[0].Bone != bone || v.SkinWeights[0].Weight != 255 {
					t.Fatal("subdivision lost skinning")
				}
				sample := sampleBakeTexture(mask, imath.Vector2{v.Position[0], v.Position[1]}, 0)
				for k := range 4 {
					if math.Abs(sample[k]-draw.mask[k]) > 1.0/8+1e-9 {
						t.Fatalf("mask approximation exceeded tolerance: %v vs %v", sample, draw.mask)
					}
				}
			}
		}
	}
	if math.Abs(area-.5) > 1e-9 {
		t.Fatalf("triangle coverage changed: %f", area)
	}
	if g.Vertices[0].TexPosition2 == nil || len(g.Faces) != 1 {
		t.Fatal("input geometry mutated during native-path eligibility check")
	}
	// Effect 0x8015 uses this same combiner and UV routing, but also fades
	// mesh RGB and alpha. Native detail must not bypass the fragment fade.
	p.shader.edge = true
	if nativeModulateMask(g, p) != nil {
		t.Fatal("edge-faded shader bypassed the fragment baker")
	}
	p.shader.edge = false
	gs := components.NewGlobalSequence(0, 1000)
	p.transforms[0] = &components.TextureAnim{Translation: &components.Animation{GlobalSeq: &gs, KeyFrames: map[int]any{0: imath.Vector3{}, 1000: imath.Vector3{1, 0, 0}}}}
	if nativeModulateMask(g, p) != nil {
		t.Fatal("animated mask incorrectly treated as static")
	}
	p.transforms[0].Translation = &components.Animation{Interpolation: components.InterpHermite, GlobalSeq: &gs, KeyFrames: map[int]any{0: imath.Vector3{}, 1000: imath.Vector3{}}, InOutTans: map[int]components.InOutTan{0: {OutTan: imath.Vector3{1, 0, 0}}, 1000: {InTan: imath.Vector3{1, 0, 0}}}}
	if nativeModulateMask(g, p) != nil {
		t.Fatal("cubic mask with equal endpoints incorrectly treated as static")
	}
}

func TestNativeMaskDoesNotMissAnInteriorTextureFeature(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			img.SetNRGBA(x, y, color.NRGBA{0, 0, 0, 255})
		}
	}
	img.SetNRGBA(6, 9, color.NRGBA{255, 255, 255, 255})
	g := &components.Geoset{}
	for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		uv2 := uv
		g.Vertices = append(g.Vertices, &components.GeosetVertex{Position: imath.Vector3{uv[0], uv[1], 0}, TexPosition: uv, TexPosition2: &uv2})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	p := bakeProgram{shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, blend: 4, count: 2, images: [4]*image.NRGBA{img, img}}
	transform := bakeTransform(nil, 0)
	if nativeMaskFootprintFits(g.Faces[0], p, transform, [4]float64{0, 0, 0, 1}) {
		t.Fatal("footprint check overlooked the interior white texel")
	}
	draws := nativeModulateMask(g, p)
	if draws == nil {
		return
	} // The exact baker is a valid bounded-cost fallback.
	bright := 0.0
	for _, draw := range draws {
		bright = max(bright, draw.mask[0])
	}
	if bright < .8 {
		t.Fatal("native conversion discarded the interior white feature")
	}
}

func TestNativeMaskTintDoesNotMutateVisibilityOrColourTracks(t *testing.T) {
	colorTrack := &components.Animation{Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{.8, .6, .4}}}
	alphaTrack := &components.Animation{Interpolation: components.InterpHermite, KeyFrames: map[int]any{0: 0.5}, InOutTans: map[int]components.InOutTan{0: {InTan: imath.Vector3{.4, 0, 0}, OutTan: imath.Vector3{.8, 0, 0}}}}
	source := components.GeosetAnim{Color: &components.AnimatedOrStatic[imath.Vector3]{Anim: colorTrack}, Alpha: &components.AnimatedOrStatic[float64]{Anim: alphaTrack}}
	got := tintMaskAnimation(source, [4]float64{.25, .5, .75, .5})
	value := got.Color.Anim.KeyFrames[0].(imath.Vector3)
	if math.Abs(value[0]-.6) > 1e-9 || math.Abs(value[1]-.3) > 1e-9 || math.Abs(value[2]-.1) > 1e-9 || got.Alpha.Anim.KeyFrames[0] != .25 {
		t.Fatalf("wrong BGR tint or visibility: %+v %+v", got.Color.Anim.KeyFrames, got.Alpha.Anim.KeyFrames)
	}
	if source.Color.Anim.KeyFrames[0] != (imath.Vector3{.8, .6, .4}) || source.Alpha.Anim.KeyFrames[0] != .5 {
		t.Fatal("shared source animation was mutated")
	}
	if got.Alpha.Anim.InOutTans[0].InTan[0] != .2 || got.Alpha.Anim.InOutTans[0].OutTan[0] != .4 || source.Alpha.Anim.InOutTans[0].InTan[0] != .4 {
		t.Fatal("alpha tangents were not scaled independently")
	}
}

func TestNativeMaskRefinementKeepsNeighbouringSkinnedEdgesConforming(t *testing.T) {
	mask := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			mask.SetNRGBA(x, y, color.NRGBA{uint8(x * y / 4), 0, 0, 255})
		}
	}
	bones := [2]components.Bone{}
	g := &components.Geoset{}
	for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		uv2 := uv
		g.Vertices = append(g.Vertices, &components.GeosetVertex{Position: imath.Vector3{uv[0], uv[1], 0}, TexPosition: uv, TexPosition2: &uv2, SkinWeights: []components.SkinWeight{{Bone: &bones[0], Weight: 128}, {Bone: &bones[1], Weight: 127}}})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}, {Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[2], g.Vertices[3]}}}
	p := bakeProgram{shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, blend: 4, count: 2, images: [4]*image.NRGBA{mask, mask}}
	draws := nativeModulateMask(g, p)
	if len(draws) == 0 {
		t.Fatal("smooth mask unexpectedly fell back")
	}
	segments := map[[2]float64]int{}
	for _, draw := range draws {
		for _, f := range draw.geoset.Faces {
			for k, a := range f.Vertices {
				b := f.Vertices[(k+1)%3]
				if a.Position[0] == a.Position[1] && b.Position[0] == b.Position[1] {
					segments[[2]float64{min(a.Position[0], b.Position[0]), max(a.Position[0], b.Position[0])}]++
				}
			}
		}
	}
	for segment, count := range segments {
		if count != 2 {
			t.Fatalf("skinned diagonal has a hanging edge: %v occurs %d times", segment, count)
		}
	}
	if len(segments) < 2 {
		t.Fatal("check never exercised an edge split")
	}
}

func TestNativeBlendRadianceIncludesMaskAlpha(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	a.SetNRGBA(0, 0, color.NRGBA{128, 64, 255, 128})
	g := &components.Geoset{}
	for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		uv2 := uv
		g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv2})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	p := bakeProgram{shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, blend: 2, count: 2, images: [4]*image.NRGBA{a, a}}
	draws := nativeModulateMask(g, p)
	if len(draws) != 1 || draws[0].mask[3] != 1 {
		t.Fatal("blend radiance still attenuates the destination")
	}
	for k, want := range [3]float64{128.0 * 128 / (255 * 255), 64.0 * 128 / (255 * 255), 128.0 / 255} {
		if math.Abs(draws[0].mask[k]-want) > .5/16 {
			t.Fatal("additive radiance lost the static alpha mask")
		}
	}
}

func TestNativeMaskKeepsFaintRadiance(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{64, 64, 255, 8})
	uv := imath.Vector2{.5, .5}
	v := &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv}
	g := &components.Geoset{Vertices: []*components.GeosetVertex{v}, Faces: []components.Face{{Vertices: [3]*components.GeosetVertex{v, v, v}}}}
	p := bakeProgram{shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, blend: 4, count: 2, images: [4]*image.NRGBA{img, img}}
	draws := nativeModulateMask(g, p)
	if len(draws) != 1 || draws[0].mask[0] == 0 || draws[0].mask[2] == 0 {
		t.Fatal("faint stars and scrolling detail were rounded to black")
	}
}

func TestNativeUVLoopStaysInPhaseWithRelatedBakedCutouts(t *testing.T) {
	for _, window := range []int{4000, 12000} {
		gs := components.NewGlobalSequence(0, 33000)
		otherGS := components.NewGlobalSequence(1, 33000)
		source := components.TextureAnim{Translation: &components.Animation{Interpolation: components.InterpLinear, GlobalSeq: &gs, KeyFrames: map[int]any{0: imath.Vector3{}, 1500: imath.Vector3{.2, -.1, 0}, 33000: imath.Vector3{1, 1, 0}}}}
		other := source
		otherAnim := *source.Translation
		otherAnim.GlobalSeq = &otherGS
		other.Translation = &otherAnim
		material := &components.Material{Layers: []components.Layer{{TVertexAnim: &source}, {TVertexAnim: &source}, {TVertexAnim: &other}}}
		model := mdl.New(mdl.NewMDLOptions{})
		model.Materials = []*components.Material{material}
		model.GlobalSequences = []*components.GlobalSequence{&gs, &otherGS}
		result := ConvertResult{MDL: model}
		boundNativeUVLoops(&result, map[*components.GlobalSequence]bool{&gs: true}, window)
		color, alpha := material.Layers[0].TVertexAnim, material.Layers[1].TVertexAnim
		if color != alpha || color.Translation.GlobalSeq.Duration != window || color.Translation.Interpolation != components.InterpLinear {
			t.Fatal("related color/alpha did not share a continuous bounded UV loop")
		}
		for _, at := range []int{0, 1000, 1500, window - 1} {
			want, got := sampleBakeAnimation(source.Translation, float64(at), nil), sampleBakeAnimation(color.Translation, float64(at), nil)
			for c := range want {
				if math.Abs(want[c]-got[c]) > 1e-9 {
					t.Fatal("bounded native UV loop changed source velocity")
				}
			}
		}
		if material.Layers[2].TVertexAnim != &other || source.Translation.GlobalSeq.Duration != 33000 || len(source.Translation.KeyFrames) != 3 {
			t.Fatal("independent/source loop was mutated")
		}
		if color.Translation.KeyFrames[window] == (imath.Vector3{}) {
			t.Fatal("linear final segment interpolates toward zero instead of the source endpoint")
		}
	}
}
