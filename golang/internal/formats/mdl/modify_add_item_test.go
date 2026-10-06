package mdl

import (
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

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
