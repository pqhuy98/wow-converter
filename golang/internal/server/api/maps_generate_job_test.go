package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/server/util"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

func TestFormatMapTileFailuresMatchesAPIContract(t *testing.T) {
	failures := []mapExportTileFailure{
		{TileX: 21, TileY: 27, Error: "missing ADT"},
		{TileX: 22, TileY: 27, Error: "decode failed"},
	}

	message := formatMapTileFailures(failures)
	if !strings.Contains(message, "2 tile exports failed") ||
		!strings.Contains(message, "21,27: missing ADT") ||
		!strings.Contains(message, "22,27: decode failed") {
		t.Fatalf("unexpected failure message: %q", message)
	}
}

type mapGenerateTestClient struct {
	client.Client
	mu           sync.Mutex
	readyErr     error
	failTileX    int
	exportParams []casc.ADTExportParams
}

func (c *mapGenerateTestClient) WaitUntilReady(context.Context) error {
	return c.readyErr
}

func (c *mapGenerateTestClient) ExportADTForConversion(
	_ context.Context,
	params casc.ADTExportParams,
) (*exportadt.ConversionOutput, error) {
	c.mu.Lock()
	c.exportParams = append(c.exportParams, params)
	c.mu.Unlock()
	if params.TileX == c.failTileX {
		return nil, errors.New("missing ADT")
	}
	return &exportadt.ConversionOutput{
		ObjectPath: fmt.Sprintf("maps/%s/%s_%d_%d.obj", params.MapDir, params.MapDir, params.TileX, params.TileY),
	}, nil
}

func (c *mapGenerateTestClient) exports() []casc.ADTExportParams {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]casc.ADTExportParams(nil), c.exportParams...)
}

func TestMapGenerateFailsBeforeTileLoadWhenDataIsNotReady(t *testing.T) {
	testClient := &mapGenerateTestClient{readyErr: errors.New("CASC loading")}
	job := testMapGenerateJob("Stormwind City")

	_, err := runMapGenerateJob(context.Background(), &Deps{
		Client: testClient, exportAssetDir: t.TempDir(),
	}, job)
	if err == nil || !strings.Contains(err.Error(), "WoW data not loaded") {
		t.Fatalf("expected readiness failure, got %v", err)
	}
	if exports := testClient.exports(); len(exports) != 0 {
		t.Fatalf("tile loader ran before readiness: %v", exports)
	}
}

func TestMapGenerateFailedJobPreservesTileSummary(t *testing.T) {
	testClient := &mapGenerateTestClient{failTileX: 22}
	deps := &Deps{Client: testClient, exportAssetDir: t.TempDir()}
	queue := util.NewJobQueue(mapGenerateQueueConfig(), func(
		job *util.Job[mapGenerateJobRequest, mapGenerateJobResult],
	) (mapGenerateJobResult, error) {
		return runMapGenerateJob(context.Background(), deps, job)
	})
	job := testMapGenerateJob("Stormwind City")
	job.NoTimeout = true
	queue.AddJob(job)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status := queue.GetJobStatus(job.ID)
		if status != nil && status.Status == util.JobFailed {
			if status.Result == nil {
				t.Fatal("failed map job did not preserve its result")
			}
			if len(status.Result.Succeeded) != 1 || len(status.Result.Failed) != 1 {
				t.Fatalf("unexpected tile summary: %#v", status.Result)
			}
			if status.Result.Succeeded[0].TileX != 21 || status.Result.Failed[0].TileX != 22 {
				t.Fatalf("unexpected successful/failed tiles: %#v", status.Result)
			}
			for _, params := range testClient.exports() {
				if params.MapDir != "Stormwind City" {
					t.Fatalf("CASC map dir was modified: %q", params.MapDir)
				}
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("map job did not fail before deadline")
}

func testMapGenerateJob(mapDir string) *util.Job[mapGenerateJobRequest, mapGenerateJobResult] {
	body := generateWc3Body{
		Tiles:                    []exportAdtTile{{X: 21, Y: 27}, {X: 22, Y: 27}},
		Quality:                  4096,
		MapSaveName:              "parity.w3x",
		ClampLower:               0,
		ClampUpper:               1,
		AutoClampPercent:         true,
		UnitScale:                2,
		IncludeBuildingInteriors: boolPtr(true),
		FreshExport:              true,
	}
	ordered := []exportAdtTile{{X: 21, Y: 27}, {X: 22, Y: 27}}
	return &util.Job[mapGenerateJobRequest, mapGenerateJobResult]{
		ID: "map-test",
		Request: mapGenerateJobRequest{
			MapDir:       mapDir,
			MapID:        571,
			Body:         body,
			OrderedTiles: ordered,
			TileBounds:   tileBoundsFromTiles(ordered),
		},
		Status:      util.JobPending,
		SubmittedAt: time.Now().UnixMilli(),
	}
}
