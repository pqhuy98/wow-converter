//go:build integration_tests

package metadata

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestLoadFromDataParticleEmittersRealLoaderJSON(t *testing.T) {
	ctx := context.Background()
	rawFile := downloadParticleTestCasc(t, ctx, 165893)
	loader := m2.NewLoader(buffer.NewBuffer(rawFile), nil)
	if err := loader.Load(ctx); err != nil {
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

func downloadParticleTestCasc(t *testing.T, ctx context.Context, id uint32) []byte {
	t.Helper()
	data, err := client.NewHTTPClient("").DownloadCascFile(ctx, int(id))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
