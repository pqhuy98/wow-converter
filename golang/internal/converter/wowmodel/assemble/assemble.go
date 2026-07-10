package assemble

import (
	"encoding/json"
	"log"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/mtl"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/obj"
	bundleutils "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/utils"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/math"
)

const adtUVPadding = 1.0 / 512
const adtUVEdgeEps = 1e-6

// Inputs are parsed structures for MDL assembly.
type Inputs struct {
	ObjFilePath string
	Obj         obj.Result
	Mtl         struct{ Materials []mtl.Material }
	Animation   *bundleanim.File
	Metadata    *bundlemeta.File
}

// Result is assembled MDL output.
type Result struct {
	MDL          *mdl.MDL
	TexturePaths map[string]struct{}
}

// AssembleWowModel is the shared assembly core: parsed structs -> MDL.
func AssembleWowModel(inputs Inputs, cfg config.Config) Result {
	texturePaths := map[string]struct{}{}
	isAdt := strings.Contains(inputs.ObjFilePath, "adt_")

	name := filepath.ToSlash(filepath.Join(cfg.AssetPrefix,
		relExport(cfg.ExportAssetDir, stripExt(inputs.ObjFilePath))))
	m := mdl.New(mdl.NewMDLOptions{FormatVersion: 1000, Name: name})
	inputs.Metadata.BindMdl(m)

	if len(inputs.Obj.Models) == 0 {
		log.Printf("%s %s", ansi.Red("No models found in"), inputs.ObjFilePath)
		if inputs.Metadata != nil && inputs.Metadata.IsM2File() {
			var extracted bundlemeta.ExtractResult
			extracted = inputs.Metadata.ExtractMDLTexturesMaterials()
			m.Bones = []*components.Bone{components.NewBone("bone_default")}
			if len(m.Sequences) == 0 {
				m.Sequences = []components.Sequence{{
					Name: "Stand", Interval: [2]int{0, 1000},
					Data: components.SequenceData{WC3Name: "Stand"},
					Bound: components.Bound{
						MinimumExtent: math.Vector3{-1, -1, -1},
						MaximumExtent: math.Vector3{1, 1, 1},
						BoundsRadius:  1,
					},
				}}
			}
			if inputs.Metadata.IsM2() {
				inputs.Metadata.ExtractMDLGeosetAnim()
			}
			inputs.Metadata.ExtractMDLParticlesEmitters(extracted.Textures)
			inputs.Metadata.ExtractMDLLights()
			inputs.Metadata.ExtractMDLRibbonEmitters(extracted.Textures)
			inputs.Metadata.ExtractMDLCameras()
		}
		return Result{MDL: m, TexturePaths: texturePaths}
	}

	objModel := inputs.Obj.Models[0]
	groups := map[obj.Group][]obj.Face{}
	var groupOrder []obj.Group
	var geosetPairs []struct {
		geoset *components.Geoset
		group  obj.Group
	}

	for _, face := range objModel.Faces {
		if _, ok := groups[face.Group]; !ok {
			groupOrder = append(groupOrder, face.Group)
			g := &components.Geoset{Name: face.Group.Name, WowData: components.GeosetWowData{SubmeshID: -1}}
			m.Geosets = append(m.Geosets, g)
			geosetPairs = append(geosetPairs, struct {
				geoset *components.Geoset
				group  obj.Group
			}{g, face.Group})
		}
		groups[face.Group] = append(groups[face.Group], face)
	}
	if inputs.Metadata != nil && inputs.Metadata.IsLoaded {
		inputs.Metadata.MapSubMeshesToMdlGeosets(m)
	}

	parentDir := filepath.Dir(normalize(inputs.ObjFilePath))
	var extracted bundlemeta.ExtractResult
	if inputs.Metadata != nil {
		extracted = inputs.Metadata.ExtractMDLTexturesMaterials()
		if inputs.Metadata.IsLoaded {
			inputs.Metadata.RegisterExistingExternalTexturePaths(texturePaths)
		}
	}
	submeshIDToMat := extracted.SubmeshIDToMat

	mtlNameMap := map[string]*components.Material{}
	storeMaterial := func(mat *components.Material) *components.Material {
		if mat == nil {
			return nil
		}
		linkMaterialTextures(m, mat)
		for _, existing := range m.Materials {
			if existing == mat {
				return mat
			}
		}
		m.Materials = append(m.Materials, mat)
		linkMaterialTextures(m, mat)
		return mat
	}
	resolveGeosetMaterial := func(skinSectionIndex int, matName string) *components.Material {
		var mtlMaterial *mtl.Material
		for i := range inputs.Mtl.Materials {
			if inputs.Mtl.Materials[i].Name == matName {
				mtlMaterial = &inputs.Mtl.Materials[i]
				break
			}
		}
		var textureRel string
		if mtlMaterial != nil && mtlMaterial.MapKd != "" {
			textureRel = relExport(cfg.ExportAssetDir, filepath.Join(parentDir, mtlMaterial.MapKd))
			texturePaths[textureRel] = struct{}{}
		}
		if mat, ok := submeshIDToMat[skinSectionIndex]; ok && mat != nil {
			cloned := cloneMaterial(*mat)
			for li := range cloned.Layers {
				if li < len(mat.Layers) {
					cloned.Layers[li].TVertexAnim = mat.Layers[li].TVertexAnim
				}
				layer := &cloned.Layers[li]
				if layer.Texture == nil {
					continue
				}
				if layer.Texture.WowData.PngPath != "" {
					texturePaths[layer.Texture.WowData.PngPath] = struct{}{}
				}
				blpPath := layer.Texture.Image
				if blpPath == "" && textureRel != "" {
					blpPath = filepath.ToSlash(filepath.Join(cfg.AssetPrefix, strings.ReplaceAll(textureRel, ".png", ".blp")))
				}
				if blpPath != "" {
					layer.Texture.Image = blpPath
					if pngRel := layer.Texture.WowData.PngPath; pngRel != "" {
						texturePaths[pngRel] = struct{}{}
					} else {
						texturePaths[strings.TrimPrefix(strings.ReplaceAll(blpPath, ".blp", ".png"), cfg.AssetPrefix+"/")] = struct{}{}
					}
				}
			}
			return storeMaterial(&cloned)
		}
		if inputs.Metadata != nil && inputs.Metadata.IsWmo() {
			if wmoMat := inputs.Metadata.GetWmoMaterialByMtlName(matName); wmoMat != nil {
				cloned := cloneMaterial(*wmoMat)
				for li := range cloned.Layers {
					layer := &cloned.Layers[li]
					if layer.Texture == nil {
						continue
					}
					if layer.Texture.WowData.PngPath != "" {
						texturePaths[layer.Texture.WowData.PngPath] = struct{}{}
					}
					blpPath := layer.Texture.Image
					if blpPath == "" && textureRel != "" {
						blpPath = filepath.ToSlash(filepath.Join(cfg.AssetPrefix, strings.ReplaceAll(textureRel, ".png", ".blp")))
					}
					if blpPath != "" {
						layer.Texture.Image = blpPath
						if pngRel := layer.Texture.WowData.PngPath; pngRel != "" {
							texturePaths[pngRel] = struct{}{}
						} else {
							texturePaths[strings.TrimPrefix(strings.ReplaceAll(blpPath, ".blp", ".png"), cfg.AssetPrefix+"/")] = struct{}{}
						}
					}
				}
				return storeMaterial(&cloned)
			}
		}
		if existing, ok := mtlNameMap[matName]; ok {
			return existing
		}
		if !isAdt {
			log.Printf("%s %s %s %d", ansi.Red("Warning: no material found for matName:"), matName, "submeshId:", skinSectionIndex)
		}
		tex := &components.Texture{WrapWidth: !isAdt, WrapHeight: !isAdt, WowData: components.TextureWowData{PngPath: textureRel}}
		if textureRel != "" {
			tex.Image = filepath.ToSlash(filepath.Join(cfg.AssetPrefix, strings.ReplaceAll(textureRel, ".png", ".blp")))
			texturePaths[textureRel] = struct{}{}
		}
		filter := components.BlendNone
		if textureRel != "" && !isAdt {
			filter = bundleutils.GuessFilterMode(textureRel)
		}
		m.Textures = append(m.Textures, tex)
		mat := &components.Material{
			Layers: []components.Layer{{
				Texture: tex, FilterMode: filter,
				Alpha: components.AnimatedOrStatic[float64]{Static: true, Value: 1},
			}},
		}
		canonical := storeMaterial(mat)
		mtlNameMap[matName] = canonical
		return canonical
	}

	var mdlAnim bundleanim.ToMdlResult
	if inputs.Animation != nil && inputs.Animation.IsLoaded {
		mdlAnim = inputs.Animation.ToMdl(&m.GlobalSequences)
		m.Bones = mdlAnim.Bones
		m.Sequences = mdlAnim.Sequences
		m.WowAttachments = mdlAnim.WowAttachments
	} else {
		m.Bones = []*components.Bone{components.NewBone("bone_default")}
	}
	if len(m.Sequences) == 0 {
		m.Sequences = []components.Sequence{{
			Name: "Stand", Interval: [2]int{0, 1000},
			Data: components.SequenceData{WC3Name: "Stand"},
			Bound: components.Bound{
				MinimumExtent: math.Vector3{-1, -1, -1},
				MaximumExtent: math.Vector3{1, 1, 1},
				BoundsRadius:  1,
			},
		}}
	}

	var enabledSubmeshIndices []int
	if inputs.Metadata != nil && inputs.Metadata.IsM2File() {
		if inputs.Metadata.IsM2() {
			inputs.Metadata.ExtractMDLGeosetAnim()
		}
		inputs.Metadata.ExtractMDLParticlesEmitters(extracted.Textures)
		inputs.Metadata.ExtractMDLLights()
		inputs.Metadata.ExtractMDLRibbonEmitters(extracted.Textures)
		inputs.Metadata.ExtractMDLCameras()
	}

	if inputs.Metadata != nil {
		enabledSubmeshIndices = inputs.Metadata.EnabledSubmeshIndices()
	}

	groupIdx := 0
	for _, group := range groupOrder {
		faces := groups[group]
		if len(faces) == 0 {
			continue
		}
		var geoset *components.Geoset
		for _, pair := range geosetPairs {
			if pair.group == group {
				geoset = pair.geoset
				break
			}
		}
		skinSectionIndex := -1
		if groupIdx < len(enabledSubmeshIndices) {
			skinSectionIndex = enabledSubmeshIndices[groupIdx]
		}
		if geoset != nil {
			geoset.Material = resolveGeosetMaterial(skinSectionIndex, faces[0].Material)
		}
		groupIdx++
		vMap := map[int]*components.GeosetVertex{}
		for _, face := range faces {
			var verts [3]*components.GeosetVertex
			for vi, fv := range face.Vertices {
				if fv.VertexIndex == 0 {
					continue
				}
				if v, ok := vMap[fv.VertexIndex]; ok {
					verts[vi] = v
					continue
				}
				ov := objModel.Vertices[fv.VertexIndex-1]
				on := objModel.VertexNormals[fv.VertexNormalIndex-1]
				var tu, tv float64
				if fv.TextureCoordsIndex > 0 && fv.TextureCoordsIndex-1 < len(objModel.TextureCoords) {
					tu = objModel.TextureCoords[fv.TextureCoordsIndex-1].U
					tv = objModel.TextureCoords[fv.TextureCoordsIndex-1].V
				}
				if isAdt {
					u, v := padUVEdgeAware(tu, 1-tv)
					tu, tv = u, v
				} else {
					tv = 1 - tv
				}
				var texPosition2 *math.Vector2
				if fv.TextureCoordsIndex > 0 && fv.TextureCoordsIndex-1 < len(objModel.TextureCoords2) {
					t2 := objModel.TextureCoords2[fv.TextureCoordsIndex-1]
					t2u, t2v := t2.U, t2.V
					if isAdt {
						u, v := padUVEdgeAware(t2u, 1-t2v)
						t2u, t2v = u, v
					} else {
						t2v = 1 - t2v
					}
					texPosition2 = &math.Vector2{t2u, t2v}
				}
				gv := &components.GeosetVertex{
					Position:     math.Vector3{ov.X, -ov.Z, ov.Y},
					Normal:       math.Vector3{on.X, -on.Z, on.Y},
					TexPosition:  math.Vector2{tu, tv},
					TexPosition2: texPosition2,
				}
				if inputs.Animation != nil && inputs.Animation.IsLoaded {
					swi := inputs.Metadata.GetSkinWeightIndex(fv.VertexIndex - 1)
					if swi >= 0 && swi < len(mdlAnim.SkinWeights) {
						gv.SkinWeights = mdlAnim.SkinWeights[swi]
					}
				} else if geoset != nil {
					if len(geoset.Matrices) == 0 {
						geoset.Matrices = append(geoset.Matrices, components.Matrix{Bones: []*components.Bone{m.Bones[0]}})
					}
					gv.Matrix = &geoset.Matrices[0]
				}
				if geoset != nil {
					geoset.Vertices = append(geoset.Vertices, gv)
				}
				vMap[fv.VertexIndex] = gv
				verts[vi] = gv
			}
			if geoset != nil && verts[0] != nil && verts[1] != nil && verts[2] != nil {
				geoset.Faces = append(geoset.Faces, components.Face{Vertices: verts})
			}
		}
	}

	if inputs.Metadata != nil && inputs.Metadata.IsM2() {
		for i, geoset := range m.Geosets {
			if i >= len(enabledSubmeshIndices) {
				continue
			}
			submeshIdx := enabledSubmeshIndices[i]
			submesh := inputs.Metadata.SubmeshAt(submeshIdx)
			if submesh == nil || int(submesh.VertexCount) != len(geoset.Vertices) {
				payload, _ := json.Marshal(map[string]any{
					"subMesh":         submesh,
					"geoset":          geoset.Name,
					"geosetVertices":  len(geoset.Vertices),
				})
				log.Printf("%s %s", ansi.Red("Submesh mismatch"), string(payload))
			}
		}
	}

	m.Modify.AddDoodadDeathAnimation()
	renameEffectWowAnimations(m)
	if isAdt {
		m.Modify.RecomputeNormals()
	}
	m.Modify.OptimizeKeyFrames()
	m.Modify.ComputeWalkMovespeed()
	m.Modify.Scale(cfg.RawModelScaleUp)
	m.AccumScale = 1
	m.Modify.AddCollisionShapes()
	m.Sync()
	m.Modify.AddWc3AttachmentPoint()
	registerMdlTexturePaths(m, texturePaths, cfg.AssetPrefix)
	if !cfg.IsBulkExport {
		log.Printf("%s %s", ansi.Green("Converted:"), inputs.ObjFilePath)
	}
	return Result{MDL: m, TexturePaths: texturePaths}
}

func renameEffectWowAnimations(m *mdl.MDL) {
	var hold, decay, stand *components.Sequence
	for i := range m.Sequences {
		switch m.Sequences[i].Data.WowName {
		case "Hold":
			hold = &m.Sequences[i]
		case "Decay":
			decay = &m.Sequences[i]
		case "Stand":
			stand = &m.Sequences[i]
		}
	}
	if hold != nil && stand != nil && decay != nil {
		stand.Name = "Birth"
		stand.NonLooping = true
		decay.NonLooping = true
	}
}

func registerMdlTexturePaths(m *mdl.MDL, texturePaths map[string]struct{}, assetPrefix string) {
	for _, tex := range m.Textures {
		if png := strings.TrimSpace(tex.WowData.PngPath); png != "" {
			texturePaths[png] = struct{}{}
			continue
		}
		if tex.Image == "" {
			continue
		}
		png := strings.TrimPrefix(strings.ReplaceAll(tex.Image, ".blp", ".png"), assetPrefix+"/")
		png = strings.TrimPrefix(png, assetPrefix+"\\")
		if png != "" {
			texturePaths[png] = struct{}{}
		}
	}
}

func linkMaterialTextures(m *mdl.MDL, mat *components.Material) {
	if mat == nil {
		return
	}
	for li := range mat.Layers {
		layer := &mat.Layers[li]
		if layer.Texture == nil {
			continue
		}
		idx := -1
		for i := range m.Textures {
			if m.Textures[i].Image == layer.Texture.Image &&
				m.Textures[i].WowData.PngPath == layer.Texture.WowData.PngPath &&
				m.Textures[i].WowData.Type == layer.Texture.WowData.Type {
				idx = i
				break
			}
		}
		if idx < 0 {
			m.Textures = append(m.Textures, layer.Texture)
			idx = len(m.Textures) - 1
		}
		layer.Texture = m.Textures[idx]
	}
}

func layerTextures(mat *components.Material) []components.Texture {
	var out []components.Texture
	for _, l := range mat.Layers {
		if l.Texture != nil {
			out = append(out, *l.Texture)
		}
	}
	return out
}

func padUVEdgeAware(u, v float64) (float64, float64) {
	pad := func(x float64) float64 {
		if x <= adtUVEdgeEps {
			return 0
		}
		if x >= 1-adtUVEdgeEps {
			return 1
		}
		return x*(1-2*adtUVPadding) + adtUVPadding
	}
	return pad(u), pad(v)
}

func normalize(p string) string { return strings.ReplaceAll(p, "\\", "/") }

func relExport(root, abs string) string {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}

func stripExt(p string) string {
	ext := filepath.Ext(p)
	if ext != "" {
		return strings.TrimSuffix(p, ext)
	}
	return p
}

func cloneMaterial(src components.Material) components.Material {
	dst := src
	dst.Layers = make([]components.Layer, len(src.Layers))
	for i, layer := range src.Layers {
		cl := layer
		if layer.Texture != nil {
			tex := *layer.Texture
			cl.Texture = &tex
		}
		dst.Layers[i] = cl
	}
	return dst
}

// placeholder to keep texturesource referenced
var _ = texturesource.Source{}
