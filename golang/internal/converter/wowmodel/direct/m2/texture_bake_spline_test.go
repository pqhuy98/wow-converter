package directm2

import (
	"bytes"
	"context"
	"fmt"
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

func TestSplineBakeKeepsSpatialSampleBudgetAndBlendAddRadiance(t *testing.T) {
	cfg := config.DefaultConfig()
	model := mdl.New(mdl.NewMDLOptions{Name: "spline bake"})
	for i := 0; i < 8; i++ {
		node := components.NewParticleEmitter2(fmt.Sprintf("ParticleEmitter_0_spline_%d", i))
		node.LifeSpan = 1
		node.EmissionRate = components.AnimatedOrStatic[float64]{Static: true, Value: 10}
		model.ParticleEmitter2s = append(model.ParticleEmitter2s, node)
	}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	loader := &m2.Loader{Textures: []m2.TextureEntry{{}}, ParticleEmitters: []m2.ParticleEmitterEntry{{
		EmitterType: 3, SplinePoints: make([][3]float32, 4), Flags: 0x10000000, BlendingType: 7,
		ScaleTrack: m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{1, 1}}},
	}}}
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, color.NRGBA{200, 100, 50, 32})
	if err := bakeM2Particles(context.Background(), cfg, &result, loader, func(int) (*image.NRGBA, error) { return source, nil }); err != nil {
		t.Fatal(err)
	}
	if len(model.ParticleEmitter2s) != 8 {
		t.Fatal("multiplied spatial samples by phase count")
	}
	total := 0.0
	for _, node := range model.ParticleEmitter2s {
		total += node.EmissionRate.Value
		defer texturesource.Unregister(node.Texture.WowData.PngPath)
		registered, _ := texturesource.Get(node.Texture.WowData.PngPath)
		atlas, err := png.Decode(bytes.NewReader(registered.PNG))
		if err != nil {
			t.Fatal(err)
		}
		tile := atlas.Bounds().Dx() / node.Columns
		_, _, _, alpha := atlas.At(tile/2, tile/2).RGBA()
		if alpha != 65535 || node.SegmentAlphas != ([3]float64{255, 255, 255}) {
			t.Fatal("BlendAdd radiance attenuated by alpha twice")
		}
	}
	if total != 80 {
		t.Fatalf("rate changed: %v", total)
	}
}
