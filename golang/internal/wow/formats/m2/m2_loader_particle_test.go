//go:build integration_tests

package m2

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func TestBloodboilParticleEmitters(t *testing.T) {
	cases := map[string]uint32{
		"spells/deathknight_bloodboil":     165893,
		"spells/deathknight_bloodboil_new": 467953,
	}
	ctx := context.Background()
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			raw := downloadCascFile(t, ctx, id)
			loader := NewLoader(buffer.NewBuffer(raw), nil)
			if err := loader.Load(ctx); err != nil {
				t.Fatalf("Load: %v", err)
			}
			t.Logf("version=%d particles=%d ribbons=%d textures=%d bones=%d",
				loader.Version, len(loader.ParticleEmitters), len(loader.RibbonEmitters),
				len(loader.Textures), len(loader.Bones))
			if len(loader.ParticleEmitters) == 0 {
				t.Fatalf("expected particle emitters")
			}
			b, err := json.Marshal(loader.ParticleEmitters[0])
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var round map[string]any
			if err := json.Unmarshal(b, &round); err != nil {
				t.Fatalf("unmarshal round: %v", err)
			}
			if round["particleId"] == nil {
				t.Fatalf("particle json keys missing particleId: %v", round)
			}
		})
	}
}

func downloadCascFile(t *testing.T, ctx context.Context, fileDataID uint32) []byte {
	t.Helper()
	data, err := client.NewHTTPClient("").DownloadCascFile(ctx, int(fileDataID))
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	return data
}
