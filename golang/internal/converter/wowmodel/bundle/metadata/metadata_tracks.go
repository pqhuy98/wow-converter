package metadata

import (
	"math"

	"github.com/pqhuy98/wow-converter/internal/config"
	bundleutils "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/utils"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

type m2TrackRaw = Track

type m2AnimMeta struct {
	Duration uint32
}

func (f *File) getGlobalSeq(id int) *components.GlobalSequence {
	if f.globalSequenceMap == nil {
		f.globalSequenceMap = map[int]*components.GlobalSequence{}
	}
	if gs, ok := f.globalSequenceMap[id]; ok {
		return gs
	}
	created := components.NewGlobalSequence(id, 1)
	if id >= 0 && id < len(f.globalLoops) {
		created.Duration = int(f.globalLoops[id])
	}
	gs := &created
	f.globalSequenceMap[id] = gs
	if f.mdl != nil {
		f.mdl.GlobalSequences = append(f.mdl.GlobalSequences, gs)
	}
	return gs
}

func (f *File) sequenceDuration(animID int) (int, bool) {
	if f.Animation != nil && animID < len(f.Animation.Animations) {
		return int(f.Animation.Animations[animID].Duration), true
	}
	if animID < len(f.m2Animations) {
		return int(f.m2Animations[animID].Duration), true
	}
	return 0, false
}

func (f *File) m2TrackToAnimation(track m2TrackRaw, animType components.AnimationType, transform func([]float64) any) *components.Animation {
	result := &components.Animation{
		Interpolation: bundleutils.WowToWc3Interpolation(track.Interpolation),
		KeyFrames:     map[int]any{},
		Type:          animType,
	}
	if int(track.GlobalSeq) != config.BlizzardNull {
		result.GlobalSeq = f.getGlobalSeq(int(track.GlobalSeq))
	}

	accum := 0
	for animID, timestamps := range track.Timestamps {
		if result.GlobalSeq != nil {
			if timestamps == nil {
				continue
			}
			for ti, ts := range timestamps {
				if ts == nil {
					continue
				}
				var value []float64
				if animID < len(track.Values) && ti < len(track.Values[animID]) {
					value = track.Values[animID][ti]
				}
				result.KeyFrames[int(*ts)] = transform(value)
			}
			continue
		}
		duration, ok := f.sequenceDuration(animID)
		if !ok {
			continue
		}
		if timestamps == nil {
			accum += duration + 1
			continue
		}
		maxTS := -1
		for ti, ts := range timestamps {
			if ts == nil {
				continue
			}
			var value []float64
			if animID < len(track.Values) && ti < len(track.Values[animID]) {
				value = track.Values[animID][ti]
			}
			t := int(*ts) + accum
			result.KeyFrames[t] = transform(value)
			if t > maxTS {
				maxTS = t
			}
		}
		if maxTS >= 0 && result.GlobalSeq == nil {
			if v, ok := result.KeyFrames[maxTS]; ok {
				result.KeyFrames[accum+duration] = v
			}
		}
		accum += duration + 1
	}
	if len(result.KeyFrames) == 0 {
		return nil
	}
	return result
}

func (f *File) buildTextureAnims() []components.TextureAnim {
	if len(f.textureTransforms) == 0 || f.mdl == nil {
		return nil
	}
	anims := make([]components.TextureAnim, len(f.textureTransforms))
	for i, transform := range f.textureTransforms {
		ta := components.TextureAnim{ID: i}
		{
			track := transform.Translation
			ta.Translation = f.m2TrackToAnimation(track, components.AnimTypeOthers, func(v []float64) any {
				if len(v) < 3 {
					return imath.Vector3{}
				}
				return imath.Vector3{v[0], v[1], v[2]}
			})
		}
		{
			track := transform.Rotation
			ta.Rotation = f.m2TrackToAnimation(track, components.AnimTypeRotation, func(v []float64) any {
				if len(v) < 4 {
					return imath.QuatNoRotation()
				}
				return imath.QuaternionRotation{v[0], v[1], v[2], v[3]}
			})
		}
		{
			track := transform.Scaling
			ta.Scaling = f.m2TrackToAnimation(track, components.AnimTypeScaling, func(v []float64) any {
				if len(v) < 3 {
					return imath.Vector3{1, 1, 1}
				}
				return imath.Vector3{v[0], v[1], v[2]}
			})
		}
		anims[i] = ta
	}
	f.mdl.TextureAnims = anims
	return anims
}

// TextureWeight returns the native unit weight on the same timeline as bones
// and UV tracks. Missing weights are neutral, rather than hiding the surface.
func (f *File) TextureWeight(combo int) components.AnimatedOrStatic[float64] {
	neutral := components.AnimatedOrStatic[float64]{Static: true, Value: 1}
	if combo < 0 || combo >= len(f.transparencyLookup) {
		return neutral
	}
	index := int(f.transparencyLookup[combo])
	if index >= len(f.textureWeights) {
		return neutral
	}
	track := f.textureWeights[index]
	hasValue := false
	for _, row := range track.Values {
		for _, value := range row {
			hasValue = hasValue || len(value) > 0
		}
	}
	if !hasValue {
		return neutral
	}
	return f.m2trackToAnimationOrStaticFloat(track, components.AnimTypeAlpha, func(v []float64) any {
		if len(v) == 0 {
			return float64(1)
		}
		return v[0] / 32767
	})
}

func textureTransformIndex(lookup []int, comboIndex, layerIndex int) int {
	idx := comboIndex + layerIndex
	if idx < 0 || idx >= len(lookup) {
		return config.BlizzardNull
	}
	return lookup[idx]
}

func shouldDisableTextureTransform(shaderID, textureCount, layerIndex int) bool {
	envBit := shaderID&0x80 != 0
	envComboBit := shaderID&0x08 != 0
	usesT2 := textureCount > 1 && !envBit && !envComboBit && shaderID&0x4000 != 0
	if layerIndex == 0 && envBit {
		return true
	}
	if layerIndex == 1 && !usesT2 {
		return true
	}
	return layerIndex > 1
}

func trackRawFromM2(t m2.Track) m2TrackRaw {
	raw := m2TrackRaw{
		GlobalSeq:     t.GlobalSeq,
		Interpolation: t.Interpolation,
		Values:        t.Values,
	}
	for _, row := range t.Timestamps {
		cols := make([]*uint32, len(row))
		for j, v := range row {
			vv := v
			cols[j] = &vv
		}
		raw.Timestamps = append(raw.Timestamps, cols)
	}
	return raw
}

func trackIsStatic(t m2.Track) bool {
	if len(t.Values) != 1 {
		return false
	}
	return len(t.Values[0]) == 1
}

func (f *File) m2trackToAnimation(track m2.Track, animType components.AnimationType, transform func([]float64) any) *components.Animation {
	return f.m2TrackToAnimation(trackRawFromM2(track), animType, transform)
}

func (f *File) m2trackToAnimationOrStaticFloat(track m2.Track, animType components.AnimationType, transform func([]float64) any) components.AnimatedOrStatic[float64] {
	if trackIsStatic(track) && len(track.Values[0][0]) > 0 {
		v := transform(track.Values[0][0])
		if fv, ok := v.(float64); ok {
			return components.AnimatedOrStatic[float64]{Static: true, Value: fv}
		}
	}
	if anim := f.m2trackToAnimation(track, animType, transform); anim != nil {
		return components.AnimatedOrStatic[float64]{Static: false, Anim: anim}
	}
	return components.AnimatedOrStatic[float64]{Static: true, Value: 0}
}

func (f *File) m2trackToAnimationOrStaticVec3(track m2.Track, animType components.AnimationType, transform func([]float64) any) components.AnimatedOrStatic[imath.Vector3] {
	if trackIsStatic(track) && len(track.Values[0][0]) > 0 {
		v := transform(track.Values[0][0])
		if fv, ok := v.(imath.Vector3); ok {
			return components.AnimatedOrStatic[imath.Vector3]{Static: true, Value: fv}
		}
	}
	if anim := f.m2trackToAnimation(track, animType, transform); anim != nil {
		return components.AnimatedOrStatic[imath.Vector3]{Static: false, Anim: anim}
	}
	return components.AnimatedOrStatic[imath.Vector3]{Static: true, Value: imath.Vector3{}}
}

func degrees(v []float64) any {
	if len(v) == 0 {
		return float64(0)
	}
	return v[0] * 180 / math.Pi
}
