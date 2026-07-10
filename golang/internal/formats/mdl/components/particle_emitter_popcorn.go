package components

import (
	"strings"

	"github.com/pqhuy98/wow-converter/internal/math"
)

type ParticleEmitterPopcornFlag string

const (
	PopcornUnshaded     ParticleEmitterPopcornFlag = "Unshaded"
	PopcornSortPrimsFarZ ParticleEmitterPopcornFlag = "SortPrimsFarZ"
	PopcornUnfogged     ParticleEmitterPopcornFlag = "Unfogged"
)

type ParticleEmitterPopcorn struct {
	NodeBase
	FlagsPop                 []ParticleEmitterPopcornFlag
	LifeSpan                 *AnimatedOrStatic[float64]
	EmissionRate             *AnimatedOrStatic[float64]
	Speed                    *AnimatedOrStatic[float64]
	Color                    *AnimatedOrStatic[math.Vector3]
	Alpha                    *AnimatedOrStatic[float64]
	Visibility               *Animation
	ReplaceableID            *int
	Path                     string
	AnimationVisibilityGuide string
}

func NewParticleEmitterPopcorn(name string) *ParticleEmitterPopcorn {
	return &ParticleEmitterPopcorn{NodeBase: NodeBase{Name: name, Type: "ParticleEmitterPopcorn"}}
}

func ParticleEmitterPopcornsToString(emitters []*ParticleEmitterPopcorn) string {
	var blocks []string
	for _, e := range emitters {
		var b strings.Builder
		b.WriteString("ParticleEmitterPopcorn \"")
		b.WriteString(e.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&e.NodeBase))
		for _, flag := range e.FlagsPop {
			b.WriteString(string(flag))
			b.WriteString(",\n")
		}
		if e.LifeSpan != nil {
			b.WriteString(AnimatedValueToString("LifeSpan", e.LifeSpan))
			b.WriteString("\n")
		}
		if e.EmissionRate != nil {
			b.WriteString(AnimatedValueToString("EmissionRate", e.EmissionRate))
			b.WriteString("\n")
		}
		if e.Speed != nil {
			b.WriteString(AnimatedValueToString("Speed", e.Speed))
			b.WriteString("\n")
		}
		if e.Color != nil {
			b.WriteString(AnimatedValueToString("Color", e.Color))
			b.WriteString("\n")
		}
		if e.Alpha != nil {
			b.WriteString(AnimatedValueToString("Alpha", e.Alpha))
			b.WriteString("\n")
		}
		b.WriteString(AnimationToString("Visibility", e.Visibility))
		b.WriteString("\n")
		if e.ReplaceableID != nil {
			b.WriteString("ReplaceableId ")
			b.WriteString(FVal(float64(*e.ReplaceableID)))
			b.WriteString(",\n")
		}
		if e.Path != "" {
			b.WriteString("Path \"")
			b.WriteString(e.Path)
			b.WriteString("\",\n")
		}
		if e.AnimationVisibilityGuide != "" {
			b.WriteString("AnimVisibilityGuide \"")
			b.WriteString(e.AnimationVisibilityGuide)
			b.WriteString("\",\n")
		}
		b.WriteString(NodeAnimations(&e.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}
