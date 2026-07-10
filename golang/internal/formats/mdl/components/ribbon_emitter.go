package components

import (
	"strings"

	"github.com/pqhuy98/wow-converter/internal/math"
)

type RibbonEmitter struct {
	NodeBase
	HeightAbove  *AnimatedOrStatic[float64]
	HeightBelow  *AnimatedOrStatic[float64]
	Alpha        *AnimatedOrStatic[float64]
	Color        *AnimatedOrStatic[math.Vector3]
	TextureSlot  *AnimatedOrStatic[float64]
	Visibility   *Animation
	EmissionRate float64
	LifeSpan     float64
	Rows         int
	Columns      int
	MaterialID   int
	Gravity      float64
}

func NewRibbonEmitter(name string) *RibbonEmitter {
	return &RibbonEmitter{NodeBase: NodeBase{Name: name, Type: "RibbonEmitter"}}
}

func RibbonEmittersToString(ribbons []*RibbonEmitter) string {
	if len(ribbons) == 0 {
		return ""
	}
	var blocks []string
	for _, e := range ribbons {
		var b strings.Builder
		b.WriteString("RibbonEmitter \"")
		b.WriteString(e.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&e.NodeBase))
		if e.HeightAbove != nil {
			b.WriteString(AnimatedValueToString("HeightAbove", e.HeightAbove))
			b.WriteString("\n")
		}
		if e.HeightBelow != nil {
			b.WriteString(AnimatedValueToString("HeightBelow", e.HeightBelow))
			b.WriteString("\n")
		}
		if e.Alpha != nil {
			b.WriteString(AnimatedValueToString("Alpha", e.Alpha))
			b.WriteString("\n")
		}
		if e.Color != nil {
			b.WriteString(AnimatedValueToString("Color", e.Color))
			b.WriteString("\n")
		}
		if e.TextureSlot != nil {
			b.WriteString(AnimatedValueToString("TextureSlot", e.TextureSlot))
			b.WriteString("\n")
		}
		b.WriteString(AnimationToString("Visibility", e.Visibility))
		b.WriteString("\n")
		b.WriteString("EmissionRate ")
		b.WriteString(FVal(e.EmissionRate))
		b.WriteString(",\n")
		b.WriteString("LifeSpan ")
		b.WriteString(FVal(e.LifeSpan))
		b.WriteString(",\n")
		if e.Gravity != 0 {
			b.WriteString("Gravity ")
			b.WriteString(FVal(e.Gravity))
			b.WriteString(",\n")
		}
		b.WriteString("Rows ")
		b.WriteString(FVal(float64(e.Rows)))
		b.WriteString(",\n")
		b.WriteString("Columns ")
		b.WriteString(FVal(float64(e.Columns)))
		b.WriteString(",\n")
		b.WriteString("MaterialID ")
		b.WriteString(FVal(float64(e.MaterialID)))
		b.WriteString(",\n")
		b.WriteString(NodeAnimations(&e.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}
