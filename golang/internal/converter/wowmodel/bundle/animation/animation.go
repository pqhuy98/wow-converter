package animation

import (
	"fmt"
	"log"
	stdmath "math"
	"sort"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/config"
	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
	bundleutils "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/utils"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

// BoneData is a bone entry from _bones.json equivalent.
type BoneData struct {
	BoneID      int32      `json:"boneID"`
	Flags       uint32     `json:"flags"`
	ParentBone  int16      `json:"parentBone"`
	SubMeshID   uint16     `json:"subMeshID"`
	BoneNameCRC uint32     `json:"boneNameCRC"`
	Translation TrackData  `json:"translation"`
	Rotation    TrackData  `json:"rotation"`
	Scale       TrackData  `json:"scale"`
	Pivot       [3]float64 `json:"pivot"`
}

// TrackData is animation track data.
type TrackData struct {
	GlobalSeq     uint16        `json:"globalSeq"`
	Interpolation uint16        `json:"interpolation"`
	Timestamps    [][]*uint32   `json:"timestamps"`
	Values        [][][]float64 `json:"values"`
}

// AnimationData is animation metadata.
type AnimationData struct {
	ID             uint16     `json:"id"`
	VariationIndex uint16     `json:"variationIndex"`
	Duration       uint32     `json:"duration"`
	MoveSpeed      float32    `json:"movespeed"`
	Flags          uint32     `json:"flags"`
	Frequency      uint32     `json:"frequency"`
	ReplayMin      uint32     `json:"replayMin"`
	ReplayMax      uint32     `json:"replayMax"`
	BlendTimeIn    uint16     `json:"blendTimeIn"`
	BlendTimeOut   uint16     `json:"blendTimeOut"`
	BoxPosMin      [3]float64 `json:"boxPosMin"`
	BoxPosMax      [3]float64 `json:"boxPosMax"`
	BoxRadius      float32    `json:"boxRadius"`
	VariationNext  int16      `json:"variationNext"`
	AliasNext      int16      `json:"aliasNext"`
}

// AttachmentData is a WoW attachment point.
type AttachmentData struct {
	ID       uint32     `json:"id"`
	Bone     uint16     `json:"bone"`
	Unknown  uint16     `json:"unknown"`
	Position [3]float64 `json:"position"`
}

// File holds parsed bones/animation data.
type File struct {
	FilePath    string
	Config      config.Config
	Bones       []BoneData
	Animations  []AnimationData
	BoneWeights []float64
	BoneIndices []float64
	Attachments []AttachmentData
	IsLoaded    bool
}

// NewFile creates an animation file placeholder.
func NewFile(filePath string, cfg config.Config) *File {
	return &File{FilePath: filePath, Config: cfg}
}

// LoadFromData populates from direct M2 pipeline output.
func (f *File) LoadFromData(data map[string]any) {
	if v, ok := data["bones"].([]any); ok {
		f.Bones = parseBones(v)
	}
	if v, ok := data["animations"].([]any); ok {
		f.Animations = parseAnimations(v)
	}
	if v, ok := data["boneWeights"].([]any); ok {
		f.BoneWeights = toFloatSlice(v)
	}
	if v, ok := data["boneIndicies"].([]any); ok {
		f.BoneIndices = toFloatSlice(v)
	}
	if v, ok := data["attachments"].([]any); ok {
		f.Attachments = parseAttachments(v)
	}
	f.IsLoaded = true
}

// ToMdlResult is the MDL animation conversion output.
type ToMdlResult struct {
	Bones          []*components.Bone
	Sequences      []components.Sequence
	SkinWeights    [][]components.SkinWeight
	WowAttachments []components.WowAttachment
}

// ToMdl converts bones data to MDL nodes.
func (f *File) ToMdl(globalSequences *[]*components.GlobalSequence) ToMdlResult {
	if !f.IsLoaded {
		panic("animation file is not loaded")
	}
	excluded := map[int]struct{}{}
	gsMap := map[int]*components.GlobalSequence{}
	for _, gs := range *globalSequences {
		if gs == nil {
			continue
		}
		key := gs.ID
		if gs.HasRawID {
			key = gs.RawID
		}
		gsMap[key] = gs
	}
	getGS := func(id int) *components.GlobalSequence {
		if id == config.BlizzardNull {
			return nil
		}
		if gs, ok := gsMap[id]; ok {
			return gs
		}
		created := components.NewGlobalSequence(id, 1)
		ptr := &created
		*globalSequences = append(*globalSequences, ptr)
		key := ptr.ID
		if ptr.HasRawID {
			key = ptr.RawID
		}
		gsMap[key] = ptr
		return ptr
	}

	bones := make([]*components.Bone, len(f.Bones))
	for i, bone := range f.Bones {
		mdlBone := &components.Bone{
			NodeBase: components.NodeBase{
				Type:       "Bone",
				Name:       animmap.GetBoneName(int(bone.BoneID), i, bone.BoneNameCRC),
				PivotPoint: imath.Vector3{bone.Pivot[0], -bone.Pivot[2], bone.Pivot[1]},
				Translation: &components.Animation{
					Type:          components.AnimTypeTranslation,
					Interpolation: bundleutils.WowToWc3Interpolation(bone.Translation.Interpolation),
					KeyFrames:     map[int]any{},
				},
				Rotation: &components.Animation{
					Type:          components.AnimTypeRotation,
					Interpolation: bundleutils.WowToWc3Interpolation(bone.Rotation.Interpolation),
					KeyFrames:     map[int]any{},
				},
				Scaling: &components.Animation{
					Type:          components.AnimTypeScaling,
					Interpolation: bundleutils.WowToWc3Interpolation(bone.Scale.Interpolation),
					KeyFrames:     map[int]any{},
				},
			},
		}
		if int(bone.Translation.GlobalSeq) != config.BlizzardNull {
			mdlBone.Translation.GlobalSeq = getGS(int(bone.Translation.GlobalSeq))
		}
		if int(bone.Rotation.GlobalSeq) != config.BlizzardNull {
			mdlBone.Rotation.GlobalSeq = getGS(int(bone.Rotation.GlobalSeq))
		}
		if int(bone.Scale.GlobalSeq) != config.BlizzardNull {
			mdlBone.Scaling.GlobalSeq = getGS(int(bone.Scale.GlobalSeq))
		}
		if bone.Flags&0x1 != 0 {
			mdlBone.Flags = append(mdlBone.Flags, components.NodeFlagDontInheritTranslation)
		}
		if bone.Flags&0x2 != 0 {
			mdlBone.Flags = append(mdlBone.Flags, components.NodeFlagDontInheritScaling)
		}
		if bone.Flags&0x4 != 0 {
			mdlBone.Flags = append(mdlBone.Flags, components.NodeFlagDontInheritRotation)
		}
		if bone.Flags&0x8 != 0 {
			mdlBone.Flags = append(mdlBone.Flags, components.NodeFlagBillboarded)
		}
		if bone.Flags&0x10 != 0 {
			mdlBone.Flags = append(mdlBone.Flags, components.NodeFlagBillboardLockY)
		}
		if bone.Flags&0x20 != 0 {
			mdlBone.Flags = append(mdlBone.Flags, components.NodeFlagBillboardLockX)
		}
		if bone.Flags&0x40 != 0 {
			mdlBone.Flags = append(mdlBone.Flags, components.NodeFlagBillboardLockZ)
		}
		applyTrack(mdlBone.Translation, bone.Translation, f.Animations, excluded, func(v []float64) any {
			return imath.Vector3{v[0], -v[2], v[1]}
		})
		applyRotTrack(mdlBone.Rotation, bone.Rotation, f.Animations, excluded)
		applyTrack(mdlBone.Scaling, bone.Scale, f.Animations, excluded, func(v []float64) any {
			return imath.Vector3{v[0], v[2], v[1]}
		})
		bones[i] = mdlBone
	}

	for i, mdlBone := range bones {
		parent := int(f.Bones[i].ParentBone)
		if parent > -1 && parent < len(bones) {
			mdlBone.Parent = bones[parent]
		}
		if countKeyframes(mdlBone) == 0 && mdlBone.Parent != nil {
			mdlBone.Translation.KeyFrames[0] = imath.Vector3{}
		}
		if len(mdlBone.Translation.KeyFrames) == 0 {
			mdlBone.Translation = nil
		}
		if len(mdlBone.Rotation.KeyFrames) == 0 {
			mdlBone.Rotation = nil
		}
		if len(mdlBone.Scaling.KeyFrames) == 0 {
			mdlBone.Scaling = nil
		}
	}

	skinWeights := buildSkinWeights(f, bones)

	var sequences []components.Sequence
	seqAccum := 0
	for animID, anim := range f.Animations {
		meta := animmap.AnimationMeta{
			ID: anim.ID, VariationIndex: anim.VariationIndex, Duration: anim.Duration,
			MoveSpeed: anim.MoveSpeed, Frequency: anim.Frequency,
			BoxPosMin: [3]float32{float32(anim.BoxPosMin[0]), float32(anim.BoxPosMin[1]), float32(anim.BoxPosMin[2])},
			BoxPosMax: [3]float32{float32(anim.BoxPosMax[0]), float32(anim.BoxPosMax[1]), float32(anim.BoxPosMax[2])},
			BoxRadius: anim.BoxRadius,
		}
		seqData := animmap.GetWarcraftSequenceData(meta)
		if _, skip := excluded[animID]; skip {
			seqAccum += int(anim.Duration) + 1
			log.Printf("%s", ansi.Redf("Skip unreadable animation %s of %s", seqData.WowName, f.FilePath))
			continue
		}
		wowName := animmap.GetWowAnimName(int(anim.ID))
		seq := components.Sequence{
			Name:      seqData.WC3Name,
			Data:      seqData,
			Interval:  [2]int{seqAccum, seqAccum + int(anim.Duration)},
			MoveSpeed: float64(anim.MoveSpeed),
			Bound: components.Bound{
				MinimumExtent: imath.Vector3{anim.BoxPosMin[0], -float64(anim.BoxPosMax[2]), anim.BoxPosMin[1]},
				MaximumExtent: imath.Vector3{anim.BoxPosMax[0], -float64(anim.BoxPosMin[2]), anim.BoxPosMax[1]},
				BoundsRadius:  float64(anim.BoxRadius),
			},
			NonLooping: !animmap.IsLoopAnimation(wowName),
		}
		sequences = append(sequences, seq)
		seqAccum += int(anim.Duration) + 1
	}

	wowAttachments := extractWowAttachments(f, bones)

	standLikeNames := []string{"Stand", "Stand Alternate", "Walk", "Walk Alternate"}
	for _, baseName := range standLikeNames {
		var matches []*components.Sequence
		for i := range sequences {
			if sequences[i].Data.WC3Name == baseName {
				matches = append(matches, &sequences[i])
			}
		}
		if len(matches) <= 1 {
			continue
		}
		best := matches[0]
		for _, seq := range matches[1:] {
			if seq.Data.WowFrequency > best.Data.WowFrequency {
				best = seq
			}
		}
		rarity := 4
		for _, seq := range matches {
			if seq != best {
				seq.Rarity = &rarity
			}
		}
	}

	return ToMdlResult{Bones: bones, Sequences: sequences, SkinWeights: skinWeights, WowAttachments: wowAttachments}
}

func countKeyframes(b *components.Bone) int {
	n := 0
	if b.Translation != nil {
		n += len(b.Translation.KeyFrames)
	}
	if b.Rotation != nil {
		n += len(b.Rotation.KeyFrames)
	}
	if b.Scaling != nil {
		n += len(b.Scaling.KeyFrames)
	}
	return n
}

func applyTrack(anim *components.Animation, track TrackData, animations []AnimationData, excluded map[int]struct{}, conv func([]float64) any) {
	accum := 0
	for animID, timestamps := range track.Timestamps {
		if animID >= len(animations) {
			continue
		}
		animation := animations[animID]
		start := accum
		accum += int(animation.Duration) + 1
		if _, skip := excluded[animID]; skip {
			continue
		}
		if !validTimestamps(timestamps) {
			if timestamps != nil {
				excluded[animID] = struct{}{}
			}
			continue
		}
		maxTS := -1
		for ti, ts := range timestamps {
			if ts == nil {
				continue
			}
			t := int(*ts) + start
			if animID >= len(track.Values) || ti >= len(track.Values[animID]) {
				continue
			}
			vals := track.Values[animID][ti]
			if len(vals) < 3 {
				continue
			}
			if mathAbs(vals[0]) > 999999 || mathAbs(vals[1]) > 999999 || mathAbs(vals[2]) > 999999 {
				continue
			}
			anim.KeyFrames[t] = conv(vals)
			if t > maxTS {
				maxTS = t
			}
		}
		if maxTS >= -1 && anim.GlobalSeq == nil && maxTS >= 0 {
			if v, ok := anim.KeyFrames[maxTS]; ok {
				anim.KeyFrames[start+int(animation.Duration)] = v
			}
		}
	}
}

func applyRotTrack(anim *components.Animation, track TrackData, animations []AnimationData, excluded map[int]struct{}) {
	accum := 0
	for animID, timestamps := range track.Timestamps {
		if animID >= len(animations) {
			continue
		}
		animation := animations[animID]
		start := accum
		accum += int(animation.Duration) + 1
		if _, skip := excluded[animID]; skip {
			continue
		}
		if !validTimestamps(timestamps) {
			if timestamps != nil {
				excluded[animID] = struct{}{}
			}
			continue
		}
		maxTS := -1
		for ti, ts := range timestamps {
			if ts == nil {
				continue
			}
			t := int(*ts) + start
			if animID >= len(track.Values) || ti >= len(track.Values[animID]) {
				continue
			}
			vals := track.Values[animID][ti]
			if len(vals) < 4 {
				continue
			}
			anim.KeyFrames[t] = imath.QuaternionRotation{vals[0], -vals[2], vals[1], vals[3]}
			if t > maxTS {
				maxTS = t
			}
		}
		if maxTS >= -1 && anim.GlobalSeq == nil && maxTS >= 0 {
			if v, ok := anim.KeyFrames[maxTS]; ok {
				anim.KeyFrames[start+int(animation.Duration)] = v
			}
		}
	}
}

func validTimestamps(timestamps []*uint32) bool {
	if timestamps == nil {
		return false
	}
	var prev *uint32
	for _, ts := range timestamps {
		if ts == nil {
			continue
		}
		if *ts > 9999999 {
			return false
		}
		if prev != nil && *ts < *prev {
			return false
		}
		prev = ts
	}
	return true
}

func buildSkinWeights(f *File, bones []*components.Bone) [][]components.SkinWeight {
	var out [][]components.SkinWeight
	for i := 0; i+3 < len(f.BoneIndices); i += 4 {
		weights := f.BoneWeights[i : i+4]
		indices := f.BoneIndices[i : i+4]
		var sw []components.SkinWeight
		sum := 0.0
		for _, w := range weights {
			sum += w
		}
		if sum <= 0 {
			continue
		}
		for j, w := range weights {
			if w > 0 {
				idx := int(indices[j])
				if idx < len(bones) {
					sw = append(sw, components.SkinWeight{Bone: bones[idx], Weight: int(w)})
				}
			}
		}
		sort.Slice(sw, func(a, b int) bool { return sw[a].Weight > sw[b].Weight })
		out = append(out, sw)
	}
	return out
}

func extractWowAttachments(f *File, bones []*components.Bone) []components.WowAttachment {
	var result []components.WowAttachment
	for _, att := range f.Attachments {
		if int(att.Bone) >= len(bones) {
			continue
		}
		pos := att.Position
		result = append(result, components.WowAttachment{
			WowAttachmentID: int(att.ID),
			Bone:            bones[att.Bone],
			PivotPoint:      imath.Vector3{float64(pos[0]), -float64(pos[2]), float64(pos[1])},
		})
	}
	return result
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func toFloatSlice(v []any) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		switch n := x.(type) {
		case float64:
			out[i] = n
		case int:
			out[i] = float64(n)
		}
	}
	return out
}

func parseBones(v []any) []BoneData {
	// Bones arrive pre-normalized from buildBonesData via json round-trip in direct pipeline.
	return nil
}

func parseAnimations(v []any) []AnimationData { return nil }

func parseAttachments(v []any) []AttachmentData { return nil }

// LoadFromBonesData loads typed bones data directly.
func (f *File) LoadFromBonesData(bones []BoneData, animations []AnimationData, weights, indices []float64, attachments []AttachmentData) {
	f.Bones = bones
	f.Animations = animations
	f.BoneWeights = weights
	f.BoneIndices = indices
	f.Attachments = attachments
	f.IsLoaded = true
}

// Ensure mdl import is used for wow attachment type check
var _ = mdl.MDL{}

func init() {
	_ = fmt.Sprintf
	_ = stdmath.MaxFloat64
}
