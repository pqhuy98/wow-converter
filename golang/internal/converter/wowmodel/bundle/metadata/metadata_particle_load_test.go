package metadata

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
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

func TestLoadFromDataParticleEmittersRealLoaderJSON(t *testing.T) {
	if os.Getenv("WOW_CASC_TEST") == "" {
		t.Skip("set WOW_CASC_TEST=1")
	}
	base := os.Getenv("WOW_DATA_SERVER_URL")
	if base == "" {
		base = "http://127.0.0.1:17753"
	}
	rawFile := downloadParticleTestCasc(t, base, 165893)
	loader := m2.NewLoader(buffer.NewBuffer(rawFile), nil)
	if err := loader.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	metaObj := map[string]any{
		"fileType":         "m2",
		"particleEmitters": loader.ParticleEmitters,
		"textures":         []any{},
	}
	b, err := json.Marshal(metaObj)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var normalized map[string]any
	if err := json.Unmarshal(b, &normalized); err != nil {
		t.Fatalf("normalize unmarshal: %v", err)
	}
	f := NewFile("test.json", config.Config{}, nil)
	f.LoadFromData(normalized)
	t.Logf("loader=%d loaded=%d", len(loader.ParticleEmitters), len(f.particleEmitters))
	if len(f.particleEmitters) != len(loader.ParticleEmitters) {
		t.Fatalf("lost particles during json roundtrip")
	}
}

func downloadParticleTestCasc(t *testing.T, base string, id uint32) []byte {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/rest/cascFile?fileDataID=%d", base, id))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
