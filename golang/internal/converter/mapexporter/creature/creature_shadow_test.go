package creature

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestSDShadowUsesScaledBodySizeIndependentlyOfCollision(t *testing.T) {
	root := t.TempDir()
	m := mdx.NewModel()
	// A tall, narrow body's height prevents a tiny footprint-only shadow.
	g := shadowBox(10, 20, 100)
	m.Geosets = append(m.Geosets, g)
	original := m.SaveMdx()
	modelPath := filepath.Join(root, "creature-1.mdx")
	if err := os.WriteFile(modelPath, original, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		radius, scale float64
		character     bool
		want          float64
	}{
		{"body without source collision", 0, 1, false, 100},
		{"tiny source collision", 1, 1, false, 100},
		{"oversized source collision", 1000, 1, false, 100},
		{"small model", 5, .1, false, 10},
		{"scaled model", 12, 2, false, 200},
		{"character enlargement", 12, .5, true, 60},
		{"editor limit", 12, 100, false, 2048},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 1, DisplayScale: 99}}
			mm := extra.NewMapManager()
			u := mm.AddUnitType(false, "hfoo", creatureUnitFields(c, "creature-1.mdx", "Fixture", tc.scale, 56, 56))
			assets := character.CreatureAssets{fixtureCreatureAssetKey(): {ModelPath: "creature-1.mdx", CollisionRadius: tc.radius, CharacterModel: tc.character}}
			if err := EnrichCreatureModelFields(context.Background(), root, mm, []PlacedUnit{{Creature: c, TypeCode: u.Code}}, assets); err != nil {
				t.Fatal(err)
			}
			fields := fieldValues(t, mm.UnitTypes[0].Data)
			if fields["ushu"] != "Shadow" {
				t.Fatal("missing SD blob image")
			}
			for _, id := range []string{"ushw", "ushh", "ushx", "ushy"} {
				want := tc.want
				if id == "ushx" || id == "ushy" {
					want /= 2
				}
				if math.Abs(float64(fields[id].(float32))-want) > 1e-5 {
					t.Fatalf("%s=%v, want %g", id, fields[id], want)
				}
				for _, field := range mm.UnitTypes[0].Data {
					if field.ID == id && field.Type != data.ModificationReal {
						t.Fatalf("%s has wrong object-data type", id)
					}
				}
			}
		})
	}
	if after, err := os.ReadFile(modelPath); err != nil || string(after) != string(original) {
		t.Fatal("shadow sizing modified the cached model")
	}
}

func TestShadowBodyApproximationIgnoresThinExtremitiesAndMeshOrientation(t *testing.T) {
	body := shadowBox(10, 20, 100)
	baseline := creatureShadowDiameter([]*mdx.Geoset{body})
	// A very long, thin tail and a tiny detail far above the head have little
	// surface area. Neither may turn the shadow into an enclosing-circle size.
	tail := mdx.NewGeoset()
	tail.Vertices = []float32{0, 0, 50, 500, 0, 50, 0, .001, 50, 0, 0, 10000, .001, 0, 10000, 0, .001, 10000}
	tail.Faces = []uint16{0, 1, 2, 3, 4, 5}
	if got := creatureShadowDiameter([]*mdx.Geoset{body, tail}); math.Abs(got-baseline) > .01 {
		t.Fatalf("thin extremities inflated body shadow: %g vs %g", got, baseline)
	}
	wide := shadowBox(120, 80, 40)
	want := 1.25 * 2 * math.Sqrt(120*80/math.Pi)
	if got := creatureShadowDiameter([]*mdx.Geoset{wide}); math.Abs(got-want) > 1e-6 {
		t.Fatalf("broad body ignored projected area: %g, want %g", got, want)
	}
	for i := 0; i < len(wide.Vertices); i += 3 {
		x, y := float64(wide.Vertices[i]), float64(wide.Vertices[i+1])
		wide.Vertices[i] = float32((x-y)/math.Sqrt2 + 1000)
		wide.Vertices[i+1] = float32((x+y)/math.Sqrt2 - 500)
		wide.Vertices[i+2] += 100
	}
	if got := creatureShadowDiameter([]*mdx.Geoset{wide}); math.Abs(got-want) > .001 {
		t.Fatalf("rotation/translation changed body size: %g, want %g", got, want)
	}
	broken := mdx.NewGeoset()
	broken.Vertices = []float32{float32(math.NaN()), 0, 0, 3, 4, 0}
	broken.Faces = []uint16{0, 1, 9}
	if got := creatureShadowDiameter([]*mdx.Geoset{broken}); got != 10 || creatureShadowDiameter(nil) != 0 {
		t.Fatal("invalid/empty geometry has no safe fallback")
	}
}

func shadowBox(width, depth, height float32) *mdx.Geoset {
	g := mdx.NewGeoset()
	g.Vertices = []float32{0, 0, 0, width, 0, 0, width, depth, 0, 0, depth, 0, 0, 0, height, width, 0, height, width, depth, height, 0, depth, height}
	g.Faces = []uint16{0, 2, 1, 0, 3, 2, 4, 5, 6, 4, 6, 7, 0, 1, 5, 0, 5, 4, 1, 2, 6, 1, 6, 5, 2, 3, 7, 2, 7, 6, 3, 0, 4, 3, 4, 7}
	return g
}

func TestEmptyNPCModelDoesNotInheritFootmanShadow(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "creature-1.mdx"), mdx.NewModel().SaveMdx(), 0600); err != nil {
		t.Fatal(err)
	}
	c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 1}}
	mm := extra.NewMapManager()
	u := mm.AddUnitType(false, "hfoo", creatureUnitFields(c, "creature-1.mdx", "Fixture", 1, 56, 56))
	assets := character.CreatureAssets{fixtureCreatureAssetKey(): {ModelPath: "creature-1.mdx", CollisionRadius: 12}}
	if err := EnrichCreatureModelFields(context.Background(), root, mm, []PlacedUnit{{Creature: c, TypeCode: u.Code}}, assets); err != nil {
		t.Fatal(err)
	}
	f := fieldValues(t, mm.UnitTypes[0].Data)
	if f["ushu"] != "" || f["ushw"] != float32(0) || f["ushh"] != float32(0) || f["ushx"] != float32(0) || f["ushy"] != float32(0) {
		t.Fatal("empty geometry inherited a shadow")
	}
}
