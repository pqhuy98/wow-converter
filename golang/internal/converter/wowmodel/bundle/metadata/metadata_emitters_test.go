package metadata

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestExtractMDLParticlesEmittersBasic(t *testing.T) {
	f := &File{
		IsLoaded: true,
		particleEmitters: []m2.ParticleEmitterEntry{{
			ParticleID: 1, Bone: 0, TexturePacked: 0, TextureRows: 1, TextureCols: 1,
		}},
	}
	model := mdl.New(mdl.NewMDLOptions{Name: "test"})
	model.Bones = []*components.Bone{components.NewBone("root")}
	f.BindMdl(model)
	textures := []components.Texture{{Image: "wow/spells/tex0.blp", WowData: components.TextureWowData{PngPath: "spells/tex0.png"}}}
	f.ExtractMDLParticlesEmitters(textures)
	if len(model.ParticleEmitter2s) != 1 {
		t.Fatalf("expected 1 particle emitter, got %d", len(model.ParticleEmitter2s))
	}
}

	func TestExtractMDLLightsTypeMapping(t *testing.T) {
	tests := []struct {
		name string
		raw  uint16
		want components.LightType
	}{
		{name: "raw 0 becomes directional", raw: 0, want: components.LightDirectional},
		{name: "raw 1 becomes omnidirectional", raw: 1, want: components.LightOmnidirectional},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{
				IsLoaded: true,
				lights: []m2.LightEntry{
					{Type: tt.raw},
				},
			}

			model := mdl.New(mdl.NewMDLOptions{Name: "test"})
			model.Bones = []*components.Bone{components.NewBone("root")}
			f.BindMdl(model)

			f.ExtractMDLLights()

			if len(model.Lights) != 1 {
				t.Fatalf("expected 1 light, got %d", len(model.Lights))
			}
			if got := model.Lights[0].LightType; got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}
