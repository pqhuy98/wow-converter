package directm2

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestParticleHeadTexturesMatchWoWCornerMapping(t *testing.T) {
	// Six distinct rectangular atlas cells expose frame reordering, axis swaps,
	// flips, alpha loss, and an accidental second Advanced Textures downscale.
	source := image.NewNRGBA(image.Rect(0, 0, 4, 9))
	for y := range 9 {
		for x := range 4 {
			source.SetNRGBA(x, y, color.NRGBA{uint8(20 + x*50), uint8(10 + y*25), uint8(x + y), 123})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	path := t.Name() + ".png"
	texturesource.Register(path, texturesource.Source{Kind: texturesource.KindPNG, PNG: encoded.Bytes()})
	t.Cleanup(func() { texturesource.Unregister(path) })
	original := &components.Texture{Image: "shared.blp", WowData: components.TextureWowData{PngPath: path}}
	head := components.NewParticleEmitter2("ParticleEmitter_0")
	head.Texture, head.Columns, head.Rows, head.HeadOrTail = original, 2, 3, components.HeadOrTailBoth
	head.HeadIntervals = [3]float64{1, 3, 1}
	head.DecayIntervals = [3]float64{3, 5, 1}
	head.Speed = components.AnimatedOrStatic[float64]{Static: true, Value: 42}
	head.EmissionRate = components.AnimatedOrStatic[float64]{Static: true, Value: 7}
	tail := mdl.CloneParticleEmitter2(head)
	tail.HeadOrTail = components.HeadOrTailTail
	variant := mdl.CloneParticleEmitter2(head)
	variant.HeadOrTail = components.HeadOrTailHead
	mesh := &components.Material{Layers: []components.Layer{{Texture: original}}}
	model := mdl.New(mdl.NewMDLOptions{Name: "particle orientation"})
	model.Textures, model.Materials = []*components.Texture{original}, []*components.Material{mesh}
	model.ParticleEmitter2s = []*components.ParticleEmitter2{head, tail, variant}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	cfg := config.DefaultConfig()
	cfg.TextureBaking.ResolutionScale = .5
	if err := orientM2ParticleHeads(context.Background(), cfg, &result, &m2.Loader{}, func(int) (*image.NRGBA, error) {
		t.Fatal("already-baked atlas should be read from the texture registry")
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	if head.Texture == original || variant.Texture != head.Texture || mesh.Layers[0].Texture != original || tail.Texture != original {
		t.Fatal("head texture was not isolated from meshes/tails or shared across head variants")
	}
	if len(model.ParticleEmitter2s) != 4 || head.HeadOrTail != components.HeadOrTailHead {
		t.Fatal("combined emitter was not split into a head and tail")
	}
	splitTail := model.ParticleEmitter2s[3]
	if splitTail.HeadOrTail != components.HeadOrTailTail || splitTail.Texture != original || splitTail.EmissionRate.Value != 7 {
		t.Fatal("combined emitter tail changed texture or birth rate")
	}
	if head.Speed.Value != 42 || head.EmissionRate.Value != 7 || head.HeadIntervals != ([3]float64{1, 3, 1}) || head.DecayIntervals != ([3]float64{3, 5, 1}) {
		t.Fatal("sprite correction changed motion, birth rate, or frame order")
	}
	registered, ok := texturesource.Get(head.Texture.WowData.PngPath)
	if !ok {
		t.Fatal("corrected atlas not registered")
	}
	t.Cleanup(func() { texturesource.Unregister(head.Texture.WowData.PngPath) })
	atlas, err := png.Decode(bytes.NewReader(registered.PNG))
	if err != nil {
		t.Fatal(err)
	}
	if atlas.Bounds() != image.Rect(0, 0, 6, 6) {
		t.Fatalf("rectangular cells lost resolution: %v", atlas.Bounds())
	}
	// Same screen corners, ordered bottom-left, top-left, top-right, bottom-right.
	// Taken independently from PRE2's corner UVs and WoW's BuildQuadT3 mapping.
	pre2UV := [4][2]int{{1, 0}, {0, 0}, {0, 1}, {1, 1}}
	wowUV := [4][2]int{{0, 1}, {0, 0}, {1, 0}, {1, 1}}
	for cell := range 6 {
		for corner := range 4 {
			got := color.NRGBAModel.Convert(atlas.At(cell%2*3+pre2UV[corner][0]*2, cell/2*2+pre2UV[corner][1]))
			want := source.At(cell%2*2+wowUV[corner][0], cell/2*3+wowUV[corner][1]*2)
			if got != want {
				t.Fatalf("cell %d screen corner %d: got %v, want WoW %v", cell, corner, got, want)
			}
		}
	}
}

func TestParticleHeadTexturesLoadSourceAndPackedTexture(t *testing.T) {
	for _, flags := range []uint32{0, 0x10000000} {
		model := mdl.New(mdl.NewMDLOptions{Name: "source particle"})
		head := components.NewParticleEmitter2("ParticleEmitter_0_direction_2_cell_1")
		head.Texture = &components.Texture{}
		head.Rows, head.Columns = 1, 1
		model.ParticleEmitter2s = []*components.ParticleEmitter2{head}
		result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
		packed := uint16(3)
		if flags != 0 {
			packed |= 2 << 5
		}
		loader := &m2.Loader{ParticleEmitters: []m2.ParticleEmitterEntry{{Flags: flags, TexturePacked: packed}}}
		if err := orientM2ParticleHeads(context.Background(), config.DefaultConfig(), &result, loader, func(index int) (*image.NRGBA, error) {
			if index != 3 {
				t.Fatalf("loaded texture %d, want primary texture 3", index)
			}
			return image.NewNRGBA(image.Rect(0, 0, 2, 2)), nil
		}); err != nil {
			t.Fatal(err)
		}
		texturesource.Unregister(head.Texture.WowData.PngPath)
	}
}

func TestParticleHeadConstantSamplerSupportsAnAtlas(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, color.NRGBA{255, 32, 16, 0})
	atlas, err := transposeParticleCells(source, 4, 4)
	if err != nil || atlas.Bounds() != source.Bounds() || atlas.NRGBAAt(0, 0) != source.NRGBAAt(0, 0) {
		t.Fatalf("constant sampler changed: atlas=%v error=%v", atlas, err)
	}
}
