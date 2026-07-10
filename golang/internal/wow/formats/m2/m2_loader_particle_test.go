package m2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/buffer"
)

func TestBloodboilParticleEmitters(t *testing.T) {
	if os.Getenv("WOW_CASC_TEST") == "" {
		t.Skip("set WOW_CASC_TEST=1 to run CASC-backed loader tests")
	}
	base := os.Getenv("WOW_DATA_SERVER_URL")
	if base == "" {
		base = "http://127.0.0.1:17753"
	}
	cases := map[string]uint32{
		"spells/deathknight_bloodboil":     165893,
		"spells/deathknight_bloodboil_new": 467953,
	}
	ctx := context.Background()
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			raw := downloadCascFile(t, base, id)
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

func downloadCascFile(t *testing.T, base string, fileDataID uint32) []byte {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/rest/cascFile?fileDataID=%d", base, fileDataID))
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	return data
}
