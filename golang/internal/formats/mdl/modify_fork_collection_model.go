package mdl

import "github.com/pqhuy98/wow-converter/internal/formats/mdl/components"

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
	textureAnims := append([]components.TextureAnim(nil), src.TextureAnims...)
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
	mdl.Attachments = append([]*components.AttachmentPoint(nil), src.Attachments...)
	mdl.Lights = append([]*components.Light(nil), src.Lights...)
	mdl.RibbonEmitters = append([]*components.RibbonEmitter(nil), src.RibbonEmitters...)
	mdl.ParticleEmitter2s = append([]*components.ParticleEmitter2(nil), src.ParticleEmitter2s...)
	mdl.Helpers = append([]*components.Helper(nil), src.Helpers...)
	mdl.Cameras = append([]components.Camera(nil), src.Cameras...)
	mdl.EventObjects = append([]*components.EventObject(nil), src.EventObjects...)
	mdl.Bones = src.Bones
	mdl.WowAttachments = src.WowAttachments

	return CollectionModel{RelativePath: template.RelativePath, MDL: mdl}
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
