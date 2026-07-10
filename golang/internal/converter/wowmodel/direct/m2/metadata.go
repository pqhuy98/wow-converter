package directm2

import (
	"context"
	"encoding/json"

	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

// BuildMetadataObject builds the .json-equivalent metadata object.
func BuildMetadataObject(ctx context.Context, loader *m2.Loader, skin *m2.Skin, fileDataID int, fileName string, geosetMask []m2export.GeosetMaskEntry, validTextures map[any]m2export.TextureManifestEntry, dataTextures map[int]struct{}, getName func(context.Context, int) (string, error)) (map[string]any, error) {
	subMeshes := make([]map[string]any, len(skin.SubMeshes))
	for i, sm := range skin.SubMeshes {
		enabled := geosetMask == nil || (i < len(geosetMask) && geosetMask[i].Checked)
		subMeshes[i] = map[string]any{
			"enabled": enabled, "submeshID": sm.SubmeshID, "level": sm.Level,
			"vertexStart": sm.VertexStart, "vertexCount": sm.VertexCount,
			"triangleStart": sm.TriangleStart, "triangleCount": sm.TriangleCount,
			"boneCount": sm.BoneCount, "boneStart": sm.BoneStart,
			"boneInfluences": sm.BoneInfluences, "centerBoneIndex": sm.CenterBoneIndex,
			"centerPosition": sm.CenterPosition, "sortCenterPosition": sm.SortCenterPosition,
			"sortRadius": sm.SortRadius,
		}
	}

	textures := make([]map[string]any, len(loader.Textures))
	for i, tex := range loader.Textures {
		texType := loader.TextureTypes[i]
		var entry m2export.TextureManifestEntry
		if e, ok := validTextures[tex.FileDataID]; ok {
			entry = e
		} else if tex.FileDataID == 0 && tex.FileName != "" {
			entry, _ = validTextures[tex.FileName]
		} else if dataTextures != nil {
			if _, ok := dataTextures[int(texType)]; ok {
				entry, _ = validTextures["data-"+itoa(int(texType))]
			}
		}
		var internalName any
		if tex.FileDataID > 0 {
			if n, err := getName(ctx, int(tex.FileDataID)); err == nil {
				internalName = n
			}
		} else if tex.FileName != "" {
			internalName = tex.FileName
		}
		textures[i] = map[string]any{
			"fileNameInternal": internalName,
			"fileNameExternal": entry.MatPathRelative,
			"mtlName":          entry.MatName,
			"flags":            tex.Flags,
			"fileDataID":       tex.FileDataID,
		}
	}

	materials := make([]map[string]any, len(loader.Materials))
	for i, mat := range loader.Materials {
		materials[i] = map[string]any{"flags": mat.Flags, "blendingMode": mat.BlendingMode}
	}

	return map[string]any{
		"fileType": "m2", "fileDataID": fileDataID, "fileName": fileName,
		"internalName": loader.Name, "textures": textures, "textureTypes": loader.TextureTypes,
		"materials": materials, "textureCombos": loader.TextureCombos,
		"skeletonFileID": loader.SkeletonFileID, "boneFileIDs": loader.BoneFileIDs,
		"m2Animations": loader.Animations, "textureTransforms": loader.TextureTransforms,
		"textureTransformsLookup": loader.TextureTransformsLookup,
		"transparencyLookup":      loader.TransparencyLookup,
		"boundingBox":             loader.BoundingBox, "boundingSphereRadius": loader.BoundingSphereRadius,
		"collisionBox": loader.CollisionBox, "collisionSphereRadius": loader.CollisionSphereRadius,
		"lights": loader.Lights, "cameras": loader.Cameras, "cameraLookup": loader.CameraLookup,
		"ribbonEmitters": loader.RibbonEmitters, "particleEmitters": loader.ParticleEmitters,
		"colors": loaderColorsJSON(loader.Colors),
		"skin": map[string]any{
			"subMeshes": subMeshes, "textureUnits": skinTextureUnitsJSON(skin.TextureUnits),
			"fileName": skin.FileName, "fileDataID": skin.FileDataID,
		},
	}, nil
}

func skinTextureUnitsJSON(textureUnits []m2.SkinTextureUnit) []map[string]any {
	out := make([]map[string]any, len(textureUnits))
	for i, tu := range textureUnits {
		out[i] = map[string]any{
			"flags":                      tu.Flags,
			"priority":                   tu.Priority,
			"shaderID":                   tu.ShaderID,
			"skinSectionIndex":           tu.SkinSectionIndex,
			"flags2":                     tu.Flags2,
			"colorIndex":                 tu.ColorIndex,
			"materialIndex":              tu.MaterialIndex,
			"materialLayer":              tu.MaterialLayer,
			"textureCount":               tu.TextureCount,
			"textureComboIndex":          tu.TextureComboIndex,
			"textureCoordComboIndex":     tu.TextureCoordComboIndex,
			"textureWeightComboIndex":    tu.TextureWeightComboIndex,
			"textureTransformComboIndex": tu.TextureTransformComboIndex,
		}
	}
	return out
}

func loaderColorsJSON(colors []m2.ColorEntry) []map[string]any {
	if len(colors) == 0 {
		return nil
	}
	b, err := json.Marshal(colors)
	if err != nil {
		return nil
	}
	var out []map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}
