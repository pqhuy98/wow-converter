package creature

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestCreatureCollisionSourcePriorityFallbackAndScale(t *testing.T) {
	for _, version := range []int{800, 1000} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			root := t.TempDir()
			model := mdx.NewModel()
			model.Version = version
			mesh := mdx.NewGeoset()
			// The upper-body/weapon vertices must not inflate the lower-body footprint.
			mesh.Vertices = []float32{12, 5, 0, -4, -3, 20, 100, 0, 70, -200, 0, 100}
			model.Geosets = append(model.Geosets, mesh)
			path := filepath.Join(root, "creature-1.mdx")
			original := model.SaveMdx()
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			profile, err := readCreatureModelFields(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if profile.LowerBodyRadius != 13 {
				t.Fatalf("lower body radius=%g", profile.LowerBodyRadius)
			}
			creature := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 1, DisplayScale: 9, BoundingRadius: 999, SelectionRadius: new(float64)}}
			for _, source := range []struct{ radius, scale, want float64 }{{0, 2, 26}, {20, 2, 40}, {20, .5, 10}, {20, .1, 8}, {20, 100, 1024}} {
				mm := extra.NewMapManager()
				unit := mm.AddUnitType(false, "hfoo", creatureUnitFields(creature, "creature-1.mdx", "Fixture", source.scale, 56, 56))
				assets := character.CreatureAssets{fixtureCreatureAssetKey(): {ModelPath: "creature-1.mdx", CollisionRadius: source.radius}}
				if err := EnrichCreatureModelFields(context.Background(), root, mm, []PlacedUnit{{Creature: creature, TypeCode: unit.Code}}, assets); err != nil {
					t.Fatal(err)
				}
				fields := fieldValues(t, mm.UnitTypes[0].Data)
				if got := fields["ucol"].(float32); math.Abs(float64(got)-source.want) > 1e-5 {
					t.Fatalf("source %g scale %g: got %g want %g", source.radius, source.scale, got, source.want)
				}
				if fields["usca"].(float32) != float32(source.scale) {
					t.Fatal("collision changed model scale")
				}
				if fields["ussc"] == nil {
					t.Fatal("selection circle removed")
				}
			}
			if after, err := os.ReadFile(path); err != nil || string(after) != string(original) {
				t.Fatal("cached model was modified")
			}
		})
	}
}

func TestCharacterNPCEnlargementChangesOnlyUnitObjectSizing(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "creature-1.mdx")
	original := mdx.NewModel().SaveMdx()
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 1, DisplayScale: 1.5}}
	for _, scale := range []float64{.5, 1, 2.5} {
		var baseline map[string]interface{}
		for _, isCharacter := range []bool{false, true} {
			mm := extra.NewMapManager()
			unit := mm.AddUnitType(false, "hfoo", creatureUnitFields(c, "creature-1.mdx", "Fixture", scale, 56, 56))
			assets := character.CreatureAssets{fixtureCreatureAssetKey(): {ModelPath: "creature-1.mdx", CharacterModel: isCharacter, CollisionRadius: 12}}
			if err := EnrichCreatureModelFields(context.Background(), root, mm, []PlacedUnit{{Creature: c, TypeCode: unit.Code}}, assets); err != nil {
				t.Fatal(err)
			}
			fields := fieldValues(t, mm.UnitTypes[0].Data)
			if !isCharacter {
				baseline = fields
				continue
			}
			if fields["usca"] != float32(scale*1.2) {
				t.Fatalf("character scale = %v, want %v", fields["usca"], scale*1.2)
			}
			for key, value := range baseline {
				if key == "ucol" || key == "ussc" || key == "ushw" || key == "ushh" || key == "ushx" || key == "ushy" {
					want := float64(value.(float32)) * 1.2
					if math.Abs(float64(fields[key].(float32))-want) > 1e-5 {
						t.Fatalf("character %s = %v, want %v", key, fields[key], want)
					}
				} else if key != "usca" && fields[key] != value {
					t.Fatalf("character multiplier changed %s", key)
				}
			}
		}
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(original) {
		t.Fatal("character multiplier changed model bytes")
	}
}

func TestLowerBodyCollisionHandlesEmptyAndInvalidGeometry(t *testing.T) {
	if lowerBodyCollisionRadius(nil) != 0 {
		t.Fatal("empty geometry has a footprint")
	}
	mesh := mdx.NewGeoset()
	mesh.Vertices = []float32{float32(math.NaN()), 0, 0, 1, 0, float32(math.Inf(1)), 3, 4, 0}
	if got := lowerBodyCollisionRadius([]*mdx.Geoset{mesh}); got != 5 {
		t.Fatalf("bad vertices affected footprint: %g", got)
	}
	// Planar bodies still use their actual horizontal footprint.
	mesh.Vertices = []float32{3, 4, 0, -3, -4, 0}
	if got := lowerBodyCollisionRadius([]*mdx.Geoset{mesh}); got != 5 {
		t.Fatalf("planar footprint=%g", got)
	}
}
