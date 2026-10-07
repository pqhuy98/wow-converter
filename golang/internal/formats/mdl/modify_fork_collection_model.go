package mdl

import (
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
)

// CollectionModel is a minimal model wrapper for collection armor fork/merge.
type CollectionModel struct {
	RelativePath string
	MDL          *MDL
}

func ForkCollectionModel(template CollectionModel, enabledGeosets []*components.Geoset) CollectionModel {
	src := template.MDL
	enabledSet := map[*components.Geoset]struct{}{}
	for _, g := range enabledGeosets {
		enabledSet[g] = struct{}{}
	}
	materialSet := map[*components.Material]struct{}{}
	var materialOrder []*components.Material
	for _, g := range enabledGeosets {
		if g.Material != nil {
			if _, ok := materialSet[g.Material]; !ok {
				materialOrder = append(materialOrder, g.Material)
			}
			materialSet[g.Material] = struct{}{}
		}
	}
	for _, ribbon := range src.RibbonEmitters {
		if ribbon.Material != nil {
			if _, ok := materialSet[ribbon.Material]; !ok {
				materialOrder = append(materialOrder, ribbon.Material)
			}
			materialSet[ribbon.Material] = struct{}{}
		}
	}
	textures := make([]*components.Texture, len(src.Textures))
	for i, tex := range src.Textures {
		if tex == nil {
			continue
		}
		cloned := *tex
		textures[i] = &cloned
	}
	textureMap := map[*components.Texture]*components.Texture{}
	for i, tex := range src.Textures {
		if tex != nil {
			textureMap[tex] = textures[i]
		}
	}
	textureAnims := make([]components.TextureAnim, len(src.TextureAnims))
	for i, textureAnim := range src.TextureAnims {
		textureAnims[i] = cloneTextureAnim(textureAnim)
	}
	textureAnimMap := map[*components.TextureAnim]*components.TextureAnim{}
	for i := range src.TextureAnims {
		textureAnimMap[&src.TextureAnims[i]] = &textureAnims[i]
	}
	materials := make([]*components.Material, 0, len(materialOrder))
	materialMap := map[*components.Material]*components.Material{}
	for _, mat := range materialOrder {
		cloned := *mat
		cloned.Layers = append([]components.Layer(nil), mat.Layers...)
		for i := range cloned.Layers {
			if !cloned.Layers[i].Alpha.Static {
				cloned.Layers[i].Alpha.Anim = cloneAnimation(cloned.Layers[i].Alpha.Anim)
			}
			if anim := cloned.Layers[i].TextureIDAnim; anim != nil {
				anim = cloneAnimation(anim)
				for time, value := range anim.KeyFrames {
					if texture, ok := value.(*components.Texture); ok {
						anim.KeyFrames[time] = textureMap[texture]
					}
				}
				cloned.Layers[i].TextureIDAnim = anim
			}
			tex := cloned.Layers[i].Texture
			if tex == nil {
				continue
			}
			if mapped := textureMap[tex]; mapped != nil {
				cloned.Layers[i].Texture = mapped
			} else {
				texCopy := *tex
				cloned.Layers[i].Texture = &texCopy
			}
			if tv := cloned.Layers[i].TVertexAnim; tv != nil {
				if mapped := textureAnimMap[tv]; mapped != nil {
					cloned.Layers[i].TVertexAnim = mapped
				} else if tv.ID >= 0 && tv.ID < len(textureAnims) {
					cloned.Layers[i].TVertexAnim = &textureAnims[tv.ID]
				}
			}
		}
		materials = append(materials, &cloned)
		materialMap[mat] = materials[len(materials)-1]
	}

	geosetMap := map[*components.Geoset]*components.Geoset{}
	geosets := make([]*components.Geoset, 0, len(enabledGeosets))
	for _, geoset := range enabledGeosets {
		if geoset == nil {
			continue
		}
		cloned := *geoset
		cloned.Vertices = cloneGeosetVertices(geoset.Vertices)
		cloned.Faces = cloneFaces(geoset.Faces, geoset.Vertices, cloned.Vertices)
		cloned.Matrices = cloneMatrices(geoset.Matrices)
		if cloned.Material != nil {
			cloned.Material = materialMap[cloned.Material]
		}
		geosets = append(geosets, &cloned)
		geosetMap[geoset] = &cloned
	}

	var filteredGeosetAnims []components.GeosetAnim
	for _, ga := range src.GeosetAnims {
		if _, ok := enabledSet[ga.Geoset]; ok {
			cloned := ga
			cloned.Geoset = geosetMap[ga.Geoset]
			cloned.Alpha = cloneAnimatedValue(cloned.Alpha)
			cloned.Color = cloneAnimatedValue(cloned.Color)
			filteredGeosetAnims = append(filteredGeosetAnims, cloned)
		}
	}

	mdl := New(NewMDLOptions{FormatVersion: src.Version.FormatVersion, Name: src.Model.Name})
	mdl.Model = src.Model
	mdl.AccumScale = src.AccumScale
	mdl.Geosets = geosets
	mdl.Materials = materials
	mdl.Textures = textures
	mdl.TextureAnims = textureAnims
	mdl.GeosetAnims = filteredGeosetAnims
	mdl.GlobalSequences = append([]*components.GlobalSequence(nil), src.GlobalSequences...)
	mdl.Sequences = append([]components.Sequence(nil), src.Sequences...)
	mdl.Attachments = cloneAttachments(src.Attachments)
	mdl.Lights = cloneLights(src.Lights)
	mdl.RibbonEmitters = append([]*components.RibbonEmitter(nil), src.RibbonEmitters...)
	for i, ribbon := range mdl.RibbonEmitters {
		cloned := *ribbon
		cloneNodeBaseAnimations(&cloned.NodeBase)
		cloned.HeightAbove = cloneAnimatedValue(cloned.HeightAbove)
		cloned.HeightBelow = cloneAnimatedValue(cloned.HeightBelow)
		cloned.Alpha = cloneAnimatedValue(cloned.Alpha)
		cloned.Color = cloneAnimatedValue(cloned.Color)
		cloned.TextureSlot = cloneAnimatedValue(cloned.TextureSlot)
		cloned.Visibility = cloneAnimation(cloned.Visibility)
		cloned.Material = materialMap[ribbon.Material]
		mdl.RibbonEmitters[i] = &cloned
	}
	mdl.ParticleEmitter2s = cloneParticleEmitters(src.ParticleEmitter2s)
	mdl.Helpers = cloneHelpers(src.Helpers)
	mdl.Cameras = append([]components.Camera(nil), src.Cameras...)
	for i := range mdl.Cameras {
		mdl.Cameras[i].Translation = cloneAnimation(mdl.Cameras[i].Translation)
		mdl.Cameras[i].Rotation = cloneAnimation(mdl.Cameras[i].Rotation)
		mdl.Cameras[i].Scaling = cloneAnimation(mdl.Cameras[i].Scaling)
	}
	mdl.EventObjects = cloneEventObjects(src.EventObjects)
	mdl.CollisionShapes = cloneCollisionShapes(src.CollisionShapes)
	mdl.Bones = src.Bones
	mdl.WowAttachments = src.WowAttachments
	rebindForkNodeParents(src, mdl)

	return CollectionModel{RelativePath: template.RelativePath, MDL: mdl}
}

func rebindForkNodeParents(source, fork *MDL) {
	sourceNodes := source.GetNodes()
	forkNodes := fork.GetNodes()
	if len(sourceNodes) != len(forkNodes) {
		return
	}
	nodeMap := make(map[components.Node]components.Node, len(sourceNodes))
	for i, sourceNode := range sourceNodes {
		nodeMap[sourceNode] = forkNodes[i]
	}
	for _, forkNode := range forkNodes {
		if parent := forkNode.NodeParent(); parent != nil {
			if clonedParent, ok := nodeMap[parent]; ok {
				forkNode.SetNodeParent(clonedParent)
			}
		}
	}
}

func cloneNodeBaseAnimations(base *components.NodeBase) {
	base.Translation = cloneAnimation(base.Translation)
	base.Rotation = cloneAnimation(base.Rotation)
	base.Scaling = cloneAnimation(base.Scaling)
}

func cloneAnimatedValue[T any](value *components.AnimatedOrStatic[T]) *components.AnimatedOrStatic[T] {
	if value == nil {
		return nil
	}
	cloned := *value
	if !cloned.Static {
		cloned.Anim = cloneAnimation(cloned.Anim)
	}
	return &cloned
}

func cloneAttachments(source []*components.AttachmentPoint) []*components.AttachmentPoint {
	out := make([]*components.AttachmentPoint, len(source))
	for i, value := range source {
		if value == nil {
			continue
		}
		cloned := *value
		cloneNodeBaseAnimations(&cloned.NodeBase)
		out[i] = &cloned
	}
	return out
}

func cloneLights(source []*components.Light) []*components.Light {
	out := make([]*components.Light, len(source))
	for i, value := range source {
		if value == nil {
			continue
		}
		cloned := *value
		cloneNodeBaseAnimations(&cloned.NodeBase)
		cloned.AttenuationStart.Anim = cloneAnimation(cloned.AttenuationStart.Anim)
		cloned.AttenuationEnd.Anim = cloneAnimation(cloned.AttenuationEnd.Anim)
		cloned.Intensity.Anim = cloneAnimation(cloned.Intensity.Anim)
		cloned.Color.Anim = cloneAnimation(cloned.Color.Anim)
		cloned.AmbientIntensity.Anim = cloneAnimation(cloned.AmbientIntensity.Anim)
		cloned.AmbientColor.Anim = cloneAnimation(cloned.AmbientColor.Anim)
		cloned.Visibility = cloneAnimation(cloned.Visibility)
		out[i] = &cloned
	}
	return out
}

func cloneParticleEmitters(source []*components.ParticleEmitter2) []*components.ParticleEmitter2 {
	out := make([]*components.ParticleEmitter2, len(source))
	for i, value := range source {
		if value == nil {
			continue
		}
		cloned := *value
		cloneNodeBaseAnimations(&cloned.NodeBase)
		cloned.Visibility = cloneAnimation(cloned.Visibility)
		cloned.Width.Anim = cloneAnimation(cloned.Width.Anim)
		cloned.Length.Anim = cloneAnimation(cloned.Length.Anim)
		cloned.EmissionRate.Anim = cloneAnimation(cloned.EmissionRate.Anim)
		cloned.Latitude.Anim = cloneAnimation(cloned.Latitude.Anim)
		cloned.Speed.Anim = cloneAnimation(cloned.Speed.Anim)
		cloned.Variation.Anim = cloneAnimation(cloned.Variation.Anim)
		cloned.Gravity.Anim = cloneAnimation(cloned.Gravity.Anim)
		out[i] = &cloned
	}
	return out
}

func cloneHelpers(source []*components.Helper) []*components.Helper {
	out := make([]*components.Helper, len(source))
	for i, value := range source {
		if value == nil {
			continue
		}
		cloned := *value
		cloneNodeBaseAnimations(&cloned.NodeBase)
		out[i] = &cloned
	}
	return out
}

func cloneEventObjects(source []*components.EventObject) []*components.EventObject {
	out := make([]*components.EventObject, len(source))
	for i, value := range source {
		if value == nil {
			continue
		}
		cloned := *value
		cloneNodeBaseAnimations(&cloned.NodeBase)
		cloned.Track = append([]components.EventTrackEntry(nil), value.Track...)
		out[i] = &cloned
	}
	return out
}

func cloneCollisionShapes(source []*components.CollisionShape) []*components.CollisionShape {
	out := make([]*components.CollisionShape, len(source))
	for i, value := range source {
		if value == nil {
			continue
		}
		cloned := *value
		cloneNodeBaseAnimations(&cloned.NodeBase)
		cloned.Vertices = append([]imath.Vector3(nil), value.Vertices...)
		out[i] = &cloned
	}
	return out
}

func cloneGeosetVertices(vertices []*components.GeosetVertex) []*components.GeosetVertex {
	out := make([]*components.GeosetVertex, len(vertices))
	for i, vertex := range vertices {
		if vertex == nil {
			continue
		}
		cloned := *vertex
		cloned.SkinWeights = append([]components.SkinWeight(nil), vertex.SkinWeights...)
		out[i] = &cloned
	}
	return out
}

func cloneMatrices(matrices []components.Matrix) []components.Matrix {
	out := make([]components.Matrix, len(matrices))
	for i, matrix := range matrices {
		out[i] = matrix
		out[i].Bones = append([]*components.Bone(nil), matrix.Bones...)
	}
	return out
}

func cloneFaces(faces []components.Face, oldVertices, newVertices []*components.GeosetVertex) []components.Face {
	vertexMap := map[*components.GeosetVertex]*components.GeosetVertex{}
	for i, old := range oldVertices {
		if i < len(newVertices) {
			vertexMap[old] = newVertices[i]
		}
	}
	out := make([]components.Face, len(faces))
	for i, face := range faces {
		out[i] = face
		for j, vertex := range face.Vertices {
			if cloned, ok := vertexMap[vertex]; ok {
				out[i].Vertices[j] = cloned
			}
		}
	}
	return out
}
