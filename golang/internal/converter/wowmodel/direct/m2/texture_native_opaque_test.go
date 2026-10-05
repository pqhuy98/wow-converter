package directm2

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestNativeDetailAlphaProofIncludesAnimationAndWrap(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
		}
	}
	img.SetNRGBA(8, 14, color.NRGBA{})
	p := newOpaqueAlphaRect(img)
	lo, hi := imath.Vector2{.5, .1}, imath.Vector2{.5, .2}
	if !p.contains(lo, hi, [2]bool{}, 3) || p.contains(lo, hi, [2]bool{false, true}, 3) {
		t.Fatal("animation sweep was not included in the alpha proof")
	}
	img.SetNRGBA(15, 2, color.NRGBA{})
	p = newOpaqueAlphaRect(img)
	if p.contains(imath.Vector2{0, .15}, imath.Vector2{0, .15}, [2]bool{}, 3) {
		t.Fatal("bilinear sampling ignored the neighbour across the repeat seam")
	}
	if !p.contains(imath.Vector2{0, .15}, imath.Vector2{0, .15}, [2]bool{}, 0) {
		t.Fatal("clamped sampling read the opposite edge")
	}
}

func TestOpaqueProductPreservesTwoUVSetsAndRejectsSweptCutoutHoles(t *testing.T) {
	a, b := image.NewNRGBA(image.Rect(0, 0, 4, 2)), image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := range 2 {
		for x := range 4 {
			a.SetNRGBA(x, y, color.NRGBA{uint8(x * 60), 17, 101, 255})
			alpha := uint8(255)
			if x >= 2 && y == 1 {
				alpha = 100
			}
			b.SetNRGBA(x, y, color.NRGBA{100, 100, 100, alpha})
		}
	}
	g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{}}}}
	for fi := range 2 {
		f := components.Face{}
		for vi, uv := range []imath.Vector2{{.125, .25}, {.375, .25}, {.125, .5}} {
			uv2 := imath.Vector2{.125 + float64(fi)*.75, .25}
			v := &components.GeosetVertex{Position: imath.Vector3{float64(vi), float64(fi), 0}, TexPosition: uv, TexPosition2: &uv2}
			g.Vertices = append(g.Vertices, v)
			f.Vertices[vi] = v
		}
		g.Faces = append(g.Faces, f)
	}
	gs := components.NewGlobalSequence(0, 33333)
	p := bakeProgram{shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, count: 2, blend: 1, images: [4]*image.NRGBA{a, b}, flags: [4]uint32{0, 3}}
	p.transforms[1] = &components.TextureAnim{Translation: &components.Animation{GlobalSeq: &gs, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 33333: imath.Vector3{0, 1, 0}}}}
	rest, base, factor := nativeOpaqueProduct(g, p)
	if rest == nil || len(rest.Faces) != 1 || len(base.Faces) != 1 || len(factor.Faces) != 1 {
		t.Fatal("opaque native product did not separate the future cutout hole")
	}
	for i, v := range g.Faces[0].Vertices {
		bv, fv := base.Faces[0].Vertices[i], factor.Faces[0].Vertices[i]
		if bv.Position != v.Position || fv.Position != v.Position || bv.TexPosition != *v.TexPosition2 || fv.TexPosition != v.TexPosition {
			t.Fatal("two native UV draws changed geometry or sample coordinates")
		}
	}
	if rest.Material == g.Material || g.Vertices[0].TexPosition2 == nil {
		t.Fatal("native product mutated source data")
	}
}

func TestModulateColorRetainsRGBUnderZeroAlpha(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{40, 20, 80, 0})
	got := nativeModulateColor(img).NRGBAAt(0, 0)
	if got != (color.NRGBA{40, 20, 80, 255}) || img.NRGBAAt(0, 0).A != 0 {
		t.Fatal("opaque multiplication lost source color or modified alpha in place")
	}
}

func TestOpaqueMasksKeepOriginalTrianglesAndBakeVariableCoverage(t *testing.T) {
	a := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := range 2 {
		for x := range 4 {
			alpha := uint8(255)
			if x >= 2 {
				alpha = 128
			}
			a.SetNRGBA(x, y, color.NRGBA{0, 0, 0, alpha})
		}
	}
	g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{FilterMode: components.BlendTransparent}}}}
	for fi := range 2 {
		face := components.Face{}
		for i, uv2 := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
			v := &components.GeosetVertex{Position: imath.Vector3{uv2[0] + float64(fi)*2, uv2[1], 0}, TexPosition: imath.Vector2{.125 + float64(fi)*.75, .5}, TexPosition2: &uv2}
			g.Vertices = append(g.Vertices, v)
			face.Vertices[i] = v
		}
		g.Faces = append(g.Faces, face)
	}
	p := bakeProgram{shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, count: 2, blend: 1, images: [4]*image.NRGBA{a, a}}
	rest, draws := nativeOpaqueMasks(g, p)
	if rest == nil || len(rest.Faces) != 1 || len(draws) != 1 || len(draws[0].geoset.Faces) != 1 || draws[0].mask != ([4]float64{0, 0, 0, 1}) {
		t.Fatal("black opaque surface or variable coverage was lost")
	}
	for i, original := range g.Faces[0].Vertices {
		v := draws[0].geoset.Faces[0].Vertices[i]
		if v.Position != original.Position || v.TexPosition != *original.TexPosition2 {
			t.Fatal("opaque path changed original geometry or source detail UVs")
		}
	}
	if len(g.Faces) != 2 || g.Vertices[0].TexPosition2 == nil || rest.Material == g.Material {
		t.Fatal("source geometry/material mutated")
	}
}

func TestNativeCutoutAlphaPreservesClassicCoverageAndColor(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 1))
	for a := range 256 {
		img.SetNRGBA(a, 0, color.NRGBA{10, 20, 30, uint8(a)})
	}
	got := nativeCutoutAlpha(img)
	for a := range 256 {
		pixel := got.NRGBAAt(a, 0)
		if pixel.R != 10 || pixel.G != 20 || pixel.B != 30 || (float64(pixel.A)/255 >= .75) != (a >= 128) {
			t.Fatalf("alpha %d changed Classic cutout coverage or color: %v", a, pixel)
		}
		if math.Abs(float64(pixel.A)/255-(.5+.5*float64(a)/255)) > .5/255+1e-9 {
			t.Fatal("alpha remap exceeded byte quantization error")
		}
	}
	if img.NRGBAAt(0, 0).A != 0 {
		t.Fatal("source detail texture was mutated")
	}
}

func TestOpaqueMaskTintRetainsDarkColorPrecision(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{21, 9, 57, 255})
	img.SetNRGBA(1, 0, color.NRGBA{25, 9, 61, 255})
	g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{}}}}
	for fi := range 2 {
		f := components.Face{}
		for vi, uv2 := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
			v := &components.GeosetVertex{TexPosition: imath.Vector2{.25 + float64(fi)*.5, .5}, TexPosition2: &uv2}
			g.Vertices = append(g.Vertices, v)
			f.Vertices[vi] = v
		}
		g.Faces = append(g.Faces, f)
	}
	p := bakeProgram{count: 2, blend: 0, shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, images: [4]*image.NRGBA{img, img}}
	_, draws := nativeOpaqueMasks(g, p)
	if len(draws) != 2 {
		t.Fatal("distinct dark opaque tints were merged")
	}
	for i, draw := range draws {
		want := sampleBakeTexture(img, imath.Vector2{.25 + float64(i)*.5, .5}, 0)
		for c := range 3 {
			if math.Abs(draw.mask[c]-want[c]) > 1.0/255 {
				t.Fatalf("dark opaque tint lost byte precision: %v versus %v", draw.mask, want)
			}
		}
	}
}

func TestNativeMaskAlphaCropPreservesBilinearSampling(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for y := range 128 {
		for x := range 128 {
			img.SetNRGBA(x, y, color.NRGBA{255, 64, 32, uint8(x + y)})
		}
	}
	g := &components.Geoset{}
	for _, uv := range []imath.Vector2{{.2, .3}, {.25, .3}, {.2, .4}} {
		uv2 := uv
		g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv2})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	part, crop := nativeMaskAlpha(g, bakeProgram{images: [4]*image.NRGBA{img}})
	if crop.Bounds().Dx() >= 128 || crop.Bounds().Dy() >= 128 {
		t.Fatal("small swatch retained the whole source image")
	}
	for i, original := range g.Vertices {
		want := sampleBakeTexture(img, original.TexPosition, 0)
		got := sampleBakeTexture(crop, part.Vertices[i].TexPosition, 0)
		if got[0] != 0 || got[1] != 0 || got[2] != 0 || math.Abs(got[3]-want[3]) > 1e-9 {
			t.Fatalf("cropped black alpha draw changed sampling: %v vs %v", got, want)
		}
		if original.TexPosition2 == nil || part.Vertices[i].TexPosition2 != nil {
			t.Fatal("alpha crop mutated the source or retained UV2")
		}
	}
}
