package mapexporter

import (
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	mapterrain "github.com/pqhuy98/wow-converter/internal/converter/mapexporter/terrain"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	imath "github.com/pqhuy98/wow-converter/internal/math"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	exportadt "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

func startupTestRoot(typ common.WowObjectType, x, y, angle float64) *common.WowObject {
	model := mdl.New(mdl.NewMDLOptions{Name: string(typ) + ".mdx"})
	model.Geosets = []*components.Geoset{{Vertices: []*components.GeosetVertex{
		{Position: imath.Vector3{-1, -1, -1}}, {Position: imath.Vector3{1, -1, 1}},
		{Position: imath.Vector3{1, 1, 1}}, {Position: imath.Vector3{-1, 1, -1}},
	}}}
	return &common.WowObject{Type: typ, Position: imath.Vector3{x, y, 0}, Rotation: imath.Vector3{0, 0, angle}, ScaleFactor: 1,
		Model: &common.Model{RelativePath: string(typ) + ".mdx", MDL: model}}
}

func TestMinimapTileOrientationFollowsExportRotation(t *testing.T) {
	red, green, blue, yellow := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}, color.RGBA{255, 255, 0, 255}
	tile := image.NewRGBA(image.Rect(0, 0, 2, 2))
	tile.SetRGBA(0, 0, red)
	tile.SetRGBA(1, 0, green)
	tile.SetRGBA(0, 1, blue)
	tile.SetRGBA(1, 1, yellow)
	for _, tc := range []struct {
		angle float64
		want  [4]color.RGBA
	}{
		{math.Pi, [4]color.RGBA{red, green, blue, yellow}},
		{-math.Pi / 2, [4]color.RGBA{green, yellow, red, blue}},
		{0, [4]color.RGBA{yellow, blue, green, red}},
		{math.Pi / 2, [4]color.RGBA{blue, red, yellow, green}},
	} {
		root := startupTestRoot(common.WowObjectADT, 0, 0, tc.angle)
		bounds := common.ComputeAbsoluteMinMaxExtents([]*common.WowObject{root})
		canvas := image.NewRGBA(image.Rect(0, 0, 8, 8))
		paintMinimapTile(canvas, tile, root, bounds, 8, 8, 1)
		got := [4]color.RGBA{canvas.RGBAAt(1, 1), canvas.RGBAAt(6, 1), canvas.RGBAAt(1, 6), canvas.RGBAAt(6, 6)}
		if got != tc.want {
			t.Fatalf("angle %v: got %v, want %v", tc.angle, got, tc.want)
		}
	}
}

func TestMinimapPreservesGapsAndRectangularMapPadding(t *testing.T) {
	roots := []*common.WowObject{startupTestRoot(common.WowObjectADT, -2, 0, math.Pi), startupTestRoot(common.WowObjectADT, 2, 0, math.Pi)}
	bounds := common.ComputeAbsoluteMinMaxExtents(roots)
	canvas := image.NewRGBA(image.Rect(0, 0, 12, 12))
	for _, root := range roots {
		paintMinimapTile(canvas, image.NewUniform(color.White), root, bounds, 6, 2, 1)
	}
	if canvas.RGBAAt(1, 5).A != 255 || canvas.RGBAAt(10, 5).A != 255 || canvas.RGBAAt(6, 5).A != 0 || canvas.RGBAAt(1, 1).A != 0 {
		t.Fatal("minimap filled a gap, stretched the map, or lost selected tiles")
	}
	root := startupTestRoot(common.WowObjectADT, 0, 0, math.Pi/4)
	canvas = image.NewRGBA(image.Rect(0, 0, 12, 12))
	paintMinimapTile(canvas, image.NewUniform(color.White), root, common.ComputeAbsoluteMinMaxExtents([]*common.WowObject{root}), 6, 6, 1)
	if canvas.RGBAAt(0, 0).A != 0 || canvas.RGBAAt(6, 6).A != 255 {
		t.Fatal("arbitrary rotation did not leave black corners")
	}
}

func TestPreloadLocationsSelectPlacedADTsOrRootWMOs(t *testing.T) {
	for _, adt := range []bool{true, false} {
		root := startupTestRoot(common.WowObjectWMO, 20, 30, .3)
		root.Children = []*common.WowObject{startupTestRoot(common.WowObjectWMO, 0, 0, 0)}
		roots := []*common.WowObject{root}
		if adt {
			roots = append(roots, startupTestRoot(common.WowObjectADT, 30, 40, .3))
		} else {
			empty := startupTestRoot(common.WowObjectADT, 0, 0, 0)
			empty.Model.MDL.Geosets = nil
			roots = append(roots, empty)
		}
		manager := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
		manager.Roots = roots
		mm := extra.NewMapManager()
		mm.SetTerrain(mapterrain.GetInitialTerrain(16, 16))
		converter := mapterrain.NewWc3Converter(DefaultMapExportConfig())
		if _, err := converter.PlaceDoodads(mm, manager, func(*common.WowObject) bool { return true }); err != nil {
			t.Fatal(err)
		}
		idx := 0
		if adt {
			idx = 2
		}
		placed := mm.Doodads[idx].Doodad.Position
		want := [][2]float32{{placed[0], placed[1]}}
		if !reflect.DeepEqual(converter.PreloadLocations, want) {
			t.Fatalf("ADT=%v: got %v, want final WC3 placement %v", adt, converter.PreloadLocations, want)
		}
	}
}

func TestEmptyADTStillPaintsItsMinimapOverPromotedWMOs(t *testing.T) {
	root := startupTestRoot(common.WowObjectADT, 0, 0, math.Pi)
	root.TileX, root.TileY = 32, 32
	root.Model.MDL.Geosets = nil
	x0, y0, x1, y1 := exportadt.TileBounds(32, 32)
	bounds := common.AbsoluteExtents{Min: imath.Vector3{-y1, x0, 0}, Max: imath.Vector3{-y0, x1, 1}}
	canvas := image.NewRGBA(image.Rect(0, 0, 8, 8))
	paintMinimapTile(canvas, image.NewUniform(color.White), root, bounds, 8, 8, 1)
	if canvas.RGBAAt(0, 0).A != 255 || canvas.RGBAAt(7, 7).A != 255 {
		t.Fatal("empty ADT lost its selected cell's minimap")
	}
}

type minimapTestClient struct {
	client.Client
	file      []byte
	requested []string
}

func (c *minimapTestClient) GetFileByName(ctx context.Context, name string) (casc.ListfileEntry, error) {
	c.requested = append(c.requested, name)
	return casc.ListfileEntry{FileDataID: 123}, ctx.Err()
}

func (c *minimapTestClient) DownloadCascFile(ctx context.Context, id int) ([]byte, error) {
	return c.file, ctx.Err()
}

func TestExportMinimapReadsSelectedTilesAndWritesBLP(t *testing.T) {
	// The source is WoW BLP2; the exporter writes WC3 BLP1. A tiny
	// uncompressed BLP2 avoids depending on CASC or the template's BLP1.
	raw := make([]byte, 148+4*4*4)
	copy(raw, "BLP2")
	binary.LittleEndian.PutUint32(raw[4:], 1)
	raw[8], raw[9] = 3, 8
	binary.LittleEndian.PutUint32(raw[12:], 4)
	binary.LittleEndian.PutUint32(raw[16:], 4)
	binary.LittleEndian.PutUint32(raw[20:], 148)
	binary.LittleEndian.PutUint32(raw[84:], 4*4*4)
	for i := 148; i < len(raw); i += 4 {
		raw[i], raw[i+3] = 255, 255
	}
	c := &minimapTestClient{file: raw}
	cfg := DefaultMapExportConfig()
	cfg.CascMapDir = "IcecrownCitadel"
	manager := common.NewWowObjectManager(config.DefaultConfig(), c, nil)
	root := startupTestRoot(common.WowObjectADT, 0, 0, math.Pi)
	root.TileX, root.TileY = 26, 23
	manager.Roots = []*common.WowObject{root}
	mm := extra.NewMapManager()
	mm.SetTerrain(mapterrain.GetInitialTerrain(16, 16))
	e := &MapExporter{MapExportConfig: &cfg, MapManager: mm, WowObjectManager: manager, wowClient: c}
	dir := t.TempDir()
	if err := e.exportMinimap(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(c.requested, []string{"world/minimaps/icecrowncitadel/map26_23.blp"}) {
		t.Fatalf("wrong tile lookup: %v", c.requested)
	}
	size := blp.ReadBlpSizeSync(filepath.Join(dir, "minimap.blp"))
	if size == nil || size.Width != 256 || size.Height != 256 {
		t.Fatalf("invalid minimap BLP: %+v", size)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.exportMinimap(ctx, dir); err != context.Canceled {
		t.Fatalf("cancellation not propagated: %v", err)
	}
}
