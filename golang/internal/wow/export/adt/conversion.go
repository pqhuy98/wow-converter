package adt

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/export"
	"github.com/pqhuy98/wow-converter/internal/wow/export/writers"
	"github.com/pqhuy98/wow-converter/internal/wow/formats/png"
	"github.com/pqhuy98/wow-converter/internal/wow/server"
)

func encodePNGBytes(pixels []byte, width, height int) ([]byte, error) {
	w := png.NewWriter(width, height)
	copy(w.Pixels(), pixels)
	return w.Encode()
}

func storePNG(path string, pixels []byte, width, height int, conv *ConversionOutput) error {
	data, err := encodePNGBytes(pixels, width, height)
	if err != nil {
		return err
	}
	if conv != nil {
		rel, err := filepath.Rel(conv.ExportAssetDir, path)
		if err != nil {
			rel = filepath.Base(path)
		}
		conv.Textures = append(conv.Textures, BakedTexture{
			RelPath: filepath.ToSlash(rel),
			PNG:     data,
		})
		return nil
	}
	return writers.WriteOutputFile(path, data)
}

// ExportForConversion loads an ADT tile into memory for WC3 map conversion (no disk artifacts).
func (e *Exporter) ExportForConversion(
	ctx context.Context,
	source casc.Source,
	exportAssetDir string,
	quality int,
	options export.ADTExportOptions,
	gameObjects map[uint32]db.DB2Row,
	progress *export.ProgressReporter,
) (*ConversionOutput, error) {
	exportMapDir := normalizeConversionMapDir(e.MapDir, server.GetConfig().RemovePathSpaces)
	mapRelDir := filepath.ToSlash(filepath.Join("maps", exportMapDir))
	conv := &ConversionOutput{
		ExportAssetDir: exportAssetDir,
		TileX:          e.TileX,
		TileY:          e.TileY,
		ObjectPath:     filepath.ToSlash(filepath.Join(mapRelDir, "adt_"+e.TileID)),
		WmoPlacements:  map[string][]PlacementRow{},
	}
	dir := filepath.Join(exportAssetDir, mapRelDir)
	conv.ObjFilePath = filepath.Join(dir, "adt_"+e.TileID+".obj")
	_, err := e.Export(ctx, source, dir, quality, options, gameObjects, progress, conv)
	if err != nil {
		return nil, err
	}
	return conv, nil
}

func normalizeConversionMapDir(mapDir string, removePathSpaces bool) string {
	if removePathSpaces {
		return strings.ReplaceAll(mapDir, " ", "")
	}
	return mapDir
}
