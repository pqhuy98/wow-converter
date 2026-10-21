package mapexporter

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/server/util"
	"github.com/pqhuy98/wow-converter/internal/workspace"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

func TestTerrainDimensionsClampLimit(t *testing.T) {
	min, max := imath.Vector3{}, imath.Vector3{122881, 100, 10000}
	minimum := minimumTerrainClampWidth(min, max)
	for _, lower := range []float64{0, 1 - minimum} {
		h, w, err := terrainDimensions(min, max, TerrainClampPercent{Lower: lower, Upper: lower + minimum})
		if err != nil || w != 480 || h != 4 {
			t.Fatalf("exact limit should fit: %dx%d, %v", w, h, err)
		}
	}
	for _, clamp := range []TerrainClampPercent{
		{Upper: minimum - 1e-6}, {Upper: 1e-100}, {Lower: .5, Upper: .5},
		{Upper: math.NaN()}, {Upper: math.Inf(1)}, {Lower: -.1, Upper: .8},
	} {
		if _, _, err := terrainDimensions(min, max, clamp); err == nil {
			t.Fatalf("invalid range accepted: %+v", clamp)
		}
	}
	max[2] = 0
	if _, _, err := terrainDimensions(min, max, TerrainClampPercent{Upper: 1}); err == nil {
		t.Fatal("flat bounds should fail safely")
	}
	max[2] = 1
	if _, _, err := terrainDimensions(min, max, TerrainClampPercent{Upper: 1}); err == nil || !strings.Contains(err.Error(), "fewer cells") {
		t.Fatalf("impossible full range should explain selection limit: %v", err)
	}
}

func TestInvalidClampPreservesExistingMapOnFreshExport(t *testing.T) {
	mapRoot := workspace.ResolveRepoPath("maps")
	if err := os.MkdirAll(mapRoot, 0755); err != nil {
		t.Fatal(err)
	}
	out, err := os.MkdirTemp(mapRoot, "clamp-validation-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(out) })
	marker := filepath.Join(out, "existing-model.mdx")
	if err := os.WriteFile(marker, []byte("keep existing asset"), 0644); err != nil {
		t.Fatal(err)
	}
	registry := common.NewTileRegistry()
	registry.Register(&exportadt.ConversionOutput{
		ObjectPath: "maps/fixture/adt_0_0", ObjFilePath: "maps/fixture/adt_0_0.obj",
		ObjText: "v 0 0 0\nv 1000 1000 1000\nv 1000 0 0\nvt 0 0\nvn 0 1 0\ng terrain\nusemtl terrain\nf 1/1/1 2/1/1 3/1/1\n",
		MtlText: "newmtl terrain\nKd 1 1 1\n",
	})
	cfg := DefaultMapExportConfig()
	cfg.MapID = -1
	cfg.WowExportFolder = "fixture"
	cfg.Doodads.Enable.Gobj = false
	cfg.Creatures.Enable = false
	cfg.Terrain.ClampPercent.Upper = 1e-100
	_, err = RunMapGenerateConversion(context.Background(), MapGenerateConversionOptions{
		Config: config.DefaultConfig(), MapExportConfig: &cfg, TileRegistry: registry,
		MapSaveName: filepath.Base(out), FreshExport: true,
	})
	if err == nil || !strings.Contains(err.Error(), "map size is too large") {
		t.Fatalf("expected map size validation, got %v", err)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "keep existing asset" {
		t.Fatalf("invalid fresh export removed existing assets: %q, %v", data, err)
	}
}

func TestOversizedTerrainFailsJobAndQueueContinues(t *testing.T) {
	m := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	model := mdl.New(mdl.NewMDLOptions{Name: "terrain"})
	model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{
		{Position: imath.Vector3{}}, {Position: imath.Vector3{1000, 1000, 1000}},
	}}}
	m.Roots = []*common.WowObject{{Model: &common.Model{MDL: model}}}
	queue := util.NewJobQueue(util.QueueConfig[float64, int]{Concurrency: 1, JobTTL: time.Minute, JobTimeout: time.Minute}, func(job *util.Job[float64, int]) (int, error) {
		cfg := DefaultMapExportConfig()
		cfg.Terrain.ClampPercent.Upper = job.Request
		terrain, err := NewWc3Converter(cfg).GenerateTerrainWithHeight(m)
		return terrain.Map.Width, err
	})
	queue.AddJob(&util.Job[float64, int]{ID: "oversized", Request: 1e-100, Status: util.JobPending})
	queue.AddJob(&util.Job[float64, int]{ID: "valid", Request: 1, Status: util.JobPending})
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		failed, valid := queue.GetJobStatus("oversized"), queue.GetJobStatus("valid")
		if valid.Status == util.JobDone {
			if failed.Status != util.JobFailed || !strings.Contains(failed.Error, "map size is too large") || valid.Result == nil || *valid.Result != 32 {
				t.Fatalf("bad job must fail and release queue: %+v, %+v", failed, valid)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("queue did not complete after oversized terrain")
}
