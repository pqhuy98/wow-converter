package components

import (
	"strings"

	"github.com/pqhuy98/wow-converter/internal/math"
)

type LightType string

const (
	LightOmnidirectional LightType = "Omnidirectional"
	LightDirectional     LightType = "Directional"
	LightAmbient         LightType = "Ambient"
)

type Light struct {
	NodeBase
	LightType         LightType
	AttenuationStart  AnimatedOrStatic[float64]
	AttenuationEnd    AnimatedOrStatic[float64]
	Intensity         AnimatedOrStatic[float64]
	Color             AnimatedOrStatic[math.Vector3]
	AmbientIntensity  AnimatedOrStatic[float64]
	AmbientColor      AnimatedOrStatic[math.Vector3]
	Visibility        *Animation
}

func NewLight(name string) *Light {
	return &Light{NodeBase: NodeBase{Name: name, Type: "Light"}}
}

func LightsToString(lights []*Light) string {
	var blocks []string
	for _, l := range lights {
		var b strings.Builder
		b.WriteString("Light \"")
		b.WriteString(l.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&l.NodeBase))
		b.WriteString(string(l.LightType))
		b.WriteString(",\n\n")
		b.WriteString(animationOrStaticNumber("AttenuationStart", l.AttenuationStart))
		b.WriteString("\n")
		b.WriteString(animationOrStaticNumber("AttenuationEnd", l.AttenuationEnd))
		b.WriteString("\n")
		b.WriteString(animationOrStaticNumber("Intensity", l.Intensity))
		b.WriteString("\n")
		b.WriteString(animationOrStaticColor("Color", l.Color))
		b.WriteString("\n")
		b.WriteString(animationOrStaticNumber("AmbIntensity", l.AmbientIntensity))
		b.WriteString("\n")
		b.WriteString(animationOrStaticColor("AmbColor", l.AmbientColor))
		b.WriteString("\n")
		b.WriteString(AnimationToString("Visibility", l.Visibility))
		b.WriteString("\n")
		b.WriteString(NodeAnimations(&l.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}

func animationOrStaticNumber(typeName string, value AnimatedOrStatic[float64]) string {
	if value.Static {
		return "static " + typeName + " " + FVal(value.Value) + ","
	}
	return AnimationToString(typeName, value.Anim)
}

func animationOrStaticColor(typeName string, value AnimatedOrStatic[math.Vector3]) string {
	if value.Static {
		return "static " + typeName + " { " + FVector3(value.Value) + " },"
	}
	return AnimationToString(typeName, value.Anim)
}
