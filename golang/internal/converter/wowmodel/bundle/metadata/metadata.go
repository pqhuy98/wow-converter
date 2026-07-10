package metadata

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
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
	raw       map[string]any

	fileType                string
	textures                []textureMeta
	textureTypes            []int
	materials               []materialMeta
	textureCombos           []int
	textureTransforms       []map[string]any
	textureTransformsLookup []int
	m2Animations            []m2AnimMeta
	colors                  []map[string]any
	cameras                 []m2.CameraEntry
	lights                  []m2.LightEntry
	ribbonEmitters          []m2.RibbonEmitterEntry
	particleEmitters        []m2.ParticleEmitterEntry
	skin                    skinMeta
	objToSubmesh            map[int]int
	mdl                     *mdl.MDL
	globalSequenceMap       map[int]*components.GlobalSequence
	wmoMaterials            []wmoMaterialMeta
	wmoMaterialNameToMat    map[string]*components.Material
}

type textureMeta struct {
	FileNameExternal string `json:"fileNameExternal"`
	MtlName          string `json:"mtlName"`
	Flags            int    `json:"flags"`
	FileDataID       int    `json:"fileDataID"`
}

type materialMeta struct {
	Flags        int `json:"flags"`
	BlendingMode int `json:"blendingMode"`
}

type skinMeta struct {
	SubMeshes    []subMeshMeta     `json:"subMeshes"`
	TextureUnits []textureUnitMeta `json:"textureUnits"`
}

type subMeshMeta struct {
	Enabled     bool `json:"enabled"`
	SubmeshID   int  `json:"submeshID"`
	VertexStart int  `json:"vertexStart"`
	VertexCount int  `json:"vertexCount"`
}

type textureUnitMeta struct {
	ShaderID                   int `json:"shaderID"`
	SkinSectionIndex           int `json:"skinSectionIndex"`
	MaterialIndex              int `json:"materialIndex"`
	TextureCount               int `json:"textureCount"`
	TextureComboIndex          int `json:"textureComboIndex"`
	TextureTransformComboIndex int `json:"textureTransformComboIndex"`
	ColorIndex                 int `json:"colorIndex"`
}

// NewFile creates an empty metadata file.
func NewFile(filePath string, cfg config.Config, anim *bundleanim.File) *File {
	return &File{FilePath: filePath, Config: cfg, Animation: anim}
}

// LoadFromData populates metadata from a direct pipeline object.
func (f *File) LoadFromData(data map[string]any) {
	f.raw = data
	f.fileType, _ = data["fileType"].(string)
	if f.fileType == "wmo" {
		if v, ok := data["textures"].([]any); ok {
			b, _ := json.Marshal(v)
			_ = json.Unmarshal(b, &f.textures)
		}
		if v, ok := data["materials"].([]any); ok {
			b, _ := json.Marshal(v)
			_ = json.Unmarshal(b, &f.wmoMaterials)
		}
		f.IsLoaded = true
		return
	}
	if f.fileType != "m2" {
		f.IsLoaded = false
		return
	}
	if v, ok := data["textures"].([]any); ok {
		b, _ := json.Marshal(v)
		_ = json.Unmarshal(b, &f.textures)
	}
	if v, ok := data["textureTypes"].([]any); ok {
		for _, x := range v {
			switch n := x.(type) {
			case float64:
				f.textureTypes = append(f.textureTypes, int(n))
			case int:
				f.textureTypes = append(f.textureTypes, n)
			}
		}
	} else if v, ok := data["textureTypes"].([]uint32); ok {
		for _, n := range v {
			f.textureTypes = append(f.textureTypes, int(n))
		}
	} else if v, ok := data["textureTypes"].([]int); ok {
		f.textureTypes = append(f.textureTypes, v...)
	}
	if v, ok := data["materials"].([]any); ok {
		b, _ := json.Marshal(v)
		_ = json.Unmarshal(b, &f.materials)
	}
	if v, ok := data["textureCombos"].([]any); ok {
		for _, x := range v {
			switch n := x.(type) {
			case float64:
				f.textureCombos = append(f.textureCombos, int(n))
			case int:
				f.textureCombos = append(f.textureCombos, n)
			}
		}
	}
	if v, ok := data["textureTransforms"].([]any); ok {
		for _, x := range v {
			if m, ok := x.(map[string]any); ok {
				f.textureTransforms = append(f.textureTransforms, m)
			}
		}
	}
	if v, ok := data["textureTransformsLookup"].([]any); ok {
		for _, x := range v {
			switch n := x.(type) {
			case float64:
				f.textureTransformsLookup = append(f.textureTransformsLookup, int(n))
			case int:
				f.textureTransformsLookup = append(f.textureTransformsLookup, n)
			}
		}
	}
	loadM2AnimationDurations(data["m2Animations"], &f.m2Animations)
	if v, ok := data["colors"].([]any); ok {
		for _, x := range v {
			if m, ok := x.(map[string]any); ok {
				f.colors = append(f.colors, m)
			}
		}
	}
	if skinRaw, ok := data["skin"].(map[string]any); ok {
		b, _ := json.Marshal(skinRaw)
		_ = json.Unmarshal(b, &f.skin)
	}
	loadJSONField(data, "cameras", &f.cameras)
	loadJSONField(data, "lights", &f.lights)
	loadJSONField(data, "ribbonEmitters", &f.ribbonEmitters)
	loadJSONField(data, "particleEmitters", &f.particleEmitters)
	f.IsLoaded = true
}

func loadM2AnimationDurations(raw any, out *[]m2AnimMeta) {
	if raw == nil {
		return
	}
	var animations []m2.AnimationEntry
	b, err := json.Marshal(raw)
	if err != nil {
		return
	}
	if err := json.Unmarshal(b, &animations); err != nil {
		return
	}
	for _, anim := range animations {
		*out = append(*out, m2AnimMeta{Duration: anim.Duration})
	}
}

func loadJSONField[T any](data map[string]any, key string, out *[]T) {
	v, ok := data[key]
	if !ok || v == nil {
		return
	}
	if typed, ok := v.([]T); ok {
		*out = typed
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, out)
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
func (f *File) SubmeshAt(index int) *subMeshMeta {
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
			submeshMaterials[submeshID] = &components.Material{TwoSided: twoSided}
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
			layer := components.Layer{
				Texture: &textures[textureID], FilterMode: *filterMode, Alpha: alpha,
				Unlit: material.Flags&0x01 > 0, Unfogged: material.Flags&0x02 > 0,
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
