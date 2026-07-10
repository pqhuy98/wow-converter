package directm2

import (
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// BonesExcludeAnimations strips animation keyframes for excluded WoW animation IDs.
func BonesExcludeAnimations(bones []m2.BoneEntry, animations []m2.AnimationEntry, excluded map[int]struct{}) []m2.BoneEntry {
	if len(excluded) == 0 || len(bones) == 0 {
		return bones
	}

	animIdx := map[uint16]int{}
	for i, anim := range animations {
		animIdx[anim.ID] = i
	}
	excludedIndices := map[int]struct{}{}
	for id := range excluded {
		if idx, ok := animIdx[uint16(id)]; ok {
			excludedIndices[idx] = struct{}{}
		}
	}
	if len(excludedIndices) == 0 {
		return bones
	}

	out := make([]m2.BoneEntry, len(bones))
	for i, bone := range bones {
		out[i] = bone
		out[i].Translation = excludeTrack(bone.Translation, excludedIndices)
		out[i].Rotation = excludeTrack(bone.Rotation, excludedIndices)
		out[i].Scale = excludeTrack(bone.Scale, excludedIndices)
	}
	return out
}

func excludeTrack(track m2.Track, excludedIndices map[int]struct{}) m2.Track {
	if track.GlobalSeq != config.BlizzardNull {
		return track
	}
	if len(excludedIndices) == 0 {
		return track
	}
	ts := make([][]uint32, len(track.Timestamps))
	vals := make([][][]float64, len(track.Values))
	for i := range track.Timestamps {
		if _, skip := excludedIndices[i]; skip {
			ts[i] = nil
			if i < len(track.Values) {
				vals[i] = nil
			}
			continue
		}
		ts[i] = track.Timestamps[i]
		if i < len(track.Values) {
			vals[i] = track.Values[i]
		}
	}
	track.Timestamps = ts
	track.Values = vals
	return track
}
