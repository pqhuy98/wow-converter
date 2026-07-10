package directwmo

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/assemble"
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/mtl"
	objpkg "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/obj"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

// ConvertOptions configures WMO -> MDL conversion.
type ConvertOptions struct {
	FileDataID         int
	FileName           string
	Raw                []byte
	ExportPathOverride string
}

type textureMapEntry struct {
	matPathRelative string
	matPath         string
	matName         string
}

func virtualExportPath(exportRoot, file string) string {
	return filepath.Clean(filepath.Join(exportRoot, strings.ReplaceAll(file, " ", "")))
}

func wmoMaterialJSON(m wmo.Material) map[string]any {
	return map[string]any{
		"flags": m.Flags, "shader": m.Shader, "blendMode": m.BlendMode,
		"texture1": m.Texture1, "color1": m.Color1, "color1b": m.Color1b,
		"texture2": m.Texture2, "color2": m.Color2, "groupType": m.GroupType,
		"texture3": m.Texture3, "color3": m.Color3, "flags3": m.Flags3,
		"runtimeData": m.RuntimeData,
	}
}

func exportTextureSlots(material wmo.Material) []uint32 {
	slots := []uint32{material.Texture1, material.Texture2, material.Texture3}
	if material.Shader == 23 {
		slots = append(slots, material.Flags3, material.Color3,
			material.RuntimeData[0], material.RuntimeData[1], material.RuntimeData[2], material.RuntimeData[3])
	}
	return slots
}

func metaTextureSlots(material wmo.Material) []uint32 {
	slots := []uint32{material.Texture1, material.Texture2, material.Texture3}
	if material.Shader == 23 {
		slots = append(slots, material.Color3, material.Flags3,
			material.RuntimeData[0], material.RuntimeData[1], material.RuntimeData[2], material.RuntimeData[3])
	}
	return slots
}

func resolveWmoTextures(
	ctx context.Context,
	root *wmo.Loader,
	outDir, exportRoot string,
	getRaw func(context.Context, int) ([]byte, error),
	getName func(context.Context, int) (string, error),
) (map[int]textureMapEntry, map[int]string, []mtl.Material, error) {
	textureMap := map[int]textureMapEntry{}
	materialMap := map[int]string{}
	var mtlMaterials []mtl.Material

	isClassic := root.TextureNames != nil
	materials := root.Materials

	for i, material := range materials {
		dontUseFirstTexture := material.Shader == 23
		for _, materialTexture := range exportTextureSlots(material) {
			if materialTexture == 0 {
				continue
			}
			var fileDataID int
			var fileName string
			if isClassic {
				fileName = root.TextureNames[int(materialTexture)]
				if id, ok := archivecasc.GetByFilename(fileName); ok {
					fileDataID = id
				}
				fileName = strings.ReplaceAll(fileName, " ", "")
			} else {
				fileDataID = int(materialTexture)
			}
			if fileDataID == 0 {
				continue
			}
			texFile := fmt.Sprintf("%d.png", fileDataID)
			texPath := filepath.Join(outDir, texFile)
			matName := fmt.Sprintf("mat_%d", fileDataID)

			if fileName == "" {
				if n, err := getName(ctx, fileDataID); err == nil {
					fileName = n
				}
			}
			if fileName != "" {
				matName = "mat_" + strings.TrimSuffix(strings.ToLower(filepath.Base(fileName)), ".blp")
				matName = strings.ReplaceAll(matName, " ", "")
			}

			if fileName != "" {
				fileName = writers.ReplaceExtension(fileName, ".png")
			} else {
				fileName = filepath.Join("unknown", texFile)
			}
			texPath = virtualExportPath(exportRoot, fileName)
			texFile = relPath(outDir, texPath)

			if _, err := getRaw(ctx, fileDataID); err != nil {
				log.Printf("Failed to resolve texture %d for WMO: %v", fileDataID, err)
				continue
			}
			relTex := relPath(exportRoot, texPath)
			texturesource.Register(relTex, texturesource.Source{Kind: texturesource.KindBLP, FileDataID: fileDataID})

			mtlMaterials = append(mtlMaterials, mtl.Material{Name: matName, MapKd: texFile})
			textureMap[fileDataID] = textureMapEntry{matPathRelative: texFile, matPath: texPath, matName: matName}
			if _, ok := materialMap[i]; !ok && !dontUseFirstTexture {
				materialMap[i] = matName
			}
			dontUseFirstTexture = false
		}
	}
	return textureMap, materialMap, mtlMaterials, nil
}

func loadGroups(ctx context.Context, root *wmo.Loader, fileName string, getRaw func(context.Context, int) ([]byte, error)) ([]*wmo.Loader, error) {
	groups := make([]*wmo.Loader, root.GroupCount)
	for i := 0; i < int(root.GroupCount); i++ {
		var raw []byte
		var err error
		if root.GroupIDs != nil {
			raw, err = getRaw(ctx, int(root.GroupIDs[i]))
		} else {
			groupName := strings.Replace(fileName, ".wmo", fmt.Sprintf("_%03d.wmo", i), 1)
			id, ok := archivecasc.GetByFilename(groupName)
			if !ok || id == 0 {
				return nil, fmt.Errorf("unable to resolve WMO group file: %s", groupName)
			}
			raw, err = getRaw(ctx, id)
		}
		if err != nil {
			return nil, err
		}
		group := wmo.NewLoader(buffer.From(raw), 0, "", false)
		if err := group.Load(); err != nil {
			return nil, err
		}
		groups[i] = group
		root.Groups[i] = group
	}
	return groups, nil
}

func buildWmoObjResult(root *wmo.Loader, allGroups []*wmo.Loader, materialMap map[int]string, modelName, mtlLib string) objpkg.Result {
	type groupRef struct {
		group   *wmo.Loader
		indOfs  int
		indCount int
	}
	var groups []groupRef
	nInd := 0
	maxLayerCount := 0
	for _, group := range allGroups {
		if len(group.RenderBatches) == 0 {
			continue
		}
		indCount := len(group.Vertices) / 3
		nInd += indCount
		if len(group.UVs) > maxLayerCount {
			maxLayerCount = len(group.UVs)
		}
		groups = append(groups, groupRef{group: group, indCount: indCount})
	}

	vertsArray := make([]float64, nInd*3)
	normalsArray := make([]float64, nInd*3)
	uvArrays := make([][]float32, maxLayerCount)
	for i := range uvArrays {
		uvArrays[i] = make([]float32, nInd*2)
	}

	var meshes []directm2.ObjMesh
	indOfs := 0
	for _, ref := range groups {
		group := ref.group
		indCount := ref.indCount
		vertOfs := indOfs * 3
		for i, n := 0, len(group.Vertices); i < n; i++ {
			vertsArray[vertOfs+i] = float64(group.Vertices[i])
		}
		for i, n := 0, len(group.Normals); i < n; i++ {
			normalsArray[vertOfs+i] = float64(group.Normals[i])
		}
		uvsOfs := indOfs * 2
		uvCount := indCount * 2
		for layer := 0; layer < maxLayerCount; layer++ {
			var uv []float32
			if layer < len(group.UVs) {
				uv = group.UVs[layer]
			}
			for j := 0; j < uvCount; j++ {
				if uv != nil && j < len(uv) {
					uvArrays[layer][uvsOfs+j] = uv[j]
				}
			}
		}

		groupName := ""
		if root.GroupNames != nil && group.NameOfs != 0 {
			groupName = root.GroupNames[int(group.NameOfs)]
		}
		for bI, batch := range group.RenderBatches {
			indices := make([]int, batch.NumFaces)
			for i := 0; i < int(batch.NumFaces); i++ {
				indices[i] = int(group.Indices[batch.FirstFace+uint32(i)]) + indOfs
			}
			matID := int(batch.MaterialID)
			if batch.Flags&2 == 2 && len(batch.PossibleBox2) > 2 {
				matID = int(batch.PossibleBox2[2])
			}
			matName := materialMap[matID]
			meshes = append(meshes, directm2.ObjMesh{
				Name: groupName + fmt.Sprintf("%d", bI), Triangles: indices, MatName: matName,
			})
		}
		indOfs += indCount
	}

	if maxLayerCount > 2 {
		uvArrays = uvArrays[:2]
	}
	return directm2.BuildRawObjResult(vertsArray, normalsArray, uvArrays, meshes, modelName, mtlLib)
}

func buildWmoMetadataObject(
	ctx context.Context,
	root *wmo.Loader,
	fileDataID int,
	fileName string,
	textureMap map[int]textureMapEntry,
	getName func(context.Context, int) (string, error),
) map[string]any {
	textures := []any{}
	textureCache := map[int]struct{}{}
	for _, material := range root.Materials {
		for _, materialTexture := range metaTextureSlots(material) {
			texID := int(materialTexture)
			if materialTexture == 0 {
				continue
			}
			if _, ok := textureCache[texID]; ok {
				continue
			}
			textureCache[texID] = struct{}{}
			entry := textureMap[texID]
			internalName, _ := getName(ctx, texID)
			textures = append(textures, map[string]any{
				"fileDataID":       texID,
				"fileNameInternal": internalName,
				"fileNameExternal": entry.matPathRelative,
				"mtlName":          entry.matName,
			})
		}
	}
	materials := make([]any, len(root.Materials))
	for i, m := range root.Materials {
		materials[i] = wmoMaterialJSON(m)
	}
	return map[string]any{
		"fileType":   "wmo",
		"fileDataID": fileDataID,
		"fileName":   fileName,
		"textures":   textures,
		"materials":  materials,
	}
}

// ConvertWmoToMdl converts a WMO root file to MDL via the direct pipeline.
func ConvertWmoToMdl(ctx context.Context, cfg config.Config, src directm2.FileSource, opts ConvertOptions) (directm2.ConvertResult, error) {
	exportRoot := cfg.ExportAssetDir
	raw := opts.Raw
	var err error
	if raw == nil {
		raw, err = src.GetRawFile(ctx, opts.FileDataID)
		if err != nil {
			return directm2.ConvertResult{}, err
		}
	}
	listfileName := opts.FileName
	if listfileName == "" {
		listfileName, _ = src.GetFileName(ctx, opts.FileDataID)
	}
	fileName := listfileName
	if fileName == "" {
		fileName = fmt.Sprintf("unknown/%d.wmo", opts.FileDataID)
	}

	exportPath := opts.ExportPathOverride
	if exportPath == "" {
		exportPath = virtualExportPath(exportRoot, fileName)
	}
	outDir := filepath.Dir(exportPath)

	root := wmo.NewLoader(buffer.From(raw), opts.FileDataID, fileName, false)
	if err := root.Load(); err != nil {
		return directm2.ConvertResult{}, err
	}

	getRaw := func(c context.Context, id int) ([]byte, error) { return src.GetRawFile(c, id) }
	getName := func(c context.Context, id int) (string, error) { return src.GetFileName(c, id) }

	allGroups, err := loadGroups(ctx, root, fileName, getRaw)
	if err != nil {
		return directm2.ConvertResult{}, err
	}

	textureMap, materialMap, mtlMaterials, err := resolveWmoTextures(ctx, root, outDir, exportRoot, getRaw, getName)
	if err != nil {
		return directm2.ConvertResult{}, err
	}

	animFile := bundleanim.NewFile(writers.ReplaceExtension(exportPath, "_bones.json"), cfg)
	metaObj := buildWmoMetadataObject(ctx, root, opts.FileDataID, listfileName, textureMap, getName)
	if norm := directm2.NormalizeJSONValues(metaObj); norm != nil {
		if normMap, ok := norm.(map[string]any); ok {
			metaObj = normMap
		}
	}
	meta := bundlemeta.NewFile(writers.ReplaceExtension(exportPath, ".json"), cfg, animFile)
	meta.LoadFromData(metaObj)

	var mtlLib string
	if len(mtlMaterials) > 0 {
		mtlLib = filepath.Base(writers.ReplaceExtension(exportPath, ".mtl"))
	}
	modelName := strings.TrimSuffix(filepath.Base(exportPath), filepath.Ext(exportPath))
	objResult := buildWmoObjResult(root, allGroups, materialMap, modelName, mtlLib)

	assembled := assemble.AssembleWowModel(assemble.Inputs{
		ObjFilePath: exportPath,
		Obj:         objResult,
		Mtl:         struct{ Materials []mtl.Material }{Materials: mtlMaterials},
		Animation:   animFile,
		Metadata:    meta,
	}, cfg)

	return directm2.ConvertResult{MDL: assembled.MDL, TexturePaths: assembled.TexturePaths}, nil
}

func relPath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}
