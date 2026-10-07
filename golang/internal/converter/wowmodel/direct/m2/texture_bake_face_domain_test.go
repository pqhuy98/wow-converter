package directm2

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestM2BakeUsesHealthyUVPerFaceAndPreservesShaderUVs(t *testing.T) {
	primary := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	mask := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for y := range 128 {
		for x := range 128 {
			primary.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 2), A: 255})
			mask.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	g := &components.Geoset{
		Name:     "mixed UV domains",
		Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}, FilterMode: components.BlendBlend}}},
	}
	shared := &components.GeosetVertex{TexPosition: imath.Vector2{.55, .55}, TexPosition2: uvPtr(imath.Vector2{.5, .5})}
	a := &components.GeosetVertex{TexPosition: imath.Vector2{.51, .5}, TexPosition2: uvPtr(imath.Vector2{.1, .5})}
	b := &components.GeosetVertex{TexPosition: imath.Vector2{.5, .51}, TexPosition2: uvPtr(imath.Vector2{.5, .1})}
	c := &components.GeosetVertex{TexPosition: imath.Vector2{.8, .55}, TexPosition2: uvPtr(imath.Vector2{.500001, .5})}
	d := &components.GeosetVertex{TexPosition: imath.Vector2{.55, .8}, TexPosition2: uvPtr(imath.Vector2{.5, .500001})}
	g.Vertices = []*components.GeosetVertex{shared, a, b, c, d}
	g.Faces = []components.Face{
		{Vertices: [3]*components.GeosetVertex{shared, a, b}},
		{Vertices: [3]*components.GeosetVertex{shared, c, d}},
	}
	p := bakeProgram{
		count:  2,
		blend:  2,
		shader: m2Shader{coords: [4]m2Coord{coordT1M0, coordT2M1}},
		images: [4]*image.NRGBA{primary, mask},
		shade: func(pixel bakePixel, _ bakeMoment) m2Fragment {
			sampled := sampleBakeTexture(primary, pixel.uv1, 0)
			return m2Fragment{diffuse: [3]float64{sampled[0], sampled[1], sampled[2]}, alpha: 1}
		},
	}
	if !bakePreferUV2(g, p) {
		t.Fatal("fixture should prefer UV2 overall while its second face prefers UV1")
	}
	if domains := bakeDomainsByFace(g, p, true); domains[0] != true || domains[1] != false {
		t.Fatalf("unexpected per-face UV choices: %v", domains)
	}
	domains := bakeDomainsByFace(g, p, true)
	_, faceCharts, err := buildBakeCharts(g, imath.Vector2{.1, .1}, imath.Vector2{.7, .7}, 128, 128, [2]bool{}, true, nil, bakeChartOptions{domain2ByFace: domains})
	if err != nil {
		t.Fatal(err)
	}
	if faceCharts[0] != faceCharts[1] {
		t.Fatalf("fixture should exercise mixed domains in one chart, got %v", faceCharts)
	}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	if err := bakeM2Geoset(context.Background(), config.Config{}, &result, g, p); err != nil {
		t.Fatal(err)
	}
	rel := g.Material.Layers[0].Texture.WowData.PngPath
	defer texturesource.Unregister(rel)
	source, ok := texturesource.Get(rel)
	if !ok {
		t.Fatalf("bake texture source %q was not registered", rel)
	}
	atlas, err := png.Decode(bytes.NewReader(source.PNG))
	if err != nil {
		t.Fatal(err)
	}
	card := g.Faces[1]
	uv := [3]imath.Vector2{}
	for i, vertex := range card.Vertices {
		uv[i] = vertex.TexPosition
	}
	area := triangleUVArea(uv)
	if area < .01 {
		t.Fatalf("fallback face collapsed in atlas: area %g", area)
	}
	if card.Vertices[0] == g.Faces[0].Vertices[0] {
		t.Fatal("a shared vertex used by different raster domains was incorrectly reused")
	}
	if card.Vertices[0].TexPosition == g.Faces[0].Vertices[0].TexPosition {
		t.Fatal("the shared vertex did not retain its different per-face raster coordinates")
	}
	center := imath.Vector2{(uv[0][0] + uv[1][0] + uv[2][0]) / 3, (uv[0][1] + uv[1][1] + uv[2][1]) / 3}
	x := min(atlas.Bounds().Dx()-1, int(center[0]*float64(atlas.Bounds().Dx())))
	y := min(atlas.Bounds().Dy()-1, int(center[1]*float64(atlas.Bounds().Dy())))
	r, _, _, _ := atlas.At(x, y).RGBA()
	if got := uint8(r >> 8); got < 130 || got > 175 {
		t.Fatalf("card center sampled a wrong UV1 location: red=%d at (%d,%d), center=%v", got, x, y, center)
	}
}

func TestM2BakePlanarFallbackHandlesMissingUV2(t *testing.T) {
	g := &components.Geoset{
		Name:     "missing UV2",
		Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}, FilterMode: components.BlendBlend}}},
	}
	for _, position := range []imath.Vector3{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}} {
		g.Vertices = append(g.Vertices, &components.GeosetVertex{Position: position, TexPosition: imath.Vector2{.3, .3}})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	textureImage := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for i := 3; i < len(textureImage.Pix); i += 4 {
		textureImage.Pix[i] = 255
	}
	checkedSourceUV := false
	p := bakeProgram{
		count:  1,
		blend:  2,
		shader: m2Shader{coords: [4]m2Coord{coordT1M0}},
		images: [4]*image.NRGBA{textureImage},
		shade: func(pixel bakePixel, _ bakeMoment) m2Fragment {
			if !uvNear(pixel.uv1, imath.Vector2{.3, .3}) || !uvNear(pixel.uv2, imath.Vector2{.3, .3}) {
				t.Fatalf("planar raster coordinates replaced source shader UVs: uv1=%v uv2=%v", pixel.uv1, pixel.uv2)
			}
			checkedSourceUV = true
			return m2Fragment{diffuse: [3]float64{1, 1, 1}, alpha: 1}
		},
	}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	if err := bakeM2Geoset(context.Background(), config.Config{}, &result, g, p); err != nil {
		t.Fatal(err)
	}
	if !checkedSourceUV {
		t.Fatal("planar fallback did not sample the original UV channels")
	}
	uv := [3]imath.Vector2{}
	for i, vertex := range g.Faces[0].Vertices {
		uv[i] = vertex.TexPosition
	}
	if triangleUVArea(uv) < .01 || g.Vertices[0].TexPosition2 != nil {
		t.Fatalf("planar fallback did not produce a single atlas UV set: area=%g, UV2=%v", triangleUVArea(uv), g.Vertices[0].TexPosition2)
	}
	defer texturesource.Unregister(g.Material.Layers[0].Texture.WowData.PngPath)
}

func uvPtr(uv imath.Vector2) *imath.Vector2 { return &uv }

func triangleUVArea(uv [3]imath.Vector2) float64 {
	return math.Abs((uv[1][0]-uv[0][0])*(uv[2][1]-uv[0][1])-(uv[1][1]-uv[0][1])*(uv[2][0]-uv[0][0])) / 2
}

func uvNear(a, b imath.Vector2) bool {
	return math.Abs(a[0]-b[0]) < 1e-9 && math.Abs(a[1]-b[1]) < 1e-9
}
