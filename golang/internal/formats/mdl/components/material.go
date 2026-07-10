package components

import (
	"strings"
)

type BlendMode string

const (
	BlendNone      BlendMode = "None"
	BlendTransparent BlendMode = "Transparent"
	BlendBlend     BlendMode = "Blend"
	BlendAdditive  BlendMode = "Additive"
	BlendAddAlpha  BlendMode = "AddAlpha"
	BlendModulate  BlendMode = "Modulate"
	BlendModulate2x BlendMode = "Modulate2x"
)

type Layer struct {
	FilterMode   BlendMode
	Texture      *Texture
	TVertexAnim  *TextureAnim
	Alpha        AnimatedOrStatic[float64]
	CoordID      *int
	Unshaded     bool
	SphereEnvMap bool
	TwoSided     bool
	Unfogged     bool
	Unlit        bool
	NoDepthTest  bool
	NoDepthSet   bool
}

type Material struct {
	ID            int
	ConstantColor bool
	TwoSided      bool
	Layers        []Layer
}

func MaterialsToString(version int, materials []*Material) string {
	if len(materials) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Materials ")
	b.WriteString(FVal(float64(len(materials))))
	b.WriteString(" {\n")
	for _, material := range materials {
		if material == nil {
			continue
		}
		b.WriteString("Material {\n")
		if material.ConstantColor {
			b.WriteString("ConstantColor,\n")
		}
		for _, layer := range material.Layers {
			b.WriteString("Layer {\n")
			b.WriteString("FilterMode ")
			b.WriteString(string(layer.FilterMode))
			b.WriteString(",\n")
			b.WriteString("static TextureID ")
			b.WriteString(FVal(float64(layer.Texture.ID)))
			b.WriteString(",\n")
			if layer.Unshaded {
				b.WriteString("Unshaded,\n")
			}
			if layer.SphereEnvMap {
				b.WriteString("SphereEnvMap,\n")
			}
			if layer.TwoSided {
				b.WriteString("TwoSided,\n")
			}
			if layer.Unfogged {
				b.WriteString("Unfogged,\n")
			}
			if layer.NoDepthTest {
				b.WriteString("NoDepthTest,\n")
			}
			if layer.NoDepthSet {
				b.WriteString("NoDepthSet,\n")
			}
			if version > 800 && layer.Unlit {
				b.WriteString("Unlit,\n")
			}
			if layer.CoordID != nil && *layer.CoordID != 0 {
				b.WriteString("CoordId ")
				b.WriteString(FVal(float64(*layer.CoordID)))
				b.WriteString(",\n")
			}
			if layer.TVertexAnim != nil {
				b.WriteString("TVertexAnimId ")
				b.WriteString(FVal(float64(layer.TVertexAnim.ID)))
				b.WriteString(",\n")
			}
			b.WriteString(AnimatedValueToString("Alpha", layer.Alpha))
			b.WriteString("\n")
			b.WriteString("}\n")
		}
		b.WriteString("}\n")
	}
	b.WriteString("}")
	return b.String()
}
