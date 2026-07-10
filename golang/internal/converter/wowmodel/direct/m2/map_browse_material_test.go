package directm2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/wow/bootstrap"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

func TestMapVsBrowseMaterialTextures(t *testing.T) {
	if os.Getenv("WOW_INTEGRATION") == "" {
		t.Skip("set WOW_INTEGRATION=1 to run")
	}
	_ = workspace.LoadEnvFile(filepath.Join(workspace.FindRepoRoot(), ".env"))

	ctx := context.Background()
	handler, err := bootstrap.StartWowDataServer(ctx)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	src := client.NewInProcessClient(handler)

	cfg := config.DefaultConfig()
	cfg.IsBulkExport = true

	const fileDataID = 189929
	browseRef := `world/azeroth/elwynn/passivedoodads/trees/elwynntreecanopy03`
	mapRef := `azeroth/32_48/world/maps/elwynn/elwynntreecanopy03`

	convert := func(objectPath string) ([]string, *mdl.MDL) {
		exportPath := filepath.Join(cfg.ExportAssetDir, objectPath+".m2")
		result, err := ConvertM2ToMdl(ctx, cfg, cascSource{src}, ConvertOptions{
			FileDataID:         fileDataID,
			ExportPathOverride: exportPath,
		})
		if err != nil {
			t.Fatalf("convert %s: %v", objectPath, err)
		}
		var images []string
		for _, mat := range result.MDL.Materials {
			for _, layer := range mat.Layers {
				if layer.Texture != nil {
					images = append(images, layer.Texture.Image)
				}
			}
		}
		return images, result.MDL
	}

	browseImages, _ := convert(browseRef)
	mapImages, mapMDL := convert(mapRef)
	t.Logf("browse materials: %v", browseImages)
	t.Logf("map materials: %v", mapImages)

	browseSet := map[string]struct{}{}
	for _, img := range browseImages {
		browseSet[img] = struct{}{}
	}
	mapSet := map[string]struct{}{}
	for _, img := range mapImages {
		mapSet[img] = struct{}{}
	}
	if len(mapSet) == 1 && len(browseSet) > 1 {
		t.Fatalf("map export collapsed to one texture %v, browse had %v", mapImages, browseImages)
	}
	if len(browseSet) != len(mapSet) {
		t.Fatalf("texture count mismatch browse=%d map=%d", len(browseSet), len(mapSet))
	}
	for img := range browseSet {
		if _, ok := mapSet[img]; !ok {
			t.Fatalf("browse texture %q missing from map export", img)
		}
	}

	mdlText := mapMDL.ToMdl()
	if strings.Count(mdlText, "MaterialID 0") > 1 {
		t.Fatalf("map export MDL collapsed geoset materials to ID 0:\n%s", mdlText)
	}
}

type cascSource struct {
	client client.Client
}

func (s cascSource) GetRawFile(ctx context.Context, fileDataID int) ([]byte, error) {
	return s.client.DownloadCascFile(ctx, fileDataID)
}

func (s cascSource) GetFileName(ctx context.Context, fileDataID int) (string, error) {
	entry, err := s.client.GetFileByID(ctx, fileDataID)
	if err != nil {
		return "", err
	}
	return entry.FileName, nil
}

func (s cascSource) GetModelSkins(ctx context.Context, fileDataID int) ([]ModelSkin, error) {
	return nil, nil
}

func (s cascSource) GetBuildKey(ctx context.Context) (string, error) {
	info, err := s.client.GetCASCInfo(ctx)
	if err != nil {
		return "", err
	}
	return info.BuildKey, nil
}
