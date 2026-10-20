package directwmo

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wow/formats/wmo"
)

func TestWmoCollisionIncludesHiddenFacesAndTransformsAxes(t *testing.T) {
	g := &wmo.Loader{Vertices: []float32{1, 2, 3, 4, 5, 6, 7, 8, 9},
		Indices:      []uint16{0, 1, 2, 0, 1, 2, 0, 1, 2, 0, 1, 2},
		MaterialInfo: []wmo.MaterialInfo{{Flags: 8}, {Flags: 0x20}, {Flags: 0x24}, {MaterialID: 255}}}
	triangles, err := buildWmoCollision([]*wmo.Loader{g}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(triangles) != 3 {
		t.Fatal("collision and hidden material-255 faces must survive; noncolliding detail faces must not")
	}
	if got := triangles[0][0]; got[0] != 2 || got[1] != -6 || got[2] != 4 {
		t.Fatalf("collision must match the scaled Z-up render coordinates: %v", got)
	}
}
