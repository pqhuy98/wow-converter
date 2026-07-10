package client

import (
	"context"
)

// DesiredConfig mirrors wow-data-client.ts desiredConfig pushed before each export.
var DesiredConfig = map[string]any{
	"copyMode":                   "FULL",
	"listfileShowFileDataIDs":    true,
	"enableM2Skins":              true,
	"enableSharedTextures":       true,
	"enableSharedChildren":       true,
	"enableAbsoluteMTLPaths":     false,
	"enableAbsoluteCSVPaths":     false,
	"removePathSpaces":           true,
	"removePathSpacesCopy":       true,
	"exportTextureFormat":        "PNG",
	"exportModelFormat":          "OBJ",
	"exportM2Bones":              true,
	"exportM2Meta":               true,
	"exportWMOMeta":              true,
	"modelsExportSkin":           true,
	"modelsExportSkel":           true,
	"modelsExportBone":           true,
	"modelsExportAnim":           true,
	"modelsExportUV2":            true,
	"modelsExportTextures":       true,
	"modelsExportAlpha":          true,
	"modelsExportAnimations":     true,
	"modelsExportCollision":      true,
}

// SyncConfig pushes DesiredConfig keys to wow-data-server when they differ.
func SyncConfig(ctx context.Context, c Client) error {
	current, err := c.GetConfig(ctx, "")
	if err != nil {
		return err
	}
	for key, want := range DesiredConfig {
		if current[key] == want {
			continue
		}
		if _, err := c.SetConfig(ctx, key, want); err != nil {
			return err
		}
	}
	return nil
}
