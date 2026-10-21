package mdl

import (
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

// OptimizeNPCSequences is used only by map NPC export. Keep the earliest
// preferred variants in the existing stable/rarity ordering, deduplicating
// before the four-variant budget so duplicates cannot crowd out real variety.
func (mod *Modify) OptimizeNPCSequences() *Modify {
	animations := mod.MDL.GetAnimated()
	curves := make([]animationCurve, len(animations))
	for i, anim := range animations {
		curves[i] = newAnimationCurve(anim)
	}
	original := mod.MDL.Sequences
	groups := map[string][]int{}
	var kept []components.Sequence
	for i, seq := range original {
		family := npcSequenceFamily(seq.Name)
		poseNames := strings.ToLower(family + " " + seq.Data.WowName)
		for _, source := range seq.Data.SourceAnimations {
			poseNames += " " + strings.ToLower(source.WowName)
		}
		if strings.Contains(poseNames, "mount") || strings.Contains(poseNames, "sleep") {
			continue
		}
		duplicate := false
		for _, previous := range groups[family] {
			if similarNPCSequences(mod.MDL, animations, curves, original[previous], seq) {
				duplicate = true
				break
			}
		}
		if duplicate || len(groups[family]) >= 4 {
			continue
		}
		groups[family] = append(groups[family], i)
		kept = append(kept, seq)
	}
	mod.MDL.Sequences = kept
	// Events refer to sequence objects as well as timestamps. Rebind retained
	// events and discard those belonging to a removed sequence.
	byInterval := map[[2]int]*components.Sequence{}
	for i := range mod.MDL.Sequences {
		seq := &mod.MDL.Sequences[i]
		byInterval[seq.Interval] = seq
	}
	for _, event := range mod.MDL.EventObjects {
		entries := event.Track[:0]
		for _, entry := range event.Track {
			if entry.Sequence != nil {
				if sequence := byInterval[entry.Sequence.Interval]; sequence != nil {
					entry.Sequence = sequence
					entries = append(entries, entry)
				}
			}
		}
		event.Track = entries
	}
	// Prune removed timelines on every channel, including short/static channels
	// that the normal key optimizer intentionally leaves alone.
	for _, anim := range animations {
		if anim.GlobalSeq != nil {
			continue
		}
		for timestamp := range anim.KeyFrames {
			inside := false
			for _, seq := range kept {
				if seq.Interval[0] <= timestamp && timestamp <= seq.Interval[1] {
					inside = true
					break
				}
			}
			if !inside {
				delete(anim.KeyFrames, timestamp)
				delete(anim.InOutTans, timestamp)
			}
		}
	}
	return mod
}

func npcSequenceFamily(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexByte(name, ' '); i >= 0 {
		if _, err := strconv.Atoi(name[i+1:]); err == nil {
			return name[:i]
		}
	}
	return name
}

func similarNPCSequences(m *MDL, animations []*components.Animation, curves []animationCurve, a, b components.Sequence) bool {
	if a.Interval[1]-a.Interval[0] != b.Interval[1]-b.Interval[0] || a.NonLooping != b.NonLooping || a.MoveSpeed != b.MoveSpeed {
		return false
	}
	for _, event := range m.EventObjects {
		var offsetsA, offsetsB []int
		for _, entry := range event.Track {
			if entry.Sequence == nil {
				continue
			}
			if entry.Sequence.Interval == a.Interval {
				offsetsA = append(offsetsA, entry.Offset)
			}
			if entry.Sequence.Interval == b.Interval {
				offsetsB = append(offsetsB, entry.Offset)
			}
		}
		slices.Sort(offsetsA)
		slices.Sort(offsetsB)
		if !slices.Equal(offsetsA, offsetsB) {
			return false
		}
	}
	for i, anim := range animations {
		if anim.GlobalSeq != nil {
			continue
		}
		ca, cb := curves[i].window(a.Interval), curves[i].window(b.Interval)
		if len(ca.times) == 0 && len(cb.times) == 0 {
			continue
		}
		if len(ca.times) == 0 || len(cb.times) == 0 {
			return false
		}
		if anim.Interpolation != components.InterpLinear && anim.Interpolation != components.InterpDontInterp {
			// Preserve nonlinear curves unless both keys and tangents match exactly.
			if len(ca.times) != len(cb.times) {
				return false
			}
			for j, timestamp := range ca.times {
				other := cb.times[j]
				if timestamp-a.Interval[0] != other-b.Interval[0] || !reflect.DeepEqual(ca.values[j], cb.values[j]) || anim.InOutTans[timestamp] != anim.InOutTans[other] {
					return false
				}
			}
			continue
		}
		// Check the union of original key times plus segment midpoints. This
		// catches differing timing and step changes as well as quaternion arcs.
		offsets := []int{0, a.Interval[1] - a.Interval[0]}
		for _, timestamp := range ca.times {
			offsets = append(offsets, timestamp-a.Interval[0])
		}
		for _, timestamp := range cb.times {
			offsets = append(offsets, timestamp-b.Interval[0])
		}
		slices.Sort(offsets)
		offsets = slices.Compact(offsets)
		tolerance := 0.0001
		switch anim.Type {
		case components.AnimTypeTranslation:
			tolerance = 0.05
		case components.AnimTypeRotation:
			tolerance = 0.1 * math.Pi / 180
		case components.AnimTypeScaling:
			tolerance = 0.001
		}
		matches := func(offset float64) bool {
			va := ca.sample(float64(a.Interval[0])+offset, anim.Interpolation)
			vb := cb.sample(float64(b.Interval[0])+offset, anim.Interpolation)
			return reflect.DeepEqual(va, vb) || animationValueError(va, vb) <= tolerance
		}
		for j, offset := range offsets {
			if !matches(float64(offset)) {
				return false
			}
			if j > 0 && !matches(float64(offsets[j-1]+offset)/2) {
				return false
			}
		}
	}
	return true
}
