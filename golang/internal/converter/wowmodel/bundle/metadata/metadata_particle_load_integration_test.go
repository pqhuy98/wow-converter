//go:build integration_tests

package metadata

import (
	"context"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/m2"
)

func TestLoadFromDataParticleEmittersRealLoader(t *testing.T) {
	ctx := context.Background()
	rawFile := downloadParticleTestCasc(t, ctx, 165893)
	loader := m2.NewLoader(buffer.NewBuffer(rawFile), nil)
	if err := loader.Load(ctx); err != nil {
		t.Fatal(err)
	}
	f := NewFile("test.json", config.Config{}, nil)
	f.LoadFromData(Data{FileType: "m2", ParticleEmitters: loader.ParticleEmitters})
	t.Logf("loader=%d loaded=%d", len(loader.ParticleEmitters), len(f.particleEmitters))
	if len(f.particleEmitters) != len(loader.ParticleEmitters) {
		t.Fatalf("lost particles from typed loader")
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
