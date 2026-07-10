package mdl

import (
	"cmp"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

var animPrefixOrder = []string{"Stand", "Walk", "Attack", "Spell", "Morph", "Death", "Decay", "Portrait"}

func getPrefixIndex(str string) int {
	for i, prefix := range animPrefixOrder {
		if strings.HasPrefix(str, prefix) {
			return i
		}
	}
	return len(animPrefixOrder)
}

func animNameAsc(a, b components.Sequence) int {
	if a.Name == b.Name {
		ar, br := 0, 0
		if a.Rarity != nil {
			ar = *a.Rarity
		}
		if b.Rarity != nil {
			br = *b.Rarity
		}
		return cmp.Compare(ar, br)
	}
	indexA := getPrefixIndex(a.Name)
	indexB := getPrefixIndex(b.Name)
	if indexA != indexB {
		return cmp.Compare(indexA, indexB)
	}
	return strings.Compare(a.Name, b.Name)
}

func (mod *Modify) SortSequences() *Modify {
	sort.SliceStable(mod.MDL.Sequences, func(i, j int) bool {
		return animNameAsc(mod.MDL.Sequences[i], mod.MDL.Sequences[j]) < 0
	})
	return mod
}

func (mod *Modify) RemoveWowSequence(wowName string, variant *int) *Modify {
	var filtered []components.Sequence
	for _, s := range mod.MDL.Sequences {
		if s.Data.WowName == wowName && (variant == nil || s.Data.WowVariant == *variant) {
			continue
		}
		filtered = append(filtered, s)
	}
	mod.MDL.Sequences = filtered
	return mod
}

func (mod *Modify) UseWalkSequenceByWowName(wowName WowAnimName) *Modify {
	var seq *components.Sequence
	for i := range mod.MDL.Sequences {
		if mod.MDL.Sequences[i].Data.WowName == string(wowName) {
			seq = &mod.MDL.Sequences[i]
			break
		}
	}
	if seq != nil {
		for i := range mod.MDL.Sequences {
			if mod.MDL.Sequences[i].Name == "Walk" {
				mod.MDL.Sequences[i].Name = "Cinematic " + mod.MDL.Sequences[i].Data.WowName
				break
			}
		}
		seq.Name = "Walk"
	}
	return mod
}

func (mod *Modify) RenameSequencesByWowName(wowName WowAnimName, wc3Name string) *Modify {
	for i := range mod.MDL.Sequences {
		if mod.MDL.Sequences[i].Data.WowName == string(wowName) {
			mod.MDL.Sequences[i].Name = wc3Name
			mod.MDL.Sequences[i].Data.WC3Name = wc3Name
		}
	}
	return mod
}

func (mod *Modify) DebugSequence() *Modify {
	for i := range mod.MDL.Sequences {
		s := &mod.MDL.Sequences[i]
		s.Name = s.Name + " " + s.Data.WowName + " " + components.FVal(float64(s.Data.WowVariant))
	}
	return mod
}

func (mod *Modify) AddEventObjectBySequenceName(name, sequenceName string, offset int) *Modify {
	var event *components.EventObject
	for _, s := range mod.MDL.Sequences {
		if s.Name != sequenceName {
			continue
		}
		if event == nil {
			for _, e := range mod.MDL.EventObjects {
				if e.Name == name {
					event = e
					break
				}
			}
		}
		if event == nil {
			event = components.NewEventObject(name)
			mod.MDL.EventObjects = append(mod.MDL.EventObjects, event)
		}
		event.Track = append(event.Track, components.EventTrackEntry{Sequence: &s, Offset: offset})
	}
	return mod
}

func (mod *Modify) AddDecayAnimation() *Modify {
	const decayDuration = 60000
	const offsetDuration = 2 * (decayDuration + 1)

	var deathSequence *components.Sequence
	for i := range mod.MDL.Sequences {
		if mod.MDL.Sequences[i].Name == "Death" {
			deathSequence = &mod.MDL.Sequences[i]
			break
		}
	}
	if deathSequence == nil {
		return mod
	}
	deathTimestamp := deathSequence.Interval[1]

	updateKeyFrame := func(keyFrame map[int]any) map[int]any {
		newKeyFrame := map[int]any{}
		for timestamp, value := range keyFrame {
			if timestamp <= deathTimestamp {
				newKeyFrame[timestamp] = value
				continue
			}
			newKeyFrame[timestamp+offsetDuration] = components.CloneKeyFrames(map[int]any{timestamp: value})[timestamp]
		}
		return newKeyFrame
	}

	for _, bone := range mod.MDL.Bones {
		if bone.Translation != nil && bone.Translation.GlobalSeq == nil {
			bone.Translation.KeyFrames = updateKeyFrame(bone.Translation.KeyFrames)
		}
		if bone.Scaling != nil && bone.Scaling.GlobalSeq == nil {
			bone.Scaling.KeyFrames = updateKeyFrame(bone.Scaling.KeyFrames)
		}
		if bone.Rotation != nil && bone.Rotation.GlobalSeq == nil {
			bone.Rotation.KeyFrames = updateKeyFrame(bone.Rotation.KeyFrames)
		}
	}
	for _, texAnim := range mod.MDL.TextureAnims {
		if texAnim.Translation != nil && texAnim.Translation.GlobalSeq == nil {
			texAnim.Translation.KeyFrames = updateKeyFrame(texAnim.Translation.KeyFrames)
		}
		if texAnim.Scaling != nil && texAnim.Scaling.GlobalSeq == nil {
			texAnim.Scaling.KeyFrames = updateKeyFrame(texAnim.Scaling.KeyFrames)
		}
		if texAnim.Rotation != nil && texAnim.Rotation.GlobalSeq == nil {
			texAnim.Rotation.KeyFrames = updateKeyFrame(texAnim.Rotation.KeyFrames)
		}
	}
	for i := range mod.MDL.GeosetAnims {
		ga := &mod.MDL.GeosetAnims[i]
		if ga.Alpha != nil && !ga.Alpha.Static && ga.Alpha.Anim != nil {
			ga.Alpha.Anim.KeyFrames = updateKeyFrame(ga.Alpha.Anim.KeyFrames)
		}
	}
	for i := range mod.MDL.Sequences {
		if mod.MDL.Sequences[i].Interval[0] > deathTimestamp {
			mod.MDL.Sequences[i].Interval[0] += offsetDuration
			mod.MDL.Sequences[i].Interval[1] += offsetDuration
		}
	}

	decayFleshSequence := components.Sequence{
		Name: "Decay Flesh",
		Bound: components.Bound{
			MinimumExtent: deathSequence.MinimumExtent,
			MaximumExtent: deathSequence.MaximumExtent,
			BoundsRadius:  deathSequence.BoundsRadius,
		},
		Data: components.SequenceData{
			WC3Name: "Decay Flesh", WowName: "", WowVariant: 0, AttackTag: "", WowFrequency: 1,
		},
		Interval:   [2]int{deathTimestamp + 1, deathTimestamp + 1 + decayDuration},
		NonLooping: true,
		MoveSpeed:  0,
	}
	decayBoneSequence := components.Sequence{
		Name: "Decay Bone",
		Bound: components.Bound{
			MinimumExtent: deathSequence.MinimumExtent,
			MaximumExtent: deathSequence.MaximumExtent,
			BoundsRadius:  deathSequence.BoundsRadius,
		},
		Data: components.SequenceData{
			WC3Name: "Decay Bone", WowName: "", WowVariant: 0, AttackTag: "", WowFrequency: 1,
		},
		Interval:   [2]int{decayFleshSequence.Interval[1] + 1, decayFleshSequence.Interval[1] + 1 + decayDuration},
		NonLooping: true,
		MoveSpeed:  0,
	}
	mod.MDL.Sequences = append(mod.MDL.Sequences, decayFleshSequence, decayBoneSequence)

	copyAnimKeyFrames := func(anim *components.Animation) {
		if anim == nil || anim.GlobalSeq != nil {
			return
		}
		updateAnimKeyFrames := func(timestampFrom, timestampTo int) {
			if value, ok := anim.KeyFrames[timestampFrom]; ok {
				anim.KeyFrames[timestampTo] = components.CloneKeyFrames(map[int]any{timestampFrom: value})[timestampFrom]
			}
		}
		updateAnimKeyFrames(deathTimestamp, decayFleshSequence.Interval[0])
		updateAnimKeyFrames(deathTimestamp, decayFleshSequence.Interval[1])
		updateAnimKeyFrames(deathTimestamp, decayBoneSequence.Interval[0])
		updateAnimKeyFrames(deathTimestamp, decayBoneSequence.Interval[1])
	}

	for i := range mod.MDL.GeosetAnims {
		ga := &mod.MDL.GeosetAnims[i]
		if ga.Alpha != nil && !ga.Alpha.Static {
			copyAnimKeyFrames(ga.Alpha.Anim)
		}
		if ga.Color != nil && !ga.Color.Static {
			copyAnimKeyFrames(ga.Color.Anim)
		}
	}
	for i := range mod.MDL.TextureAnims {
		ta := &mod.MDL.TextureAnims[i]
		copyAnimKeyFrames(ta.Translation)
		copyAnimKeyFrames(ta.Rotation)
		copyAnimKeyFrames(ta.Scaling)
	}

	for _, p := range mod.MDL.ParticleEmitter2s {
		if p.Visibility != nil && p.Visibility.GlobalSeq != nil {
			continue
		}
		if p.Visibility == nil {
			p.Visibility = &components.Animation{
				Interpolation: components.InterpLinear,
				KeyFrames:     map[int]any{},
				Type:          components.AnimTypeOthers,
			}
			for _, s := range mod.MDL.Sequences {
				if s.Name == decayFleshSequence.Name || s.Name == decayBoneSequence.Name {
					continue
				}
				p.Visibility.KeyFrames[s.Interval[0]] = float64(1)
				p.Visibility.KeyFrames[s.Interval[1]] = float64(1)
			}
		}
		p.Visibility.KeyFrames[decayFleshSequence.Interval[0]] = float64(0)
		p.Visibility.KeyFrames[decayFleshSequence.Interval[1]] = float64(0)
		p.Visibility.KeyFrames[decayBoneSequence.Interval[0]] = float64(0)
		p.Visibility.KeyFrames[decayBoneSequence.Interval[1]] = float64(0)
	}

	maxZ := mod.GetMaxZAtTimestamp(*deathSequence, deathSequence.Interval[1]-deathSequence.Interval[0])
	if maxZ < 0 {
		maxZ = 0
	}

	for _, bone := range mod.MDL.Bones {
		value := InterpolateTransformQuat(bone, *deathSequence, deathTimestamp)
		setBoneKeyframes := func(anim *components.Animation, val any) {
			if anim == nil || anim.GlobalSeq != nil {
				return
			}
			anim.KeyFrames[decayFleshSequence.Interval[0]] = components.CloneKeyFrames(map[int]any{0: val})[0]
			anim.KeyFrames[decayFleshSequence.Interval[1]] = components.CloneKeyFrames(map[int]any{0: val})[0]
			anim.KeyFrames[decayBoneSequence.Interval[0]] = components.CloneKeyFrames(map[int]any{0: val})[0]
			anim.KeyFrames[decayBoneSequence.Interval[1]] = components.CloneKeyFrames(map[int]any{0: val})[0]
		}
		setBoneKeyframes(bone.Translation, value.Position)
		setBoneKeyframes(bone.Rotation, value.Rotation)
		setBoneKeyframes(bone.Scaling, value.Scaling)

		if bone.ParentBone == nil && bone.Parent == nil {
			if bone.Translation == nil || bone.Translation.GlobalSeq == nil {
				if bone.Translation == nil {
					bone.Translation = &components.Animation{
						Interpolation: components.InterpLinear,
						KeyFrames:     map[int]any{},
						Type:          components.AnimTypeTranslation,
					}
				}
				pos := value.Position
				bone.Translation.KeyFrames[decayFleshSequence.Interval[0]] = imath.Vector3{pos[0], pos[1], pos[2]}
				bone.Translation.KeyFrames[decayFleshSequence.Interval[1]] = imath.Vector3{pos[0], pos[1], pos[2]}
				bone.Translation.KeyFrames[decayBoneSequence.Interval[0]] = imath.Vector3{pos[0], pos[1], pos[2]}
				translation := imath.Vector3{pos[0], pos[1], pos[2]}
				translation[2] -= maxZ
				bone.Translation.KeyFrames[decayBoneSequence.Interval[1]] = translation
			}
		}
	}
	return mod
}

func (mod *Modify) AddDoodadDeathAnimation() *Modify {
	for _, seq := range mod.MDL.Sequences {
		if seq.Name == "Death" {
			return mod
		}
	}

	maxTimestamp := 0
	for _, s := range mod.MDL.Sequences {
		if end := s.Interval[1] + 1; end > maxTimestamp {
			maxTimestamp = end
		}
	}

	deathSequence := components.Sequence{
		Name:       "Death",
		Interval:   [2]int{maxTimestamp, maxTimestamp + 1000},
		NonLooping: true,
		MoveSpeed:  0,
		Data: components.SequenceData{
			WC3Name: "Death", WowName: "", WowVariant: 0, AttackTag: "", WowFrequency: 1,
		},
	}
	mod.MDL.Sequences = append(mod.MDL.Sequences, deathSequence)

	for _, bone := range mod.MDL.Bones {
		if bone.ParentBone != nil || bone.Parent != nil {
			continue
		}
		if bone.Scaling == nil {
			kf := map[int]any{}
			for _, s := range mod.MDL.Sequences {
				kf[s.Interval[0]] = imath.Vector3{1, 1, 1}
				kf[s.Interval[1]] = imath.Vector3{1, 1, 1}
			}
			bone.Scaling = &components.Animation{
				Interpolation: components.InterpDontInterp,
				KeyFrames:     kf,
				Type:          components.AnimTypeScaling,
			}
		}
		bone.Scaling.KeyFrames[deathSequence.Interval[0]] = imath.Vector3{0, 0, 0}
		bone.Scaling.KeyFrames[deathSequence.Interval[1]] = imath.Vector3{0, 0, 0}
	}
	return mod
}

func (mod *Modify) CloneSequence(sequence *components.Sequence, newWc3Name string) *Modify {
	maxInterval := 0
	for _, s := range mod.MDL.Sequences {
		if s.Interval[1] > maxInterval {
			maxInterval = s.Interval[1]
		}
	}

	newSequence := *sequence
	newSequence.Name = newWc3Name
	newSequence.Data.WC3Name = newWc3Name
	newSequence.Interval = [2]int{maxInterval + 1, maxInterval + 1 + (sequence.Interval[1] - sequence.Interval[0])}
	newSequence.Data.WowName = ""
	mod.MDL.Sequences = append(mod.MDL.Sequences, newSequence)

	cloneSeqKeyFrame := func(keyFrame map[int]any) {
		times := components.SortedKeyInts(keyFrame)
		minTimestamp := int(^uint(0) >> 1)
		maxTimestamp := -1
		var minValue, maxValue any
		hasMin, hasMax := false, false
		for _, timestamp := range times {
			if newSequence.Interval[0] <= timestamp && timestamp <= newSequence.Interval[1] {
				delete(keyFrame, timestamp)
				continue
			}
			if timestamp < sequence.Interval[0] || timestamp > sequence.Interval[1] {
				continue
			}
			newTimestamp := timestamp - sequence.Interval[0] + newSequence.Interval[0]
			value := components.CloneKeyFrames(map[int]any{timestamp: keyFrame[timestamp]})[timestamp]
			keyFrame[newTimestamp] = value
			if newTimestamp < minTimestamp {
				minTimestamp = newTimestamp
				minValue = value
				hasMin = true
			}
			if newTimestamp > maxTimestamp {
				maxTimestamp = newTimestamp
				maxValue = value
				hasMax = true
			}
		}
		if hasMin && mdlValueTruthy(minValue) {
			keyFrame[newSequence.Interval[0]] = minValue
		}
		if hasMax && mdlValueTruthy(maxValue) {
			keyFrame[newSequence.Interval[1]] = maxValue
		}
	}

	for _, anim := range mod.MDL.GetAnimated() {
		if anim.GlobalSeq != nil {
			continue
		}
		cloneSeqKeyFrame(anim.KeyFrames)
	}
	return mod
}

func (mod *Modify) ConcatenateSequences(sequences []*components.Sequence, newWc3Name string) *components.Sequence {
	maxInterval := 0
	for _, s := range mod.MDL.Sequences {
		if s.Interval[1] > maxInterval {
			maxInterval = s.Interval[1]
		}
	}
	totalDuration := 0
	for i, s := range sequences {
		totalDuration += s.Interval[1] - s.Interval[0]
		if i < len(sequences)-1 {
			totalDuration++
		}
	}
	newSequence := *sequences[0]
	newSequence.Name = newWc3Name
	newSequence.Data.WC3Name = newWc3Name
	newSequence.Interval = [2]int{maxInterval + 1, maxInterval + 1 + totalDuration}
	newSequence.Data.WowName = ""
	mod.MDL.Sequences = append(mod.MDL.Sequences, newSequence)

	for _, anim := range mod.MDL.GetAnimated() {
		if anim.GlobalSeq != nil {
			continue
		}
		keyFrame := anim.KeyFrames
		times := components.SortedKeyInts(keyFrame)
		minTimestamp := int(^uint(0) >> 1)
		maxTimestamp := -1
		var minValue, maxValue any
		hasMin, hasMax := false, false
		durationAccum := 0
		for _, seq := range sequences {
			for _, timestamp := range times {
				if newSequence.Interval[0] <= timestamp && timestamp <= newSequence.Interval[1] {
					delete(keyFrame, timestamp)
					continue
				}
				if timestamp < seq.Interval[0] || timestamp > seq.Interval[1] {
					continue
				}
				newTimestamp := timestamp - seq.Interval[0] + newSequence.Interval[0] + durationAccum
				value := components.CloneKeyFrames(map[int]any{timestamp: keyFrame[timestamp]})[timestamp]
				keyFrame[newTimestamp] = value
				if newTimestamp < minTimestamp {
					minTimestamp = newTimestamp
					minValue = value
					hasMin = true
				}
				if newTimestamp > maxTimestamp {
					maxTimestamp = newTimestamp
					maxValue = value
					hasMax = true
				}
			}
			durationAccum += seq.Interval[1] - seq.Interval[0] + 1
		}
		if hasMin && mdlValueTruthy(minValue) {
			keyFrame[newSequence.Interval[0]] = minValue
		}
		if hasMax && mdlValueTruthy(maxValue) {
			keyFrame[newSequence.Interval[1]] = maxValue
		}
	}
	return &newSequence
}

func mdlValueTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case float64:
		return v != 0
	case int:
		return v != 0
	default:
		return true
	}
}
