package metadata

import (
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	bundleutils "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/utils"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// File holds M2/WMO metadata for MDL assembly.
type File struct {
	FilePath  string
	Config    config.Config
	Animation *bundleanim.File
	IsLoaded  bool

	fileType                string
	textures                []Texture
	textureTypes            []int
	materials               []m2.MaterialEntry
	textureCombos           []int
	textureTransforms       []TextureTransform
	textureTransformsLookup []int
	transparencyLookup      []uint16
	textureWeights          []m2.Track
	globalLoops             []uint32
	m2Animations            []m2AnimMeta
	colors                  []Color
	cameras                 []m2.CameraEntry
	lights                  []m2.LightEntry
	ribbonEmitters          []m2.RibbonEmitterEntry
	particleEmitters        []m2.ParticleEmitterEntry
	skin                    Skin
	objToSubmesh            map[int]int
	mdl                     *mdl.MDL
	globalSequenceMap       map[int]*components.GlobalSequence
	wmoMaterials            []wmoMaterialMeta
	wmoMaterialNameToMat    map[string]*components.Material
}

// NewFile creates an empty metadata file.
func NewFile(filePath string, cfg config.Config, anim *bundleanim.File) *File {
	return &File{FilePath: filePath, Config: cfg, Animation: anim}
}

// LoadFromData replaces metadata with an owned, typed assembly snapshot.
func (f *File) LoadFromData(data Data) {
	f.fileType = data.FileType
	f.textures = slices.Clone(data.Textures)
	f.materials = slices.Clone(data.Materials)
	f.wmoMaterials = slices.Clone(data.WMOMaterials)
	for i := range f.wmoMaterials {
		f.wmoMaterials[i].RuntimeData = slices.Clone(f.wmoMaterials[i].RuntimeData)
	}
	f.textureTypes = integerSlice(data.TextureTypes)
	f.textureCombos = integerSlice(data.TextureCombos)
	f.textureTransformsLookup = integerSlice(data.TextureTransformsLookup)
	f.transparencyLookup = slices.Clone(data.TransparencyLookup)
	f.globalLoops = slices.Clone(data.GlobalLoops)
	f.m2Animations = nil
	for _, anim := range data.M2Animations {
		f.m2Animations = append(f.m2Animations, m2AnimMeta{Duration: anim.Duration})
	}
	f.skin = Skin{SubMeshes: slices.Clone(data.Skin.SubMeshes), TextureUnits: slices.Clone(data.Skin.TextureUnits)}
	f.textureTransforms = slices.Clone(data.TextureTransforms)
	for i := range f.textureTransforms {
		t := &f.textureTransforms[i]
		t.Translation, t.Rotation, t.Scaling = cloneAnimationTrack(t.Translation), cloneAnimationTrack(t.Rotation), cloneAnimationTrack(t.Scaling)
	}
	f.colors = slices.Clone(data.Colors)
	for i := range f.colors {
		f.colors[i].Color, f.colors[i].Alpha = cloneAnimationTrack(f.colors[i].Color), cloneAnimationTrack(f.colors[i].Alpha)
	}
	f.textureWeights = slices.Clone(data.TextureWeights)
	for i := range f.textureWeights {
		f.textureWeights[i] = cloneTrack(f.textureWeights[i])
	}
	f.cameras = cloneCameras(data.Cameras)
	f.lights = cloneLights(data.Lights)
	f.ribbonEmitters = cloneRibbons(data.RibbonEmitters)
	f.particleEmitters = cloneParticles(data.ParticleEmitters)
	f.objToSubmesh, f.globalSequenceMap, f.wmoMaterialNameToMat, f.mdl = nil, nil, nil, nil
	f.IsLoaded = data.FileType == "m2" || data.FileType == "wmo"
}

func integerSlice[T ~uint16 | ~uint32](values []T) []int {
	if values == nil {
		return nil
	}
	out := make([]int, len(values))
	for i, value := range values {
		out[i] = int(value)
	}
	return out
}

// IsM2 reports whether metadata includes M2 skin/geoset layout.
func (f *File) IsM2() bool {
	return f.IsLoaded && f.fileType == "m2" && len(f.skin.SubMeshes) > 0
}

// IsM2File reports whether metadata describes any M2 model, including particle-only effects.
func (f *File) IsM2File() bool {
	return f.IsLoaded && f.fileType == "m2"
}

// EnabledSubmeshIndices returns skin section indices for enabled submeshes, in order.
func (f *File) EnabledSubmeshIndices() []int {
	if !f.IsM2() {
		return nil
	}
	var out []int
	for i, subMesh := range f.skin.SubMeshes {
		if subMesh.Enabled {
			out = append(out, i)
		}
	}
	return out
}

// SubmeshAt returns the skin submesh at index, or nil when out of range.
func (f *File) SubmeshAt(index int) *SubMesh {
	if index < 0 || index >= len(f.skin.SubMeshes) {
		return nil
	}
	sm := f.skin.SubMeshes[index]
	return &sm
}

// BindMdl associates the metadata with an MDL being assembled.
func (f *File) BindMdl(m *mdl.MDL) {
	f.mdl = m
	f.globalSequenceMap = map[int]*components.GlobalSequence{}
	for _, gs := range m.GlobalSequences {
		if gs == nil {
			continue
		}
		key := gs.ID
		if gs.HasRawID {
			key = gs.RawID
		}
		f.globalSequenceMap[key] = gs
	}
}

// MapSubMeshesToMdlGeosets assigns submesh IDs to geosets.
func (f *File) MapSubMeshesToMdlGeosets(m *mdl.MDL) {
	geosetIdx := 0
	for _, subMesh := range f.skin.SubMeshes {
		if !subMesh.Enabled {
			continue
		}
		if geosetIdx < len(m.Geosets) {
			m.Geosets[geosetIdx].WowData.SubmeshID = subMesh.SubmeshID
			geosetIdx++
		}
	}
}

// ExtractResult holds texture/material extraction output.
type ExtractResult struct {
	Textures       []components.Texture
	SubmeshIDToMat map[int]*components.Material
}

// RegisterExistingExternalTexturePaths adds resolvable metadata texture paths to texturePaths.
func (f *File) RegisterExistingExternalTexturePaths(texturePaths map[string]struct{}) {
	if !f.IsLoaded {
		return
	}
	parentDir := filepath.Dir(normalizePath(f.FilePath))
	for _, tex := range f.textures {
		if tex.FileNameExternal == "" {
			continue
		}
		absPath := filepath.Join(parentDir, tex.FileNameExternal)
		relPath := relExport(f.Config.ExportAssetDir, absPath)
		if _, err := os.Stat(filepath.Clean(absPath)); err != nil && !texturesource.Has(relPath) {
			log.Printf("Skipping texture not found %s for model %s", absPath, f.FilePath)
			continue
		}
		texturePaths[relPath] = struct{}{}
	}
}

// ExtractMDLTexturesMaterials builds MDL textures and per-submesh materials.
func (f *File) ExtractMDLTexturesMaterials() ExtractResult {
	if !f.IsLoaded {
		return ExtractResult{SubmeshIDToMat: map[int]*components.Material{}}
	}
	if f.IsWmo() {
		return f.extractWmoTexturesMaterials()
	}
	parentDir := filepath.Dir(normalizePath(f.FilePath))
	textures := make([]components.Texture, len(f.textures))
	for i, tex := range f.textures {
		pngPath := ""
		if tex.FileNameExternal != "" {
			pngPath = relExport(f.Config.ExportAssetDir, filepath.Join(parentDir, tex.FileNameExternal))
		}
		image := ""
		if pngPath != "" {
			image = filepath.ToSlash(filepath.Join(f.Config.AssetPrefix, strings.ReplaceAll(pngPath, ".png", ".blp")))
		}
		texType := 0
		if i < len(f.textureTypes) {
			texType = f.textureTypes[i]
		}
		textures[i] = components.Texture{
			Image: image, WrapWidth: tex.Flags&1 > 0, WrapHeight: tex.Flags&2 > 0,
			WowData: components.TextureWowData{Type: texType, PngPath: pngPath},
		}
	}

	textureAnims := f.buildTextureAnims()

	submeshMaterials := map[int]*components.Material{}
	for _, tu := range f.skin.TextureUnits {
		submeshID := tu.SkinSectionIndex
		if tu.MaterialIndex >= len(f.materials) {
			continue
		}
		material := f.materials[tu.MaterialIndex]
		twoSided := material.Flags&0x04 > 0
		if _, ok := submeshMaterials[submeshID]; !ok {
			submeshMaterials[submeshID] = &components.Material{TwoSided: twoSided, PriorityPlane: tu.Priority}
		}
		layers := &submeshMaterials[submeshID].Layers
		textureCount := tu.TextureCount
		if textureCount > 4 {
			textureCount = 4
		}
		for i := 0; i < textureCount; i++ {
			comboIdx := tu.TextureComboIndex + i
			if comboIdx >= len(f.textureCombos) {
				continue
			}
			textureID := f.textureCombos[comboIdx]
			if textureID >= len(textures) {
				continue
			}
			textAnimID := textureTransformIndex(f.textureTransformsLookup, tu.TextureTransformComboIndex, i)
			if shouldDisableTextureTransform(tu.ShaderID, textureCount, i) {
				textAnimID = config.BlizzardNull
			}
			filterMode := bundleutils.GetLayerFilterMode(uint16(material.BlendingMode), uint16(tu.ShaderID), i, textures[textureID].Image)
			if filterMode == nil {
				continue
			}
			alpha := components.AnimatedOrStatic[float64]{Static: true, Value: 1}
			if tu.Flags&0x40 == 0 {
				alpha = f.TextureWeight(tu.TextureWeightComboIndex)
			}
			layer := components.Layer{
				Texture: &textures[textureID], FilterMode: *filterMode, Alpha: alpha,
				Unlit: material.Flags&0x01 > 0, Unshaded: material.Flags&0x01 > 0, Unfogged: material.Flags&0x02 > 0,
				TwoSided: material.Flags&0x04 > 0, NoDepthTest: material.Flags&0x08 > 0,
				NoDepthSet: material.Flags&0x10 > 0,
			}
			if textAnimID != config.BlizzardNull && textAnimID < len(textureAnims) {
				layer.TVertexAnim = &textureAnims[textAnimID]
			}
			*layers = append(*layers, layer)
		}
	}
	return ExtractResult{Textures: textures, SubmeshIDToMat: submeshMaterials}
}

// GetSkinWeightIndex maps OBJ vertex index to M2 skin weight index.
func (f *File) GetSkinWeightIndex(geosetVertexIndex int) int {
	if f.objToSubmesh == nil {
		f.objToSubmesh = map[int]int{}
		idx := 0
		for _, submesh := range f.skin.SubMeshes {
			if !submesh.Enabled {
				continue
			}
			for v := submesh.VertexStart; v < submesh.VertexStart+submesh.VertexCount; v++ {
				f.objToSubmesh[idx] = v
				idx++
			}
		}
	}
	if v, ok := f.objToSubmesh[geosetVertexIndex]; ok {
		return v
	}
	return geosetVertexIndex
}

func normalizePath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

func relExport(exportRoot, abs string) string {
	rel, err := filepath.Rel(exportRoot, abs)
	if err != nil {
		return abs
	}
	return filepath.ToSlash(rel)
}
