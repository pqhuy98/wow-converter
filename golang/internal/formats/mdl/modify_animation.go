package mdl

import (
	"math"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

// FreezeHeadOutsidePortrait keeps the Head transform and portrait expressions,
// freezing only its descendant bones in ordinary sequences. The descendant
// rig can also contain ears/hair; callers expose this as an optional export pass.
func (mod *Modify) FreezeHeadOutsidePortrait() *Modify {
	var portraits [][2]int
	for _, seq := range mod.MDL.Sequences {
		if strings.HasPrefix(strings.ToLower(seq.Name), "portrait") {
			portraits = append(portraits, seq.Interval)
		}
	}
	for _, bone := range mod.MDL.Bones {
		headChild := false
		seen := map[components.Node]bool{}
		for parent := bone.Parent; parent != nil && !seen[parent]; parent = parent.NodeParent() {
			seen[parent] = true
			if strings.EqualFold(parent.NodeName(), "Head") {
				headChild = true
				break
			}
		}
		if !headChild {
			continue
		}
		for _, anim := range []*components.Animation{bone.Translation, bone.Rotation, bone.Scaling} {
			if anim == nil || anim.GlobalSeq != nil || (anim.Interpolation != components.InterpLinear && anim.Interpolation != components.InterpDontInterp) {
				continue
			}
			curve := newAnimationCurve(anim)
			for _, seq := range mod.MDL.Sequences {
				if strings.HasPrefix(strings.ToLower(seq.Name), "portrait") {
					continue
				}
				overlapsPortrait := false
				for _, interval := range portraits {
					if seq.Interval[0] <= interval[1] && interval[0] <= seq.Interval[1] {
						overlapsPortrait = true
						break
					}
				}
				if overlapsPortrait {
					continue
				}
				part := curve.window(seq.Interval)
				if len(part.times) == 0 {
					continue
				}
				first, last := part.times[0], part.times[len(part.times)-1]
				for _, timestamp := range part.times[1:] {
					delete(anim.KeyFrames, timestamp)
				}
				anim.KeyFrames[last] = anim.KeyFrames[first]
			}
		}
	}
	return mod
}

// ReduceKeyFramesConservative bounds error at every original key, using slerp for
// rotations. Independent global tracks are reduced on their own timeline.
// Run after the model's final scaling so tolerances stay in exported units.
func (mod *Modify) ReduceKeyFramesConservative() *Modify {
	for _, anim := range mod.MDL.GetAnimated() {
		if anim.Interpolation != components.InterpLinear || len(anim.KeyFrames) < 3 {
			continue
		}
		tolerance := 0.0
		switch anim.Type {
		case components.AnimTypeTranslation:
			tolerance = 0.05
		case components.AnimTypeRotation:
			tolerance = 0.1 * math.Pi / 180
		case components.AnimTypeScaling:
			tolerance = 0.0001
		default:
			continue
		}
		curve := newAnimationCurve(anim)
		var windows [][2]int
		if anim.GlobalSeq != nil {
			windows = append(windows, [2]int{0, anim.GlobalSeq.Duration})
		} else {
			for _, seq := range mod.MDL.Sequences {
				windows = append(windows, seq.Interval)
			}
		}
		keepTimes := map[int]bool{}
		for _, window := range windows {
			part := curve.window(window)
			if len(part.times) == 0 {
				continue
			}
			keep := make([]bool, len(part.times))
			keep[0], keep[len(keep)-1] = true, true
			stack := [][2]int{{0, len(keep) - 1}}
			for len(stack) > 0 {
				span := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				lo, hi := span[0], span[1]
				worst, at := tolerance, -1
				for i := lo + 1; i < hi; i++ {
					t := float64(part.times[i]-part.times[lo]) / float64(part.times[hi]-part.times[lo])
					expected := interpolateAnimationValue(part.values[lo], part.values[hi], t)
					if error := animationValueError(part.values[i], expected); error > worst {
						worst, at = error, i
					}
				}
				if at >= 0 {
					keep[at] = true
					stack = append(stack, [2]int{lo, at}, [2]int{at, hi})
				}
			}
			for i, timestamp := range part.times {
				keepTimes[timestamp] = keepTimes[timestamp] || keep[i]
			}
		}
		for timestamp, keep := range keepTimes {
			if !keep {
				delete(anim.KeyFrames, timestamp)
			}
		}
	}
	return mod
}

type animationCurve struct {
	times  []int
	values []any
}

func newAnimationCurve(anim *components.Animation) animationCurve {
	c := animationCurve{times: components.SortedKeyInts(anim.KeyFrames)}
	for _, timestamp := range c.times {
		c.values = append(c.values, anim.KeyFrames[timestamp])
	}
	return c
}

func (c animationCurve) window(interval [2]int) animationCurve {
	start := sort.SearchInts(c.times, interval[0])
	end := sort.Search(len(c.times), func(i int) bool { return c.times[i] > interval[1] })
	return animationCurve{times: c.times[start:end], values: c.values[start:end]}
}

func (c animationCurve) sample(timestamp float64, interpolation components.Interpolation) any {
	i := sort.Search(len(c.times), func(i int) bool { return float64(c.times[i]) >= timestamp })
	if i == 0 {
		return c.values[0]
	}
	if i == len(c.times) {
		return c.values[i-1]
	}
	if float64(c.times[i]) == timestamp {
		return c.values[i]
	}
	if interpolation == components.InterpDontInterp {
		return c.values[i-1]
	}
	t := (timestamp - float64(c.times[i-1])) / float64(c.times[i]-c.times[i-1])
	return interpolateAnimationValue(c.values[i-1], c.values[i], t)
}

func interpolateAnimationValue(a, b any, t float64) any {
	switch a := a.(type) {
	case imath.Vector3:
		if b, ok := b.(imath.Vector3); ok {
			return imath.V3Lerp(a, b, t)
		}
	case imath.QuaternionRotation:
		if b, ok := b.(imath.QuaternionRotation); ok {
			a, b = normalizeAnimationQuaternion(a), normalizeAnimationQuaternion(b)
			return imath.V3Slerp(a, b, t)
		}
	case float64:
		if b, ok := b.(float64); ok {
			return a + (b-a)*t
		}
	}
	return nil
}

func normalizeAnimationQuaternion(q imath.QuaternionRotation) imath.QuaternionRotation {
	norm := math.Sqrt(q[0]*q[0] + q[1]*q[1] + q[2]*q[2] + q[3]*q[3])
	if norm == 0 {
		return q
	}
	for i := range q {
		q[i] /= norm
	}
	return q
}

func animationValueError(a, b any) float64 {
	switch a := a.(type) {
	case imath.Vector3:
		if b, ok := b.(imath.Vector3); ok {
			return math.Max(math.Abs(a[0]-b[0]), math.Max(math.Abs(a[1]-b[1]), math.Abs(a[2]-b[2])))
		}
	case imath.QuaternionRotation:
		if b, ok := b.(imath.QuaternionRotation); ok {
			a, b = normalizeAnimationQuaternion(a), normalizeAnimationQuaternion(b)
			dot := math.Abs(a[0]*b[0] + a[1]*b[1] + a[2]*b[2] + a[3]*b[3])
			return 2 * math.Acos(math.Min(1, dot))
		}
	case float64:
		if b, ok := b.(float64); ok {
			return math.Abs(a - b)
		}
	case int:
		if b, ok := b.(int); ok && a == b {
			return 0
		}
	}
	return math.Inf(1)
}
