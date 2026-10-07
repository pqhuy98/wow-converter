package mdl

import (
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
)

func TestTexturePagesSurviveForkDeduplicationAndSerialization(t *testing.T) {
	p0, p1 := &components.Texture{Image: "page0.blp"}, &components.Texture{Image: "page1.blp"}
	gs := components.NewGlobalSequence(0, 200)
	track := &components.Animation{GlobalSeq: &gs, Type: components.AnimTypeOthers, Interpolation: components.InterpDontInterp, KeyFrames: map[int]any{0: p0, 100: p1, 200: p0}}
	mat := &components.Material{Layers: []components.Layer{{Texture: p0, TextureIDAnim: track, FilterMode: components.BlendBlend, Alpha: components.AnimatedOrStatic[float64]{Static: true, Value: 1}}}}
	g := &components.Geoset{Material: mat}
	m := New(NewMDLOptions{Name: "pages", FormatVersion: 800})
	m.Textures, m.Materials, m.Geosets = []*components.Texture{p0, p1}, []*components.Material{mat}, []*components.Geoset{g}
	m.GlobalSequences = []*components.GlobalSequence{&gs}
	fork := ForkCollectionModel(CollectionModel{MDL: m}, m.Geosets).MDL
	fork.Modify.RemoveUnusedMaterialsTextures()
	fork.UpdateIDs()
	if len(fork.Textures) != 2 {
		t.Fatal("an animated page was discarded")
	}
	if fork.Materials[0].Layers[0].TextureIDAnim == track {
		t.Fatal("fork retained the original track")
	}
	for _, value := range fork.Materials[0].Layers[0].TextureIDAnim.KeyFrames {
		if value == p0 || value == p1 {
			t.Fatal("fork retained an original texture reference")
		}
	}
	parsed := mdx.NewModel()
	// Geometry is deliberately empty: this regression exercises the material
	// graph and the texture-ID track, not skinning serialization.
	fork.Geosets = nil
	if err := parsed.LoadMdl(fork.ToMdl()); err != nil {
		t.Fatal(err)
	}
	animations := parsed.Materials[0].Layers[0].Animations
	if len(animations) != 1 || animations[0].Name != "KMTF" || len(animations[0].Frames) != 3 || animations[0].GlobalSequenceID != 0 {
		t.Fatalf("invalid texture page track: %+v", animations)
	}
	if !strings.Contains(fork.ToMdl(), "100: 1,") {
		t.Fatal("texture page ID was not reindexed")
	}
}

func TestAnimationOnlyLayerKeepsItsTexturePagesAcrossItemMergeAndCleanup(t *testing.T) {
	main := New(NewMDLOptions{Name: "character"})
	main.Bones = []*components.Bone{components.NewBone("root")}
	baseTexture := &components.Texture{Image: "base.blp"}
	baseMaterial := &components.Material{Layers: []components.Layer{{Texture: baseTexture}}}
	main.Textures = []*components.Texture{baseTexture}
	main.Materials = []*components.Material{baseMaterial}
	main.Geosets = []*components.Geoset{{Name: "body", Material: baseMaterial}}

	makeItem := func(name string, textures []*components.Texture, layer components.Layer) *MDL {
		item := New(NewMDLOptions{Name: name})
		item.Bones = []*components.Bone{components.NewBone("root")}
		material := &components.Material{Layers: []components.Layer{layer}}
		item.Textures = textures
		item.Materials = []*components.Material{material}
		item.Geosets = []*components.Geoset{{Name: name, Material: material}}
		return item
	}

	headPage0 := &components.Texture{Image: "head-page-0.blp"}
	headPage1 := &components.Texture{Image: "head-page-1.blp"}
	headTrack := &components.Animation{
		Type:          components.AnimTypeOthers,
		Interpolation: components.InterpDontInterp,
		KeyFrames:     map[int]any{0: headPage0, 100: headPage1},
	}
	head := makeItem("head", []*components.Texture{headPage0, headPage1}, components.Layer{TextureIDAnim: headTrack})
	main.Modify.AddMdlCollectionItemToModel(head)

	// This later static item takes the ID that the animation-only layer's first
	// page had before unused textures are removed. If the page is discarded,
	// the stale KMTF ID silently points at this unrelated chest texture.
	chestTexture := &components.Texture{Image: "chest.blp"}
	chest := makeItem("chest", []*components.Texture{chestTexture}, components.Layer{Texture: chestTexture})
	main.Modify.AddMdlCollectionItemToModel(chest)

	main.Modify.RemoveUnusedMaterialsTextures()
	main.UpdateIDs()

	if len(main.Textures) != 4 {
		t.Fatalf("retained %d textures, want base, head pages, and chest page", len(main.Textures))
	}
	for _, page := range []*components.Texture{headPage0, headPage1} {
		id := page.ID
		if id < 0 || id >= len(main.Textures) || main.Textures[id].Image != page.Image {
			t.Fatalf("head KMTF page %q has ID %d resolving to the wrong texture", page.Image, id)
		}
	}
}

func TestRibbonMaterialSurvivesOptimizationAndMerge(t *testing.T) {
	body := &components.Material{Layers: []components.Layer{{Texture: &components.Texture{Image: "body.blp"}}}}
	trail := &components.Material{Layers: []components.Layer{{Texture: &components.Texture{Image: "trail.blp"}}}}
	m := New(NewMDLOptions{Name: "ribbon"})
	m.Materials = []*components.Material{body, trail}
	m.Geosets = []*components.Geoset{{Material: body}}
	m.RibbonEmitters = []*components.RibbonEmitter{{Material: trail}}
	m.Modify.RemoveUnusedMaterialsTextures()
	m.UpdateIDs()
	if len(m.Materials) != 2 || len(m.Textures) != 2 || m.RibbonEmitters[0].MaterialID != 1 {
		t.Fatalf("ribbon material or texture discarded: %+v", m.RibbonEmitters[0])
	}
	// A preceding model changes serialized IDs without breaking the reference.
	m.Materials = append([]*components.Material{{}}, m.Materials...)
	m.UpdateIDs()
	if !strings.Contains(components.RibbonEmittersToString(m.RibbonEmitters), "MaterialID 2,") {
		t.Fatal("ribbon did not follow material reindexing")
	}
}

func TestGeosetMaterialIDsViaUpdateIDs(t *testing.T) {
	mat0 := &components.Material{
		Layers: []components.Layer{{
			Texture: &components.Texture{Image: "wow/tex0.blp"},
		}},
	}
	mat1 := &components.Material{
		Layers: []components.Layer{{
			Texture: &components.Texture{Image: "wow/tex1.blp"},
		}},
	}

	m := &MDL{
		Materials: []*components.Material{mat0, mat1},
		Geosets: []*components.Geoset{
			{Material: mat0},
			{Material: mat1},
		},
	}
	m.UpdateIDs()

	if m.Geosets[0].Material == nil || m.Geosets[0].Material.ID != 0 {
		t.Fatalf("geoset0 material id=%d ptr=%p", m.Geosets[0].Material.ID, m.Geosets[0].Material)
	}
	if m.Geosets[1].Material == nil || m.Geosets[1].Material.ID != 1 {
		t.Fatalf("geoset1 material id=%d ptr=%p", m.Geosets[1].Material.ID, m.Geosets[1].Material)
	}
	if m.Geosets[0].Material == m.Geosets[1].Material {
		t.Fatal("geosets should reference distinct materials")
	}
	if m.Geosets[0].Material != m.Materials[0] || m.Geosets[1].Material != m.Materials[1] {
		t.Fatal("geoset materials should reference m.Materials slice entries")
	}
}
