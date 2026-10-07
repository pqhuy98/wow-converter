package directm2

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

// Wind elemental and void wraiths have more local sequences than the budget can
// fit at the initial raster size. Each sequence must survive budget reduction.
func TestBakeBudgetTerminatesWithManyLocalSequences(t *testing.T) {
	result := ConvertResult{MDL: mdl.New(mdl.NewMDLOptions{}), TexturePaths: map[string]struct{}{}}
	track := &components.Animation{Interpolation: components.InterpLinear, KeyFrames: map[int]any{}}
	for i := range 52 {
		start := i * 5000
		result.MDL.Sequences = append(result.MDL.Sequences, components.Sequence{Interval: [2]int{start, start + 4000}})
		track.KeyFrames[start] = imath.Vector3{}
		track.KeyFrames[start+4000] = imath.Vector3{1, 0, 0}
	}
	g := &components.Geoset{Material: &components.Material{Layers: []components.Layer{{Texture: &components.Texture{}}}}}
	for _, uv := range []imath.Vector2{{0, 0}, {1, 0}, {0, 1}} {
		g.Vertices = append(g.Vertices, &components.GeosetVertex{TexPosition: uv})
	}
	g.Faces = []components.Face{{Vertices: [3]*components.GeosetVertex{g.Vertices[0], g.Vertices[1], g.Vertices[2]}}}
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	program := bakeProgram{shader: m2Shader{coords: [4]m2Coord{coordT1M0}}, count: 1, images: [4]*image.NRGBA{img}, transforms: [2]*components.TextureAnim{{Translation: track}, nil}, pixelBudget: 65536}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := bakeM2Geoset(ctx, config.Config{}, &result, g, program); err != nil {
		t.Fatal(err)
	}
	for _, texture := range result.MDL.Textures {
		defer texturesource.Unregister(texture.WowData.PngPath)
	}
	animation := g.Material.Layers[0].TVertexAnim.Translation
	if animation.GlobalSeq != nil {
		t.Fatal("local sequences were collapsed into a shared global clock")
	}
	for _, sequence := range result.MDL.Sequences {
		if _, ok := animation.KeyFrames[sequence.Interval[0]]; !ok {
			t.Fatalf("lost sequence start %d", sequence.Interval[0])
		}
	}
}
