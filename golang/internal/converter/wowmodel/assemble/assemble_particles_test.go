package assemble

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	bundleanim "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/animation"
	bundlemeta "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/metadata"
	"github.com/pqhuy98/wow-converter/internal/converter/wowmodel/bundle/obj"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestAssembleParticleOnlyM2WithoutFaces(t *testing.T) {
	meta := bundlemeta.NewFile("spells/test.json", config.Config{ExportAssetDir: "export", AssetPrefix: "wow"}, bundleanim.NewFile("bones.json", config.Config{}))
	meta.LoadFromData(map[string]any{
		"fileType": "m2",
		"textures": []any{
			map[string]any{"fileNameExternal": "spells/tex0.png", "flags": 0, "fileDataID": 1},
		},
		"textureTypes": []any{0},
		"skin": map[string]any{
			"subMeshes":    []any{},
			"textureUnits": []any{},
		},
		"particleEmitters": []m2.ParticleEmitterEntry{{
			ParticleID: 1, Bone: 0, TexturePacked: 0, TextureRows: 1, TextureCols: 1,
		}},
	})
	result := AssembleWowModel(Inputs{
		ObjFilePath: "export/spells/test.m2",
		Obj: obj.Result{Models: []obj.Model{{
			Name: "test",
		}}},
		Metadata: meta,
	}, config.Config{ExportAssetDir: "export", AssetPrefix: "wow", RawModelScaleUp: 1})
	if len(result.MDL.ParticleEmitter2s) != 1 {
		t.Fatalf("particles=%d textures=%d", len(result.MDL.ParticleEmitter2s), len(result.MDL.Textures))
	}
}
