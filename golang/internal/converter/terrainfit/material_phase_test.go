package terrainfit

import (
	"image"
	"math"
	"testing"

	bake "github.com/pqhuy98/wow-converter/internal/wow/export/adt"
)

func TestScaledMaterialPhaseMatchesSourceUVForEveryRotation(t *testing.T) {
	catalog := []terrainMaterial{{ID: 1, Scale: 16}}
	for turns, steps := range [][2][2]float64{
		{{1, 0}, {0, 1}}, {{0, 1}, {-1, 0}}, {{-1, 0}, {0, -1}}, {{0, -1}, {1, 0}},
	} {
		g := TilesetGrid{Width: 2, Height: 2, Origin: [2]float64{.37, .51}}
		for k := 0; k < 2; k++ {
			g.StepX[k], g.StepY[k] = steps[0][k]*.75/128, steps[1][k]*.75/128
		}
		p := materialPhases{scales: []int{16}}
		features := make([]float32, 9*8)
		for i := 0; i < 9; i++ {
			features[i*8] = 1
		}
		p.allocate(features, catalog, 2, 2, &g)
		for y := 0; y <= 2; y++ {
			for x := 0; x <= 2; x++ {
				u, v := p.uv(features[(y*3+x)*8:(y*3+x+1)*8], 1, 16, .31, .69)
				for k, got := range []float64{u, v} {
					want := (g.Origin[k]+float64(x)*g.StepX[k]+float64(y)*g.StepY[k]+min(0, g.StepX[k])+min(0, g.StepY[k]))*128/16 + []float64{.31, .69}[k]*.75/16
					delta := got - want
					if math.Abs(delta-math.Round(delta)) > 1e-8 {
						t.Fatalf("turn %d cell %d,%d axis %d: %g vs source %g", turns, x, y, k, got, want)
					}
				}
			}
		}
	}
}

func TestScale16FittingUsesBoundedPhasePalette(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			at := y*img.Stride + x*4
			img.Pix[at], img.Pix[at+1], img.Pix[at+2], img.Pix[at+3] = byte(x), byte(y), byte(30+((x/2+y/2)%2)*180), 255
		}
	}
	c := materialCapture{Version: 2, Materials: []terrainMaterial{{ID: 1, Scale: 16}}, Chunks: make([]materialChunk, 256)}
	for i := range c.Chunks {
		c.Chunks[i].Layers = []int{0}
	}
	g := TilesetGrid{Width: 32, Height: 16, StepX: [2]float64{1.0 / 128, 0}, StepY: [2]float64{0, 1.0 / 128}, PaintFamilies: 48}
	f, _, centers, err := fitCaptures([]materialCapture{c}, 1, 1, map[uint32]*image.NRGBA{1: img}, &g)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.sheets) > 49 || len(f.sheets) < 3 {
		t.Fatalf("phase palette must fit the stock budget: %d", len(f.sheets))
	}
	// An authored 16-cell repeat must not become a one-cell repeat.
	if f.labels[2*(g.Width+1)+2] == f.labels[2*(g.Width+1)+10] {
		t.Fatal("opposite source phases collapsed to the same family")
	}
	if f.labels[2*(g.Width+1)+2] != f.labels[2*(g.Width+1)+18] {
		t.Fatal("source phase failed to repeat after 16 cells")
	}
	// Check actual sheet samples against the allocated diffuse phase (including
	// source detail finer than the former unconditional 64px mip).
	p := newMaterialPhases(c.Materials, map[uint32]*image.NRGBA{1: img}, false)
	for ci, center := range centers {
		u, v := p.uv(center, 1, 16, 23.5/64, 37.5/64)
		want := sampleMaterialImage(img, u, v)
		at := (16*64*64 + 37*64 + 23) * 4
		for k := 0; k < 3; k++ {
			if math.Abs(float64(f.sheets[ci].pixels[at+k])-want[k]) > .51 {
				t.Fatalf("phase %d channel %d: got %d want %g", ci, k, f.sheets[ci].pixels[at+k], want[k])
			}
		}
	}
}

func TestUniformScaledMaterialsDoNotWastePhaseSlots(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := 0; i < len(img.Pix); i += 4 {
		copy(img.Pix[i:i+4], []byte{180, 190, 200, 255})
	}
	if p := newMaterialPhases([]terrainMaterial{{ID: 1, Scale: 16}}, map[uint32]*image.NRGBA{1: img}, true); len(p.scales) != 0 {
		t.Fatal("uniform snow needs no phase allocation")
	}
}

func TestScaledHeightUsesTheSamePhaseAsDiffuse(t *testing.T) {
	images := map[uint32]*image.NRGBA{}
	for id := uint32(1); id <= 3; id++ {
		img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
		for y := 0; y < 256; y++ {
			for x := 0; x < 256; x++ {
				at := y*img.Stride + x*4
				img.Pix[at+3] = 255
				switch id {
				case 1:
					img.Pix[at] = 240
				case 2:
					img.Pix[at+1] = 220
				case 3:
					img.Pix[at+3] = byte(x)
				}
			}
		}
		images[id] = img
	}
	catalog := []terrainMaterial{{ID: 1, HeightID: 3, Scale: 16, HeightScale: 1}, {ID: 2, Scale: 1, HeightOffset: .5}}
	p := newMaterialPhases(catalog, images, true)
	// Two paints, four circular phase dimensions, three tint dimensions.
	center := []float32{.5, .5, 0, .175, .175, 0, .5, .5, .5}
	f := &fit{}
	if err := f.makeMaterialSheetsFrom(catalog, [][]float32{center}, true, images, p); err != nil {
		t.Fatal(err)
	}
	u, v := p.uv(center, 2, 16, 23.5/64, 37.5/64)
	height := sampleMaterialImage(images[3], u, v)[3] / 255
	weights := bake.TerrainHeightWeights([8]float64{.5, .5}, [8]float64{height, .5}, 2)
	at := (16*64*64 + 37*64 + 23) * 4
	for k, want := range []float64{240 * weights[0], 220 * weights[1]} {
		if math.Abs(float64(f.sheets[0].pixels[at+k])-want) > .51 {
			t.Fatalf("height phase channel %d: got %d want %g", k, f.sheets[0].pixels[at+k], want)
		}
	}
}
