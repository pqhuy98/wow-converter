//go:build integration_tests

package wmoexport_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/workspace"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	exportpkg "github.com/pqhuy98/wow-converter/internal/wow/export"
	wmoexport "github.com/pqhuy98/wow-converter/internal/wow/export/wmo"
)

// Set WOW_WMO_CHILD_VALIDATION to a WMO file data ID or listfile search pattern.
// Reads real loaded source data without writing assets or changing products.
func TestLoadedWMOChildClassification(t *testing.T) {
	query := os.Getenv("WOW_WMO_CHILD_VALIDATION")
	if query == "" {
		t.Skip("set WOW_WMO_CHILD_VALIDATION for loaded source validation")
	}
	pattern, err := regexp.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := workspace.LoadEnvFile(filepath.Join(workspace.FindRepoRoot(), ".env")); err != nil {
		t.Fatal(err)
	}
	handler, err := bootstrap.StartWowDataServer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	c := client.NewInProcessClient(handler)
	entries, err := c.SearchFiles(ctx, query, true)
	if err != nil {
		t.Fatal(err)
	}
	id, idErr := strconv.Atoi(query)
	if idErr == nil {
		entry, err := c.GetFileByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries[:0], entry)
	}
	if len(entries) == 0 {
		t.Fatal("no matching source WMO")
	}
	removedTotal, keptTotal := 0, 0
	groupSuffix := regexp.MustCompile(`_\d{3}(?:_lod\d)?\.wmo$`)
	for _, entry := range entries {
		name := strings.ToLower(entry.FileName)
		if !strings.HasSuffix(name, ".wmo") || groupSuffix.MatchString(name) || (idErr != nil && !pattern.MatchString(name)) {
			continue
		}
		raw, err := c.DownloadCascFile(ctx, entry.FileDataID)
		if err != nil {
			t.Fatal(err)
		}
		e := wmoexport.NewExporter(raw, entry.FileDataID)
		all, err := e.CollectDoodadPlacements("building.obj", exportpkg.ADTExportOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := e.ExcludeInteriorDoodads(ctx, func(ctx context.Context, id uint32) ([]byte, error) {
			return c.DownloadCascFile(ctx, int(id))
		}); err != nil {
			t.Fatal(err)
		}
		kept, err := e.CollectDoodadPlacements("building.obj", exportpkg.ADTExportOptions{})
		if err != nil || len(kept) > len(all) {
			t.Fatalf("filtered %d / %d: %v", len(kept), len(all), err)
		}
		removedTotal += len(all) - len(kept)
		keptTotal += len(kept)
		t.Logf("%s (%d): kept %d exterior/shared/unknown, omitted %d interior-only, total %d", entry.FileName, entry.FileDataID, len(kept), len(all)-len(kept), len(all))
	}
	if removedTotal == 0 || keptTotal == 0 {
		t.Fatalf("validation needs both kept and interior-only children: kept %d removed %d", keptTotal, removedTotal)
	}
}
