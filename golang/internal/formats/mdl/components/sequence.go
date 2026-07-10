package components

import (
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/math"
)

type SequenceData struct {
	WC3Name      string
	WowName      string
	WowVariant   int
	WowFrequency float64
	AttackTag    string
	Loop         *bool
}

type Sequence struct {
	Bound
	Name       string
	Interval   [2]int
	NonLooping bool
	MoveSpeed  float64
	Data       SequenceData
	Rarity     *int
	Keep       bool
}

func SequencesToString(sequences []Sequence) string {
	if len(sequences) == 0 {
		return ""
	}

	animNameCount := map[string]int{}
	seqName := map[int]string{}
	for i, seq := range sequences {
		animNameCount[seq.Name]++
		seqName[i] = seq.Name + " " + FVal(float64(animNameCount[seq.Name]))
	}

	var b strings.Builder
	b.WriteString("Sequences ")
	b.WriteString(FVal(float64(len(sequences))))
	b.WriteString(" {\n")
	for i, sequence := range sequences {
		b.WriteString("Anim \"")
		b.WriteString(seqName[i])
		b.WriteString("\" {\n")
		b.WriteString("Interval { ")
		b.WriteString(FVal(float64(sequence.Interval[0])))
		b.WriteString(", ")
		b.WriteString(FVal(float64(sequence.Interval[1])))
		b.WriteString(" },\n")
		if sequence.NonLooping {
			b.WriteString("NonLooping,\n")
		}
		if sequence.MoveSpeed > 0 {
			b.WriteString("MoveSpeed ")
			b.WriteString(strconv.FormatFloat(sequence.MoveSpeed, 'f', -1, 64))
			b.WriteString(",\n")
		}
		if sequence.Rarity != nil && *sequence.Rarity > 0 {
			b.WriteString("Rarity ")
			b.WriteString(FVal(float64(*sequence.Rarity)))
			b.WriteString(",\n")
		}
		b.WriteString("MinimumExtent { ")
		b.WriteString(FVector3(sequence.MinimumExtent))
		b.WriteString(" },\n")
		b.WriteString("MaximumExtent { ")
		b.WriteString(FVector3(sequence.MaximumExtent))
		b.WriteString(" },\n")
		b.WriteString("BoundsRadius ")
		b.WriteString(FVal(sequence.BoundsRadius))
		b.WriteString(",\n")
		b.WriteString("}\n")
	}
	b.WriteString("}")
	return b.String()
}

func CopyVector3(v math.Vector3) math.Vector3 {
	return math.Vector3{v[0], v[1], v[2]}
}
