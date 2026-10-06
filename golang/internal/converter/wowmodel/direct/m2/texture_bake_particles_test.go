package directm2

import (
	"context"
	"image"
	"image/color"
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
