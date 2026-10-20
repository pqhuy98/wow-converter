package mapexporter

import (
	"context"
	"encoding/binary"
	"fmt"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/workspace"
)

func TestNavigationWithoutBlockersClearsCachedPathing(t *testing.T) {
	if DefaultMapExportConfig().PlacePathBlockers {
		t.Fatal("path blockers must be opt-in")
	}
	dir := t.TempDir()
	r := &terrainNavigation{Width: 2, Height: 2, pathing: []byte{14, 14, 14, 14}}
	if err := r.write(dir, true); err != nil {
		t.Fatal(err)
	}
	if err := r.write(dir, false); err != nil {
		t.Fatal(err)
	}
	cfg := DefaultMapExportConfig()
	mm := extra.NewMapManager()
	mm.SetTerrain(getInitialTerrain(2, 3))
	exporter := &MapExporter{MapExportConfig: &cfg, MapManager: mm}
	if err := exporter.SaveWar3mapFiles(context.Background(), dir, "no-blockers"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "war3map.wpm")); !os.IsNotExist(err) {
		t.Fatalf("disabled blockers left cached engine pathing: %v", err)
	}
	for _, name := range []string{"terrain-navigation.json", "terrain-navigation.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSolidSquaresAlignIsolatedCellsToNativeGrid(t *testing.T) {
	for _, flags := range []byte{10, 14} {
		cells := make([]byte, 12*8)
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				cells[y*12+x] = flags
			}
		}
		cells[6*12+8] = flags
		expected := append([]byte(nil), cells...)
		for y := 6; y < 8; y++ {
			for x := 8; x < 10; x++ {
				expected[y*12+x] = flags
			}
		}
		mm := extra.NewMapManager()
		mm.SetTerrain(getInitialTerrain(2, 3))
		dir := t.TempDir()
		r := &terrainNavigation{Width: 12, Height: 8, pathing: cells}
		if err := r.placeBlockers(mm, dir); err != nil {
			t.Fatal(err)
		}
		assertPathBlockerModel(t, mm, dir)
		if r.BlockerInstances != 2 {
			t.Fatal("aligned square and isolated cell each need one solid square doodad")
		}
		mm.EnsureMapInfo("solid-squares")
		if err := mm.Save(dir); err != nil {
			t.Fatal(err)
		}
		loaded := extra.NewMapManager()
		if err := loaded.Load(dir); err != nil {
			t.Fatal(err)
		}
		for _, typ := range loaded.DoodadTypes {
			clickHelper := false
			for _, field := range typ.Data {
				if field.ID == "duch" && field.Value == int32(1) {
					clickHelper = true
				}
			}
			if !clickHelper {
				t.Errorf("blocker %s must enable its click helper", typ.Code)
			}
		}
		assertSavedBlockers(t, dir, loaded, expected, 12, 8)
		if r.BlockerPadding != 3 {
			t.Fatal("isolated cell needs three native-grid neighbor cells")
		}
	}
}

// Reconstruct from serialized types, placements and decoded image pixels, not
// packBlockers' squares. HiveWE's doodads.ixx/pathing_map.ixx apply facing+90
// and blit a decoded top-origin image south-to-north around its center.
func savedBlockerPathing(t *testing.T, dir string, mm *extra.MapManager, width, height int) []byte {
	t.Helper()
	textures := map[string]string{}
	for _, typ := range mm.DoodadTypes {
		blocker, texture := false, ""
		for _, field := range typ.Data {
			if field.ID == "dnam" {
				name, _ := field.Value.(string)
				blocker = strings.HasPrefix(name, "~Path Blocker ")
			}
			if field.ID == "dptx" {
				texture, _ = field.Value.(string)
			}
			if field.ID == "dfil" {
				model, _ := field.Value.(string)
				name := strings.ToLower(strings.ReplaceAll(model, "\\", "/"))
				blocker = blocker || strings.Contains(name, "losblocker") || name == pathBlockerModel
			}
		}
		if blocker {
			textures[typ.Code] = strings.ReplaceAll(texture, "\\", "/")
		}
	}
	actual := make([]byte, width*height)
	terrain := mm.GetTerrain()
	for _, placed := range mm.Doodads {
		texture, ok := textures[placed.Doodad.Type]
		if !ok {
			continue
		}
		var w, h int
		var pixels []byte // decoded image rows, north-to-south
		if strings.HasPrefix(texture, "PathTextures/") {
			if _, err := fmt.Sscanf(filepath.Base(texture), "%dx%d", &w, &h); err != nil {
				t.Fatal(err)
			}
			pixels = make([]byte, w*h)
			for i := range pixels {
				pixels[i] = 10
				if strings.Contains(texture, "Unflyable") {
					pixels[i] |= 4
				}
			}
		} else {
			b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(texture)))
			if err != nil {
				t.Fatal(err)
			}
			if len(b) < 18 || b[2] != 2 || b[16] != 24 {
				t.Fatal("expected uncompressed 24-bit pathing TGA")
			}
			w, h = int(binary.LittleEndian.Uint16(b[12:])), int(binary.LittleEndian.Uint16(b[14:]))
			pixels = make([]byte, w*h)
			for y := 0; y < h; y++ {
				row := y
				if b[17]&32 == 0 { // bottom-origin TGA
					row = h - 1 - y
				}
				for x := 0; x < w; x++ {
					i := 18 + int(b[0]) + (row*w+x)*3
					if b[i+2] > 250 {
						pixels[y*w+x] |= 2
					}
					if b[i+1] > 250 {
						pixels[y*w+x] |= 4
					}
					if b[i] > 250 {
						pixels[y*w+x] |= 8
					}
				}
			}
		}
		if w != h {
			t.Fatalf("blocker %s is %dx%d instead of square", texture, w, h)
		}
		for _, flags := range pixels {
			if flags != 10 && flags != 14 || flags != pixels[0] {
				t.Fatalf("blocker %s must be completely solid, without bitmap masks", texture)
			}
		}
		rotation := (int(math.Round(float64(placed.Angle)/90))*90 + 90) % 360
		rw, rh := w, h
		if rotation%180 != 0 {
			rw, rh = h, w
		}
		cx := float64(placed.Position[0]-terrain.Map.Offset.X) / 32
		cy := float64(placed.Position[1]-terrain.Map.Offset.Y) / 32
		// Measured from a native editor save: centers snap to 64 units,
		// offset by 32 when the integer half-size is odd (sizes 2/3 mod 4).
		snap := func(c float64, dimension int) float64 {
			offset := 0.0
			if dimension/2%2 != 0 {
				offset = 1
			}
			return math.Floor((c+offset)/2+.5)*2 - offset
		}
		if snap(cx, rw) != cx || snap(cy, rh) != cy {
			t.Fatalf("footprint %s at (%g,%g) would move under native editor snapping", texture, placed.Position[0], placed.Position[1])
		}
		for j := 0; j < h; j++ {
			for i := 0; i < w; i++ {
				x, y := i, j
				switch rotation {
				case 90:
					x, y = h-1-j, i
				case 180:
					x, y = w-1-i, h-1-j
				case 270:
					x, y = j, w-1-i
				}
				x, y = int(cx)+x-(rw+1)/2, int(cy)+y-(rh+1)/2
				if x >= 0 && x < width && y >= 0 && y < height {
					actual[y*width+x] |= pixels[(h-1-j)*w+i]
				}
			}
		}
	}
	return actual
}

// A separately saved native map is independent evidence for the raster rules.
// Native terrain/other objects can add pathing, but every predicted blocker
// must exist at exactly that location in World Editor's saved WPM.
func TestNativeEditorSavedBlockerRaster(t *testing.T) {
	dir := os.Getenv("WOW_NAV_NATIVE_MAP")
	if dir == "" {
		t.Skip("set WOW_NAV_NATIVE_MAP to a separately saved native editor map")
	}
	dir = workspace.ResolveRepoPath(dir)
	mm := extra.NewMapManager()
	if err := mm.Load(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "war3map.wpm"))
	if err != nil {
		t.Fatal(err)
	}
	w, h := int(binary.LittleEndian.Uint32(b[8:])), int(binary.LittleEndian.Uint32(b[12:]))
	predicted := savedBlockerPathing(t, dir, mm, w, h)
	occupied, additional := 0, 0
	for i, flags := range predicted {
		native := b[16+i] & 14
		if flags & ^native != 0 {
			t.Fatalf("predicted blocker absent from native WPM at (%d,%d): predicted %d, native %d", i%w, i/w, flags, native)
		}
		if flags != 0 {
			occupied++
		}
		if flags != native {
			additional++
		}
	}
	if occupied == 0 || additional > w*h/100 {
		t.Fatalf("native raster comparison has insufficient coverage: %d blocker cells, %d additional native cells", occupied, additional)
	}
	t.Logf("native save confirms %d blocker cells; %d cells add terrain/other-object pathing", occupied, additional)
}

func assertPathBlockerModel(t *testing.T, mm *extra.MapManager, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, pathBlockerModel)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "empty.mdx")); err != nil {
		t.Fatal(err)
	}
	for _, typ := range mm.DoodadTypes {
		got := ""
		for _, field := range typ.Data {
			if field.ID == "dfil" {
				got, _ = field.Value.(string)
			}
		}
		if got != pathBlockerModel {
			t.Fatalf("blocker model %q, want %s", got, pathBlockerModel)
		}
	}
}

func assertSavedBlockers(t *testing.T, dir string, mm *extra.MapManager, expected []byte, width, height int) {
	t.Helper()
	actual := savedBlockerPathing(t, dir, mm, width, height)
	// Liquid movement flags are terrain pathing, independent of blocker doodads.
	// Verify their union without allowing structural footprints to drift.
	if f, err := os.Open(filepath.Join(dir, "terrain-water.png")); err == nil {
		defer f.Close()
		water, err := png.Decode(f)
		if err != nil {
			t.Fatal(err)
		}
		for i := range actual {
			switch color.NRGBAModel.Convert(water.At(i%width, height-1-i/width)) {
			case color.NRGBA{40, 150, 220, 255}:
				actual[i] |= 64 | 8
			case color.NRGBA{25, 65, 155, 255}:
				actual[i] |= 64 | 8 | 2
			case color.NRGBA{210, 85, 35, 255}:
				actual[i] |= 8 | 2
			}
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	mismatches, first := 0, -1
	for i := range expected {
		if actual[i] != expected[i] {
			mismatches++
			if first < 0 {
				first = i
			}
		}
	}
	if mismatches > 0 {
		t.Fatalf("saved doodad footprints differ from WPM in %d cells; first (%d,%d): got %d, want %d", mismatches, first%width, first/width, actual[first], expected[first])
	}
}

func TestExportedNavigationBlockerFootprints(t *testing.T) {
	dir := os.Getenv("WOW_NAV_VERIFY_MAP")
	if dir == "" {
		t.Skip("set WOW_NAV_VERIFY_MAP to validate an existing exported map")
	}
	dir = workspace.ResolveRepoPath(dir)
	mm := extra.NewMapManager()
	if err := mm.Load(dir); err != nil {
		t.Fatal(err)
	}
	wpm, err := os.ReadFile(filepath.Join(dir, "war3map.wpm"))
	if err != nil {
		t.Fatal(err)
	}
	w, h := int(binary.LittleEndian.Uint32(wpm[8:])), int(binary.LittleEndian.Uint32(wpm[12:]))
	assertSavedBlockers(t, dir, mm, wpm[16:], w, h)
	f, err := os.Open(filepath.Join(dir, "terrain-navigation.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	preview, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	for i, flags := range savedBlockerPathing(t, dir, mm, w, h) {
		want := color.NRGBA{R: 52, G: 180, B: 90, A: 255}
		if flags&2 != 0 {
			want = color.NRGBA{R: 190, G: 90, B: 40, A: 255}
		}
		if flags&4 != 0 {
			want = color.NRGBA{R: 130, G: 60, B: 180, A: 255}
		}
		if color.NRGBAModel.Convert(preview.At(i%w, h-1-i/w)) != want {
			t.Fatalf("PNG differs from saved pathing at cell %d", i)
		}
	}
	t.Logf("all %d saved pathing cells match blocker footprints plus liquid pathing", w*h)
}
