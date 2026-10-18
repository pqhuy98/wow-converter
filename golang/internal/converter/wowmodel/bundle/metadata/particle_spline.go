package metadata

import (
	"math"
	"sort"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// Spline generators use the area tracks as a normalized interval along a Bezier
// path, not as a rectangular birth area. PRE2 approximates it with 32 emitters.
func (f *File) particleSplineVariants(node *components.ParticleEmitter2, p m2.ParticleEmitterEntry) []*components.ParticleEmitter2 {
	if p.EmitterType != 3 || len(p.SplinePoints) < 4 {
		return f.particleDirectionVariants(node, p)
	}
	const count = 32
	points := splineParticleSamples(p.SplinePoints)
	start, end := node.Length, node.Width
	// Independent animation clocks cannot share a single PRE2 translation track.
	if !start.Static && !end.Static && (start.Anim.GlobalSeq != end.Anim.GlobalSeq || start.Anim.Interpolation != end.Anim.Interpolation) {
		return []*components.ParticleEmitter2{node}
	}
	times := map[int]bool{}
	for _, area := range []components.AnimatedOrStatic[float64]{start, end} {
		if area.Anim != nil {
			for time := range area.Anim.KeyFrames {
				times[time] = true
			}
		}
	}
	keys := make([]int, 0, len(times))
	for time := range times {
		keys = append(keys, time)
	}
	sort.Ints(keys)
	// A linear area parameter follows a curved path. Sample it rather than
	// interpolating one straight chord between the source's sparse area keys.
	for i := 1; i < len(keys); i++ {
		for time := keys[i-1] + 20; time < keys[i]; time += 20 {
			times[time] = true
		}
	}
	weights := make([]float64, count)
	total := 0.0
	for i := range weights {
		weights[i] = .5 + math.Mod(float64(i)*.61803398875, 1)
		total += weights[i]
	}
	result := make([]*components.ParticleEmitter2, 0, count)
	for i := range count {
		fraction := (float64(i) + .5) / count
		position := func(a, b float64) imath.Vector3 {
			a, b = math.Max(0, math.Min(1, a)), math.Max(0, math.Min(1, b))
			return splineParticlePosition(points, a+(b-a)*fraction)
		}
		variant := mdl.CloneParticleEmitter2(node)
		variant.Name += "_spline_" + itoa(i)
		variant.Width = components.AnimatedOrStatic[float64]{Static: true}
		variant.Length = components.AnimatedOrStatic[float64]{Static: true}
		global := f.constantParticleRotation(imath.QuaternionRotation{0, 0, 0, 1}).GlobalSeq
		variant.Translation = &components.Animation{Type: components.AnimTypeTranslation, Interpolation: components.InterpLinear, GlobalSeq: global, KeyFrames: map[int]any{0: position(start.Value, end.Value)}}
		base := end.Anim
		if base == nil {
			base = start.Anim
		}
		if base != nil {
			animation := *base
			animation.Type = components.AnimTypeTranslation
			animation.Interpolation = components.InterpLinear
			animation.InOutTans = nil
			animation.KeyFrames = map[int]any{}
			if base.Interpolation == components.InterpDontInterp {
				animation.Interpolation = components.InterpDontInterp
			}
			for time := range times {
				animation.KeyFrames[time] = position(splineAreaAt(start, time), splineAreaAt(end, time))
			}
			variant.Translation = &animation
		}
		applyAnimatedOrStaticFloat(&variant.EmissionRate, func(rate float64) float64 { return rate * weights[i] / total })
		result = append(result, variant)
	}
	return result
}

func splineAreaAt(area components.AnimatedOrStatic[float64], time int) float64 {
	if area.Static || area.Anim == nil || len(area.Anim.KeyFrames) == 0 {
		return area.Value
	}
	keys := make([]int, 0, len(area.Anim.KeyFrames))
	for key := range area.Anim.KeyFrames {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	i := sort.SearchInts(keys, time)
	if i == 0 {
		return area.Anim.KeyFrames[keys[0]].(float64)
	}
	if i == len(keys) {
		return area.Anim.KeyFrames[keys[i-1]].(float64)
	}
	a, b := area.Anim.KeyFrames[keys[i-1]].(float64), area.Anim.KeyFrames[keys[i]].(float64)
	if area.Anim.Interpolation == components.InterpDontInterp && time != keys[i] {
		return a
	}
	ratio := float64(time-keys[i-1]) / float64(keys[i]-keys[i-1])
	return a + (b-a)*ratio
}

type splineParticleSample struct {
	position imath.Vector3
	distance float64
}

func splineParticleSamples(points [][3]float32) []splineParticleSample {
	result := []splineParticleSample{{position: imath.Vector3{float64(points[0][0]), float64(points[0][1]), float64(points[0][2])}}}
	for segment := 0; segment+3 < len(points); segment += 3 {
		for step := 1; step <= 20; step++ {
			t := float64(step) / 20
			u := 1 - t
			coefficients := [4]float64{u * u * u, 3 * u * u * t, 3 * u * t * t, t * t * t}
			var position imath.Vector3
			for i, coefficient := range coefficients {
				for axis := range position {
					position[axis] += coefficient * float64(points[segment+i][axis])
				}
			}
			previous := result[len(result)-1]
			dx, dy, dz := position[0]-previous.position[0], position[1]-previous.position[1], position[2]-previous.position[2]
			result = append(result, splineParticleSample{position, previous.distance + math.Sqrt(dx*dx+dy*dy+dz*dz)})
		}
	}
	return result
}

func splineParticlePosition(points []splineParticleSample, fraction float64) imath.Vector3 {
	distance := math.Max(0, math.Min(1, fraction)) * points[len(points)-1].distance
	i := sort.Search(len(points), func(i int) bool { return points[i].distance >= distance })
	if i == 0 {
		return points[0].position
	}
	if i == len(points) {
		return points[i-1].position
	}
	a, b := points[i-1], points[i]
	span := b.distance - a.distance
	if span == 0 {
		return b.position
	}
	ratio := (distance - a.distance) / span
	return imath.Vector3{a.position[0] + (b.position[0]-a.position[0])*ratio, a.position[1] + (b.position[1]-a.position[1])*ratio, a.position[2] + (b.position[2]-a.position[2])*ratio}
}
