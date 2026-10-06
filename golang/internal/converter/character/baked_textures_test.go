package character

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestBakedTextureFileDedupKeepsFlagsAndEncodingSemantics(t *testing.T) {
	paths := []string{"baked/uv2/dedup_left.png", "baked/uv2/dedup_right.png", "baked/uv2/dedup_opaque.png", "baked/uv2/dedup_different.png", "baked/uv2/dedup_alpha_unused.png"}
	textures := make([]*components.Texture, len(paths))
	for i, path := range paths {
		payload := []byte{1, 2, 3}
		if i == 3 {
			payload = []byte{1, 2, 4}
		}
		texturesource.Register(path, texturesource.Source{Kind: texturesource.KindPNG, PNG: payload, Opaque: i == 2, PreserveAlpha: true, IgnoreAlpha: i == 4})
		t.Cleanup(func() { texturesource.Unregister(path) })
		textures[i] = &components.Texture{Image: path + ".blp", WrapWidth: i == 1, WowData: components.TextureWowData{Type: -1, PngPath: path}}
	}
	model := mdl.New(mdl.NewMDLOptions{})
	model.Textures = textures
	exporter := CharacterExporter{Models: [][2]interface{}{{model, "test"}}}
	exporter.deduplicateBakedTexturePaths()
	if textures[1].Image != textures[0].Image || textures[1].WowData.PngPath != textures[0].WowData.PngPath || !textures[1].WrapWidth || textures[0].WrapWidth {
		t.Fatal("identical attachment PNGs did not share a file with independent wrap flags")
	}
	if textures[2].WowData.PngPath != paths[2] || textures[3].WowData.PngPath != paths[3] || textures[4].WowData.PngPath != paths[4] {
		t.Fatal("different pixels or alpha encoding semantics were merged")
	}
}
