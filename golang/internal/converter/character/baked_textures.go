package character

import (
	"bytes"
	"crypto/sha256"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

// Independently baked attachments can produce identical PNGs under different
// listfile stems. Share their file without merging texture objects or changing
// wrap flags, material states, UV tracks, or replacement semantics.
func (e *CharacterExporter) deduplicateBakedTexturePaths() {
	type candidate struct {
		texture *components.Texture
		source  texturesource.Source
	}
	seen := map[[32]byte][]candidate{}
	for _, pair := range e.Models {
		m, ok := pair[0].(*mdl.MDL)
		if !ok {
			continue
		}
		for _, tex := range m.Textures {
			if tex == nil || tex.WowData.Type != -1 || !strings.HasPrefix(tex.WowData.PngPath, "baked/") {
				continue
			}
			src, ok := texturesource.Get(tex.WowData.PngPath)
			if !ok || src.Kind != texturesource.KindPNG || src.FileDataID != 0 || len(src.PNG) == 0 {
				continue
			}
			key := sha256.Sum256(src.PNG)
			matched := false
			for _, existing := range seen[key] {
				if src.Opaque == existing.source.Opaque && src.PreserveAlpha == existing.source.PreserveAlpha && src.IgnoreAlpha == existing.source.IgnoreAlpha && bytes.Equal(src.PNG, existing.source.PNG) {
					tex.Image = existing.texture.Image
					tex.WowData.PngPath = existing.texture.WowData.PngPath
					matched = true
					break
				}
			}
			if !matched {
				seen[key] = append(seen[key], candidate{texture: tex, source: src})
			}
		}
	}
}
