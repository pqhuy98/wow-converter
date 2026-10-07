package common

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

type cancellingTextureClient struct {
	client.Client
	started chan context.Context
	calls   atomic.Int32
}

func (c *cancellingTextureClient) DownloadCascFile(ctx context.Context, _ int) ([]byte, error) {
	c.calls.Add(1)
	c.started <- ctx
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestTextureExportCancellationStopsDownloadAndFurtherWork(t *testing.T) {
	const rel = "queue-context-test/texture.png"
	texturesource.Register(rel, texturesource.Source{Kind: texturesource.KindBLP, FileDataID: 123})
	t.Cleanup(func() { texturesource.Unregister(rel) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &cancellingTextureClient{started: make(chan context.Context, 1)}
	manager := NewAssetManager(config.Config{ExportAssetDir: t.TempDir(), AssetPrefix: "wow", MaxTextureSize: 256}, client, nil)
	manager.AddPngTexture(rel, false)
	dir := t.TempDir()
	result := make(chan error, 1)
	go func() { _, err := manager.ExportTextures(ctx, dir); result <- err }()
	select {
	case downloadContext := <-client.started:
		if downloadContext != ctx {
			t.Fatal("texture download lost export context")
		}
	case <-time.After(time.Second):
		t.Fatal("texture download did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("export error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("texture export did not stop after cancellation")
	}
	if client.calls.Load() != 1 {
		t.Fatal("another download started after cancellation")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("texture export wrote output after cancellation")
	}
}
