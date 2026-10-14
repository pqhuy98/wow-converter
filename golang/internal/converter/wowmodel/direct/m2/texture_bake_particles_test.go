package directm2

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
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

func TestParticleBakeIncludesDirectionAndCellVariants(t *testing.T) {
	cfg := config.DefaultConfig()
	model := mdl.New(mdl.NewMDLOptions{Name: "variants"})
	for cell := 0; cell < 2; cell++ {
		node := components.NewParticleEmitter2("ParticleEmitter_0_direction_" + string(rune('0'+cell)) + "_cell_" + string(rune('0'+cell)))
		node.LifeSpan = 1
		node.HeadIntervals = [3]float64{float64(cell), float64(cell), 1}
		node.EmissionRate = components.AnimatedOrStatic[float64]{Static: true, Value: 10}
		model.ParticleEmitter2s = append(model.ParticleEmitter2s, node)
	}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	loader := &m2.Loader{Textures: []m2.TextureEntry{{}}, ParticleEmitters: []m2.ParticleEmitterEntry{{Flags: 0x10000000, TextureCols: 2, TextureRows: 1, ScaleTrack: m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{1, 1}}}}}}
	source := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			source.SetNRGBA(x, y, color.NRGBA{200, 100, 50, 128})
		}
	}
	if err := bakeM2Particles(context.Background(), cfg, &result, loader, func(int) (*image.NRGBA, error) { return source, nil }); err != nil {
		t.Fatal(err)
	}
	if len(model.ParticleEmitter2s) != 4 {
		t.Fatalf("lost variants: %d", len(model.ParticleEmitter2s))
	}
	total := 0.0
	for _, node := range model.ParticleEmitter2s {
		if node.Texture == nil {
			t.Fatal("variant was not baked")
		}
		total += node.EmissionRate.Value
		defer texturesource.Unregister(node.Texture.WowData.PngPath)
	}
	if total != 20 {
		t.Fatalf("birth rate changed: %v", total)
	}
}

func TestParticleBakePreservesDelayedOpacityPeak(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RawModelScaleUp = 1
	model := mdl.New(mdl.NewMDLOptions{Name: "delayed flame"})
	node := components.NewParticleEmitter2("ParticleEmitter_0")
	node.LifeSpan = 1
	node.TimeMiddle = .4
	node.EmissionRate = components.AnimatedOrStatic[float64]{Static: true, Value: 10}
	model.ParticleEmitter2s = []*components.ParticleEmitter2{node}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	loader := &m2.Loader{Textures: []m2.TextureEntry{{}}, ParticleEmitters: []m2.ParticleEmitterEntry{{
		AlphaTrack: m2.PartTrack{Timestamps: []uint16{0, 6553, 13107, 32767}, Values: [][]float64{{0}, {0}, {32767}, {0}}},
		ScaleTrack: m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{1, 1}}},
	}}}
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, color.NRGBA{255, 255, 255, 255})
	if err := bakeM2Particles(context.Background(), cfg, &result, loader, func(int) (*image.NRGBA, error) { return source, nil }); err != nil {
		t.Fatal(err)
	}
	defer texturesource.Unregister(node.Texture.WowData.PngPath)
	registered, ok := texturesource.Get(node.Texture.WowData.PngPath)
	if !ok {
		t.Fatal("atlas not registered")
	}
	atlas, err := png.Decode(bytes.NewReader(registered.PNG))
	if err != nil {
		t.Fatal(err)
	}
	tile := atlas.Bounds().Dx() / node.Columns
	_, _, _, early := atlas.At(2*tile+tile/2, tile/2).RGBA()
	_, _, _, peak := atlas.At(2*tile+tile/2, tile+tile/2).RGBA()
	if early != 0 || peak < 60000 {
		t.Fatalf("delayed curve lost: early=%d peak=%d", early, peak)
	}
	if node.SegmentAlphas != ([3]float64{255, 255, 255}) {
		t.Fatal("opacity was applied twice")
	}
	if node.EmissionRate.Value != 10 {
		t.Fatal("birth rate changed")
	}
}

func TestParticleBakeNormalizesAgeByMaximumLifetime(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RawModelScaleUp = 1
	model := mdl.New(mdl.NewMDLOptions{Name: "variable lifetime"})
	node := components.NewParticleEmitter2("ParticleEmitter_0")
	node.LifeSpan = 1
	node.TimeMiddle = .4
	model.ParticleEmitter2s = []*components.ParticleEmitter2{node}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	loader := &m2.Loader{Textures: []m2.TextureEntry{{}}, ParticleEmitters: []m2.ParticleEmitterEntry{{
		LifespanVary: 1,
		AlphaTrack:   m2.PartTrack{Timestamps: []uint16{0, 13107, 32767}, Values: [][]float64{{0}, {32767}, {0}}},
		ScaleTrack:   m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{1, 1}}},
	}}}
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, color.NRGBA{255, 255, 255, 255})
	if err := bakeM2Particles(context.Background(), cfg, &result, loader, func(int) (*image.NRGBA, error) { return source, nil }); err != nil {
		t.Fatal(err)
	}
	defer texturesource.Unregister(node.Texture.WowData.PngPath)
	registered, _ := texturesource.Get(node.Texture.WowData.PngPath)
	atlas, err := png.Decode(bytes.NewReader(registered.PNG))
	if err != nil {
		t.Fatal(err)
	}
	tile := atlas.Bounds().Dx() / node.Columns
	_, _, _, a := atlas.At(3*tile+tile/2, 3*tile+tile/2).RGBA()
	if a < 50000 {
		t.Fatalf("mean-lifetime endpoint wrongly used the final zero key: %d", a)
	}
}

func TestParticleBakeKeepsColorClockIndependentOfAlphaPeak(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RawModelScaleUp = 1
	model := mdl.New(mdl.NewMDLOptions{Name: "independent color curve"})
	node := components.NewParticleEmitter2("ParticleEmitter_0")
	node.LifeSpan = 1
	node.TimeMiddle = float64(2000) / 32767
	model.ParticleEmitter2s = []*components.ParticleEmitter2{node}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	loader := &m2.Loader{Textures: []m2.TextureEntry{{}}, ParticleEmitters: []m2.ParticleEmitterEntry{{
		Flags:      0x10000000,
		ColorTrack: m2.PartTrack{Timestamps: []uint16{0, 16384, 32767}, Values: [][]float64{{255, 0, 0}, {0, 0, 255}, {0, 255, 0}}},
		AlphaTrack: m2.PartTrack{Timestamps: []uint16{0, 2000, 32767}, Values: [][]float64{{32767}, {32767}, {32767}}},
		ScaleTrack: m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{1, 1}}},
	}}}
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, color.NRGBA{255, 255, 255, 255})
	if err := bakeM2Particles(context.Background(), cfg, &result, loader, func(int) (*image.NRGBA, error) { return source, nil }); err != nil {
		t.Fatal(err)
	}
	for _, p := range model.ParticleEmitter2s {
		defer texturesource.Unregister(p.Texture.WowData.PngPath)
	}
	registered, _ := texturesource.Get(node.Texture.WowData.PngPath)
	atlas, err := png.Decode(bytes.NewReader(registered.PNG))
	if err != nil {
		t.Fatal(err)
	}
	tile := atlas.Bounds().Dx() / node.Columns
	frame := int(math.Round(float64(node.Columns*node.Rows-1) / 2))
	r, _, b, _ := atlas.At((frame%node.Columns)*tile+tile/2, (frame/node.Columns)*tile+tile/2).RGBA()
	if b <= r {
		t.Fatalf("midlife blue was replaced by alpha-peak color: r=%d b=%d", r, b)
	}
}

func TestParticleStaticBakeKeepsNativeOpacityAndColor(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RawModelScaleUp = 1
	animate := false
	cfg.TextureBaking.Animate = &animate
	model := mdl.New(mdl.NewMDLOptions{Name: "static anisotropic particle"})
	node := components.NewParticleEmitter2("ParticleEmitter_0_direction_0")
	node.LifeSpan = 1
	node.SegmentAlphas = [3]float64{0, 255, 0}
	node.SegmentColors = [3]imath.Vector3{{0, 0, 1}, {0, 1, 0}, {1, 0, 0}}
	model.ParticleEmitter2s = []*components.ParticleEmitter2{node}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	loader := &m2.Loader{Textures: []m2.TextureEntry{{}}, ParticleEmitters: []m2.ParticleEmitterEntry{{
		AlphaTrack: m2.PartTrack{Timestamps: []uint16{0, 2000, 10000, 32767}, Values: [][]float64{{0}, {0}, {32767}, {0}}},
		ColorTrack: m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{255, 0, 0}}},
		ScaleTrack: m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{1, 2}}},
	}}}
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, color.NRGBA{255, 255, 255, 255})
	if err := bakeM2Particles(context.Background(), cfg, &result, loader, func(int) (*image.NRGBA, error) { return source, nil }); err != nil {
		t.Fatal(err)
	}
	defer texturesource.Unregister(node.Texture.WowData.PngPath)
	registered, _ := texturesource.Get(node.Texture.WowData.PngPath)
	atlas, err := png.Decode(bytes.NewReader(registered.PNG))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, a := atlas.At(atlas.Bounds().Dx()/2, atlas.Bounds().Dy()/2).RGBA()
	if a == 0 || r != g || g != b || node.SegmentAlphas != ([3]float64{0, 255, 0}) || node.SegmentColors[0] != (imath.Vector3{0, 0, 1}) {
		t.Fatalf("static bake erased or duplicated age curves: rgba=%d,%d,%d,%d alphas=%v colors=%v", r, g, b, a, node.SegmentAlphas, node.SegmentColors)
	}
}

func TestParticleBakePreservesBillboardSpinWithoutClipping(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.RawModelScaleUp = 1
	model := mdl.New(mdl.NewMDLOptions{Name: "rotating rectangle"})
	node := components.NewParticleEmitter2("ParticleEmitter_0")
	node.LifeSpan = 1
	node.HeadOrTail = components.HeadOrTailHead
	node.EmissionRate = components.AnimatedOrStatic[float64]{Static: true, Value: 8}
	model.ParticleEmitter2s = []*components.ParticleEmitter2{node}
	result := ConvertResult{MDL: model, TexturePaths: map[string]struct{}{}}
	loader := &m2.Loader{Textures: []m2.TextureEntry{{}}, ParticleEmitters: []m2.ParticleEmitterEntry{{
		Spin:       math.Pi / 2,
		ScaleTrack: m2.PartTrack{Timestamps: []uint16{0}, Values: [][]float64{{1, 2}}},
	}}}
	source := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	source.SetNRGBA(0, 0, color.NRGBA{255, 255, 255, 255})
	if err := bakeM2Particles(context.Background(), cfg, &result, loader, func(int) (*image.NRGBA, error) { return source, nil }); err != nil {
		t.Fatal(err)
	}
	defer texturesource.Unregister(node.Texture.WowData.PngPath)
	registered, _ := texturesource.Get(node.Texture.WowData.PngPath)
	atlas, err := png.Decode(bytes.NewReader(registered.PNG))
	if err != nil {
		t.Fatal(err)
	}
	tile := atlas.Bounds().Dx() / node.Columns
	alpha := func(frame, x, y int) uint32 {
		_, _, _, a := atlas.At((frame%node.Columns)*tile+x, (frame/node.Columns)*tile+y).RGBA()
		return a
	}
	last := int(node.DecayIntervals[1])
	center, offset := tile/2, int(.3*float64(tile))
	if alpha(0, center, center+offset) == 0 || alpha(0, center+offset, center) != 0 || alpha(last, center, center+offset) != 0 || alpha(last, center+offset, center) == 0 {
		t.Fatal("quad did not rotate from vertical to horizontal")
	}
	if math.Abs(node.SegmentScaling[0]-math.Sqrt(5)) > 1e-6 || node.EmissionRate.Value != 8 {
		t.Fatal("rotation clipped geometry or changed birth budget")
	}
}
