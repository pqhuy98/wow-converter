package directm2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestBakeSamplesCubicMaterialMotionBetweenEqualEndpoints(t *testing.T) {
	for _, tc := range []struct {
		mode     components.Interpolation
		at, want float64
	}{{components.InterpHermite, 250, .09375}, {components.InterpBezier, 500, .75}} {
		a := &components.Animation{Interpolation: tc.mode, KeyFrames: map[int]any{0: 0.0, 1000: 0.0}, InOutTans: map[int]components.InOutTan{0: {OutTan: imath.Vector3{1, 0, 0}}, 1000: {InTan: imath.Vector3{1, 0, 0}}}}
		got := sampleBakeAnimation(a, tc.at, nil)
		if math.Abs(got[0]-tc.want) > 1e-9 || constantBakeTrack(a) {
			t.Fatalf("%s curve lost interior motion: %v", tc.mode, got)
		}
	}
}

func TestSurfaceBudgetPrioritizesBodyAreaAndIgnoresMissingSections(t *testing.T) {
	loader := &m2.Loader{Materials: []m2.MaterialEntry{{BlendingMode: 1}, {BlendingMode: 4}}}
	skin := &m2.Skin{TextureUnits: []m2.SkinTextureUnit{
		{SkinSectionIndex: 0, TextureCount: 2, ShaderID: 0x4011},
		{SkinSectionIndex: 1, TextureCount: 2, ShaderID: 0x4011},
		{SkinSectionIndex: 2, MaterialIndex: 1, TextureCount: 2, ShaderID: 0x4011},
		{SkinSectionIndex: 3, TextureCount: 2, ShaderID: 0x4011},
	}}
	geosets := map[int]*components.Geoset{}
	for i, length := range []float64{3, 1, 20} {
		a, b, c := &components.GeosetVertex{}, &components.GeosetVertex{Position: imath.Vector3{length, 0, 0}}, &components.GeosetVertex{Position: imath.Vector3{0, length, 0}}
		geosets[i] = &components.Geoset{Vertices: []*components.GeosetVertex{a, b, c}, Faces: []components.Face{{Vertices: [3]*components.GeosetVertex{a, b, c}}}}
	}
	got := m2BakeBudgets(loader, skin, geosets)
	if got[3] != 0 || math.Abs(float64(got[0])/float64(got[1])-9) > 1e-4 || got[2] != bakeMaterialPixels/3 {
		t.Fatalf("body/card/overlay allocation is incorrect: %v", got)
	}
	if got[0]+got[1]+got[2] > bakeMaterialPixels*5/3 {
		t.Fatal("surface allocations exceed the model budget")
	}
	for _, g := range geosets {
		for _, v := range g.Vertices {
			for axis := range 3 {
				v.Position[axis] *= 100
			}
		}
	}
	scaled := m2BakeBudgets(loader, skin, geosets)
	for i := range got {
		if got[i] != scaled[i] {
			t.Fatal("export scale changed texture detail allocation")
		}
	}
}

func TestNativeBlendRetainsDetailedColorAndAlphaWithSharedUVLoop(t *testing.T) {
	root := t.TempDir()
	resolved := ResolvedTextures{ValidTextures: map[any]m2export.TextureManifestEntry{}}
	for id, pixel := range map[uint32]color.NRGBA{1: {255, 128, 64, 128}, 2: {200, 80, 40, 128}} {
		img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
		for y := range 2 {
			for x := range 2 {
				img.SetNRGBA(x, y, pixel)
			}
		}
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, img); err != nil {
			t.Fatal(err)
		}
		rel := fmt.Sprintf("native-blend-test-%d.png", id)
		texturesource.Register(rel, texturesource.Source{Kind: texturesource.KindPNG, PNG: encoded.Bytes(), PreserveAlpha: true})
		t.Cleanup(func() { texturesource.Unregister(rel) })
		resolved.ValidTextures[id] = m2export.TextureManifestEntry{MatPath: filepath.Join(root, rel)}
	}
	gs := components.NewGlobalSequence(0, 33000)
	ta := components.TextureAnim{Translation: &components.Animation{Interpolation: components.InterpLinear, GlobalSeq: &gs, KeyFrames: map[int]any{0: imath.Vector3{}, 33000: imath.Vector3{0, 1, 0}}}}
	g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{Alpha: components.AnimatedOrStatic[float64]{Static: true, Value: 1}}}}}
	bone := &components.Bone{}
	g.Matrices = []components.Matrix{{Bones: []*components.Bone{bone}}}
	face := components.Face{}
	for i, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		uv2 := uv
		v := &components.GeosetVertex{Position: imath.Vector3{uv[0], uv[1], 0}, TexPosition: imath.Vector2{.5, .5}, TexPosition2: &uv2, Matrix: &g.Matrices[0]}
		face.Vertices[i] = v
		g.Vertices = append(g.Vertices, v)
	}
	g.Faces = []components.Face{face}
	model := mdl.New(mdl.NewMDLOptions{FormatVersion: 800})
	model.Bones, model.GlobalSequences = []*components.Bone{bone}, []*components.GlobalSequence{&gs}
	model.Geosets, model.TextureAnims = []*components.Geoset{g}, []components.TextureAnim{ta}
	model.GeosetAnims = []components.GeosetAnim{{Geoset: g, Alpha: &components.AnimatedOrStatic[float64]{Static: true, Value: .5}}}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	loader := &m2.Loader{Textures: []m2.TextureEntry{{FileDataID: 1, Flags: 3}, {FileDataID: 2, Flags: 3}}, TextureCombos: []uint16{0, 1}, Materials: []m2.MaterialEntry{{BlendingMode: 2, Flags: 17}}, TextureTransformsLookup: []uint16{65535, 0}, TextureTransforms: []m2.TextureTransformEntry{{}}}
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{{}}, TextureUnits: []m2.SkinTextureUnit{{TextureCount: 2, ShaderID: 0x4011, ColorIndex: 65535}}}
	if err := bakeM2Materials(context.Background(), config.Config{ExportAssetDir: root}, nil, loader, skin, nil, resolved, nil, &result); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for rel := range result.TexturePaths {
			texturesource.Unregister(rel)
		}
	})
	if len(model.Geosets) != 2 || len(model.TextureAnims) != 1 {
		t.Fatal("native color/alpha unnecessarily acquired a flipbook")
	}
	for _, geo := range model.Geosets {
		layer := geo.Material.Layers[0]
		if layer.TVertexAnim == nil || layer.TVertexAnim.Translation.Interpolation != components.InterpLinear || layer.TVertexAnim.Translation.GlobalSeq.Duration != 33000 {
			t.Fatal("native color and alpha did not share the complete UV loop")
		}
		source, ok := texturesource.Get(layer.Texture.WowData.PngPath)
		if !ok {
			t.Fatal("native sample missing")
		}
		img, err := png.Decode(bytes.NewReader(source.PNG))
		if err != nil {
			t.Fatal(err)
		}
		pixel := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
		if layer.FilterMode == components.BlendBlend {
			if pixel != (color.NRGBA{0, 0, 0, 128}) {
				t.Fatal("opacity draw lost the detailed alpha")
			}
		} else if layer.FilterMode != components.BlendAddAlpha || pixel != (color.NRGBA{200, 80, 40, 128}) {
			t.Fatal("radiance draw lost source detail")
		}
		for _, v := range geo.Vertices {
			if v.TexPosition2 != nil {
				t.Fatal("native draw still requires two UV sets")
			}
		}
	}
	if model.ToMdl() == "" {
		t.Fatal("empty Classic serialization")
	}
}

func TestUV2BakeCombinesAlphaAndSplitsConflictingUVIslands(t *testing.T) {
	primary := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	mask := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			primary.SetNRGBA(x, y, color.NRGBA{200, 100, 50, 128})
			mask.SetNRGBA(x, y, color.NRGBA{128, 128, 128, uint8(64 + x*128)})
		}
	}
	newFace := func(maskU float64) components.Face {
		var face components.Face
		for i, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
			uv2 := imath.Vector2{maskU, 0.5}
			face.Vertices[i] = &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv2}
		}
		return face
	}
	g := &components.Geoset{Name: "conflicting cards", Faces: []components.Face{newFace(0.25), newFace(0.75)}}
	for _, face := range g.Faces {
		g.Vertices = append(g.Vertices, face.Vertices[:]...)
	}
	g.Material = &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}, FilterMode: components.BlendBlend, Unlit: true}}}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{FormatVersion: 1000}), TexturePaths: map[string]struct{}{}}
	if err := bakeUV2Geoset(context.Background(), config.Config{AssetPrefix: "wow"}, &result, g, 0x4014, [2]*image.NRGBA{primary, mask}, [2]uint32{}, [2]*components.TextureAnim{}); err != nil {
		t.Fatal(err)
	}
	if !g.Material.Layers[0].Unshaded {
		t.Fatal("WoW unlit did not become WC3 Unshaded")
	}
	rel := g.Material.Layers[0].Texture.WowData.PngPath
	t.Cleanup(func() { texturesource.Unregister(rel) })
	source, ok := texturesource.Get(rel)
	if !ok {
		t.Fatal("baked texture missing")
	}
	img, err := png.Decode(bytes.NewReader(source.PNG))
	if err != nil {
		t.Fatal(err)
	}
	for fi, face := range g.Faces {
		var uv imath.Vector2
		for _, v := range face.Vertices {
			uv[0] += v.TexPosition[0] / 3
			uv[1] += v.TexPosition[1] / 3
			if v.TexPosition2 != nil {
				t.Fatal("baked card still requires UV2")
			}
		}
		x, y := int(uv[0]*float64(img.Bounds().Dx())), int(uv[1]*float64(img.Bounds().Dy()))
		actual := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
		// Mod_Mod2x multiplies straight RGB and alpha independently, then doubles
		// both. This must not turn the RGB into premultiplied color.
		want := [4]float64{200 * 128.0 / 255 * 2, 100 * 128.0 / 255 * 2, 50 * 128.0 / 255 * 2, 128 * float64(64+fi*128) / 255 * 2}
		for channel, value := range []uint8{actual.R, actual.G, actual.B, actual.A} {
			if math.Abs(float64(value)-want[channel]) > 1 {
				t.Fatalf("face %d channel %d: got %d want %.1f", fi, channel, value, want[channel])
			}
		}
	}
	if g.Faces[0].Vertices[0].TexPosition == g.Faces[1].Vertices[0].TexPosition {
		t.Fatal("conflicting UV domains were overlaid")
	}
}

func TestUV2BakePreservesDeferredCharacterTextures(t *testing.T) {
	material := &components.Material{Layers: []components.Layer{{Texture: &components.Texture{Image: "deferred.blp"}}}}
	g := &components.Geoset{Material: material, Faces: []components.Face{{}}}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{FormatVersion: 1000})}
	result.MDL.Geosets = []*components.Geoset{g}
	loader := &m2.Loader{Textures: []m2.TextureEntry{{}, {}}, TextureCombos: []uint16{0, 1}}
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{{}}, TextureUnits: []m2.SkinTextureUnit{{TextureCount: 2, ShaderID: 0x4014}}}
	if err := bakeUV2Materials(context.Background(), config.Config{}, nil, loader, skin, nil, ResolvedTextures{}, &result); err != nil {
		t.Fatal(err)
	}
	if g.Material != material || len(result.MDL.Textures) != 0 {
		t.Fatal("deferred character material was changed")
	}
	gs := components.NewGlobalSequence(0, 70000)
	track := &components.Animation{GlobalSeq: &gs, KeyFrames: map[int]any{0: imath.Vector3{}, 70000: imath.Vector3{1, 0, 0}}}
	if _, err := bakePeriod([2]*components.TextureAnim{{Translation: track}, nil}); !errors.Is(err, errUV2BakeUnsupported) {
		t.Fatalf("unsupported loop should retain legacy conversion: %v", err)
	}
}

func TestUV2BakeTransformAndSampler(t *testing.T) {
	gs := components.NewGlobalSequence(0, 1000)
	anim := &components.TextureAnim{Translation: &components.Animation{GlobalSeq: &gs, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 1000: imath.Vector3{1, 0, 0}}}}
	uv := transformBakeUV(imath.Vector2{0.25, 0.5}, bakeTransform(anim, 500))
	if math.Abs(uv[0]-0.75) > 1e-9 || uv[1] != 0.5 {
		t.Fatalf("midpoint UV %v", uv)
	}
	uv = transformBakeUV(imath.Vector2{0.25, 0.5}, bakeTransform(anim, 1250))
	if math.Abs(uv[0]-0.5) > 1e-9 {
		t.Fatalf("loop UV %v", uv)
	}
	rotation := &components.TextureAnim{Rotation: &components.Animation{GlobalSeq: &gs, Type: components.AnimTypeRotation, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.QuaternionRotation{0, 0, 0, 1}, 1000: imath.QuaternionRotation{0, 0, -1, 0}}}}
	uv = transformBakeUV(imath.Vector2{1, 0.5}, bakeTransform(rotation, 250))
	if math.Abs(uv[0]-(0.5+math.Sqrt(0.125))) > 1e-9 || math.Abs(uv[1]-(0.5-math.Sqrt(0.125))) > 1e-9 {
		t.Fatalf("quarter-turn UV %v", uv)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	img.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	img.SetNRGBA(1, 0, color.NRGBA{0, 0, 255, 0})
	wrapped := sampleBakeTexture(img, imath.Vector2{1, 0.5}, 1)
	clamped := sampleBakeTexture(img, imath.Vector2{1, 0.5}, 0)
	if wrapped != [4]float64{0.5, 0, 0.5, 0.5} || clamped != [4]float64{0, 0, 1, 0} {
		t.Fatalf("straight-alpha sample: wrapped=%v clamped=%v", wrapped, clamped)
	}
}

func TestUV2BakeLongLoopIgnoresConstantScaleTrack(t *testing.T) {
	gs := components.NewGlobalSequence(0, 33333)
	constant := components.NewGlobalSequence(1, 9433)
	anim := &components.TextureAnim{
		Translation: &components.Animation{GlobalSeq: &gs, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 33333: imath.Vector3{0, 1, 0}}},
		Scaling:     &components.Animation{GlobalSeq: &constant, KeyFrames: map[int]any{0: imath.Vector3{1, 1, 0}, 9433: imath.Vector3{1.0000001, 1, 0}}},
	}
	period, err := bakePeriod([2]*components.TextureAnim{anim, nil})
	if err != nil || period != 33333 {
		t.Fatalf("long UV loop: period=%d err=%v", period, err)
	}
}

func TestUV2BakeUsesSurfaceUVWhenPrimaryIsDegenerate(t *testing.T) {
	primary := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	mask := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			primary.SetNRGBA(x, y, color.NRGBA{200, 100, 50, 255})
			mask.SetNRGBA(x, y, color.NRGBA{255, 255, 255, uint8(30 + x*200)})
		}
	}
	g := &components.Geoset{Name: "constant gradient swatch", Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}, FilterMode: components.BlendBlend}}}}
	face := components.Face{}
	for i, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		copyUV := uv
		v := &components.GeosetVertex{TexPosition: imath.Vector2{.3, .3}, TexPosition2: &copyUV}
		face.Vertices[i] = v
		g.Vertices = append(g.Vertices, v)
	}
	g.Faces = []components.Face{face}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	if err := bakeUV2Geoset(context.Background(), config.Config{}, &result, g, 0x4011, [2]*image.NRGBA{primary, mask}, [2]uint32{}, [2]*components.TextureAnim{}); err != nil {
		t.Fatal(err)
	}
	rel := g.Material.Layers[0].Texture.WowData.PngPath
	defer texturesource.Unregister(rel)
	source, _ := texturesource.Get(rel)
	img, err := png.Decode(bytes.NewReader(source.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if g.Vertices[0].TexPosition == g.Vertices[1].TexPosition || g.Vertices[0].TexPosition2 != nil {
		t.Fatal("degenerate primary UV was retained")
	}
	uv0, uv1 := g.Vertices[0].TexPosition, g.Vertices[1].TexPosition
	_, _, _, a0 := img.At(int(uv0[0]*float64(img.Bounds().Dx())), int(uv0[1]*float64(img.Bounds().Dy()))).RGBA()
	_, _, _, a1 := img.At(int(uv1[0]*float64(img.Bounds().Dx())), int(uv1[1]*float64(img.Bounds().Dy()))).RGBA()
	if a1 <= a0 {
		t.Fatalf("surface mask did not survive: alpha %d -> %d", a0, a1)
	}
}

func TestBakePackingCropsSmallIslandsWithoutOverlap(t *testing.T) {
	const tile = 128
	charts := make([]bakeChart, 16)
	for i := range charts {
		charts[i].pixels = make([]bakePixel, tile*tile)
		for y := 50; y < 54; y++ {
			for x := 60; x < 64; x++ {
				charts[i].pixels[y*tile+x] = bakePixel{set: true, covered: true}
			}
		}
	}
	w, h := packBakeCharts(charts, tile, tile)
	if w*h > 4096 {
		t.Fatalf("small islands still reserve full UV tiles: %dx%d", w, h)
	}
	for i, a := range charts {
		if a.minX >= 60 || a.minY >= 50 || a.width <= 4 || a.height <= 4 {
			t.Fatalf("island %d lost filtering padding: %+v", i, a)
		}
		if a.x+a.width > w || a.y+a.height > h {
			t.Fatalf("island %d lies outside packed atlas", i)
		}
		for j := 0; j < i; j++ {
			b := charts[j]
			if a.x < b.x+b.width && b.x < a.x+a.width && a.y < b.y+b.height && b.y < a.y+a.height {
				t.Fatalf("islands %d and %d overlap", i, j)
			}
		}
	}
}

func TestBakeAdjacentTrianglesShareConservativeBoundary(t *testing.T) {
	g := &components.Geoset{}
	for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {1, 1}, {0, 1}} {
		uv2 := imath.Vector2{uv[0]*.7 + .1, uv[1]*.3 + .2}
		g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv2})
	}
	g.Faces = []components.Face{
		{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}},
		{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[2], g.Vertices[3]}},
	}
	charts, _ := buildBakeCharts(g, imath.Vector2{}, imath.Vector2{1, 1}, 32, 32, [2]bool{}, false, nil)
	if len(charts) != 1 {
		t.Fatalf("continuous UVs split at a conservative shared boundary: %d charts", len(charts))
	}
}

func TestLongBakeFitsItsSharedPixelBudget(t *testing.T) {
	for _, options := range []config.TextureBakingOptions{{}, {FPS: 12, WindowMS: 4000}, {FPS: 21, WindowMS: 4000}, {FPS: 30, WindowMS: 4000}, {FPS: 12, WindowMS: 12000}, {FPS: 21, WindowMS: 12000}, {FPS: 30, WindowMS: 12000}} {
		t.Run(fmt.Sprintf("%dfps-%dms", options.FPS, options.WindowMS), func(t *testing.T) {
			g := &components.Geoset{Name: "budgeted effect", Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}, FilterMode: components.BlendBlend}}}}
			for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
				copyUV := uv
				g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv, TexPosition2: &copyUV})
			}
			g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
			img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
			for i := range img.Pix {
				img.Pix[i] = 128
			}
			gs := components.NewGlobalSequence(0, 36000)
			transform := &components.TextureAnim{Translation: &components.Animation{GlobalSeq: &gs, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 36000: imath.Vector3{36, 0, 0}}}}
			p := bakeProgram{shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, blend: 2, count: 2, images: [4]*image.NRGBA{img, img}, transforms: [2]*components.TextureAnim{transform, nil}, pixelBudget: 65536}
			result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
			if err := bakeM2Geoset(context.Background(), config.Config{TextureBaking: options}, &result, g, p); err != nil {
				t.Fatal(err)
			}
			pixels := 0
			for _, tex := range result.MDL.Textures {
				src, _ := texturesource.Get(tex.WowData.PngPath)
				decoded, err := png.Decode(bytes.NewReader(src.PNG))
				if err != nil {
					t.Fatal(err)
				}
				pixels += decoded.Bounds().Dx() * decoded.Bounds().Dy()
				// A long loop must surrender temporal samples before surface detail.
				if density := math.Abs(g.Vertices[1].TexPosition[0]-g.Vertices[0].TexPosition[0]) * float64(decoded.Bounds().Dx()); density < 50 {
					t.Fatalf("budget shrank the effect surface instead of reducing frames: %.1f texels", density)
				}
				texturesource.Unregister(tex.WowData.PngPath)
			}
			if options.FPS == 0 && pixels > p.pixelBudget {
				t.Fatalf("unbounded texture allocation: %d pixels", pixels)
			}
			wantWindow := 4000
			if options.WindowMS > 0 {
				wantWindow = options.WindowMS
			}
			track := g.Material.Layers[0].TVertexAnim.Translation
			if got := track.GlobalSeq.Duration; got != wantWindow {
				t.Fatalf("native long loop retained: %d ms", got)
			}
			if options.FPS > 0 {
				wantFrames := options.FPS * options.WindowMS / 1000
				if got := len(track.KeyFrames); got != wantFrames+1 {
					t.Fatalf("requested FPS lost under pixel budget: %d keys, want %d frames and reset", got, wantFrames)
				}
				for frame := range wantFrames {
					key := int(math.Round(float64(frame) * 1000 / float64(options.FPS)))
					if _, ok := track.KeyFrames[key]; !ok {
						t.Fatalf("missing requested frame at %d ms", key)
					}
				}
			}
		})
	}
}

func TestEnvironmentFactorKeepsDiffuseDetailOutsideTheAtlas(t *testing.T) {
	g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}, FilterMode: components.BlendModulate2x, Unshaded: true}}}}
	for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		copyUV := uv
		g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv, TexPosition2: &copyUV, Normal: imath.Vector3{-1, 0, 0}})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	albedo, env := image.NewNRGBA(image.Rect(0, 0, 1, 1)), image.NewNRGBA(image.Rect(0, 0, 1, 1))
	albedo.SetNRGBA(0, 0, color.NRGBA{204, 153, 102, 128})
	env.SetNRGBA(0, 0, color.NRGBA{51, 102, 153, 255})
	p := bakeProgram{shader: m2Shader{pixel: 12, coords: [4]m2Coord{coordT1M0, coordEnv}}, blend: 6, count: 2, images: [4]*image.NRGBA{albedo, env}, environmentFactor: true}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	if err := bakeM2Geoset(context.Background(), config.Config{}, &result, g, p); err != nil {
		t.Fatal(err)
	}
	tex := g.Material.Layers[0].Texture
	src, _ := texturesource.Get(tex.WowData.PngPath)
	defer texturesource.Unregister(tex.WowData.PngPath)
	decoded, err := png.Decode(bytes.NewReader(src.PNG))
	if err != nil {
		t.Fatal(err)
	}
	uv := imath.Vector2{}
	for _, v := range g.Vertices {
		uv[0] += v.TexPosition[0] / 3
		uv[1] += v.TexPosition[1] / 3
	}
	actual := color.NRGBAModel.Convert(decoded.At(int(uv[0]*float64(decoded.Bounds().Dx())), int(uv[1]*float64(decoded.Bounds().Dy())))).(color.NRGBA)
	want := evaluateM2Combiner(12, [4][4]float64{{.8, .6, .4, 128.0 / 255}, {.2, .4, .6, 1}}, [4]float64{})
	for k, channel := range []uint8{actual.R, actual.G, actual.B} {
		if math.Abs(float64(channel)/255*2*[]float64{.8, .6, .4}[k]-want.diffuse[k]) > 2.0/255 {
			t.Fatalf("factor pass did not reproduce shader: %v %+v", actual, want)
		}
	}
}
