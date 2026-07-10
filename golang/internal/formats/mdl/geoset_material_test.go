package mdl

import (
	"testing"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
)

func TestGeosetMaterialIDsViaUpdateIDs(t *testing.T) {
	mat0 := &components.Material{
		Layers: []components.Layer{{
			Texture: &components.Texture{Image: "wow/tex0.blp"},
		}},
	}
	mat1 := &components.Material{
		Layers: []components.Layer{{
			Texture: &components.Texture{Image: "wow/tex1.blp"},
		}},
	}

	m := &MDL{
		Materials: []*components.Material{mat0, mat1},
		Geosets: []*components.Geoset{
			{Material: mat0},
			{Material: mat1},
		},
	}
	m.UpdateIDs()

	if m.Geosets[0].Material == nil || m.Geosets[0].Material.ID != 0 {
		t.Fatalf("geoset0 material id=%d ptr=%p", m.Geosets[0].Material.ID, m.Geosets[0].Material)
	}
	if m.Geosets[1].Material == nil || m.Geosets[1].Material.ID != 1 {
		t.Fatalf("geoset1 material id=%d ptr=%p", m.Geosets[1].Material.ID, m.Geosets[1].Material)
	}
	if m.Geosets[0].Material == m.Geosets[1].Material {
		t.Fatal("geosets should reference distinct materials")
	}
	if m.Geosets[0].Material != m.Materials[0] || m.Geosets[1].Material != m.Materials[1] {
		t.Fatal("geoset materials should reference m.Materials slice entries")
	}
}
