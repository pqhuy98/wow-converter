package components

import (
	"strings"

	"github.com/pqhuy98/wow-converter/internal/math"
)

type ParticleEmitter2Flag string

const (
	PE2Unshaded       ParticleEmitter2Flag = "Unshaded"
	PE2SortPrimsFarZ  ParticleEmitter2Flag = "SortPrimsFarZ"
	PE2LineEmitter    ParticleEmitter2Flag = "LineEmitter"
	PE2Unfogged       ParticleEmitter2Flag = "Unfogged"
	PE2ModelSpace     ParticleEmitter2Flag = "ModelSpace"
	PE2XYQuad         ParticleEmitter2Flag = "XYQuad"
)

type ParticleFilterMode string

const (
	PFilterBlend      ParticleFilterMode = "Blend"
	PFilterAdditive   ParticleFilterMode = "Additive"
	PFilterModulate   ParticleFilterMode = "Modulate"
	PFilterModulate2x ParticleFilterMode = "Modulate2x"
	PFilterAlphaKey   ParticleFilterMode = "AlphaKey"
)

type HeadOrTail string

const (
	HeadOrTailHead HeadOrTail = "Head"
	HeadOrTailTail HeadOrTail = "Tail"
	HeadOrTailBoth HeadOrTail = "Both"
)

type ParticleEmitter2 struct {
	NodeBase
	Flags2            []ParticleEmitter2Flag
	Visibility        *Animation
	Width             AnimatedOrStatic[float64]
	Length            AnimatedOrStatic[float64]
	EmissionRate      AnimatedOrStatic[float64]
	Speed             AnimatedOrStatic[float64]
	Variation         AnimatedOrStatic[float64]
	Latitude          AnimatedOrStatic[float64]
	HeadOrTail        HeadOrTail
	TailLength        float64
	LifeSpan          float64
	TimeMiddle        float64
	SegmentColors     [3]math.Vector3
	SegmentAlphas     [3]float64
	SegmentScaling    [3]float64
	Texture           *Texture
	ReplaceableID     *int
	Rows              int
	Columns           int
	HeadIntervals     [3]float64
	DecayIntervals    [3]float64
	TailIntervals     [3]float64
	TailDecayIntervals [3]float64
	Squirt            bool
	PriorityPlane     *int
	FilterMode        ParticleFilterMode
	Gravity           AnimatedOrStatic[float64]
}

func NewParticleEmitter2(name string) *ParticleEmitter2 {
	return &ParticleEmitter2{NodeBase: NodeBase{Name: name, Type: "ParticleEmitter2"}}
}

func ParticleEmitter2sToString(emitters []*ParticleEmitter2) string {
	var blocks []string
	for _, e := range emitters {
		var b strings.Builder
		b.WriteString("ParticleEmitter2 \"")
		b.WriteString(e.Name)
		b.WriteString("\" {\n")
		b.WriteString(NodeHeaders(&e.NodeBase))
		for _, flag := range e.Flags2 {
			b.WriteString(string(flag))
			b.WriteString(",\n")
		}
		b.WriteString(AnimatedValueToString("Speed", e.Speed))
		b.WriteString("\n")
		b.WriteString(AnimatedValueToString("Variation", e.Variation))
		b.WriteString("\n")
		b.WriteString(AnimatedValueToString("Latitude", e.Latitude))
		b.WriteString("\n")
		b.WriteString(AnimatedValueToString("Gravity", e.Gravity))
		b.WriteString("\n")
		b.WriteString(AnimationToString("Visibility", e.Visibility))
		b.WriteString("\n")
		if e.Squirt {
			b.WriteString("Squirt,\n")
		}
		b.WriteString("LifeSpan ")
		b.WriteString(FVal(e.LifeSpan))
		b.WriteString(",\n")
		b.WriteString(AnimatedValueToString("EmissionRate", e.EmissionRate))
		b.WriteString("\n")
		b.WriteString(AnimatedValueToString("Width", e.Width))
		b.WriteString("\n")
		b.WriteString(AnimatedValueToString("Length", e.Length))
		b.WriteString("\n\n")
		b.WriteString(string(e.FilterMode))
		b.WriteString(",\n")
		b.WriteString("Rows ")
		b.WriteString(FVal(float64(e.Rows)))
		b.WriteString(",\n")
		b.WriteString("Columns ")
		b.WriteString(FVal(float64(e.Columns)))
		b.WriteString(",\n")
		b.WriteString(string(e.HeadOrTail))
		b.WriteString(",\n")
		b.WriteString("TailLength ")
		b.WriteString(FVal(e.TailLength))
		b.WriteString(",\n")
		b.WriteString("Time ")
		b.WriteString(FVal(e.TimeMiddle))
		b.WriteString(",\n\n")
		b.WriteString("SegmentColor {\n")
		for _, c := range e.SegmentColors {
			b.WriteString("Color { ")
			b.WriteString(FVector3(c))
			b.WriteString(" },\n")
		}
		b.WriteString("},\n")
		b.WriteString("Alpha { ")
		b.WriteString(FVector(e.SegmentAlphas[:]))
		b.WriteString(" },\n")
		b.WriteString("ParticleScaling { ")
		b.WriteString(FVector(e.SegmentScaling[:]))
		b.WriteString(" },\n")
		b.WriteString("LifeSpanUVAnim { ")
		b.WriteString(FVector(e.HeadIntervals[:]))
		b.WriteString(" },\n")
		b.WriteString("DecayUVAnim { ")
		b.WriteString(FVector(e.DecayIntervals[:]))
		b.WriteString(" },\n")
		b.WriteString("TailUVAnim { ")
		b.WriteString(FVector(e.TailIntervals[:]))
		b.WriteString(" },\n")
		b.WriteString("TailDecayUVAnim { ")
		b.WriteString(FVector(e.TailDecayIntervals[:]))
		b.WriteString(" },\n")
		if e.Texture != nil {
			b.WriteString("TextureID ")
			b.WriteString(FVal(float64(e.Texture.ID)))
			b.WriteString(",\n")
		}
		if e.ReplaceableID != nil {
			b.WriteString("ReplaceableId ")
			b.WriteString(FVal(float64(*e.ReplaceableID)))
			b.WriteString(",\n")
		}
		if e.PriorityPlane != nil {
			b.WriteString("PriorityPlane ")
			b.WriteString(FVal(float64(*e.PriorityPlane)))
			b.WriteString(",\n")
		}
		b.WriteString(NodeAnimations(&e.NodeBase))
		b.WriteString("}")
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n")
}
