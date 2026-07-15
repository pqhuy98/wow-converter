package character

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

type failingModelCacheClient struct {
	client.Client
}

func (failingModelCacheClient) InitModelCaches(context.Context) error {
	return errors.New("cache unavailable")
}

func TestExportCreatureModelsPropagatesModelCacheFailure(t *testing.T) {
	err := ExportCreatureModels(nil, t.TempDir(), config.DefaultConfig(), failingModelCacheClient{}, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "cache unavailable") {
		t.Fatalf("expected model cache error, got %v", err)
	}
}

type failingCreatureFileClient struct {
	client.Client
}

func (failingCreatureFileClient) InitModelCaches(context.Context) error { return nil }

func (failingCreatureFileClient) GetCASCInfo(context.Context) (client.CASCInfo, error) {
	return client.CASCInfo{Build: casc.BuildInfo{Product: "wow"}}, nil
}

func (failingCreatureFileClient) ResolveNpcDisplayMeta(
	context.Context,
	int,
) (casc.NpcDisplayMeta, error) {
	return casc.NpcDisplayMeta{Found: true, Model: 456}, nil
}

func (failingCreatureFileClient) GetFileByID(
	context.Context,
	int,
) (casc.ListfileEntry, error) {
	return casc.ListfileEntry{}, errors.New("model file unavailable")
}

func (failingCreatureFileClient) DownloadCascFile(context.Context, int) ([]byte, error) {
	return nil, errors.New("model file unavailable")
}

func TestExportCreatureModelsPropagatesPerCreatureFailure(t *testing.T) {
	creatures := []azerothcore.Creature{{
		Template: azerothcore.CreatureTemplate{Name: "Broken creature"},
		Model:    azerothcore.CreatureTemplateModel{CreatureDisplayID: 123},
	}}
	err := ExportCreatureModels(
		creatures,
		t.TempDir(),
		config.DefaultConfig(),
		failingCreatureFileClient{},
		1,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "export creature creature-123") {
		t.Fatalf("expected per-creature export error, got %v", err)
	}
}
