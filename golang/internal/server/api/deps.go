package api

import (
	"context"

	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

// Deps bundles shared dependencies for API handlers.
type Deps struct {
	Client client.Client
	Config Config

	exportAssetDir string
	startup        *startupAnnouncer
}

// NewDeps creates handler dependencies and probes the data client.
func NewDeps(c client.Client, cfg Config) *Deps {
	d := &Deps{
		Client:         c,
		Config:         cfg,
		exportAssetDir: workspace.ResolveExportAssetDir(""),
		startup:        newStartupAnnouncer(cfg.Port),
	}
	ctx := context.Background()
	if resp, err := c.GetConfig(ctx, "exportDirectory"); err == nil {
		if v, ok := resp["exportDirectory"].(string); ok && v != "" {
			d.exportAssetDir = workspace.ResolveExportAssetDir(v)
		}
	}
	return d
}

// IsDataServerReady reports whether the wow-data client can reach the runtime.
func (d *Deps) IsDataServerReady(ctx context.Context) bool {
	_, err := d.Client.GetConfig(ctx, "exportDirectory")
	return err == nil
}

func (d *Deps) ExportAssetDir() string { return d.exportAssetDir }

// IsClassic reports whether the active CASC product is a Classic variant.
func (d *Deps) IsClassic(ctx context.Context) bool {
	info, err := d.Client.GetCASCInfo(ctx)
	if err != nil {
		return false
	}
	return client.IsClassicProduct(info.Build.Product)
}
