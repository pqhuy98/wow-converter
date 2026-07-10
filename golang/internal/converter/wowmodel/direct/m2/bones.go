package directm2

import (
	"context"

	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// BuildBonesData constructs _bones.json-equivalent data from an M2 loader.
func BuildBonesData(ctx context.Context, loader *m2.Loader, excluded map[int]struct{}, m2FileDataID int, buildKey string) (*animation.File, error) {
	graph, err := loadSkeletonGraph(ctx, loader, m2FileDataID, buildKey)
	if err != nil {
		return nil, err
	}

	bones := BonesExcludeAnimations(graph.Bones, graph.Animations, excluded)
	boneData := make([]animation.BoneData, len(bones))
	for i, b := range bones {
		boneData[i] = animation.BoneData{
			BoneID: b.BoneID, Flags: b.Flags, ParentBone: b.ParentBone,
			SubMeshID: b.SubMeshID, BoneNameCRC: b.BoneNameCRC,
			Translation: convertTrack(b.Translation),
			Rotation:    convertTrack(b.Rotation),
			Scale:       convertTrack(b.Scale),
			Pivot:       [3]float64{float64(b.Pivot[0]), float64(b.Pivot[1]), float64(b.Pivot[2])},
		}
	}

	anims := make([]animation.AnimationData, len(graph.Animations))
	for i, a := range graph.Animations {
		anims[i] = animation.AnimationData{
			ID: a.ID, VariationIndex: a.VariationIndex, Duration: a.Duration,
			MoveSpeed: a.MoveSpeed, Flags: a.Flags, Frequency: a.Frequency,
			ReplayMin: a.ReplayMin, ReplayMax: a.ReplayMax,
			BlendTimeIn: a.BlendTimeIn, BlendTimeOut: a.BlendTimeOut,
			BoxPosMin: [3]float64{float64(a.BoxPosMin[0]), float64(a.BoxPosMin[1]), float64(a.BoxPosMin[2])},
			BoxPosMax: [3]float64{float64(a.BoxPosMax[0]), float64(a.BoxPosMax[1]), float64(a.BoxPosMax[2])},
			BoxRadius: a.BoxRadius, VariationNext: a.VariationNext, AliasNext: a.AliasNext,
		}
	}

	weights := make([]float64, len(loader.BoneWeights))
	for i, w := range loader.BoneWeights {
		weights[i] = float64(w)
	}
	indices := make([]float64, len(loader.BoneIndices))
	for i, idx := range loader.BoneIndices {
		indices[i] = float64(idx)
	}

	attachmentsSource := loader.Attachments
	if len(attachmentsSource) == 0 {
		attachmentsSource = graph.SkelAttachments
	}
	attachments := make([]animation.AttachmentData, len(attachmentsSource))
	for i, a := range attachmentsSource {
		attachments[i] = animation.AttachmentData{
			ID: a.ID, Bone: a.Bone, Unknown: a.Unknown,
			Position: [3]float64{float64(a.Position[0]), float64(a.Position[1]), float64(a.Position[2])},
		}
	}

	f := &animation.File{}
	f.LoadFromBonesData(boneData, anims, weights, indices, attachments)
	return f, nil
}

func convertTrack(t m2.Track) animation.TrackData {
	ts := make([][]*uint32, len(t.Timestamps))
	for i, sub := range t.Timestamps {
		ts[i] = make([]*uint32, len(sub))
		for j, v := range sub {
			val := v
			ts[i][j] = &val
		}
	}
	vals := make([][][]float64, len(t.Values))
	for i, sub := range t.Values {
		vals[i] = make([][]float64, len(sub))
		for j, v := range sub {
			vals[i][j] = append([]float64(nil), v...)
		}
	}
	return animation.TrackData{
		GlobalSeq: t.GlobalSeq, Interpolation: t.Interpolation,
		Timestamps: ts, Values: vals,
	}
}
