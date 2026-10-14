package metadata

import (
	"math"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func (f *File) orientSphereParticle(node *components.ParticleEmitter2, p m2.ParticleEmitterEntry) {
	if p.EmitterType != 2 || p.Flags&0x100 != 0 {
		return
	}
	// Keep the bone frame for full-sphere model-space emitters with gravity.
	// An added basis rotation turns PRE2 acceleration sideways.
	if p.Flags&0x10 != 0 && !particleTrackZero(p.Gravity) && particleTrackMax(p.HorizontalRange) >= math.Pi && particleTrackMax(p.VerticalRange) >= math.Pi {
		return
	}
	// WoW sphere particles point out from the equator, then the client applies
	// a 90-degree Z basis correction. PRE2 points along +Z. This rotation maps
	// PRE2's two angular samples to WoW's azimuth/elevation around local +Y.
	rotation := imath.EulerToQuaternion(imath.EulerRotation{0, math.Pi / 2, math.Pi / 2})
	if particleTrackZero(p.VerticalRange) {
		// A zero elevation is an exact circle, including full 360-degree bursts.
		node.Latitude = f.m2trackToAnimationOrStaticFloat(p.HorizontalRange, components.AnimTypeOthers, degrees)
		node.Flags2 = append(node.Flags2, components.PE2LineEmitter)
	} else if particleTrackZero(p.HorizontalRange) {
		// With zero azimuth, only the elevation varies around local +Y.
		rotation = imath.EulerToQuaternion(imath.EulerRotation{-math.Pi / 2, 0, 0})
		node.Flags2 = append(node.Flags2, components.PE2LineEmitter)
	}
	node.Rotation = f.constantParticleRotation(rotation)
}

func (f *File) particleDirectionVariants(node *components.ParticleEmitter2, p m2.ParticleEmitterEntry) []*components.ParticleEmitter2 {
	if p.EmitterType == 1 && trackIsStatic(p.ZSource) && len(p.ZSource.Values[0][0]) > 0 && p.ZSource.Values[0][0][0] >= 0.001 {
		// Plane generators aim away from (0,0,zSource), independently of their
		// angular tracks. PRE2 approximates the rectangular fan with a cone.
		z := p.ZSource.Values[0][0][0]
		radius := math.Hypot(particleTrackMax(p.EmissionAreaWidth), particleTrackMax(p.EmissionAreaLength)) / 2
		node.Latitude = components.AnimatedOrStatic[float64]{Static: true, Value: math.Atan2(radius, z) * 180 / math.Pi}
		node.Rotation = f.constantParticleRotation(imath.EulerToQuaternion(imath.EulerRotation{math.Pi, 0, 0}))
		if p.Flags&0x10 != 0 {
			// Model-space PRE2 rotates its simulated gravity with the node too.
			applyAnimatedOrStaticFloat(&node.Gravity, func(v float64) float64 { return -v })
		}
		return []*components.ParticleEmitter2{node}
	}
	f.orientSphereParticle(node, p)
	plane := p.EmitterType == 1
	if (!plane && p.EmitterType != 2) || (!plane && p.Flags&0x100 != 0) {
		return []*components.ParticleEmitter2{node}
	}
	horizontal := math.Min(math.Pi, particleTrackMax(p.HorizontalRange))
	vertical := particleTrackMax(p.VerticalRange)
	if plane && horizontal < 1e-6 {
		node.Flags2 = append(node.Flags2, components.PE2LineEmitter)
		return []*components.ParticleEmitter2{node}
	}
	radiusTrack, radiusSupported := sphereParticleRadius(node.Width, node.Length)
	radius := radiusTrack.Value
	if radiusTrack.Anim != nil {
		for _, value := range radiusTrack.Anim.KeyFrames {
			if v, ok := value.(float64); ok {
				radius = max(radius, math.Abs(v))
			}
		}
	}
	if !plane && !radiusSupported {
		return []*components.ParticleEmitter2{node}
	}
	unequal := horizontal > 1e-6 && vertical > 1e-6 && math.Abs(horizontal-vertical) > 1e-6
	// PRE2 cannot couple a spherical spawn point to its outgoing direction.
	// ponytail: sample at most 32 azimuth sectors for visible shells or unequal
	// angular ranges; retain total emission rate and each source animation track.
	if plane && (!node.Width.Static || !node.Length.Static || math.Max(node.Width.Value, node.Length.Value) > 0.2) {
		return []*components.ParticleEmitter2{node}
	}
	if horizontal < 1e-6 || (!plane && !unequal && radius < 0.1) {
		return []*components.ParticleEmitter2{node}
	}
	count := int(math.Ceil(2 * horizontal / (math.Pi / 16)))
	count = max(1, min(32, count))
	// Sparse effects must not be split into clocks that cannot spawn a single
	// particle during its lifetime; that makes warmed impact captures disappear.
	population := maxTrackFloat(p.EmissionRate) * node.LifeSpan
	if population > 0 {
		perSector := 2.0
		// Burst clocks start at zero. Too many fractional-rate sectors delay
		// their first births and make the same elapsed frame look much younger.
		if !trackIsStatic(p.EmissionRate) {
			for _, row := range p.EmissionRate.Values {
				for _, key := range row {
					if len(key) > 0 && key[0] <= 0 {
						perSector = 8
					}
				}
			}
		}
		count = min(count, max(1, int(population/perSector)))
	}
	result := make([]*components.ParticleEmitter2, 0, count)
	weights := make([]float64, count)
	weightSum := 0.0
	for i := range weights {
		weights[i] = 0.5 + math.Mod(float64(i)*0.61803398875, 1)
		weightSum += weights[i]
	}
	for i := 0; i < count; i++ {
		fraction := -1 + 2*(float64(i)+0.5)/float64(count)
		phi := horizontal * fraction
		variant := mdl.CloneParticleEmitter2(node)
		variant.Name += "_direction_" + itoa(i)
		variant.Flags2 = append([]components.ParticleEmitter2Flag(nil), node.Flags2...)
		line := false
		for _, flag := range variant.Flags2 {
			line = line || flag == components.PE2LineEmitter
		}
		if !line {
			variant.Flags2 = append(variant.Flags2, components.PE2LineEmitter)
		}
		variant.Latitude = f.m2trackToAnimationOrStaticFloat(p.VerticalRange, components.AnimTypeOthers, degrees)
		circle := !plane && particleTrackZero(p.VerticalRange)
		if circle {
			variant.Latitude = f.m2trackToAnimationOrStaticFloat(p.HorizontalRange, components.AnimTypeOthers, func(v []float64) any {
				if len(v) == 0 {
					return float64(0)
				}
				return math.Min(math.Pi, math.Abs(v[0])) * 180 / math.Pi / float64(count)
			})
		}
		rotation := func(v []float64) any {
			angle := phi
			if len(v) > 0 {
				angle = math.Min(math.Pi, math.Abs(v[0])) * fraction
			}
			roll := -math.Pi / 2
			if plane {
				roll = 0
			}
			if circle {
				return imath.EulerToQuaternion(imath.EulerRotation{0, math.Pi / 2, angle + math.Pi/2})
			}
			return imath.EulerToQuaternion(imath.EulerRotation{roll, 0, angle})
		}
		if trackIsStatic(p.HorizontalRange) {
			variant.Rotation = f.constantParticleRotation(rotation(p.HorizontalRange.Values[0][0]).(imath.QuaternionRotation))
		} else {
			variant.Rotation = f.m2trackToAnimation(p.HorizontalRange, components.AnimTypeRotation, rotation)
		}
		if variant.Rotation == nil {
			variant.Rotation = f.constantParticleRotation(rotation(nil).(imath.QuaternionRotation))
		}
		if !plane && radius > 0 {
			variant.Width = components.AnimatedOrStatic[float64]{Static: true}
			variant.Length = components.AnimatedOrStatic[float64]{Static: true}
			global := f.constantParticleRotation(rotation(nil).(imath.QuaternionRotation)).GlobalSeq
			offset := func(r float64) imath.Vector3 { return imath.Vector3{-math.Sin(phi) * r, math.Cos(phi) * r, 0} }
			variant.Translation = &components.Animation{Type: components.AnimTypeTranslation, Interpolation: components.InterpDontInterp, GlobalSeq: global, KeyFrames: map[int]any{0: offset(radius)}}
			if radiusTrack.Anim != nil {
				anim := *radiusTrack.Anim
				anim.Type = components.AnimTypeTranslation
				anim.KeyFrames = map[int]any{}
				for time, value := range radiusTrack.Anim.KeyFrames {
					anim.KeyFrames[time] = offset(value.(float64))
				}
				if radiusTrack.Anim.InOutTans != nil {
					anim.InOutTans = map[int]components.InOutTan{}
					for time, tan := range radiusTrack.Anim.InOutTans {
						anim.InOutTans[time] = components.InOutTan{InTan: offset(tan.InTan[0]), OutTan: offset(tan.OutTan[0])}
					}
				}
				variant.Translation = &anim
			}
			if circle {
				// Jitter births along each arc instead of stacking every particle on its center ray.
				arc := 2 * math.Sin(horizontal/float64(count))
				variant.Length = radiusTrack
				if radiusTrack.Anim != nil {
					cloned := *radiusTrack.Anim
					cloned.KeyFrames = map[int]any{}
					for time, value := range radiusTrack.Anim.KeyFrames {
						cloned.KeyFrames[time] = value
					}
					if radiusTrack.Anim.InOutTans != nil {
						cloned.InOutTans = map[int]components.InOutTan{}
						for time, tan := range radiusTrack.Anim.InOutTans {
							tan.InTan[0] *= arc
							tan.OutTan[0] *= arc
							cloned.InOutTans[time] = tan
						}
					}
					variant.Length.Anim = &cloned
				}
				applyAnimatedOrStaticFloat(&variant.Length, func(r float64) float64 { return r * arc })
			}
		}
		// Equal fractional rates synchronize every sector into artificial rings.
		// Deterministic weights stagger those clocks while conserving the source rate.
		applyAnimatedOrStaticFloat(&variant.EmissionRate, func(v float64) float64 { return v * weights[i] / weightSum })
		result = append(result, variant)
	}
	return result
}

func (f *File) constantParticleRotation(rotation imath.QuaternionRotation) *components.Animation {
	// A constant global track keeps the basis active in every sequence.
	var global *components.GlobalSequence
	for _, sequence := range f.mdl.GlobalSequences {
		if sequence.Duration == 1 {
			global = sequence
			break
		}
	}
	if global == nil {
		global = &components.GlobalSequence{Duration: 1}
		f.mdl.GlobalSequences = append(f.mdl.GlobalSequences, global)
	}
	return &components.Animation{Type: components.AnimTypeRotation, Interpolation: components.InterpDontInterp, GlobalSeq: global, KeyFrames: map[int]any{0: rotation}}
}

func particleTrackMax(track m2.Track) float64 {
	result := 0.0
	for _, row := range track.Values {
		for _, v := range row {
			if len(v) > 0 {
				result = math.Max(result, math.Abs(v[0]))
			}
		}
	}
	return result
}

func particleTrackZero(track m2.Track) bool {
	for _, row := range track.Values {
		for _, value := range row {
			if len(value) > 0 && math.Abs(value[0]) > 1e-6 {
				return false
			}
		}
	}
	return true
}

// A spherical emitter's area tracks are radius bounds, not rectangle dimensions.
// ponytail: sample the mean radius on matching clocks/keys; different radius
// clocks retain the existing fallback. Sampling the full radial band needs more sectors.
func sphereParticleRadius(width, length components.AnimatedOrStatic[float64]) (components.AnimatedOrStatic[float64], bool) {
	if width.Static && length.Static {
		return components.AnimatedOrStatic[float64]{Static: true, Value: (width.Value + length.Value) / 2}, true
	}
	base, other := width, length
	if base.Static {
		base, other = length, width
	}
	if base.Anim == nil {
		return components.AnimatedOrStatic[float64]{}, false
	}
	if !other.Static && (other.Anim == nil || other.Anim.GlobalSeq != base.Anim.GlobalSeq || other.Anim.Interpolation != base.Anim.Interpolation || len(other.Anim.KeyFrames) != len(base.Anim.KeyFrames)) {
		return components.AnimatedOrStatic[float64]{}, false
	}
	anim := *base.Anim
	anim.KeyFrames = map[int]any{}
	if base.Anim.InOutTans != nil {
		anim.InOutTans = map[int]components.InOutTan{}
	}
	for time, value := range base.Anim.KeyFrames {
		second := other.Value
		if !other.Static {
			v, ok := other.Anim.KeyFrames[time].(float64)
			if !ok {
				return components.AnimatedOrStatic[float64]{}, false
			}
			second = v
		}
		anim.KeyFrames[time] = (value.(float64) + second) / 2
		if anim.InOutTans != nil {
			tan := base.Anim.InOutTans[time]
			otherTan := components.InOutTan{}
			if !other.Static {
				otherTan = other.Anim.InOutTans[time]
			} else if anim.Interpolation == components.InterpBezier {
				otherTan.InTan[0], otherTan.OutTan[0] = other.Value, other.Value
			}
			tan.InTan[0] = (tan.InTan[0] + otherTan.InTan[0]) / 2
			tan.OutTan[0] = (tan.OutTan[0] + otherTan.OutTan[0]) / 2
			anim.InOutTans[time] = tan
		}
	}
	return components.AnimatedOrStatic[float64]{Anim: &anim}, true
}
