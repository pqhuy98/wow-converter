package components

import (
	"strings"
)

type TextureWowData struct {
	Type    int
	PngPath string
}

type Texture struct {
	ID            int
	ReplaceableID *int
	Image         string
	WrapWidth     bool
	WrapHeight    bool
	WowData       TextureWowData
}

func TexturesToString(textures []*Texture) string {
	if len(textures) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Textures ")
	b.WriteString(FVal(float64(len(textures))))
	b.WriteString(" {\n")
	for _, texture := range textures {
		if texture == nil {
			continue
		}
		b.WriteString("Bitmap {\n")
		b.WriteString("Image \"")
		b.WriteString(strings.ReplaceAll(texture.Image, "/", "\\"))
		b.WriteString("\",\n")
		if texture.ReplaceableID != nil {
			b.WriteString("ReplaceableId ")
			b.WriteString(FVal(float64(*texture.ReplaceableID)))
			b.WriteString(",\n")
		}
		if texture.WrapWidth {
			b.WriteString("WrapWidth,\n")
		}
		if texture.WrapHeight {
			b.WriteString("WrapHeight,\n")
		}
		b.WriteString("}\n")
	}
	b.WriteString("}")
	return b.String()
}
