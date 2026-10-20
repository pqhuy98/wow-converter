package mapexporter

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

func (e *MapExporter) exportMinimap(ctx context.Context, outputDir string) error {
	const size = 256
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	if e.WowObjectManager != nil {
		bounds := common.ComputeAbsoluteMinMaxExtents(e.WowObjectManager.Roots)
		roots := append([]*common.WowObject(nil), e.WowObjectManager.Roots...)
		sort.Slice(roots, func(i, j int) bool {
			if roots[i].TileX != roots[j].TileX {
				return roots[i].TileX < roots[j].TileX
			}
			return roots[i].TileY < roots[j].TileY
		})
		for _, root := range roots {
			if root.Type != common.WowObjectADT || root.Model == nil {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if e.wowClient == nil {
				return fmt.Errorf("minimap export requires the WoW data client")
			}
			name := fmt.Sprintf("world/minimaps/%s/map%02d_%02d.blp", strings.ToLower(e.MapExportConfig.CascMapDir), root.TileX, root.TileY)
			file, err := e.wowClient.GetFileByName(ctx, name)
			if err != nil {
				return fmt.Errorf("resolve minimap %s: %w", name, err)
			}
			raw, err := e.wowClient.DownloadCascFile(ctx, file.FileDataID)
			if err != nil {
				return fmt.Errorf("download minimap %s: %w", name, err)
			}
			decoded, err := blp.NewBLPImage(buffer.From(raw))
			if err != nil {
				return fmt.Errorf("decode minimap %s: %w", name, err)
			}
			pngData, err := decoded.ToPNG(0b1111, 0)
			if err != nil {
				return err
			}
			tile, err := png.Decode(bytes.NewReader(pngData.Raw()))
			if err != nil {
				return err
			}
			paintMinimapTile(canvas, tile, root, bounds, e.MapManager.GetTerrain().Map.Width, e.MapManager.GetTerrain().Map.Height, e.Config.RawModelScaleUp)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, canvas); err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	return blp.ConvertTextureToBlp(blp.EncodeInput{PNG: encoded.Bytes(), Opaque: true}, filepath.Join(outputDir, "minimap.blp"))
}

// Inverse-map each minimap pixel through the same final bounds used by
// PlaceDoodads. This handles export rotation and leaves unselected tiles black.
func paintMinimapTile(dst *image.RGBA, tile image.Image, root *common.WowObject, bounds common.AbsoluteExtents, width, height int, rawScale float64) {
	localMin := imath.Vector3{math.Inf(1), math.Inf(1), math.Inf(1)}
	localMax := imath.Vector3{math.Inf(-1), math.Inf(-1), math.Inf(-1)}
	for _, g := range root.Model.MDL.Geosets {
		for _, v := range g.Vertices {
			localMin = imath.V3Min(localMin, v.Position)
			localMax = imath.V3Max(localMax, v.Position)
		}
	}
	// Empty ADTs are not recentered by the parser. Their minimap still
	// covers the source cell around any WMOs promoted to map roots.
	if math.IsInf(localMin[0], 0) && rawScale > 0 {
		x0, y0, x1, y1 := exportadt.TileBounds(root.TileX, root.TileY)
		localMin = imath.Vector3{y0 * rawScale, -x1 * rawScale, 0}
		localMax = imath.Vector3{y1 * rawScale, -x0 * rawScale, 0}
	}
	sx, sy := localMax[0]-localMin[0], localMax[1]-localMin[1]
	if sx <= 0 || sy <= 0 || math.IsInf(sx, 0) || math.IsInf(sy, 0) || width <= 0 || height <= 0 {
		return
	}
	worldWidth, worldHeight := float64(width)*config.DistancePerTile, float64(height)*config.DistancePerTile
	side := math.Max(worldWidth, worldHeight)
	c, s := math.Cos(root.Rotation[2]), math.Sin(root.Rotation[2])
	for y := 0; y < dst.Bounds().Dy(); y++ {
		py := .5 + (.5-(float64(y)+.5)/float64(dst.Bounds().Dy()))*side/worldHeight
		if py < 0 || py > 1 {
			continue
		}
		for x := 0; x < dst.Bounds().Dx(); x++ {
			px := .5 + ((float64(x)+.5)/float64(dst.Bounds().Dx())-.5)*side/worldWidth
			if px < 0 || px > 1 {
				continue
			}
			dx := bounds.Min[0] + px*(bounds.Max[0]-bounds.Min[0]) - root.Position[0]
			dy := bounds.Min[1] + py*(bounds.Max[1]-bounds.Min[1]) - root.Position[1]
			lx, ly := c*dx+s*dy, -s*dx+c*dy
			// ADT OBJ -> MDL maps (x,y,z) to (x,-z,y): tile
			// image columns decrease local X, rows increase local Y.
			u, v := (localMax[0]-lx)/sx, (ly-localMin[1])/sy
			if u < 0 || u >= 1 || v < 0 || v >= 1 {
				continue
			}
			tb := tile.Bounds()
			dst.Set(x, y, tile.At(tb.Min.X+int(u*float64(tb.Dx())), tb.Min.Y+int(v*float64(tb.Dy()))))
		}
	}
}
