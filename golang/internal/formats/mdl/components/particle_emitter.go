package components

import (
	"strings"
)

type ParticleEmitterFlag string

const (
	ParticleEmitterUsesMDL ParticleEmitterFlag = "EmitterUsesMDL"
	ParticleEmitterUsesTGA ParticleEmitterFlag = "EmitterUsesTGA"
)

type ParticleEmitter struct {
	NodeBase
	EmitterFlags []ParticleEmitterFlag
	EmissionRate *AnimatedOrStatic[float64]
	Gravity      *AnimatedOrStatic[float64]
	Longitude    *AnimatedOrStatic[float64]
	Latitude     *AnimatedOrStatic[float64]
	Visibility   *Animation
	LifeSpan     *AnimatedOrStatic[float64]
	Speed        *AnimatedOrStatic[float64]
	Path         string
}

func NewParticleEmitter(name string) *ParticleEmitter {
	return &ParticleEmitter{NodeBase: NodeBase{Name: name, Type: "ParticleEmitter"}}
}

func ParticleEmittersToString(particleEmitters []*ParticleEmitter) string {
	var blocks []string
	for _, emitter := range particleEmitters {
		var b strings.Builder
		b.WriteString("ParticleEmitter \"")
		b.WriteString(emitter.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&emitter.NodeBase))
		for _, flag := range emitter.EmitterFlags {
			b.WriteString(string(flag))
			b.WriteString(",\n")
		}
		if emitter.EmissionRate != nil {
			b.WriteString(AnimatedValueToString("EmissionRate", emitter.EmissionRate))
			b.WriteString("\n")
		}
		if emitter.Gravity != nil {
			b.WriteString(AnimatedValueToString("Gravity", emitter.Gravity))
			b.WriteString("\n")
		}
		if emitter.Longitude != nil {
			b.WriteString(AnimatedValueToString("Longitude", emitter.Longitude))
			b.WriteString("\n")
		}
		if emitter.Latitude != nil {
			b.WriteString(AnimatedValueToString("Latitude", emitter.Latitude))
			b.WriteString("\n")
		}
		b.WriteString(AnimationToString("Visibility", emitter.Visibility))
		b.WriteString("\nParticle {\n")
		if emitter.LifeSpan != nil {
			b.WriteString(AnimatedValueToString("LifeSpan", emitter.LifeSpan))
			b.WriteString("\n")
		}
		if emitter.Speed != nil {
			b.WriteString(AnimatedValueToString("InitVelocity", emitter.Speed))
			b.WriteString("\n")
		}
		if needsParticleEmitterPath(emitter) && emitter.Path != "" {
			b.WriteString("Path \"")
			b.WriteString(emitter.Path)
			b.WriteString("\",\n")
		}
		b.WriteString("}\n")
		b.WriteString(NodeAnimations(&emitter.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}

func needsParticleEmitterPath(emitter *ParticleEmitter) bool {
	for _, flag := range emitter.EmitterFlags {
		if flag == ParticleEmitterUsesMDL || flag == ParticleEmitterUsesTGA {
			return true
		}
	}
	return false
}
