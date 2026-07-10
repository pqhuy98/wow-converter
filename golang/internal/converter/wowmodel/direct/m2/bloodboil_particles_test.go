package directm2

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

func TestBloodboilConvertHasParticleEmitters(t *testing.T) {
	if os.Getenv("WOW_INTEGRATION") == "" {
		t.Skip("set WOW_INTEGRATION=1 to run")
	}
	_ = workspace.LoadEnvFile(filepath.Join(workspace.FindRepoRoot(), ".env"))

	ctx := context.Background()
	handler, err := bootstrap.StartWowDataServer(ctx)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	src := cascSource{client: client.NewInProcessClient(handler)}
	cfg := config.DefaultConfig()
	cfg.IsBulkExport = true

	cases := []struct {
		name       string
		fileDataID int
	}{
		{"deathknight_bloodboil", 165893},
		{"deathknight_bloodboil_new", 467953},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exportPath := filepath.Join(cfg.ExportAssetDir, "spells", tc.name+".m2")
			result, err := ConvertM2ToMdl(ctx, cfg, src, ConvertOptions{
				FileDataID:         tc.fileDataID,
				ExportPathOverride: exportPath,
			})
			if err != nil {
				t.Fatalf("convert: %v", err)
			}
			t.Logf("particles=%d textures=%d geosets=%d",
				len(result.MDL.ParticleEmitter2s), len(result.MDL.Textures), len(result.MDL.Geosets))
			if len(result.MDL.ParticleEmitter2s) == 0 {
				t.Fatalf("expected particle emitters")
			}
		})
	}
}
