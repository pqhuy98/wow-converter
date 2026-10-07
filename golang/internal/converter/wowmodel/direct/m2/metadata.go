package directm2

import (
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	m2export "github.com/pqhuy98/wow-converter/internal/wow/export/m2"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func buildMetadata(loader *m2.Loader, skin *m2.Skin, geosetMask []m2export.GeosetMaskEntry, validTextures textureManifest, dataTextures map[int]struct{}) bundlemeta.Data {
	subMeshes := make([]bundlemeta.SubMesh, len(skin.SubMeshes))
	for i, sm := range skin.SubMeshes {
		subMeshes[i] = bundlemeta.SubMesh{
			Enabled:   geosetMask == nil || (i < len(geosetMask) && geosetMask[i].Checked),
			SubmeshID: int(sm.SubmeshID), VertexStart: int(sm.VertexStart), VertexCount: int(sm.VertexCount),
		}
	}
	textures := make([]bundlemeta.Texture, len(loader.Textures))
	for i, tex := range loader.Textures {
		var texType int
		if i < len(loader.TextureTypes) {
			texType = int(loader.TextureTypes[i])
		}
		entry, found := validTextures[textureKey{fileDataID: tex.FileDataID}]
		if !found && tex.FileDataID == 0 && tex.FileName != "" {
			entry = validTextures[textureKey{fileName: tex.FileName}]
		} else if !found {
			if _, ok := dataTextures[texType]; ok {
				entry = validTextures[textureKey{data: true, dataType: texType}]
			}
		}
		textures[i] = bundlemeta.Texture{FileNameExternal: entry.MatPathRelative, MtlName: entry.MatName, Flags: int(tex.Flags), FileDataID: int(tex.FileDataID)}
	}
	units := make([]bundlemeta.TextureUnit, len(skin.TextureUnits))
	for i, tu := range skin.TextureUnits {
		units[i] = bundlemeta.TextureUnit{
			Flags: int(tu.Flags), Priority: int(tu.Priority), ShaderID: int(tu.ShaderID),
			SkinSectionIndex: int(tu.SkinSectionIndex), MaterialIndex: int(tu.MaterialIndex),
			TextureCount: int(tu.TextureCount), TextureComboIndex: int(tu.TextureComboIndex),
			TextureTransformComboIndex: int(tu.TextureTransformComboIndex),
			TextureWeightComboIndex:    int(tu.TextureWeightComboIndex), ColorIndex: int(tu.ColorIndex),
		}
	}
	transforms := make([]bundlemeta.TextureTransform, len(loader.TextureTransforms))
	for i, t := range loader.TextureTransforms {
		transforms[i] = bundlemeta.TextureTransform{
			Translation: bundlemeta.Track(convertTrack(t.Translation)),
			Rotation:    bundlemeta.Track(convertTrack(t.Rotation)), Scaling: bundlemeta.Track(convertTrack(t.Scaling)),
		}
	}
	colors := make([]bundlemeta.Color, len(loader.Colors))
	for i, c := range loader.Colors {
		colors[i] = bundlemeta.Color{Color: bundlemeta.Track(convertTrack(c.Color)), Alpha: bundlemeta.Track(convertTrack(c.Alpha))}
	}
	return bundlemeta.Data{
		FileType: "m2", Textures: textures, TextureTypes: loader.TextureTypes, Materials: loader.Materials, TextureCombos: loader.TextureCombos,
		M2Animations: loader.Animations, TextureTransforms: transforms, TextureTransformsLookup: loader.TextureTransformsLookup,
		TransparencyLookup: loader.TransparencyLookup, TextureWeights: loader.TextureWeights, GlobalLoops: loader.GlobalLoops,
		Lights: loader.Lights, Cameras: loader.Cameras, RibbonEmitters: loader.RibbonEmitters, ParticleEmitters: loader.ParticleEmitters,
		Colors: colors, Skin: bundlemeta.Skin{SubMeshes: subMeshes, TextureUnits: units},
	}
}
