package directm2

import (
	"context"
	"image"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestBakeDomainUsesTriangleCoverageRatherThanOutlyingBounds(t *testing.T) {
	g := &components.Geoset{}
	for _, uv2 := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		uv1 := imath.Vector2{.1 + uv2[0]*.002, uv2[1]}
		g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv1, TexPosition2: &uv2})
	}
	// This unused vertex expands the bounding box but contributes no surface.
	other := imath.Vector2{1, 1}
	g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: other, TexPosition2: &other})
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	p := bakeProgram{count: 2, shader: m2Shader{coords: [4]m2Coord{coordT1M0, coordT2M1}}, images: [4]*image.NRGBA{img, img}}
	if !bakePreferUV2(g, p) {
		t.Fatal("compressed mask UVs displaced the detailed surface unwrap")
	}
	p.shader.coords[1] = coordT1M1
	if bakePreferUV2(g, p) {
		t.Fatal("unused secondary UVs displaced the sampled primary UVs")
	}
}

func TestUnusedVerticesCannotReduceBakeDetail(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for i := range img.Pix {
		img.Pix[i] = uint8(i % 255)
	}
	paths := []string{}
	for _, outlier := range []bool{false, true} {
		g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{}}}}
		for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
			uv2 := uv
			g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv2})
		}
		g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
		if outlier {
			uv := imath.Vector2{100, 100}
			g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv})
		}
		p := bakeProgram{count: 2, blend: 2, shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, images: [4]*image.NRGBA{img, img}}
		result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
		if err := bakeM2Geoset(context.Background(), config.Config{}, &result, g, p); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, result.MDL.Textures[0].WowData.PngPath)
	}
	if paths[0] != paths[1] {
		t.Fatal("unused UV outlier changed the baked pixel content or resolution")
	}
}

func TestBakeRasterRetainsSourceTexelProportions(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2048, 2048))
	p := bakeProgram{count: 2, shader: m2Shader{coords: [4]m2Coord{coordT1M0, coordT2M1}}, images: [4]*image.NRGBA{img, img}}
	w, h := bakeRasterSize(imath.Vector2{.6, 2}, p, true, 512)
	if w != 256 || h != 512 {
		t.Fatalf("wrapped UV strip lost its texel proportions: %dx%d", w, h)
	}
	w, h = bakeRasterSize(imath.Vector2{1, 1}, p, false, 512)
	if w != 512 || h != 512 {
		t.Fatal("square surface density changed")
	}
}
