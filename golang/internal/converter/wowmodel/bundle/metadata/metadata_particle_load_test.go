package metadata

import (
	"encoding/json"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestLoadFromDataParticleEmittersTypedSlice(t *testing.T) {
	emitters := []m2.ParticleEmitterEntry{{ParticleID: 42, Bone: 1, TexturePacked: 2}}
	f := NewFile("test.json", config.Config{}, nil)
	f.LoadFromData(map[string]any{
		"fileType":         "m2",
		"particleEmitters": emitters,
		"textures":         []any{},
	})
	if len(f.particleEmitters) != 1 {
		t.Fatalf("typed slice: got %d emitters", len(f.particleEmitters))
	}
	if f.particleEmitters[0].ParticleID != 42 {
		t.Fatalf("particleId=%d", f.particleEmitters[0].ParticleID)
	}
}

func TestLoadFromDataParticleEmittersJSONRoundtrip(t *testing.T) {
	src := []m2.ParticleEmitterEntry{{ParticleID: 7, Bone: 2, TexturePacked: 1}}
	b, err := json.Marshal(map[string]any{"particleEmitters": src})
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}
	f := NewFile("test.json", config.Config{}, nil)
	f.LoadFromData(map[string]any{"fileType": "m2", "particleEmitters": raw["particleEmitters"]})
	if len(f.particleEmitters) != 1 {
		t.Fatalf("json roundtrip: got %d emitters", len(f.particleEmitters))
	}
}
