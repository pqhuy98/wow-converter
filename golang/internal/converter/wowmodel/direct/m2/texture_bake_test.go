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
	"math/rand"
	"path/filepath"
	"runtime"
	"strings"
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

func TestBakeSkinGeosetMapSkipsEmptyCheckedSections(t *testing.T) {
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{
		{SubmeshID: 2601, TriangleCount: 0},
		{SubmeshID: 2601, TriangleCount: 3},
	}}
	mask := []m2export.GeosetMaskEntry{{ID: 2601, Checked: true}, {ID: 2601, Checked: true}}
	real := &components.Geoset{Name: "shoulders"}
	mapped, _ := mapBakeSkinGeosets(skin, mask, []*components.Geoset{real})
	if mapped[0] != nil {
		t.Fatal("empty placeholder consumed the assembled geoset")
	}
	if mapped[1] != real {
		t.Fatal("real section was not mapped to the assembled geoset")
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
	resolved := ResolvedTextures{ValidTextures: textureManifest{}}
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
		resolved.ValidTextures[textureKey{fileDataID: id}] = m2export.TextureManifestEntry{MatPath: filepath.Join(root, rel)}
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
	skin := &m2.Skin{SubMeshes: []m2.SkinSubMesh{{TriangleCount: 3}}, TextureUnits: []m2.SkinTextureUnit{{TextureCount: 2, ShaderID: 0x4011, ColorIndex: 65535}}}
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

func TestUV2BakeUsesTransparentBlackForUnboundOptionalReplaceable(t *testing.T) {
	root := t.TempDir()
	var vertices [3]*components.GeosetVertex
	for i, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		vertices[i] = &components.GeosetVertex{TexPosition: uv}
	}
	geoset := &components.Geoset{
		Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{Image: "wrong-fallback.blp"}}}},
		Vertices: vertices[:],
		Faces:    []components.Face{{Vertices: vertices}},
	}
	model := mdl.New(mdl.NewMDLOptions{FormatVersion: 1000})
	model.Geosets = []*components.Geoset{geoset}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	loader := &m2.Loader{
		Textures:      []m2.TextureEntry{{}},
		TextureTypes:  []uint32{4},
		TextureCombos: []uint16{0},
		Materials:     []m2.MaterialEntry{{BlendingMode: 0}},
	}
	skin := &m2.Skin{
		SubMeshes:    []m2.SkinSubMesh{{TriangleCount: 3}},
		TextureUnits: []m2.SkinTextureUnit{{TextureCount: 1, ShaderID: 0x8022, ColorIndex: 65535}},
	}
	if err := bakeM2Materials(context.Background(), config.Config{ExportAssetDir: root}, nil, loader, skin, nil, ResolvedTextures{ValidTextures: textureManifest{}}, nil, &result); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for rel := range result.TexturePaths {
			texturesource.Unregister(rel)
		}
	})
	texture := geoset.Material.Layers[0].Texture
	if texture == nil || texture.WowData.PngPath == "" {
		t.Fatal("unbound optional texture retained the original material instead of being baked")
	}
	source, ok := texturesource.Get(texture.WowData.PngPath)
	if !ok {
		t.Fatal("transparent-black replacement was not registered")
	}
	img, err := png.Decode(bytes.NewReader(source.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); got != (color.NRGBA{}) {
		t.Fatalf("unbound replaceable sampled %v, want transparent black", got)
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
		charts[i].pixels = map[int]bakePixel{}
		for y := 50; y < 54; y++ {
			for x := 60; x < 64; x++ {
				charts[i].pixels[y*tile+x] = bakePixel{set: true, covered: true}
			}
		}
	}
	w, h, err := packBakeCharts(charts, tile, tile)
	if err != nil {
		t.Fatal(err)
	}
	if w*h > 4096 {
		t.Fatalf("small islands still reserve full UV tiles: %dx%d", w, h)
	}
	occupied := map[int]int{}
	for i, a := range charts {
		if a.minX >= 60 || a.minY >= 50 || a.width <= 4 || a.height <= 4 {
			t.Fatalf("island %d lost filtering padding: %+v", i, a)
		}
		if a.x+a.width > w || a.y+a.height > h {
			t.Fatalf("island %d lies outside packed atlas", i)
		}
		for _, pixel := range a.active {
			x, y := a.x+pixel%tile-a.minX, a.y+pixel/tile-a.minY
			index := y*w + x
			if other, exists := occupied[index]; exists {
				t.Fatalf("islands %d and %d share a padded texel", i, other)
			}
			occupied[index] = i
		}
	}
}

func TestBakePackingInterlocksTrianglesWithoutSharingPadding(t *testing.T) {
	const raster = 128
	charts := make([]bakeChart, 8)
	for ci := range charts {
		charts[ci].pixels = map[int]bakePixel{}
		for y := range 48 {
			for x := range 48 {
				if (x+y < 48) == (ci%2 == 0) {
					charts[ci].pixels[(40+y)*raster+40+x] = bakePixel{set: true, covered: true}
				}
			}
		}
	}
	w, h, err := packBakeCharts(charts, raster, raster)
	if err != nil {
		t.Fatal(err)
	}
	if w*h > 32768 {
		t.Fatalf("triangles still reserve full rectangle shelves: %dx%d", w, h)
	}
	used := map[int]int{}
	for ci, chart := range charts {
		for _, pixel := range chart.active {
			x, y := chart.x+pixel%raster-chart.minX, chart.y+pixel/raster-chart.minY
			if x < 0 || y < 0 || x >= w || y >= h {
				t.Fatal("padded texel outside atlas")
			}
			index := y*w + x
			if previous, ok := used[index]; ok {
				t.Fatalf("triangles %d and %d share a filtering texel", ci, previous)
			}
			used[index] = ci
		}
	}
}

func TestBakePackingSeparatesDistantCompatibleIslands(t *testing.T) {
	g := &components.Geoset{}
	for _, offset := range []float64{.05, .85} {
		face := components.Face{}
		for k, uv := range []imath.Vector2{{offset, offset}, {offset + .03, offset}, {offset, offset + .03}} {
			maskUV := imath.Vector2{}
			v := &components.GeosetVertex{TexPosition: uv, TexPosition2: &maskUV}
			face.Vertices[k] = v
			g.Vertices = append(g.Vertices, v)
		}
		g.Faces = append(g.Faces, face)
	}
	charts, faces, err := buildBakeCharts(g, imath.Vector2{}, imath.Vector2{1, 1}, 128, 128, [2]bool{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	w, h, err := packBakeCharts(charts, 128, 128)
	if err != nil {
		t.Fatal(err)
	}
	if w*h > 1024 || faces[0] == faces[1] {
		t.Fatalf("distant islands still reserve their empty bounding rectangle: %dx%d, faces %v", w, h, faces)
	}
	for fi, ci := range faces {
		for _, sample := range charts[ci].pixels {
			if sample.face != fi {
				t.Fatalf("face %d mapped to another island's samples", fi)
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
	charts, _, err := buildBakeCharts(g, imath.Vector2{}, imath.Vector2{1, 1}, 32, 32, [2]bool{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(charts) != 1 {
		t.Fatalf("continuous UVs split at a conservative shared boundary: %d charts", len(charts))
	}
}

func TestBakeConflictingSubpixelIslandsDoNotShareConservativeSamples(t *testing.T) {
	g := &components.Geoset{}
	for i := range 2 {
		face := components.Face{}
		for k, uv := range []imath.Vector2{{.01, .01}, {.02, .01}, {.01, .02}} {
			uv2 := imath.Vector2{float64(i), 0}
			v := &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv2}
			face.Vertices[k] = v
			g.Vertices = append(g.Vertices, v)
		}
		g.Faces = append(g.Faces, face)
	}
	charts, faces, err := buildBakeCharts(g, imath.Vector2{}, imath.Vector2{1, 1}, 16, 16, [2]bool{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(charts) != 2 || faces[0] == faces[1] {
		t.Fatalf("conflicting subpixel surfaces share samples: %d charts / %v", len(charts), faces)
	}
	for _, chart := range charts {
		for _, pixel := range chart.pixels {
			if pixel.covered {
				t.Fatal("fixture unexpectedly contains a covered texel centre")
			}
		}
	}
}

func TestBakeOverlappingSmallIslandsUseSparseWorkingMemory(t *testing.T) {
	g := &components.Geoset{}
	for i := range 32 {
		face := components.Face{}
		for k, uv := range []imath.Vector2{{.5, .5}, {.51, .5}, {.5, .51}} {
			uv2 := imath.Vector2{float64(i), 0}
			v := &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv2}
			face.Vertices[k] = v
			g.Vertices = append(g.Vertices, v)
		}
		g.Faces = append(g.Faces, face)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	charts, faces, err := buildBakeCharts(g, imath.Vector2{}, imath.Vector2{1, 1}, 1024, 1024, [2]bool{}, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	w, h, err := packBakeCharts(charts, 1024, 1024)
	if err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	if len(charts) != 32 || len(faces) != 32 || w*h > 32768 {
		t.Fatalf("small conflicting islands lost their charts or compact packing: %d charts, %dx%d", len(charts), w, h)
	}
	// Dense storage used ~2.75 GiB for these 32 tiny islands alone.
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8*1024*1024 {
		t.Fatalf("small islands allocated %d bytes of temporary samples", allocated)
	} else {
		t.Logf("32 conflicting islands on a 1024x1024 domain: %d allocated bytes", allocated)
	}
}

func TestBakeSparsePaddingPreservesSamplesAndNeighborPriority(t *testing.T) {
	pixels := map[int]bakePixel{
		3*8 + 2: {set: true, uv1: imath.Vector2{1, 0}},
		3*8 + 4: {set: true, uv1: imath.Vector2{2, 0}},
	}
	if err := padBakeChart(pixels, 8, 8, 64); err != nil {
		t.Fatal(err)
	}
	if pixels[3*8+3].uv1[0] != 1 || pixels[3*8+4].uv1[0] != 2 || pixels[0*8+2].uv1[0] != 1 {
		t.Fatal("padding changed a covered sample, neighbor priority, or three-pixel halo")
	}
}

func TestBakeSparsePaddingMatchesDenseWaves(t *testing.T) {
	const w, h = 32, 32
	rng := rand.New(rand.NewSource(0x5eed))
	hole := [][2]int{{14, 14}, {14, 15}, {15, 14}, {15, 15}}
	manhattan := func(a, b [2]int) int { return int(math.Abs(float64(a[0]-b[0])) + math.Abs(float64(a[1]-b[1]))) }
	for pattern := range 10 {
		points := [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}}
		for attempts := 0; len(points) < 7+pattern%4 && attempts < 5000; attempts++ {
			candidate := [2]int{rng.Intn(w), rng.Intn(h)}
			valid := true
			for _, point := range points {
				if manhattan(candidate, point) <= 2*bakePadding {
					valid = false
					break
				}
			}
			for _, cell := range hole {
				if manhattan(candidate, cell) <= bakePadding {
					valid = false
					break
				}
			}
			// Keep the one-pixel-beyond-the-halo check at the upper-left seed
			// empty, even if later random islands are added nearby.
			if manhattan(candidate, [2]int{4, 0}) <= bakePadding {
				valid = false
			}
			if valid {
				points = append(points, candidate)
			}
		}
		if len(points) < 7+pattern%4 {
			t.Fatalf("pattern %d did not generate enough separated islands", pattern)
		}

		original := make(map[int]bakePixel, len(points))
		for face, point := range points {
			value := float64(face + 1)
			original[point[1]*w+point[0]] = bakePixel{
				uv1:      imath.Vector2{value + .1, value + .2},
				uv2:      imath.Vector2{value + .3, value + .4},
				env:      imath.Vector2{value + .5, value + .6},
				set:      true,
				covered:  face%2 == 0,
				prepared: face%2 != 0,
				face:     face,
				bary:     [3]float64{value + .7, value + .8, value + .9},
				color:    [6]uint8{uint8(face + 1), uint8(face + 2), uint8(face + 3), uint8(face + 4), uint8(face + 5), uint8(face + 6)},
			}
		}
		clone := func(source map[int]bakePixel) map[int]bakePixel {
			copy := make(map[int]bakePixel, len(source))
			for index, pixel := range source {
				copy[index] = pixel
			}
			return copy
		}
		got, want := clone(original), clone(original)
		if err := padBakeChart(got, w, h, w*h); err != nil {
			t.Fatalf("pattern %d sparse padding: %v", pattern, err)
		}
		if err := densePadBakeChartReference(want, w, h, w*h); err != nil {
			t.Fatalf("pattern %d dense padding: %v", pattern, err)
		}
		if len(got) != len(want) {
			t.Fatalf("pattern %d has %d samples, dense reference has %d", pattern, len(got), len(want))
		}
		for index, expected := range want {
			if actual, ok := got[index]; !ok || actual != expected {
				t.Fatalf("pattern %d sample %d differs: got %+v, want %+v", pattern, index, actual, expected)
			}
		}
		for _, point := range hole {
			if got[point[1]*w+point[0]].set {
				t.Fatalf("pattern %d padding filled the central hole at %v", pattern, point)
			}
		}
		if edge := got[3*w+0]; !edge.set || edge.face != original[0].face {
			t.Fatalf("pattern %d did not extend the edge island through wave three: %+v", pattern, edge)
		}
		if got[0+4*w].set {
			t.Fatalf("pattern %d extended the edge island beyond three waves", pattern)
		}
		for index, pixel := range original {
			if got[index] != pixel {
				t.Fatalf("pattern %d changed original sample %d", pattern, index)
			}
		}
	}
}

func densePadBakeChartReference(pixels map[int]bakePixel, w, h, limit int) error {
	directions := [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	for range bakePadding {
		next := map[int]bakePixel{}
		for pi := range pixels {
			x, y := pi%w, pi/w
			for _, direction := range directions {
				x2, y2 := x+direction[0], y+direction[1]
				index := y2*w + x2
				if x2 < 0 || x2 >= w || y2 < 0 || y2 >= h || pixels[index].set || next[index].set {
					continue
				}
				if len(pixels)+len(next) == limit {
					return errBakeChartLimit
				}
				for _, neighbor := range directions {
					nx, ny := x2+neighbor[0], y2+neighbor[1]
					if nx >= 0 && nx < w && ny >= 0 && ny < h {
						if sample := pixels[ny*w+nx]; sample.set {
							next[index] = sample
							break
						}
					}
				}
			}
		}
		for index, pixel := range next {
			pixels[index] = pixel
		}
	}
	return nil
}

func TestBakeChartPrepareIsLazyAndReusedForOverlaps(t *testing.T) {
	makeGeoset := func(rasterOffsets, sourceOffsets []float64) *components.Geoset {
		g := &components.Geoset{}
		for faceIndex, sourceOffset := range sourceOffsets {
			face := components.Face{}
			for k, uv := range []imath.Vector2{{.1, .1}, {.4, .1}, {.1, .4}} {
				rasterUV := imath.Vector2{uv[0] + rasterOffsets[faceIndex], uv[1] + rasterOffsets[faceIndex]}
				maskUV := imath.Vector2{sourceOffset + uv[0], sourceOffset + uv[1]}
				vertex := &components.GeosetVertex{TexPosition: rasterUV, TexPosition2: &maskUV}
				face.Vertices[k] = vertex
				g.Vertices = append(g.Vertices, vertex)
			}
			g.Faces = append(g.Faces, face)
		}
		return g
	}
	var prepared int
	options := bakeChartOptions{
		prepare: func(pixel *bakePixel) {
			prepared++
			pixel.color[0] = uint8(math.Round(pixel.uv2[0] * 255))
		},
		conflict: func(a, b bakePixel) bool { return a.color != b.color },
	}
	build := func(g *components.Geoset) ([]bakeChart, error) {
		charts, _, err := buildBakeCharts(g, imath.Vector2{}, imath.Vector2{1, 1}, 64, 64, [2]bool{}, false, nil, options)
		return charts, err
	}

	if _, err := build(makeGeoset([]float64{0, .55}, []float64{.1, .7})); err != nil {
		t.Fatal(err)
	}
	if prepared != 0 {
		t.Fatalf("nonoverlapping faces were prepared %d times", prepared)
	}

	prepared = 0
	charts, err := build(makeGeoset([]float64{0, 0, 0}, []float64{.1, .1, .1}))
	if err != nil {
		t.Fatal(err)
	}
	if len(charts) != 1 || prepared == 0 {
		t.Fatalf("compatible overlaps used %d charts and prepared %d samples", len(charts), prepared)
	}
	firstOverlapPass := prepared
	prepared = 0
	if _, err := build(makeGeoset([]float64{0, 0}, []float64{.1, .1})); err != nil {
		t.Fatal(err)
	}
	if firstOverlapPass*2 != prepared*3 {
		t.Fatalf("cached old samples were not reused: first pass prepared %d, next pass prepared %d", firstOverlapPass, prepared)
	}
}

func TestBakeSparsePaddingStopsBeforeExceedingWorkingLimit(t *testing.T) {
	pixels := map[int]bakePixel{4*8 + 4: {set: true}}
	if err := padBakeChart(pixels, 8, 8, 3); !errors.Is(err, errBakeChartLimit) {
		t.Fatalf("expected working-memory limit, got %v", err)
	}
	if len(pixels) > 3 {
		t.Fatal("padding exceeded its sample budget")
	}
}

func TestBakeChartsStopsAtWorkingLimitBeforePacking(t *testing.T) {
	g := &components.Geoset{}
	for i := range 9 {
		face := components.Face{}
		for k, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
			uv2 := imath.Vector2{float64(i), 0}
			face.Vertices[k] = &components.GeosetVertex{TexPosition: uv, TexPosition2: &uv2}
		}
		g.Faces = append(g.Faces, face)
	}
	charts, faces, err := buildBakeCharts(g, imath.Vector2{}, imath.Vector2{1, 1}, 512, 512, [2]bool{}, false, nil)
	if !errors.Is(err, errBakeChartLimit) || charts != nil || faces != nil {
		t.Fatalf("oversized chart workspace must abort before packing: %d charts, %d faces, %v", len(charts), len(faces), err)
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

func TestBakeBudgetReductionRetainsSurfaceTexelProportions(t *testing.T) {
	g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}}}}}
	for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	img := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	program := bakeProgram{shader: m2Shader{coords: [4]m2Coord{coordT1}}, count: 1, images: [4]*image.NRGBA{img}, pixelBudget: 32768}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	if err := bakeM2Geoset(context.Background(), config.Config{}, &result, g, program); err != nil {
		t.Fatal(err)
	}
	path := g.Material.Layers[0].Texture.WowData.PngPath
	defer texturesource.Unregister(path)
	source, _ := texturesource.Get(path)
	size, _, err := image.DecodeConfig(bytes.NewReader(source.PNG))
	if err != nil {
		t.Fatal(err)
	}
	x := (g.Faces[0].Vertices[1].TexPosition[0] - g.Faces[0].Vertices[0].TexPosition[0]) * float64(size.Width)
	y := (g.Faces[0].Vertices[2].TexPosition[1] - g.Faces[0].Vertices[0].TexPosition[1]) * float64(size.Height)
	if math.Abs(x/y-1) > .02 {
		t.Fatalf("budget distorted square source texels: %.1f by %.1f", x, y)
	}
}

func TestStillBakeUsesFirstFrameOnly(t *testing.T) {
	animate := false
	g := &components.Geoset{Name: "still effect", Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}, FilterMode: components.BlendBlend}}}}
	for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		copyUV := uv
		g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv, TexPosition2: &copyUV})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = 128
	}
	gs := components.NewGlobalSequence(0, 36000)
	transform := &components.TextureAnim{Translation: &components.Animation{GlobalSeq: &gs, Interpolation: components.InterpLinear, KeyFrames: map[int]any{0: imath.Vector3{}, 36000: imath.Vector3{36, 0, 0}}}}
	p := bakeProgram{shader: m2Shader{pixel: 6, coords: [4]m2Coord{coordT1M0, coordT2M1}}, blend: 2, count: 2, images: [4]*image.NRGBA{img, img}, transforms: [2]*components.TextureAnim{transform, nil}, pixelBudget: 65536}
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	options := config.TextureBakingOptions{Enabled: true, Animate: &animate, FPS: 30, WindowMS: 12000}
	if err := bakeM2Geoset(context.Background(), config.Config{TextureBaking: options}, &result, g, p); err != nil {
		t.Fatal(err)
	}
	if g.Material.Layers[0].TVertexAnim != nil {
		t.Fatal("still bake attached a flipbook")
	}
	if len(result.MDL.Textures) != 1 {
		t.Fatalf("still bake pages: %d", len(result.MDL.Textures))
	}
	for _, tex := range result.MDL.Textures {
		texturesource.Unregister(tex.WowData.PngPath)
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

func TestRegisterBakeTextureUsesListfileStemAnd12Hex(t *testing.T) {
	if got := BakeStemFromListfile(`creature/firehawk/Alysrazor.m2`); got != "alysrazor" {
		t.Fatalf("stem %q", got)
	}
	atlas := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	result := ConvertResult{
		MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{},
		BakeStem: BakeStemFromListfile(`creature/firehawk/Alysrazor.m2`),
	}
	tex, err := registerBakeTexture(config.Config{AssetPrefix: "wow"}, &result, atlas)
	if err != nil {
		t.Fatal(err)
	}
	rel := tex.WowData.PngPath
	t.Cleanup(func() { texturesource.Unregister(rel) })
	const prefix = "baked/uv2/alysrazor_"
	hexPart := strings.TrimSuffix(strings.TrimPrefix(rel, prefix), ".png")
	if !strings.HasPrefix(rel, prefix) || len(hexPart) != 12 {
		t.Fatalf("png path %q", rel)
	}
	if tex.Image != "wow/"+prefix+hexPart+".blp" {
		t.Fatalf("blp path %q", tex.Image)
	}
	again, err := registerBakeTexture(config.Config{AssetPrefix: "wow"}, &result, atlas)
	if err != nil {
		t.Fatal(err)
	}
	if again.WowData.PngPath != rel {
		t.Fatalf("same atlas %q vs %q", again.WowData.PngPath, rel)
	}
}
