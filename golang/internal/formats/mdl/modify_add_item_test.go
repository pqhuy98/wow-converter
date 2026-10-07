package mdl

import (
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func TestCollectionClassificationRejectsLocalKeyBoneRoot(t *testing.T) {
	main := New(NewMDLOptions{Name: "character"})
	root := components.NewBone("Root")
	buckle := components.NewBone("Buckle")
	buckle.SetNodeParent(root)
	buckle.PivotPoint = imath.Vector3{7, 0, 72}
	main.Bones = []*components.Bone{root, buckle}
	item := New(NewMDLOptions{Name: "local belt"})
	item.Bones = []*components.Bone{components.NewBone("Buckle")}
	if CanAddMdlCollectionItemToModel(main, item) {
		t.Fatal("local belt root was mistaken for a shared character skeleton")
	}
	// A partial collection (such as the bandit mask) can omit ancestors while
	// remaining in character space. Do not attach and translate it twice.
	item.Bones[0].PivotPoint = buckle.PivotPoint
	if !CanAddMdlCollectionItemToModel(main, item) {
		t.Fatal("character-space subset collection was rejected")
	}
	itemRoot := components.NewBone("Root")
	item.Bones[0].SetNodeParent(itemRoot)
	item.Bones = append(item.Bones, itemRoot)
	if !CanAddMdlCollectionItemToModel(main, item) {
		t.Fatal("shared character-space collection was rejected")
	}
}

func TestItemMergesPreserveTextureFlipbookAndGlobalSequence(t *testing.T) {
	for _, merge := range []struct {
		name string
		fn   func(main, item *MDL)
	}{
		{
			name: "bone attachment",
			fn: func(main, item *MDL) {
				main.Modify.AddMdlItemToBone(item, main.Bones[0])
			},
		},
		{
			name: "collection mesh",
			fn: func(main, item *MDL) {
				// The baker can leave a layer pointing at a value outside the
				// TextureAnims slice. Forking must still let the merge remap by ID.
				staleAnim := item.TextureAnims[0]
				item.Geosets[0].Material.Layers[0].TVertexAnim = &staleAnim
				fork := ForkCollectionModel(CollectionModel{MDL: item}, item.Geosets)
				main.Modify.AddMdlCollectionItemToModel(fork.MDL)
			},
		},
	} {
		t.Run(merge.name, func(t *testing.T) {
			main := New(NewMDLOptions{Name: "main"})
			main.Bones = []*components.Bone{components.NewBone("root")}
			item := New(NewMDLOptions{Name: "item"})
			item.Bones = []*components.Bone{components.NewBone("root")}
			item.Sequences = []components.Sequence{{Name: "Stand", Interval: [2]int{0, 1000}}}
			gs := components.NewGlobalSequence(0, 1000)
			anim := components.TextureAnim{
				ID: 0,
				Translation: &components.Animation{
					Type:          components.AnimTypeTVertexAnim,
					Interpolation: components.InterpDontInterp,
					GlobalSeq:     &gs,
					KeyFrames:     map[int]any{0: imath.Vector3{}, 500: imath.Vector3{1, 0, 0}},
				},
			}
			item.GlobalSequences = []*components.GlobalSequence{&gs}
			item.TextureAnims = []components.TextureAnim{anim}
			texture := &components.Texture{Image: "baked/shoulder.blp"}
			material := &components.Material{Layers: []components.Layer{{Texture: texture, TVertexAnim: &item.TextureAnims[0]}}}
			item.Textures = []*components.Texture{texture}
			item.Materials = []*components.Material{material}
			item.Geosets = []*components.Geoset{{Name: "shoulder", Material: material}}

			merge.fn(main, item)
			main.Modify.RemoveUnusedMaterialsTextures().OptimizeKeyFrames()

			if len(main.TextureAnims) != 1 || len(main.GlobalSequences) != 1 {
				t.Fatalf("merged %d texture animations and %d global sequences, want one each", len(main.TextureAnims), len(main.GlobalSequences))
			}
			mergedLayer := main.Geosets[0].Material.Layers[0]
			if mergedLayer.TVertexAnim == nil || mergedLayer.TVertexAnim != &main.TextureAnims[0] {
				t.Fatal("merged material layer does not point to the merged texture animation")
			}
			if mergedLayer.TVertexAnim.Translation == nil || mergedLayer.TVertexAnim.Translation.GlobalSeq != main.GlobalSequences[0] {
				t.Fatal("merged flipbook track lost its remapped global sequence")
			}
			if got := len(mergedLayer.TVertexAnim.Translation.KeyFrames); got != 2 {
				t.Fatalf("merged flipbook has %d keyframes, want 2", got)
			}
			text := components.MaterialsToString(main.Version.FormatVersion, main.Materials) +
				components.TextureAnimsToString(main.TextureAnims) +
				components.GlobalSequencesToString(main.GlobalSequences)
			if !strings.Contains(text, "TVertexAnimId 0") || !strings.Contains(text, "GlobalSeqId 0") {
				t.Fatalf("serialized merged model lost flipbook references:\n%s", text)
			}
		})
	}
}

func TestBoneAttachedItemUsesSelectedStandAsGlobalAnimationClock(t *testing.T) {
	main := New(NewMDLOptions{Name: "main"})
	main.Bones = []*components.Bone{components.NewBone("root")}
	item := New(NewMDLOptions{Name: "item"})
	item.Bones = []*components.Bone{components.NewBone("root")}
	item.Sequences = []components.Sequence{
		{Name: "Walk", Interval: [2]int{1200, 2200}},
		{Name: "Stand", Interval: [2]int{100, 1100}},
	}

	page0 := &components.Texture{Image: "page-0.blp"}
	page1 := &components.Texture{Image: "page-1.blp"}
	item.Textures = []*components.Texture{page0, page1}
	textureTrack := &components.Animation{
		Type:          components.AnimTypeOthers,
		Interpolation: components.InterpDontInterp,
		KeyFrames:     map[int]any{50: page1, 100: page0, 600: page1, 1100: page0, 1200: page1},
	}
	uvTrack := &components.Animation{
		Type:          components.AnimTypeTVertexAnim,
		Interpolation: components.InterpBezier,
		KeyFrames:     map[int]any{100: imath.Vector3{}, 1100: imath.Vector3{0, 1, 0}},
		InOutTans:     map[int]components.InOutTan{100: {InTan: imath.Vector3{}, OutTan: imath.Vector3{0, 0.5, 0}}, 1100: {InTan: imath.Vector3{0, 0.5, 0}, OutTan: imath.Vector3{0, 1, 0}}},
	}
	linearUVTrack := &components.Animation{
		Type:          components.AnimTypeTVertexAnim,
		Interpolation: components.InterpLinear,
		KeyFrames:     map[int]any{100: imath.Vector3{}, 1100: imath.Vector3{1, 0, 0}},
	}
	anim := components.TextureAnim{Translation: uvTrack, Scaling: linearUVTrack}
	item.TextureAnims = []components.TextureAnim{anim}
	material := &components.Material{Layers: []components.Layer{{
		Texture:       page0,
		TextureIDAnim: textureTrack,
		TVertexAnim:   &item.TextureAnims[0],
	}}}
	item.Materials = []*components.Material{material}
	item.Geosets = []*components.Geoset{{Name: "item", Material: material}}
	item.Bones[0].Translation = &components.Animation{
		Type:          components.AnimTypeTranslation,
		Interpolation: components.InterpLinear,
		KeyFrames:     map[int]any{100: imath.Vector3{}, 1100: imath.Vector3{2, 0, 0}},
	}

	existingGlobal := components.NewGlobalSequence(0, 2000)
	existingTrack := &components.Animation{
		GlobalSeq:     &existingGlobal,
		Type:          components.AnimTypeRotation,
		Interpolation: components.InterpLinear,
		KeyFrames:     map[int]any{0: imath.QuatNoRotation(), 2000: imath.QuatNoRotation()},
	}
	item.Bones[0].Rotation = existingTrack
	particle := components.NewParticleEmitter2("shoulder glow")
	particle.Width = components.AnimatedOrStatic[float64]{Anim: &components.Animation{
		Interpolation: components.InterpLinear,
		KeyFrames:     map[int]any{100: 0.5, 1100: 1.0},
	}}
	item.ParticleEmitter2s = []*components.ParticleEmitter2{particle}

	main.Modify.AddMdlItemToBone(item, main.Bones[0])
	mergedLayer := main.Geosets[0].Material.Layers[0]
	if mergedLayer.TextureIDAnim == nil || mergedLayer.TextureIDAnim.GlobalSeq == nil {
		t.Fatal("attached KMTF track was not moved to an independent global clock")
	}
	globalSequence := mergedLayer.TextureIDAnim.GlobalSeq
	if particle.Width.Anim.GlobalSeq != globalSequence || particle.Width.Anim.KeyFrames[0] != 0.5 || particle.Width.Anim.KeyFrames[1000] != 1.0 {
		t.Fatal("item particle channels did not follow the independent item clock")
	}
	if globalSequence.Duration != 1000 {
		t.Fatalf("global sequence duration = %d, want selected Stand duration 1000", globalSequence.Duration)
	}
	if got := components.SortedKeyInts(mergedLayer.TextureIDAnim.KeyFrames); !equalInts(got, []int{0, 500, 1000}) {
		t.Fatalf("rebased KMTF keys = %v, want [0 500 1000] including the loop endpoint", got)
	}
	if got := mergedLayer.TextureIDAnim.KeyFrames[500].(*components.Texture); got.Image != page1.Image {
		t.Fatalf("rebased middle KMTF page = %q, want %q", got.Image, page1.Image)
	}
	if mergedLayer.TVertexAnim == nil || mergedLayer.TVertexAnim.Translation == nil || mergedLayer.TVertexAnim.Translation.GlobalSeq != globalSequence {
		t.Fatal("UV translation does not share the KMTF loop clock")
	}
	if got := components.SortedKeyInts(mergedLayer.TVertexAnim.Translation.KeyFrames); !equalInts(got, []int{0, 1000}) {
		t.Fatalf("rebased two-key linear UV track = %v, want [0 1000] including the loop endpoint", got)
	}
	if got := mergedLayer.TVertexAnim.Translation.InOutTans[1000]; got.InTan != (imath.Vector3{0, 0.5, 0}) || got.OutTan != (imath.Vector3{0, 1, 0}) {
		t.Fatalf("rebased UV endpoint tangent was not preserved: %+v", got)
	}
	if mergedLayer.TVertexAnim.Scaling == nil || mergedLayer.TVertexAnim.Scaling.GlobalSeq != globalSequence || mergedLayer.TVertexAnim.Scaling.Interpolation != components.InterpLinear {
		t.Fatal("linear UV scaling did not share the selected Stand clock")
	}
	if got := components.SortedKeyInts(mergedLayer.TVertexAnim.Scaling.KeyFrames); !equalInts(got, []int{0, 1000}) {
		t.Fatalf("rebased two-key linear UV scaling = %v, want [0 1000] including the loop endpoint", got)
	}
	if _, ok := mergedLayer.TextureIDAnim.KeyFrames[50]; ok {
		t.Fatal("KMTF key outside selected Stand leaked into the global loop")
	}
	if _, ok := mergedLayer.TextureIDAnim.KeyFrames[1000]; !ok {
		t.Fatal("key at selected Stand end was dropped instead of retained for interpolation")
	}
	if _, ok := mergedLayer.TextureIDAnim.KeyFrames[1200]; ok {
		t.Fatal("KMTF key outside selected Stand leaked into the global loop")
	}
	if existingTrack.GlobalSeq != &existingGlobal || existingGlobal.Duration != 2000 {
		t.Fatal("existing global animation was modified")
	}
	if main.Bones[1].Translation == nil || main.Bones[1].Translation.GlobalSeq != globalSequence {
		t.Fatal("bone track was not moved to the selected Stand clock")
	}
	if got := components.SortedKeyInts(main.Bones[1].Translation.KeyFrames); !equalInts(got, []int{0, 1000}) {
		t.Fatalf("rebased two-key linear bone track = %v, want [0 1000] including the loop endpoint", got)
	}
}

func TestBoneAttachedItemWithoutSequencesKeepsLocalKeys(t *testing.T) {
	main := New(NewMDLOptions{Name: "main"})
	main.Bones = []*components.Bone{components.NewBone("root")}
	item := New(NewMDLOptions{Name: "item"})
	item.Bones = []*components.Bone{components.NewBone("root")}
	track := &components.Animation{Type: components.AnimTypeOthers, KeyFrames: map[int]any{0: 0, 100: 1}}
	material := &components.Material{Layers: []components.Layer{{TextureIDAnim: track}}}
	item.Materials = []*components.Material{material}
	item.Geosets = []*components.Geoset{{Name: "item", Material: material}}

	main.Modify.AddMdlItemToBone(item, main.Bones[0])
	merged := main.Geosets[0].Material.Layers[0].TextureIDAnim
	if merged == nil || merged.GlobalSeq != nil || len(merged.KeyFrames) != 2 {
		t.Fatalf("local keys without a sequence should be preserved unchanged: %+v", merged)
	}
}

func TestCollectionVisualAnimationsUseSelectedStandWithoutCopyingBoneMotion(t *testing.T) {
	main := New(NewMDLOptions{Name: "main"})
	main.Bones = []*components.Bone{components.NewBone("root")}
	template := New(NewMDLOptions{Name: "collection"})
	template.Bones = []*components.Bone{components.NewBone("root")}
	template.Sequences = []components.Sequence{
		{Name: "Walk", Interval: [2]int{1200, 2200}},
		{Name: "Stand", Interval: [2]int{100, 1100}},
	}
	page0, page1 := &components.Texture{Image: "page-0.blp"}, &components.Texture{Image: "page-1.blp"}
	template.Textures = []*components.Texture{page0, page1}
	textureTrack := &components.Animation{Type: components.AnimTypeOthers, KeyFrames: map[int]any{100: page0, 600: page1, 1100: page0}}
	alphaTrack := &components.Animation{Type: components.AnimTypeAlpha, Interpolation: components.InterpLinear, KeyFrames: map[int]any{100: 0.5, 1100: 1.0}}
	uvTrack := &components.Animation{Type: components.AnimTypeTVertexAnim, KeyFrames: map[int]any{100: imath.Vector3{}, 600: imath.Vector3{0, 0.5, 0}, 1100: imath.Vector3{0, 1, 0}}}
	template.TextureAnims = []components.TextureAnim{{Translation: uvTrack}}
	material := &components.Material{Layers: []components.Layer{{Texture: page0, TextureIDAnim: textureTrack, TVertexAnim: &template.TextureAnims[0], Alpha: components.AnimatedOrStatic[float64]{Anim: alphaTrack}}}}
	template.Materials = []*components.Material{material}
	template.Geosets = []*components.Geoset{{Name: "collection", Material: material}}
	template.Bones[0].Translation = &components.Animation{Type: components.AnimTypeTranslation, KeyFrames: map[int]any{100: imath.Vector3{}, 600: imath.Vector3{8, 0, 0}, 1100: imath.Vector3{9, 0, 0}}}

	fork := ForkCollectionModel(CollectionModel{MDL: template}, template.Geosets)
	main.Modify.AddMdlCollectionItemToModel(fork.MDL)
	if len(main.Bones) != 1 || main.Bones[0].Translation != nil {
		t.Fatal("collection bone motion was copied onto the character skeleton")
	}
	layer := main.Geosets[0].Material.Layers[0]
	if layer.TextureIDAnim == nil || layer.TextureIDAnim.GlobalSeq == nil || layer.TextureIDAnim.GlobalSeq.Duration != 1000 {
		t.Fatal("collection KMTF track was not put on the selected Stand clock")
	}
	if layer.TVertexAnim == nil || layer.TVertexAnim.Translation == nil || layer.TVertexAnim.Translation.GlobalSeq != layer.TextureIDAnim.GlobalSeq {
		t.Fatal("collection UV animation did not share the KMTF loop clock")
	}
	if layer.Alpha.Anim == nil || layer.Alpha.Anim.GlobalSeq != layer.TextureIDAnim.GlobalSeq {
		t.Fatal("collection layer alpha did not use the selected Stand clock")
	}
	if textureTrack.GlobalSeq != nil || uvTrack.GlobalSeq != nil || alphaTrack.GlobalSeq != nil || len(textureTrack.KeyFrames) != 3 || len(uvTrack.KeyFrames) != 3 || len(alphaTrack.KeyFrames) != 2 {
		t.Fatal("collection clock conversion modified its source template")
	}
}

func TestCollectionNodeParentsRemapToMainBones(t *testing.T) {
	main := New(NewMDLOptions{Name: "main"})
	mainRoot := components.NewBone("root")
	main.Bones = []*components.Bone{mainRoot}

	item := New(NewMDLOptions{Name: "collection"})
	itemRoot := components.NewBone("root")
	itemRoot.SetNodeObjectID(91)
	item.Bones = []*components.Bone{itemRoot}
	ribbon := components.NewRibbonEmitter("collection ribbon")
	ribbon.Parent = itemRoot
	helper := components.NewHelper("collection helper")
	helper.Parent = itemRoot
	item.RibbonEmitters = []*components.RibbonEmitter{ribbon}
	item.Helpers = []*components.Helper{helper}

	main.Modify.AddMdlCollectionItemToModel(item)

	if got := main.RibbonEmitters[0].NodeParent(); got != mainRoot {
		t.Fatalf("collection ribbon parent = %p, want main bone %p", got, mainRoot)
	}
	if got := main.Helpers[0].NodeParent(); got != mainRoot {
		t.Fatalf("collection helper parent = %p, want main bone %p", got, mainRoot)
	}
	if got := itemRoot.NodeObjectID(); got != 91 {
		t.Fatalf("source collection bone object ID changed to %d; collection merge should not mutate it", got)
	}

	text := main.ToMdl()
	if strings.Count(text, "Parent 0,") != 2 {
		t.Fatalf("collection child nodes did not serialize against the main root bone:\n%s", text)
	}
}

func TestForkCollectionModelRebindsClonedNodeParents(t *testing.T) {
	template := New(NewMDLOptions{Name: "collection"})
	root := components.NewBone("root")
	template.Bones = []*components.Bone{root}
	parent := components.NewHelper("parent helper")
	parent.Parent = root
	child := components.NewHelper("child helper")
	child.Parent = parent
	ribbon := components.NewRibbonEmitter("child ribbon")
	ribbon.Parent = parent
	template.Helpers = []*components.Helper{parent, child}
	template.RibbonEmitters = []*components.RibbonEmitter{ribbon}

	fork := ForkCollectionModel(CollectionModel{MDL: template}, nil).MDL
	if fork.Helpers[0] == parent || fork.Helpers[1] == child || fork.RibbonEmitters[0] == ribbon {
		t.Fatal("fork reused source node objects instead of cloning them")
	}
	if fork.Helpers[0].NodeParent() != root {
		t.Fatal("forked helper's bone parent should remain the shared source bone")
	}
	if fork.Helpers[1].NodeParent() != fork.Helpers[0] || fork.RibbonEmitters[0].NodeParent() != fork.Helpers[0] {
		t.Fatal("forked child nodes still point at source nodes")
	}
	if child.NodeParent() != parent || ribbon.NodeParent() != parent {
		t.Fatal("forking changed source child-node parents")
	}
}

func equalInts(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
