package directm2

import (
	"context"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestParticleBakeKeepsAssembledWorldSize(t *testing.T) {
	for _, unitScale := range []float64{1, 56, 112} {
		cfg := config.DefaultConfig()
		cfg.RawModelScaleUp = unitScale
		animate := false
		cfg.TextureBaking.Animate = &animate
		model := mdl.New(mdl.NewMDLOptions{Name: "gas"})
		node := &components.ParticleEmitter2{
			NodeBase:   components.NodeBase{Name: "ParticleEmitter_0"},
			FilterMode: components.PFilterAdditive, LifeSpan: 1,
			SegmentScaling: [3]float64{2 * unitScale, 2 * unitScale, 2 * unitScale},
		}
		model.ParticleEmitter2s = []*components.ParticleEmitter2{node}
		result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
		loader := &m2.Loader{
			Textures: []m2.TextureEntry{{}},
			ParticleEmitters: []m2.ParticleEmitterEntry{{
				ScaleTrack: m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{2, 4}}},
			}},
		}
		loader.ParticleEmitters[0].TwinkleScale.Min = 1
		loader.ParticleEmitters[0].TwinkleScale.Max = 1
		soft := image.NewNRGBA(image.Rect(0, 0, 8, 8))
		soft.SetNRGBA(4, 4, color.NRGBA{255, 255, 255, 128})
		if err := bakeM2Particles(context.Background(), cfg, &result, loader, func(int) (*image.NRGBA, error) { return soft, nil }); err != nil {
			t.Fatal(err)
		}
		defer texturesource.Unregister(node.Texture.WowData.PngPath)
		for _, size := range node.SegmentScaling {
			if size != 4*unitScale {
				t.Fatalf("unit scale %v: baked size %v, want %v", unitScale, size, 4*unitScale)
			}
		}
	}
}

func TestCombineParticleSamplesMatchesReferenceShader(t *testing.T) {
	tex := [3][4]float64{
		{.5, .8, .4, .6},
		{.4, .5, .9, .7},
		{.25, .5, .2, .3},
	}
	wantTwoColorTextures := [4]float64{.2, .4, .36, .126}
	wantThreeColorTextures := [4]float64{.05, .2, .072, .126}
	for _, tc := range []struct {
		name  string
		flags uint32
		want  [4]float64
	}{
		{name: "two RGB textures and three alpha textures", want: wantTwoColorTextures},
		{name: "0x20000000 does not add multipliers", flags: 0x20000000, want: wantTwoColorTextures},
		{name: "three RGB textures", flags: 0x40000000, want: wantThreeColorTextures},
		{name: "three RGB textures with 0x20000000", flags: 0x60000000, want: wantThreeColorTextures},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := combineParticleSamples(tex, tc.flags)
			for channel := range got {
				if math.Abs(got[channel]-tc.want[channel]) > 1e-12 {
					t.Fatalf("flags %#x channel %d = %g, want %g (result %v)", tc.flags, channel, got[channel], tc.want[channel], got)
				}
			}
		})
	}
}
