//go:build integration_tests

package bootstrap_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

// Opt-in real Classic data check. Initial cache access without a source must
// not poison NPC selection after the source has loaded.
func TestClassicNPCSelectionAfterUnloadedCacheAccess(t *testing.T) {
	if os.Getenv("WOW_CLASSIC_SELECTION_VALIDATION") != "1" {
		t.Skip("set WOW_CLASSIC_SELECTION_VALIDATION=1 for real Classic validation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := workspace.LoadEnvFile(filepath.Join(workspace.FindRepoRoot(), ".env")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CASC_LOCAL_PRODUCT", "wow_classic")
	server.GlobalRuntime.SetCasc(nil)
	caches.ResetDBCaches()
	defer caches.ResetDBCaches()
	if err := caches.EnsureModelCachesInitialized(ctx); err == nil || !strings.Contains(err.Error(), "no CASC source") {
		t.Fatalf("unloaded lookup: %v", err)
	}
	handler, err := bootstrap.StartWowDataServer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := client.NewInProcessClient(handler)
	info, err := c.GetCASCInfo(ctx)
	if err != nil || info.Build.Product != "wow_classic" {
		t.Fatalf("expected Classic source, got %q: %v", info.Build.Product, err)
	}
	radius, err := c.ResolveNpcSelection(ctx, 24997, casc.NpcSelectionOptions{Level: 80, ObjectScale: 1})
	if err != nil || radius <= 0 {
		t.Fatalf("Volatile Ghoul display 24997 selection: %g, %v", radius, err)
	}
	t.Logf("Classic %s: Volatile Ghoul display 24997 selection radius %g", info.BuildName, radius)
}
