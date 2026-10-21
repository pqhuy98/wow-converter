package terrainfit

import (
	"image"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestMaterialMasksKeepSharedEdgesIndependentOfOtherCorners(t *testing.T) {
	palette := [4][3]float64{{0, 0, 0}, {255, 0, 0}, {0, 255, 0}, {0, 0, 255}}
	evaluate := func(f *fit, x int, u, v float64) [4]float64 {
		l := f.cellLayers(x, 0)
		var colors [4][4]float64
		for j := 0; j < l.count; j++ {
			copy(colors[j][:3], palette[l.ids[j]][:])
			colors[j][3] = 1
			if j > 0 {
				colors[j][3] = materialMaskAlpha(l.masks[j], u, v)
			}
		}
		return compositeLayers(colors, l.count, true)
	}
	// All 4^6 assignments of a two-cell neighborhood, including three/four-way
	// junctions with a different lowest-priority material in each cell.
	f := &fit{width: 2, height: 1, labels: make([]uint8, 6)}
	for code := 0; code < 4096; code++ {
		for i := range f.labels {
			f.labels[i] = uint8((code >> (2 * i)) & 3)
		}
		for j := 0; j <= 16; j++ {
			v := float64(j) / 16
			left, right := evaluate(f, 0, 1, v), evaluate(f, 1, 0, v)
			for k := 0; k < 3; k++ {
				if math.Abs(left[k]-right[k]) > 1e-9 {
					t.Fatalf("shared-edge discontinuity: labels %v, v %g: %v / %v", f.labels, v, left, right)
				}
			}
		}
	}
	var colors [4][4]float64
	colors[0], colors[1], colors[2] = [4]float64{0, 0, 0, 1}, [4]float64{100, 0, 0, .75}, [4]float64{0, 100, 0, .5}
	if got := compositeLayers(colors, 3, true); got[0] != 37.5 || got[1] != 50 {
		t.Fatalf("overlays must use their own alpha: %v", got)
	}
}

func TestMaterialPaletteRepeatsMaterialsWithoutWorldFragments(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "materials", "test")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for id, file := range []string{"1.png", "2.png"} {
		img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				at := y*img.Stride + x*4
				color := [3]byte{byte(x * 2), 100, 20}
				if id == 1 {
					color = [3]byte{220, 180, 100}
				}
				copy(img.Pix[at:at+3], color[:])
				img.Pix[at+3] = 255
			}
		}
		if err := savePNG(filepath.Join(dir, file), img); err != nil {
			t.Fatal(err)
		}
	}
	f := &fit{}
	if err := f.makeMaterialSheets(root, "test", []terrainMaterial{{ID: 1, Scale: 1}, {ID: 2, Scale: 1}}, [][]float32{{.5, .5, .5, .5, .5}}, false); err != nil {
		t.Fatal(err)
	}
	for layer := 0; layer < 32; layer++ {
		at := (layer*subtile*subtile + 23*subtile + 17) * 4
		for k, want := range []byte{127, 140, 60} {
			if got := f.sheets[0].pixels[at+k]; got != want {
				t.Fatalf("layer %d channel %d: got %d want %d", layer, k, got, want)
			}
		}
	}
}

func TestMaterialDictionaryPreservesSeparateLayerIdentities(t *testing.T) {
	points := []float32{1, 0, .5, .5, .5, 0, 1, .5, .5, .5, 1, 0, .5, .5, .5, 0, 1, .5, .5, .5}
	centers := clusterMaterialFeatures(points, 5, 64)
	if len(centers) != 2 || nearestMaterialFeature(points[:5], centers) == nearestMaterialFeature(points[5:10], centers) {
		t.Fatalf("material identities merged or duplicate slots allocated: %v", centers)
	}
	alpha := make([]byte, 4096)
	for i := range alpha {
		alpha[i] = 127
	}
	for _, uv := range [][2]float64{{0, 0}, {1, 1}, {.5, .5}} {
		if got := sampleMaterialAlpha(alpha, uv[0], uv[1]); math.Abs(got-127.0/255) > 1e-9 {
			t.Fatalf("authored paint sampling: %g", got)
		}
	}
}

func TestSourceBlockMeshUsesContinuousWorldUV(t *testing.T) {
	m := Mesh{Positions: [][3]float64{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0}}, Indices: []uint32{0, 1, 2, 2, 1, 3}}
	mask := make([]bool, cellsPerADT*cellsPerADT)
	mask[17*cellsPerADT+9] = true
	_, groups := clipPatches(m, mask, []patchPlacement{{x: 9, y: 17, x0: 576, y0: 1088, sx: 64, sy: 64}}, 512, 0)
	if len(groups) != 1 || len(groups[0].UV) == 0 {
		t.Fatal("missing source block")
	}
	for i, p := range groups[0].Mesh.Positions {
		want := [2]float64{(p[0]*8192 - 512) / 512, (p[1]*8192 - 1024) / 512}
		if math.Abs(groups[0].UV[i][0]-want[0]) > 1e-9 || math.Abs(groups[0].UV[i][1]-want[1]) > 1e-9 {
			t.Fatal("unrelated cell coordinates mixed into source-block UV")
		}
	}
}
