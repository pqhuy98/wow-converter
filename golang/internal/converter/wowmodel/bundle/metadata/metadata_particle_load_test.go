package metadata

import (
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromDataParticleEmittersTypedSlice(t *testing.T) {
	emitters := []m2.ParticleEmitterEntry{{ParticleID: 42, Bone: 1, TexturePacked: 2}}
	f := NewFile("test.json", config.Config{}, nil)
	f.LoadFromData(Data{FileType: "m2", ParticleEmitters: emitters})
	if len(f.particleEmitters) != 1 || f.particleEmitters[0].ParticleID != 42 {
		t.Fatalf("particles = %#v", f.particleEmitters)
	}
	emitters[0].ParticleID = 99
	if f.particleEmitters[0].ParticleID != 42 {
		t.Fatal("metadata retained caller's mutable emitter slice")
	}
}

func TestParseParticleEmitterCompanion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.json")
	if err := os.WriteFile(path, []byte(`{"fileType":"m2","materials":[{"flags":4,"blendingMode":2}],"particleEmitters":[{"particleId":7,"bone":2,"texturePacked":1}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	f := NewFile(path, config.Config{}, nil)
	if err := f.Parse(); err != nil {
		t.Fatal(err)
	}
	if len(f.particleEmitters) != 1 || f.particleEmitters[0].ParticleID != 7 || len(f.materials) != 1 || f.materials[0].BlendingMode != 2 {
		t.Fatalf("companion lost typed data: %#v, %#v", f.particleEmitters, f.materials)
	}
}
