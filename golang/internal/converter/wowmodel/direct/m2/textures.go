package directm2

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// DirectDataTexture is a composited data texture from the character pipeline.
type DirectDataTexture struct {
	Filename *string
	Source   texturesource.Source
	Order    int
}

// ResolvedTextures holds texture resolution output.
type ResolvedTextures struct {
	ValidTextures textureManifest
	MtlMaterials  []mtlMaterial
}

type mtlMaterial struct {
	Name  string
	MapKd string
}

type textureKey struct {
	fileDataID uint32
	fileName   string
	dataType   int
	data       bool
}

type textureManifest map[textureKey]m2export.TextureManifestEntry

func virtualExportPath(exportRoot, file string) string {
	return filepath.Clean(filepath.Join(exportRoot, strings.ReplaceAll(file, " ", "")))
}

// ResolveTextures resolves M2 textures and registers texture sources.
func ResolveTextures(ctx context.Context, loader *m2.Loader, variantTextures []int, replaceableTextures map[int]int, dataTextures map[int]DirectDataTexture, outDir, exportRoot string, getRaw func(context.Context, int) ([]byte, error), getName func(context.Context, int) (string, error)) (ResolvedTextures, error) {
	valid := textureManifest{}
	var mtlMaterials []mtlMaterial
	addMaterial := func(name, file string) {
		mtlMaterials = append(mtlMaterials, mtlMaterial{Name: name, MapKd: file})
	}
	register := func(texPath string, source texturesource.Source) {
		rel, _ := filepath.Rel(exportRoot, texPath)
		texturesource.Register(rel, source)
	}
	registerTextureFile := func(texFileDataID uint32) (string, string, string, bool) {
		if texFileDataID == 0 {
			return "", "", "", false
		}
		if _, err := getRaw(ctx, int(texFileDataID)); err != nil {
			log.Printf("Failed to resolve texture %d for M2: %v", texFileDataID, err)
			return "", "", "", false
		}
		fileName, _ := getName(ctx, int(texFileDataID))
		matName := fmt.Sprintf("mat_%d", texFileDataID)
		if fileName != "" {
			matName = "mat_" + strings.TrimSuffix(strings.ToLower(filepath.Base(fileName)), ".blp")
			matName = strings.ReplaceAll(matName, " ", "")
			fileName = writers.ReplaceExtension(fileName, ".png")
		} else {
			fileName = fmt.Sprintf("unknown/%d.png", texFileDataID)
		}
		texPath := virtualExportPath(exportRoot, fileName)
		texFile := relPath(outDir, texPath)
		register(texPath, texturesource.Source{Kind: texturesource.KindBLP, FileDataID: int(texFileDataID)})
		return matName, texFile, texPath, true
	}

	dataTextureTypes := make([]int, 0, len(dataTextures))
	for texType := range dataTextures {
		dataTextureTypes = append(dataTextureTypes, texType)
	}
	sort.Slice(dataTextureTypes, func(i, j int) bool {
		left := dataTextures[dataTextureTypes[i]]
		right := dataTextures[dataTextureTypes[j]]
		if left.Order != right.Order {
			return left.Order < right.Order
		}
		return dataTextureTypes[i] < dataTextureTypes[j]
	})
	for _, texType := range dataTextureTypes {
		dataTexture := dataTextures[texType]
		var texPath, texFile, matName string
		if dataTexture.Filename != nil {
			fileNamePNG := writers.ReplaceExtension(*dataTexture.Filename, ".png")
			texPath = virtualExportPath(exportRoot, fileNamePNG)
			texFile = relPath(outDir, texPath)
			matName = "mat_" + strings.TrimSuffix(filepath.Base(fileNamePNG), ".png")
		} else {
			texFile = fmt.Sprintf("data-%d.png", texType)
			texPath = filepath.Join(outDir, texFile)
			matName = fmt.Sprintf("mat_%d", texType)
		}
		register(texPath, dataTexture.Source)
		addMaterial(matName, texFile)
		valid[textureKey{data: true, dataType: texType}] = m2export.TextureManifestEntry{
			MatName: matName, MatPathRelative: texFile, MatPath: texPath,
		}
	}

	for textureIndex, texture := range loader.Textures {
		texType := 0
		if textureIndex < len(loader.TextureTypes) {
			texType = int(loader.TextureTypes[textureIndex])
		}
		texFileDataID := texture.FileDataID

		if _, isData := dataTextures[texType]; isData {
			continue
		}

		if target := replaceableTextures[texType]; target > 0 {
			texFileDataID = uint32(target)
			loader.Textures[textureIndex].FileDataID = uint32(target)
		} else if texType > 0 {
			var target int
			if texType >= 11 && texType < 14 && texType-11 < len(variantTextures) {
				target = variantTextures[texType-11]
			} else if texType == 5 && len(variantTextures) > 3 {
				// CreatureDisplayInfo's fourth TextureVariation is M2 type 5.
				// It is a supplied skin texture, not a stock environment map.
				target = variantTextures[3]
			} else if texType > 1 && texType < 5 && texType-2 < len(variantTextures) {
				target = variantTextures[texType-2]
			}
			if target > 0 {
				texFileDataID = uint32(target)
				texture.FileDataID = uint32(target)
				loader.Textures[textureIndex].FileDataID = uint32(target)
			}
		}

		if texFileDataID > 0 {
			matName, texFile, texPath, ok := registerTextureFile(texFileDataID)
			if !ok {
				textureIndex++
				continue
			}
			addMaterial(matName, texFile)
			valid[textureKey{fileDataID: texFileDataID}] = m2export.TextureManifestEntry{
				MatName: matName, MatPathRelative: texFile, MatPath: texPath,
			}
		} else if texture.FileName != "" {
			fileName := writers.ReplaceExtension(texture.FileName, ".png")
			texPath := virtualExportPath(exportRoot, fileName)
			texFile := relPath(outDir, texPath)
			matName := "mat_" + strings.TrimSuffix(strings.ToLower(filepath.Base(fileName)), ".png")
			matName = strings.ReplaceAll(matName, " ", "")
			addMaterial(matName, texFile)
			valid[textureKey{fileName: texture.FileName}] = m2export.TextureManifestEntry{
				MatName: matName, MatPathRelative: texFile, MatPath: texPath,
			}
		}
	}

	return ResolvedTextures{ValidTextures: valid, MtlMaterials: mtlMaterials}, nil
}

func relPath(outDir, absPath string) string {
	rel, err := filepath.Rel(outDir, absPath)
	if err != nil {
		return filepath.Base(absPath)
	}
	return rel
}
