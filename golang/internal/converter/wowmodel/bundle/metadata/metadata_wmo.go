package metadata

import (
	"path/filepath"
	"strings"

	bundleutils "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/utils"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

type wmoMaterialMeta struct {
	Flags       uint32   `json:"flags"`
	Shader      uint32   `json:"shader"`
	BlendMode   uint32   `json:"blendMode"`
	Texture1    uint32   `json:"texture1"`
	Color1      uint32   `json:"color1"`
	Color1b     uint32   `json:"color1b"`
	Texture2    uint32   `json:"texture2"`
	Color2      uint32   `json:"color2"`
	GroupType   uint32   `json:"groupType"`
	Texture3    uint32   `json:"texture3"`
	Color3      uint32   `json:"color3"`
	Flags3      uint32   `json:"flags3"`
	RuntimeData []uint32 `json:"runtimeData"`
}

// IsWmo reports whether metadata describes a WMO model.
func (f *File) IsWmo() bool {
	return f.IsLoaded && f.fileType == "wmo"
}

// GetWmoMaterialByMtlName resolves a pre-built WC3 material by OBJ/MTL name.
func (f *File) GetWmoMaterialByMtlName(name string) *components.Material {
	if f.wmoMaterialNameToMat == nil {
		return nil
	}
	return f.wmoMaterialNameToMat[name]
}

func (f *File) extractWmoTexturesMaterials() ExtractResult {
	parentDir := filepath.Dir(normalizePath(f.FilePath))
	textures := make([]components.Texture, len(f.textures))
	fileIDToTexture := map[int]*components.Texture{}

	for i, tex := range f.textures {
		pngPath := ""
		if tex.FileNameExternal != "" {
			pngPath = relExport(f.Config.ExportAssetDir, filepath.Join(parentDir, tex.FileNameExternal))
		}
		image := ""
		if pngPath != "" {
			image = filepath.ToSlash(filepath.Join(f.Config.AssetPrefix, strings.ReplaceAll(pngPath, ".png", ".blp")))
		}
		textures[i] = components.Texture{
			Image: image, WrapWidth: true, WrapHeight: true,
			WowData: components.TextureWowData{Type: 0, PngPath: pngPath},
		}
		if tex.FileDataID > 0 {
			fileIDToTexture[tex.FileDataID] = &textures[i]
		}
	}

	f.wmoMaterialNameToMat = map[string]*components.Material{}
	for _, mat := range f.wmoMaterials {
		twoSided := mat.Flags&0x4 > 0
		unlit := mat.Flags&0x1 > 0
		unfogged := mat.Flags&0x02 > 0
		filterMode := bundleutils.WmoBlendModeToWc3FilterMode(uint16(mat.BlendMode))

		createMatForFID := func(fid uint32) {
			if fid == 0 {
				return
			}
			tex := fileIDToTexture[int(fid)]
			if tex == nil {
				return
			}
			var mtlName string
			for _, texMeta := range f.textures {
				if texMeta.FileDataID == int(fid) {
					mtlName = texMeta.MtlName
					break
				}
			}
			if mtlName == "" || f.wmoMaterialNameToMat[mtlName] != nil {
				return
			}
			f.wmoMaterialNameToMat[mtlName] = &components.Material{
				ConstantColor: false,
				TwoSided:      twoSided,
				Layers: []components.Layer{{
					Texture:     tex,
					FilterMode:  filterMode,
					Alpha:       components.AnimatedOrStatic[float64]{Static: true, Value: 1},
					Unshaded:    false,
					SphereEnvMap: false,
					TwoSided:    twoSided,
					Unfogged:    unfogged,
					Unlit:       unlit,
					NoDepthTest: false,
					NoDepthSet:  mat.BlendMode > 1,
				}},
			}
		}

		createMatForFID(mat.Texture1)
		createMatForFID(mat.Texture2)
		createMatForFID(mat.Texture3)
		createMatForFID(mat.Color2)
		createMatForFID(mat.Flags3)
		for _, fid := range mat.RuntimeData {
			createMatForFID(fid)
		}
	}

	return ExtractResult{Textures: textures, SubmeshIDToMat: map[int]*components.Material{}}
}
