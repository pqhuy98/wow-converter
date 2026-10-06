package directm2

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestBakePagesRemoveRepeatedEmptyMarginsWithoutShrinkingCharts(t *testing.T) {
	charts := []bakeChart{{x: 0, y: 0, width: 480, height: 128}, {x: 8, y: 128, width: 460, height: 10}}
	for _, frames := range []int{1, 15, 48, 120, 360} {
		p := planBakePages(charts, frames)
		if p.cellW != 480 || p.cellH != 144 || p.width&(p.width-1) != 0 || p.height&(p.height-1) != 0 {
			t.Fatalf("invalid page for %d frames: %+v", frames, p)
		}
		oldCols := min(bakePowerOfTwo(intCeilSqrt(frames)), 4)
		oldRows := min(bakePowerOfTwo((frames+oldCols-1)/oldCols), 8)
		oldPixels := (frames + oldCols*oldRows - 1) / (oldCols * oldRows) * (oldCols * 512) * (oldRows * 256)
		newPixels := (frames + p.capacity() - 1) / p.capacity() * p.width * p.height
		if newPixels > oldPixels {
			t.Fatalf("packing grew %d frames: %d > %d", frames, newPixels, oldPixels)
		}
		if frames > 1 && newPixels >= oldPixels {
			t.Fatalf("repeated empty rows remain: %d frames, %+v", frames, p)
		}
		for frame := range frames {
			uv := p.translation(frame)
			x, y := int(uv[0]*float64(p.width)), int(uv[1]*float64(p.height))
			for _, chart := range charts {
				if x+chart.x+chart.width > p.width || y+chart.y+chart.height > p.height || x%8 != 0 || y%8 != 0 {
					t.Fatalf("frame %d clipped a chart or changed mip alignment", frame)
				}
			}
		}
	}
}

func intCeilSqrt(n int) int {
	i := 1
	for i*i < n {
		i++
	}
	return i
}

func TestHalfBakeTextureFiltersOrdinaryColorWithPremultipliedAlpha(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	src.SetNRGBA(0, 0, color.NRGBA{0, 0, 0, 0})
	src.SetNRGBA(1, 0, color.NRGBA{255, 255, 255, 255})
	src.SetNRGBA(0, 1, color.NRGBA{0, 0, 0, 0})
	src.SetNRGBA(1, 1, color.NRGBA{255, 255, 255, 255})
	if got := halfBakeTexture(src, false).NRGBAAt(0, 0); got != (color.NRGBA{255, 255, 255, 128}) {
		t.Fatalf("premultiplied filtering darkened the covered edge: %v", got)
	}
	if got := halfBakeTexture(src, true).NRGBAAt(0, 0); got != (color.NRGBA{128, 128, 128, 128}) {
		t.Fatalf("independent RGB filtering changed: %v", got)
	}

	// Some additive source passes intentionally use RGB even with alpha zero.
	radiance := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for i := 0; i < len(radiance.Pix); i += 4 {
		radiance.Pix[i], radiance.Pix[i+1], radiance.Pix[i+2] = 240, 120, 60
	}
	if got := halfBakeTexture(radiance, true).NRGBAAt(0, 0); got != (color.NRGBA{240, 120, 60, 0}) {
		t.Fatalf("independent-RGB filtering lost zero-alpha radiance: %v", got)
	}
	if got := halfBakeTexture(image.NewNRGBA(image.Rect(0, 0, 1, 1)), false).Bounds(); got.Dx() != 1 || got.Dy() != 1 {
		t.Fatal("small texture disappeared")
	}
}

func TestHalfBakeRGBFilteringFollowsOutputBlendSemantics(t *testing.T) {
	for _, tt := range []struct {
		blend  uint16
		screen bool
		want   bool
	}{
		{blend: 0, want: true}, // BlendNone
		{blend: 1},             // Transparent
		{blend: 2},             // Blend
		{blend: 3},             // Additive uses source alpha
		{blend: 4},             // AddAlpha uses source alpha
		{blend: 5, want: true}, // Modulate
		{blend: 6, want: true}, // Modulate2x
		{blend: 7},             // Blend decomposition
		{blend: 2, want: true, screen: true},
	} {
		if got := m2BakeRGBIndependent(tt.blend, tt.screen); got != tt.want {
			t.Errorf("M2 blend %d screen=%v: independent RGB = %v, want %v", tt.blend, tt.screen, got, tt.want)
		}
	}
	for _, tt := range []struct {
		mode components.ParticleFilterMode
		want bool
	}{
		{mode: components.PFilterBlend},
		{mode: components.PFilterAdditive},
		{mode: components.PFilterAlphaKey},
		{mode: components.PFilterModulate, want: true},
		{mode: components.PFilterModulate2x, want: true},
	} {
		if got := particleBakeRGBIndependent(tt.mode); got != tt.want {
			t.Errorf("particle filter %s: independent RGB = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestRegisterBakeTextureHalfResolutionKeepsFullDefault(t *testing.T) {
	atlas := image.NewNRGBA(image.Rect(0, 0, 32, 16))
	for _, scale := range []float64{0, 1, .5} {
		result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
		tex, err := registerBakeTexture(config.Config{TextureBaking: config.TextureBakingOptions{ResolutionScale: scale}}, &result, atlas)
		if err != nil {
			t.Fatal(err)
		}
		src, _ := texturesource.Get(tex.WowData.PngPath)
		decoded, err := png.DecodeConfig(bytes.NewReader(src.PNG))
		texturesource.Unregister(tex.WowData.PngPath)
		if err != nil {
			t.Fatal(err)
		}
		wantW, wantH := 32, 16
		if scale == .5 {
			wantW, wantH = 16, 8
		}
		if decoded.Width != wantW || decoded.Height != wantH {
			t.Fatalf("scale %v: %dx%d", scale, decoded.Width, decoded.Height)
		}
	}
}

func TestCompactBakeRadianceOnlyCompactsZeroRGB(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 128, 64))
	for i := 3; i < len(src.Pix); i += 4 {
		src.Pix[i] = uint8(i)
	}
	if got := compactBakeRadiance(src); got.Bounds().Dx() != 1 || got.Bounds().Dy() != 1 || got.NRGBAAt(0, 0) != (color.NRGBA{}) {
		t.Fatal("zero additive output retained a full atlas")
	}
	src.Pix[0] = 1 // Even zero-alpha RGB can be radiance; preserve it exactly.
	if compactBakeRadiance(src) != src {
		t.Fatal("nonzero radiance was discarded")
	}
}

func TestRegisterBakeTextureSeparatesOpaqueAndBlendedOutputFiles(t *testing.T) {
	atlas := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	ordinary, err := registerBakeTexture(config.Config{}, &result, atlas)
	if err != nil {
		t.Fatal(err)
	}
	opaque, err := registerBakeTexture(config.Config{}, &result, atlas, bakeTextureOptions{ignoreAlpha: true})
	if err != nil {
		t.Fatal(err)
	}
	defer texturesource.Unregister(ordinary.WowData.PngPath)
	defer texturesource.Unregister(opaque.WowData.PngPath)
	plainSource, _ := texturesource.Get(ordinary.WowData.PngPath)
	opaqueSource, _ := texturesource.Get(opaque.WowData.PngPath)
	if ordinary.WowData.PngPath == opaque.WowData.PngPath || plainSource.IgnoreAlpha || !opaqueSource.IgnoreAlpha || !bytes.Equal(plainSource.PNG, opaqueSource.PNG) {
		t.Fatal("opaque storage changed PNG pixels or aliased a blended output file")
	}
}
