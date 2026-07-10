package mdl

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

func (mod *Modify) AddMdlItemToBone(item *MDL, bone *components.Bone) *Modify {
	for i, seq := range item.Sequences {
		if seq.Name == "Stand" {
			item.Sequences = []components.Sequence{item.Sequences[i]}
			break
		}
	}
	item.Modify.OptimizeKeyFrames()

	for _, b := range item.GetNodes() {
		if b.NodeParent() == nil {
			b.SetNodeParent(bone)
		}
		b.SetNodePivotPoint(imath.V3Sum(b.NodePivotPoint(), bone.PivotPoint))
	}
	for _, geoset := range item.Geosets {
		for _, v := range geoset.Vertices {
			v.Position = imath.V3Sum(v.Position, bone.PivotPoint)
		}
	}

	mergeItemObjects(mod.MDL, item)
	return mod
}

func (mod *Modify) AddItemPathToBone(itemPath string, bone *components.Bone, keepRatio bool) *components.AttachmentPoint {
	scale := imath.Vector3{1, 1, 1}
	if keepRatio {
		scale = imath.Vector3{mod.MDL.AccumScale, mod.MDL.AccumScale, mod.MDL.AccumScale}
	}
	attachment := &components.AttachmentPoint{
		NodeBase: components.NodeBase{
			Name:       "Item_" + itemPath,
			Type:       "AttachmentPoint",
			Parent:     bone,
			PivotPoint: bone.PivotPoint,
			Scaling: &components.Animation{
				Interpolation: components.InterpDontInterp,
				KeyFrames:     map[int]any{0: scale},
				Type:          components.AnimTypeScaling,
			},
		},
		Path:         itemPath,
		AttachmentID: 0,
	}
	mod.MDL.Attachments = append(mod.MDL.Attachments, attachment)
	return attachment
}

func (mod *Modify) AddMdlCollectionItemToModel(item *MDL) *Modify {
	if len(item.Sequences) > 0 {
		item.Sequences = []components.Sequence{item.Sequences[0]}
	}
	boneMap := map[string]*components.Bone{}
	for _, b := range mod.MDL.Bones {
		boneMap[b.Name] = b
	}
	getMainBone := func(bone *components.Bone) (*components.Bone, error) {
		mainBone, ok := boneMap[bone.Name]
		if !ok {
			return nil, fmt.Errorf("cannot merge item %q to model because bone %q is missing", filepath.Base(item.Model.Name), bone.Name)
		}
		return mainBone, nil
	}

	for _, geoset := range item.Geosets {
		for mi := range geoset.Matrices {
			for bi, bone := range geoset.Matrices[mi].Bones {
				mainBone, err := getMainBone(bone)
				if err != nil {
					panic(err)
				}
				geoset.Matrices[mi].Bones[bi] = mainBone
			}
		}
		for _, vertex := range geoset.Vertices {
			for i := range vertex.SkinWeights {
				mainBone, err := getMainBone(vertex.SkinWeights[i].Bone)
				if err != nil {
					panic(err)
				}
				vertex.SkinWeights[i].Bone = mainBone
			}
		}
	}
	item.Bones = nil
	mergeItemObjects(mod.MDL, item)
	return mod
}

func CanAddMdlCollectionItemToModel(main *MDL, item *MDL) bool {
	for _, b := range item.Bones {
		if b.ParentBone == nil && b.Parent == nil && strings.Contains(b.Name, "bone_") {
			return false
		}
	}
	boneMap := map[string]struct{}{}
	for _, b := range main.Bones {
		boneMap[b.Name] = struct{}{}
	}
	for _, b := range item.Bones {
		if _, ok := boneMap[b.Name]; !ok {
			return false
		}
	}
	return true
}

func mergeItemObjects(main *MDL, item *MDL) {
	for _, geoset := range item.Geosets {
		geoset.Name = "item_" + geoset.Name
	}
	globalSeqMap := map[*components.GlobalSequence]*components.GlobalSequence{}
	globalSeqStableMap := map[uint64]*components.GlobalSequence{}
	globalSeqRawMap := map[int]*components.GlobalSequence{}
	for _, gs := range item.GlobalSequences {
		cloned := components.CloneGlobalSequence(*gs)
		mapped := &cloned
		main.GlobalSequences = append(main.GlobalSequences, mapped)
		globalSeqMap[gs] = mapped
		if gs.StableID != 0 {
			globalSeqStableMap[gs.StableID] = mapped
		}
		if gs.HasRawID {
			globalSeqRawMap[gs.RawID] = mapped
		}
	}
	for _, anim := range item.GetAnimated() {
		if anim != nil && anim.GlobalSeq != nil {
			if mapped, ok := globalSeqMap[anim.GlobalSeq]; ok {
				anim.GlobalSeq = mapped
				continue
			}
			var mapped *components.GlobalSequence
			if anim.GlobalSeq.StableID != 0 {
				if mapped, ok := globalSeqStableMap[anim.GlobalSeq.StableID]; ok {
					anim.GlobalSeq = mapped
					continue
				}
			}
			if anim.GlobalSeq.HasRawID {
				mapped = globalSeqRawMap[anim.GlobalSeq.RawID]
			}
			if mapped != nil {
				anim.GlobalSeq = mapped
			}
		}
	}
	main.Textures = append(main.Textures, item.Textures...)
	textureAnimMap := map[*components.TextureAnim]*components.TextureAnim{}
	textureAnimOffset := len(main.TextureAnims)
	for i := range item.TextureAnims {
		main.TextureAnims = append(main.TextureAnims, cloneTextureAnim(item.TextureAnims[i]))
		textureAnimMap[&item.TextureAnims[i]] = &main.TextureAnims[textureAnimOffset+i]
	}
	for mi := range item.Materials {
		for li := range item.Materials[mi].Layers {
			layer := &item.Materials[mi].Layers[li]
			if layer.TVertexAnim == nil {
				continue
			}
			if mapped, ok := textureAnimMap[layer.TVertexAnim]; ok {
				layer.TVertexAnim = mapped
				continue
			}
			if id := layer.TVertexAnim.ID; id >= 0 && id < len(item.TextureAnims) {
				layer.TVertexAnim = &main.TextureAnims[textureAnimOffset+id]
			}
		}
	}
	main.Materials = append(main.Materials, item.Materials...)
	main.Geosets = append(main.Geosets, item.Geosets...)
	main.GeosetAnims = append(main.GeosetAnims, item.GeosetAnims...)
	main.Bones = append(main.Bones, item.Bones...)
	main.Attachments = append(main.Attachments, item.Attachments...)
	for _, a := range item.Attachments {
		if a.Data != nil && a.Data.WowAttachment != nil {
			a.Name = fmt.Sprintf("item_Wow:%d", a.Data.WowAttachment.WowAttachmentID)
		}
	}
	main.EventObjects = append(main.EventObjects, item.EventObjects...)
	main.Lights = append(main.Lights, item.Lights...)
	main.RibbonEmitters = append(main.RibbonEmitters, item.RibbonEmitters...)
	main.ParticleEmitter2s = append(main.ParticleEmitter2s, item.ParticleEmitter2s...)
	main.Helpers = append(main.Helpers, item.Helpers...)
	main.Cameras = append(main.Cameras, item.Cameras...)
	main.UpdateIDs()
}

func cloneTextureAnim(texAnim components.TextureAnim) components.TextureAnim {
	return components.TextureAnim{
		ID:          texAnim.ID,
		Translation: cloneAnimation(texAnim.Translation),
		Scaling:     cloneAnimation(texAnim.Scaling),
		Rotation:    cloneAnimation(texAnim.Rotation),
	}
}

func cloneAnimation(anim *components.Animation) *components.Animation {
	if anim == nil {
		return nil
	}
	cloned := *anim
	if anim.KeyFrames != nil {
		cloned.KeyFrames = make(map[int]any, len(anim.KeyFrames))
		for k, v := range anim.KeyFrames {
			cloned.KeyFrames[k] = v
		}
	}
	if anim.InOutTans != nil {
		cloned.InOutTans = make(map[int]components.InOutTan, len(anim.InOutTans))
		for k, v := range anim.InOutTans {
			cloned.InOutTans[k] = v
		}
	}
	return &cloned
}
