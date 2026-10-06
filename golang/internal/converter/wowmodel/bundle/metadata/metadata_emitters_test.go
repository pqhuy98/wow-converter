package metadata

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestExtractMDLParticlesEmittersBasic(t *testing.T) {
	f := &File{
		IsLoaded: true,
		particleEmitters: []m2.ParticleEmitterEntry{{
			ParticleID: 1, Bone: 0, TexturePacked: 0, TextureRows: 1, TextureCols: 1,
			Position: [3]float32{1, 3, -2},
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
	if got := model.ParticleEmitter2s[0].PivotPoint; got != (imath.Vector3{1, 2, 3}) {
		t.Fatalf("particle pivot %v, want model-space bind point (1,2,3)", got)
	}
}

func TestExtractMDLParticlesEmittersRefractionRouting(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags uint32
		want  int
	}{
		{"ordinary gas", 0x20001, 1},
		{"scene refraction", 0x00120001, 0},
		{"multitexture takes precedence", 0x10120001, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &File{IsLoaded: true, particleEmitters: []m2.ParticleEmitterEntry{{Flags: tc.flags}}}
			model := mdl.New(mdl.NewMDLOptions{Name: "particle shader routing"})
			model.Bones = []*components.Bone{components.NewBone("root")}
			f.BindMdl(model)
			f.ExtractMDLParticlesEmitters([]components.Texture{{Image: "test.blp"}})
			if got := len(model.ParticleEmitter2s); got != tc.want {
				t.Fatalf("got %d visible emitters, want %d", got, tc.want)
			}
		})
	}
}

func TestExtractMDLParticlesEmittersModelSpacePivots(t *testing.T) {
	for _, tc := range []struct {
		name         string
		parent, want imath.Vector3
		loaded       [3]float32
	}{
		{"nonzero offset", imath.Vector3{10, 20, 30}, imath.Vector3{11, 22, 33}, [3]float32{11, 33, -22}},
		{"model origin", imath.Vector3{10, 20, 30}, imath.Vector3{}, [3]float32{}},
		{"mirrored point", imath.Vector3{1, 2, 3}, imath.Vector3{-1, 2, 3}, [3]float32{-1, 3, -2}},
		{"point on parent", imath.Vector3{1, 2, 3}, imath.Vector3{1, 2, 3}, [3]float32{1, 3, -2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &File{IsLoaded: true, particleEmitters: []m2.ParticleEmitterEntry{{Position: tc.loaded, Bone: 0}}}
			model := mdl.New(mdl.NewMDLOptions{Name: "particle bind point"})
			parent := components.NewBone("parent")
			parent.PivotPoint = tc.parent
			model.Bones = []*components.Bone{parent}
			f.BindMdl(model)
			f.ExtractMDLParticlesEmitters([]components.Texture{{Image: "test.blp"}})
			if len(model.ParticleEmitter2s) != 1 {
				t.Fatal("expected one emitter")
			}
			emitter := model.ParticleEmitter2s[0]
			if emitter.PivotPoint != tc.want || emitter.Parent != parent {
				t.Fatalf("pivot %v, parent %p; want pivot %v, parent %p", emitter.PivotPoint, emitter.Parent, tc.want, parent)
			}
		})
	}
}

func TestExtractMDLRibbonsPreservesBindPointsAndBoneFrame(t *testing.T) {
	for _, tc := range []struct {
		name         string
		parent, want imath.Vector3
		position     [3]float32
	}{
		{"point on wing bone", imath.Vector3{1, 2, 3}, imath.Vector3{1, 2, 3}, [3]float32{1, 2, 3}},
		{"offset from bone", imath.Vector3{10, 20, 30}, imath.Vector3{11, 22, 33}, [3]float32{11, 22, 33}},
		{"mirrored point", imath.Vector3{1, 2, 3}, imath.Vector3{-1, 2, 3}, [3]float32{-1, 2, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := &File{IsLoaded: true, materials: []materialMeta{{BlendingMode: 2}}, ribbonEmitters: []m2.RibbonEmitterEntry{{BoneIndex: 0, Position: tc.position, TextureIndices: []uint16{0}, MaterialIndices: []uint16{0}}}}
			m := mdl.New(mdl.NewMDLOptions{Name: "ribbon bind point"})
			parent := components.NewBone("wing")
			parent.PivotPoint = tc.parent
			parent.Rotation = &components.Animation{Type: components.AnimTypeRotation, KeyFrames: map[int]any{0: imath.QuaternionRotation{0.5, 0.5, 0.5, 0.5}}}
			m.Bones = []*components.Bone{parent}
			f.BindMdl(m)
			f.ExtractMDLRibbonEmitters([]components.Texture{{Image: "ribbon.blp"}})
			if len(m.RibbonEmitters) != 1 {
				t.Fatal("ribbon missing")
			}
			ribbon := m.RibbonEmitters[0]
			if ribbon.Parent != parent || ribbon.PivotPoint != tc.want {
				t.Fatalf("ribbon pivot %v, parent %p; want pivot %v, parent %p", ribbon.PivotPoint, ribbon.Parent, tc.want, parent)
			}
			if ribbon.Rotation != nil {
				t.Fatal("ribbon should inherit the bone frame without an extra basis rotation")
			}
		})
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
