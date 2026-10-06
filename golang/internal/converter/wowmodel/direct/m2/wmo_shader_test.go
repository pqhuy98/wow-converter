package directm2

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

func TestWMOStillBakeRetainsDetailWithoutEmptyEnvironmentPass(t *testing.T) {
	source := &wmo.Loader{Indices: []uint16{0, 1, 2}, Vertices: []float32{0, 0, 0, 1, 0, 0, 0, 1, 0},
		UVs: [][]float32{{0, 1, 1, 1, 0, 0}}, BlendColours: []uint32{0x00ff0000, 0x00ff0000, 0x00ff0000}}
	g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}}}}}
	for i, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		g.Vertices = append(g.Vertices, &components.GeosetVertex{Position: imath.Vector3{float64(i), 0, 0}, TexPosition: uv})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	var images [9]*image.NRGBA
	images[1] = image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for y := range 512 {
		for x := range 512 {
			images[1].SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	result.MDL.Geosets = make([]*components.Geoset, 48) // A building with many batches.
	animate := false
	cfg := config.Config{TextureBaking: config.TextureBakingOptions{Enabled: true, Animate: &animate}}
	if err := BakeWMOMaterial(context.Background(), cfg, &result, g, source, wmo.RenderBatch{NumFaces: 3}, wmo.Material{Shader: 23}, images, [4]float32{}); err != nil {
		t.Fatal(err)
	}
	if len(result.MDL.Textures) != 1 || len(g.Material.Layers) != 1 {
		t.Fatal("missing environment texture generated an empty emissive pass")
	}
	path := g.Material.Layers[0].Texture.WowData.PngPath
	defer texturesource.Unregister(path)
	stored, ok := texturesource.Get(path)
	if !ok {
		t.Fatal("baked image missing")
	}
	size, _, err := image.DecodeConfig(bytes.NewReader(stored.PNG))
	if err != nil || size.Width < 512 || size.Height < 512 {
		t.Fatalf("static surface detail lost to batch count: %dx%d, %v", size.Width, size.Height, err)
	}
}

func TestWMOStillReflectionDoesNotDuplicateDiffuseCharts(t *testing.T) {
	source := &wmo.Loader{Indices: []uint16{0, 1, 2, 3, 4, 5}, Vertices: make([]float32, 18), Normals: make([]float32, 18),
		UVs: [][]float32{{0, 1, 1, 1, 0, 0, 0, 1, 1, 1, 0, 0}}, BlendColours: []uint32{0xff0000, 0xff0000, 0xff0000, 0xff0000, 0xff0000, 0xff0000}}
	g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}}}}}
	for i := range 6 {
		source.Normals[i*3] = 1
		if i >= 3 {
			source.Normals[i*3+1] = 1
		}
		g.Vertices = append(g.Vertices, &components.GeosetVertex{Position: imath.Vector3{float64(i), 0, 0}})
	}
	for i := 0; i < 6; i += 3 {
		g.Faces = append(g.Faces, components.Face{Vertices: [3]*components.GeosetVertex{g.Vertices[i], g.Vertices[i+1], g.Vertices[i+2]}})
	}
	transparent := cloneBakeGeoset(g)
	var images [9]*image.NRGBA
	for _, layer := range []int{0, 1} {
		images[layer] = image.NewNRGBA(image.Rect(0, 0, 32, 32))
		for y := range 32 {
			for x := range 32 {
				images[layer].SetNRGBA(x, y, color.NRGBA{uint8(32 + x*4), uint8(32 + y*4), 80, 255})
			}
		}
	}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	result.MDL.Geosets = []*components.Geoset{g}
	animate := false
	cfg := config.Config{TextureBaking: config.TextureBakingOptions{Enabled: true, Animate: &animate}}
	if err := BakeWMOMaterial(context.Background(), cfg, &result, g, source, wmo.RenderBatch{NumFaces: 6}, wmo.Material{Shader: 23}, images, [4]float32{}); err != nil {
		t.Fatal(err)
	}
	for path := range result.TexturePaths {
		defer texturesource.Unregister(path)
	}
	if len(result.MDL.Geosets) != 2 || len(g.Material.Layers) != 1 {
		t.Fatal("reflection was not separated from diffuse")
	}
	for corner := range 3 {
		if g.Faces[0].Vertices[corner].TexPosition != g.Faces[1].Vertices[corner].TexPosition {
			t.Fatal("identical albedo duplicated because of different normals")
		}
	}
	emission := result.MDL.Geosets[1]
	if emission.Faces[0].Vertices[0].TexPosition == emission.Faces[1].Vertices[0].TexPosition {
		t.Fatal("distinct reflection outputs shared a chart")
	}
	if !emission.Material.Layers[0].Unshaded || !emission.Material.Layers[0].NoDepthSet || emission.Material.Layers[0].FilterMode != components.BlendAddAlpha {
		t.Fatal("reflection pass lost additive unshaded material semantics")
	}
	for fi, face := range emission.Faces {
		for corner, v := range face.Vertices {
			if v.Position != (imath.Vector3{float64(fi*3 + corner), 0, 0}) {
				t.Fatal("independent layout changed geometry")
			}
		}
	}
	// Alpha-dependent blending keeps the original combined coverage/layout.
	other := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	other.MDL.Geosets = []*components.Geoset{transparent}
	if err := BakeWMOMaterial(context.Background(), cfg, &other, transparent, source, wmo.RenderBatch{NumFaces: 6}, wmo.Material{Shader: 23, BlendMode: 2}, images, [4]float32{}); err != nil {
		t.Fatal(err)
	}
	for path := range other.TexturePaths {
		defer texturesource.Unregister(path)
	}
	if len(other.MDL.Geosets) != 2 {
		t.Fatal("transparent shader lost its emissive pass")
	}
	for fi, face := range transparent.Faces {
		for corner, v := range face.Vertices {
			if v.TexPosition != other.MDL.Geosets[1].Faces[fi].Vertices[corner].TexPosition {
				t.Fatal("transparent coverage split into independent layouts")
			}
		}
	}
}

func TestWMOStillRGBComparisonUsesEncodedPixels(t *testing.T) {
	if !sameWMOStillRGB([3]float64{.2, .4, 2}, [3]float64{.20001, .40001, 3}) {
		t.Fatal("identical clamped eight-bit output duplicated")
	}
	if sameWMOStillRGB([3]float64{.2, .4, .6}, [3]float64{.21, .4, .6}) {
		t.Fatal("distinct painted output reused")
	}
}

func TestEveryWMOShaderSelectsTheRendererTable(t *testing.T) {
	want := [24][3]int{{0, 0, 1}, {1, 3, 1}, {2, 3, 1}, {3, 1, 2}, {4, 0, 1}, {5, 1, 2}, {6, 4, 2}, {7, 0, 3}, {8, 6, 2}, {9, 4, 2}, {-1, -1, 2}, {10, 2, 3}, {11, 2, 3}, {12, 4, 2}, {-1, -1, 2}, {13, 4, 2}, {0, 0, 1}, {14, 2, 3}, {15, 7, 3}, {16, 4, 2}, {17, 7, 3}, {18, 0, 1}, {19, 8, 6}, {20, 0, 9}}
	if wmoEffects != want {
		t.Fatal("WMO pixel/vertex/sampler table diverged")
	}
	if _, err := WMOShaderSamplerCount(24); err == nil {
		t.Fatal("unknown shader silently accepted")
	}
}

func TestWMOHeightBlendUsesAllFourUVSetsAndSecondVertexColour(t *testing.T) {
	var images [9]*image.NRGBA
	for i := range images {
		images[i] = image.NewNRGBA(image.Rect(0, 0, 2, 1))
		images[i].SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
		images[i].SetNRGBA(1, 0, color.NRGBA{0, 255, 0, 255})
	}
	v := wmoVarying{uv: [4]imath.Vector2{{.25, .5}, {.75, .5}, {.25, .5}, {.75, .5}}, normal: imath.Vector3{1, 0, 0}}
	// Raw BGRA's red channel selects the first diffuse/height pair at UV1.
	v.color[1] = [4]float64{0, 0, 1, 0}
	a := sampleWMOProgram(wmoEffects[23], v, images, [4]float32{}, 0)
	// With no RGB weights the fourth pair must use UV4, not UV1 or UV2.
	v.color[1] = [4]float64{}
	b := sampleWMOProgram(wmoEffects[23], v, images, [4]float32{}, 0)
	if a.diffuse != [3]float64{1, 0, 0} || b.diffuse != [3]float64{0, 1, 0} {
		t.Fatalf("UV routing/height blend: %+v %+v", a, b)
	}
	v.color[1][3] = .75
	c := sampleWMOProgram(wmoEffects[23], v, images, [4]float32{}, 0)
	if math.Abs(c.diffuse[1]-.25) > 1e-9 {
		t.Fatalf("vertex AO: %+v", c)
	}
}

func TestWMOMaskedEmissiveIsIndependentFromDiffuse(t *testing.T) {
	tx := [9][4]float64{{.8, .6, .4, .25}, {.2, .4, .6, .5}, {.1, .2, .3, .75}}
	f := evaluateWMOCombiner(11, tx, [4]float64{0, 0, 0, .4}, [4]float64{})
	if math.Abs(f.emission[0]-(.8*.25*.2+.1*.75*.4)) > 1e-9 || f.diffuse[0] != .8 {
		t.Fatalf("emissive composition %+v", f)
	}
}

func TestLongUVLoopBakesOnlyFirstFourSeconds(t *testing.T) {
	gs := components.NewGlobalSequence(0, 36000)
	track := &components.Animation{GlobalSeq: &gs, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 36000: imath.Vector3{36, 0, 0}}}
	plan := makeBakeTimeline([2]*components.TextureAnim{{Translation: track}, nil}, [4]components.AnimatedOrStatic[float64]{}, 0, nil)
	if plan.period != 4000 || len(plan.moments) != 32 || plan.keys[4000] != 0 {
		t.Fatalf("unbounded bake: %+v", plan)
	}
	uv := transformBakeUV(imath.Vector2{}, bakeTransformAt(&components.TextureAnim{Translation: track}, plan.moments[31]))
	if math.Abs(uv[0]-3.875) > 1e-9 {
		t.Fatalf("stretched whole source loop rather than truncated: %v", uv)
	}
	for _, fps := range []int{12, 21, 30} {
		for _, window := range []int{4000, 12000} {
			requested := makeBakeTimeline([2]*components.TextureAnim{{Translation: track}, nil}, [4]components.AnimatedOrStatic[float64]{}, 0, nil, config.TextureBakingOptions{FPS: fps, WindowMS: window})
			if requested.period != window || len(requested.moments) != fps*window/1000 || requested.keys[window] != 0 {
				t.Fatalf("requested sampling not honored: %d FPS %d ms: %d frames/%d ms", fps, window, len(requested.moments), requested.period)
			}
			if math.Abs(requested.moments[1].time-1000.0/float64(fps)) > 1e-9 {
				t.Fatal("sample clock does not match FPS")
			}
		}
	}
	reduced := reduceBakeTimeline(plan)
	if len(reduced.moments) != 16 || reduced.keys[4000] != 0 || reduced.keys[3875] != 15 {
		t.Fatalf("downsampled timing: %+v", reduced)
	}
}

func TestLocalBakeWindowRepeatsWithoutSamplingBeyondWindow(t *testing.T) {
	track := &components.Animation{Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 12000: imath.Vector3{12, 0, 0}, 12001: imath.Vector3{20, 0, 0}, 24001: imath.Vector3{32, 0, 0}}}
	seqs := []components.Sequence{{Interval: [2]int{0, 12000}}, {Interval: [2]int{12001, 24001}}}
	plan := makeBakeTimeline([2]*components.TextureAnim{{Translation: track}, nil}, [4]components.AnimatedOrStatic[float64]{}, 0, seqs)
	if plan.keys[0] != plan.keys[4000] || plan.keys[12001] != plan.keys[16001] {
		t.Fatal("local window does not repeat")
	}
	for _, moment := range plan.moments {
		if moment.time-float64(moment.sequence.Interval[0]) >= 4000 {
			t.Fatal("baked past window")
		}
	}
	reduced := reduceBakeTimeline(plan)
	if reduced.keys[12001] == reduced.keys[0] {
		t.Fatal("two local animation states collapsed")
	}
}

func TestWMOVaryingInterpolatesColourAndUV4(t *testing.T) {
	g := &wmo.Loader{Indices: []uint16{0, 1, 2}, Vertices: []float32{0, 0, 0, 1, 0, 0, 0, 1, 0}, UVs: [][]float32{{0, 0, 1, 0, 0, 1}, {0, 0, 1, 0, 0, 1}, {0, 0, 1, 0, 0, 1}, {1, 1, 2, 1, 1, 2}}, VertexColours: [][]uint32{{0xff000000, 0xff000000, 0xff000000}, {0xff0000ff, 0xff0000ff, 0xff0000ff}}}
	v := wmoVaryingAt(g, wmo.RenderBatch{}, 0, [3]float64{.5, .25, .25})
	if v.uv[3] != (imath.Vector2{1.25, -.25}) || v.color[0] != ([4]float64{1, 0, 0, 1}) || v.color[1] != ([4]float64{0, 0, 0, 1}) {
		t.Fatalf("varying: %+v", v)
	}
}

func TestPreparedWMOVaryingMatchesReferenceInterpolation(t *testing.T) {
	group := &wmo.Loader{
		Indices:  []uint16{0, 1, 2, 2, 3, 1},
		Vertices: []float32{0, 0, 0, 2, 0, 0, 0, 3, 1, -1, 1, 2},
		Normals:  []float32{1, 0, 0, 0, 1, 0, 0, 0, 1, 1, 1, 0},
		UVs: [][]float32{
			{0, 1, 1, 1, 0, 0, .25, .75},
			{.1, .2, .8, .1, .2, .9, .4, .6},
			{2, 3, 3, 3, 2, 4},
			{1, 1, 2, 1, 1, 2, 5, 6},
		},
		VertexColours: [][]uint32{{0xff000000, 0xff000000, 0xff000000, 0xff000000}, {0x80112233, 0xfeabcdef, 0x4080ff01, 0x01020304}},
		BlendColours:  []uint32{0x00112233, 0x80abcdef, 0xff800001, 0x7f010203},
	}
	batch := wmo.RenderBatch{FirstFace: 3}
	prepared := prepareWMOVaryings(group, batch, 1)
	for _, bary := range [][3]float64{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}, {.5, .25, .25}, {.125, .375, .5}, {1.0 / 3, 1.0 / 3, 1.0 / 3}} {
		got, want := prepared(0, bary), wmoVaryingAt(group, batch, 0, bary)
		if got != want {
			t.Fatalf("prepared varying differs at barycentric %v:\n got %+v\nwant %+v", bary, got, want)
		}
	}
	// Missing normals, UVs, and colour streams retain the renderer defaults.
	group.Normals = nil
	group.UVs = group.UVs[:1]
	group.VertexColours = nil
	group.BlendColours = nil
	prepared = prepareWMOVaryings(group, batch, 1)
	bary := [3]float64{.2, .3, .5}
	if got, want := prepared(0, bary), wmoVaryingAt(group, batch, 0, bary); got != want {
		t.Fatalf("prepared defaults differ:\n got %+v\nwant %+v", got, want)
	}
}

func TestWMOShaderGenericSamplerLists(t *testing.T) {
	want := [24][]int{
		{0}, {0}, {0}, {0, 1}, {0}, {0, 1}, {0, 1}, {0, 1, 2},
		{0, 1}, {0, 1}, {0, 1}, {0, 1, 2}, {0, 1, 2}, {0, 1}, {0, 1}, {0, 1},
		{0}, {0, 1, 2}, {0, 1, 2}, {0, 1}, {0, 1, 2}, {0}, {0}, {0, 1, 2, 3, 4, 5, 6, 7, 8},
	}
	for shader, effect := range wmoEffects {
		var got []int
		for sampler := range 9 {
			if wmoGenericSamplerUsed(effect, sampler) {
				got = append(got, sampler)
			}
		}
		if len(got) != len(want[shader]) {
			t.Fatalf("shader %d generic samplers: got %v want %v", shader, got, want[shader])
		}
		for i := range got {
			if got[i] != want[shader][i] {
				t.Fatalf("shader %d generic samplers: got %v want %v", shader, got, want[shader])
			}
		}
	}
}

func TestWMOBlendStreamsDoNotUseLightingColours(t *testing.T) {
	g := &wmo.Loader{Indices: []uint16{0, 1, 2}, Vertices: make([]float32, 9),
		VertexColours: [][]uint32{{0xff000000, 0xff000000, 0xff000000}, {0x40000000, 0x80000000, 0xc0000000}},
		BlendColours:  []uint32{0x00ff0000, 0x0000ff00, 0x000000ff}}
	v := wmoVaryingAt(g, wmo.RenderBatch{}, 0, [3]float64{.5, .25, .25})
	if math.Abs(v.color[0][3]-112.0/255) > 1e-9 || v.color[1] != ([4]float64{.25, .25, .5, 0}) {
		t.Fatalf("lighting/two-layer/four-layer streams confused: %+v", v.color)
	}
	var tx [9][4]float64
	tx[1], tx[2], tx[3], tx[4] = [4]float64{1, 0, 0, 1}, [4]float64{0, 1, 0, 1}, [4]float64{0, 0, 1, 1}, [4]float64{0, 1, 0, 1}
	for i := 5; i < 9; i++ {
		tx[i][3] = 1
	}
	f := evaluateWMOCombiner(20, tx, v.color[0], v.color[1])
	if f.diffuse != ([3]float64{4.0 / 7, 3.0 / 14, 3.0 / 14}) {
		t.Fatalf("four-layer blend: %+v", f)
	}
	// Amani has no second MOCV: that must not discard its MOC2 blend weights.
	g.VertexColours = g.VertexColours[:1]
	v = wmoVaryingAt(g, wmo.RenderBatch{}, 0, [3]float64{1, 0, 0})
	if v.color[0] != ([4]float64{0, 0, 0, 1}) || v.color[1] != ([4]float64{0, 0, 1, 0}) {
		t.Fatalf("missing second MOCV: %+v", v.color)
	}
}

func TestWMOHeightBlendRasterUsesActiveUVAndKeepsSourceInputs(t *testing.T) {
	source := &wmo.Loader{Indices: []uint16{0, 1, 2}, Vertices: []float32{0, 0, 0, 1, 0, 0, 0, 1, 0},
		UVs:          [][]float32{{0, 0, 10000, 0, 0, 10000}, {0, 0, 0, 0, 0, 0}, {2, 3, 3, 3, 2, 4}, {0, 0, 0, 0, 0, 0}},
		BlendColours: []uint32{0x000000ff, 0x000000ff, 0x000000ff}}
	var images [9]*image.NRGBA
	for i := range images {
		images[i] = image.NewNRGBA(image.Rect(0, 0, 4, 4))
	}
	var g components.Geoset
	for i := range 3 {
		g.Vertices = append(g.Vertices, &components.GeosetVertex{Position: imath.Vector3{float64(i), 0, 0}, TexPosition: imath.Vector2{0, 0}})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	before := wmoVaryingAt(source, wmo.RenderBatch{}, 0, [3]float64{.5, .25, .25})
	wmoBlendRaster(&g, source, wmo.RenderBatch{}, 20, images)
	want := [3]imath.Vector2{{0, 1}, {1, 1}, {0, 0}}
	for i, v := range g.Faces[0].Vertices {
		if v.TexPosition != want[i] || v.TexPosition2 != nil || v.Position[0] != float64(i) {
			t.Fatalf("raster corner %d: %+v", i, v)
		}
	}
	if after := wmoVaryingAt(source, wmo.RenderBatch{}, 0, [3]float64{.5, .25, .25}); after != before {
		t.Fatal("raster remap changed source shader inputs")
	}
	// An active primary unwrap retains its relative coordinates even when
	// another layer covers more texels. The whole island drops unused tile gaps.
	source.UVs[0] = []float32{2, 3, 3, 3, 2, 4}
	source.UVs[2] = []float32{0, 0, 10, 0, 0, 10}
	source.BlendColours = []uint32{0x0080007f, 0x0080007f, 0x0080007f}
	wmoBlendRaster(&g, source, wmo.RenderBatch{}, 20, images)
	for i, v := range g.Faces[0].Vertices {
		if v.TexPosition != ([3]imath.Vector2{{0, 1}, {1, 1}, {0, 0}})[i] {
			t.Fatalf("active primary unwrap split: %v", v.TexPosition)
		}
	}
}

func TestWMOTwoLayerRasterIgnoresMaskedPlaceholderUVs(t *testing.T) {
	source := &wmo.Loader{Indices: []uint16{0, 1, 2}, Vertices: make([]float32, 9), UVs: [][]float32{{-400, 4000, -300, 3900, -350, 4100}, {0, 1, 1, 1, 0, 0}}, VertexColours: [][]uint32{{0xff000000, 0xff000000, 0xff000000}, {0, 0, 0}}}
	var images [9]*image.NRGBA
	for i := range images {
		images[i] = image.NewNRGBA(image.Rect(0, 0, 4, 4))
	}
	for _, pixel := range []int{8, 12, 13} {
		g := &components.Geoset{Faces: []components.Face{{Vertices: [3]*components.GeosetVertex{{}, {}, {}}}}}
		wmoBlendRaster(g, source, wmo.RenderBatch{}, pixel, images)
		for i, v := range g.Faces[0].Vertices {
			if v.TexPosition != ([3]imath.Vector2{{0, 0}, {1, 0}, {0, 1}})[i] {
				t.Fatalf("combiner %d corner %d: %v", pixel, i, v.TexPosition)
			}
		}
	}
}

func TestWMORasterRebasesDisconnectedTilesWithoutBreakingSharedEdges(t *testing.T) {
	source := &wmo.Loader{
		Indices:      []uint16{0, 1, 2, 1, 3, 2, 4, 5, 6},
		Vertices:     make([]float32, 7*3),
		UVs:          [][]float32{{100, 200, 101, 200, 100, 201, 101, 201, -300, -400, -299, -400, -300, -399}},
		BlendColours: []uint32{0xff0000, 0xff0000, 0xff0000, 0xff0000, 0xff0000, 0xff0000, 0xff0000},
	}
	var images [9]*image.NRGBA
	for i := range images {
		images[i] = image.NewNRGBA(image.Rect(0, 0, 4, 4))
	}
	g := &components.Geoset{}
	for i := range 7 {
		g.Vertices = append(g.Vertices, &components.GeosetVertex{Position: imath.Vector3{float64(i), 0, 0}})
	}
	for i := 0; i < len(source.Indices); i += 3 {
		g.Faces = append(g.Faces, components.Face{Vertices: [3]*components.GeosetVertex{
			g.Vertices[source.Indices[i]], g.Vertices[source.Indices[i+1]], g.Vertices[source.Indices[i+2]],
		}})
	}
	before := wmoVaryingAt(source, wmo.RenderBatch{}, 2, [3]float64{.5, .25, .25})
	wmoBlendRaster(g, source, wmo.RenderBatch{}, 20, images)
	if g.Faces[0].Vertices[1] != g.Faces[1].Vertices[0] || g.Faces[0].Vertices[2] != g.Faces[1].Vertices[2] {
		t.Fatal("connected tile lost its shared edge")
	}
	for _, fi := range []int{0, 2} {
		for corner, want := range [3]imath.Vector2{{0, 1}, {1, 1}, {0, 0}} {
			if got := g.Faces[fi].Vertices[corner].TexPosition; got != want {
				t.Fatalf("tile %d corner %d: got %v, want %v", fi, corner, got, want)
			}
		}
	}
	if after := wmoVaryingAt(source, wmo.RenderBatch{}, 2, [3]float64{.5, .25, .25}); after != before {
		t.Fatal("rebasing changed original shader samples")
	}
}

func TestWMORasterTiledOutlierDoesNotShrinkOtherIslands(t *testing.T) {
	source := &wmo.Loader{
		Indices: []uint16{0, 1, 2, 3, 4, 5}, Vertices: make([]float32, 18),
		UVs:          [][]float32{{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}, {100, 200, 200, 200, 100, 250, 2, 3, 3, 3, 2, 4}},
		BlendColours: []uint32{0xff00, 0xff00, 0xff00, 0xff00, 0xff00, 0xff00},
	}
	var images [9]*image.NRGBA
	for i := range images {
		images[i] = image.NewNRGBA(image.Rect(0, 0, 32, 32))
	}
	g := &components.Geoset{}
	for range 6 {
		g.Vertices = append(g.Vertices, &components.GeosetVertex{})
	}
	for i := 0; i < 6; i += 3 {
		g.Faces = append(g.Faces, components.Face{Vertices: [3]*components.GeosetVertex{g.Vertices[i], g.Vertices[i+1], g.Vertices[i+2]}})
	}
	before := wmoVaryingAt(source, wmo.RenderBatch{}, 0, [3]float64{.25, .5, .25})
	wmoBlendRaster(g, source, wmo.RenderBatch{}, 20, images)
	for fi, want := range [][3]imath.Vector2{{{0, .5}, {1, .5}, {0, 0}}, {{0, 1}, {1, 1}, {0, 0}}} {
		for corner, v := range g.Faces[fi].Vertices {
			if v.TexPosition != want[corner] {
				t.Fatalf("island %d corner %d: %v, want %v", fi, corner, v.TexPosition, want[corner])
			}
		}
	}
	if after := wmoVaryingAt(source, wmo.RenderBatch{}, 0, [3]float64{.25, .5, .25}); after != before {
		t.Fatal("bounding the raster lost the shader's original texture repeats")
	}
}

func TestWMOSecondaryRasterRetainsEdgeAcrossTileBoundary(t *testing.T) {
	source := &wmo.Loader{
		Indices: []uint16{0, 1, 2, 1, 3, 2}, Vertices: make([]float32, 12),
		UVs:          [][]float32{{0, 0, 0, 0, 0, 0, 0, 0}, {.5, 0, 1.5, 0, 1, 1, 2, 1}},
		BlendColours: []uint32{0xff00, 0xff00, 0xff00, 0xff00},
	}
	var images [9]*image.NRGBA
	for i := range images {
		images[i] = image.NewNRGBA(image.Rect(0, 0, 32, 32))
	}
	g := &components.Geoset{}
	for range 4 {
		g.Vertices = append(g.Vertices, &components.GeosetVertex{})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}, {Vertices: [3]*components.GeosetVertex{g.Vertices[1], g.Vertices[3], g.Vertices[2]}}}
	wmoBlendRaster(g, source, wmo.RenderBatch{}, 20, images)
	if g.Faces[0].Vertices[1] != g.Faces[1].Vertices[0] || g.Faces[0].Vertices[2] != g.Faces[1].Vertices[2] || len(g.Vertices) != 4 {
		t.Fatal("per-triangle integer rebasing broke a shared secondary-UV edge")
	}
}

func TestWMOStillBakeSharesMaskedInputsButRetainsDistinctPaint(t *testing.T) {
	for _, tc := range []struct{ animate, distinct bool }{{false, false}, {false, true}, {true, false}} {
		t.Run(fmt.Sprintf("animated=%v/distinct=%v", tc.animate, tc.distinct), func(t *testing.T) {
			source := &wmo.Loader{Indices: []uint16{0, 1, 2, 3, 4, 5}, Vertices: make([]float32, 18), Normals: make([]float32, 18),
				UVs:          [][]float32{{0, 1, 1, 1, 0, 0, 0, 1, 1, 1, 0, 0}, {0, 0, 1, 0, 0, 1, 20, 30, 21, 30, 20, 31}},
				BlendColours: []uint32{0xff0000, 0xff0000, 0xff0000, 0xff0000, 0xff0000, 0xff0000}}
			if tc.distinct {
				for i := 3; i < 6; i++ {
					source.BlendColours[i] |= 0x80000000 // Different AO changes the painted result.
				}
			}
			g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}}}}}
			for i := range 6 {
				source.Normals[i*3+i/3] = 1 // The normal differs on the two surfaces.
				g.Vertices = append(g.Vertices, &components.GeosetVertex{})
			}
			for i := 0; i < 6; i += 3 {
				g.Faces = append(g.Faces, components.Face{Vertices: [3]*components.GeosetVertex{g.Vertices[i], g.Vertices[i+1], g.Vertices[i+2]}})
			}
			var images [9]*image.NRGBA
			images[1] = image.NewNRGBA(image.Rect(0, 0, 32, 32))
			for y := range 32 {
				for x := range 32 {
					images[1].SetNRGBA(x, y, color.NRGBA{uint8(32 + x*4), uint8(32 + y*4), 80, 0})
				}
			}
			animate := tc.animate
			result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
			cfg := config.Config{TextureBaking: config.TextureBakingOptions{Enabled: true, Animate: &animate}}
			if err := BakeWMOMaterial(context.Background(), cfg, &result, g, source, wmo.RenderBatch{NumFaces: 6}, wmo.Material{Shader: 23}, images, [4]float32{}); err != nil {
				t.Fatal(err)
			}
			defer func() {
				for _, tex := range result.MDL.Textures {
					texturesource.Unregister(tex.WowData.PngPath)
				}
			}()
			shared := g.Faces[0].Vertices[0].TexPosition == g.Faces[1].Vertices[0].TexPosition
			if shared != (!tc.distinct && !tc.animate) {
				t.Fatalf("sharing=%v with animated=%v, distinct painted output=%v", shared, tc.animate, tc.distinct)
			}
		})
	}
}
