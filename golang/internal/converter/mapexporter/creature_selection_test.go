package mapexporter

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
)

func TestSelectionCapsToScaledHorizontalMeshRadius(t *testing.T) {
	source := `Version { FormatVersion 800, }
Model "fixture" { BlendTime 150, BoundsRadius 99999, }
Geoset {
 Vertices 3 { { 3, 4, 0 }, { 0, 12, 10000 }, { -6, 8, -500 }, }
 Faces 1 3 { Triangles { { 0, 1, 2 }, } }
}
`
	raw, err := mdx.ConvertMdlToMdx(source)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "shared.mdx")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := readCreatureModelFields(context.Background(), path)
	if err != nil || p.HorizontalRadius != 12 {
		t.Fatalf("horizontal radius must include upper mesh XY but ignore Z/sphere: %+v, %v", p, err)
	}
	mm := extra.NewMapManager()
	var units []PlacedUnit
	var wants []float64
	for i, tc := range []struct{ nativeRadius, scale, want float64 }{
		{1.5, 2, 24.0 / 36}, // WoW radius 168 is capped to mesh radius 24.
		{.1, 2, 11.2 / 36},  // Smaller native circle must not grow.
		{1.5, .25, 3.0 / 36},
	} {
		c := azerothcore.Creature{Template: azerothcore.CreatureTemplate{Entry: i + 1}, Model: azerothcore.CreatureTemplateModel{DisplayScale: 1, SelectionRadius: &tc.nativeRadius}}
		ut := mm.AddUnitType(false, "hfoo", creatureUnitFields(c, "shared.mdx", "Fixture", tc.scale, 56*tc.scale, 56))
		units = append(units, PlacedUnit{Creature: c, TypeCode: ut.Code, ModelPath: "shared.mdx"})
		wants = append(wants, tc.want)
	}
	before := make([]map[string]interface{}, len(mm.UnitTypes))
	for i, ut := range mm.UnitTypes {
		before[i] = fieldValues(t, ut.Data)
	}
	if err := enrichCreatureModelFields(context.Background(), dir, mm, units, nil); err != nil {
		t.Fatal(err)
	}
	for i, want := range wants {
		f := fieldValues(t, mm.UnitTypes[i].Data)
		if math.Abs(float64(f["ussc"].(float32))-want) > 1e-6 {
			t.Fatalf("actor %d selection=%v, want %g", i, f["ussc"], want)
		}
		for _, id := range []string{"usca", "umvs"} {
			if f[id] != before[i][id] {
				t.Fatalf("selection cap changed %s", id)
			}
		}
	}
	if after, err := os.ReadFile(path); err != nil || string(after) != string(raw) {
		t.Fatal("selection enrichment changed the cached model")
	}
}

func TestHorizontalModelRadiusHandlesEmptyAndInvalidCoordinates(t *testing.T) {
	if horizontalModelRadius(nil) != 0 {
		t.Fatal("empty geometry must not impose a cap")
	}
	g := mdx.NewGeoset()
	g.Vertices = []float32{float32(math.NaN()), 2, 0, 1, float32(math.Inf(1)), 0, -3, -4, 100000}
	if horizontalModelRadius([]*mdx.Geoset{g}) != 5 {
		t.Fatal("horizontal radius includes invalid XY or height")
	}
}

func TestSelectionScaleConvertsNativeRadiusOnceAndIgnoresCollision(t *testing.T) {
	radius := 1.5
	c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{DisplayScale: 4, BoundingRadius: 99, SelectionRadius: &radius}}
	// 56 raw model units/yard * .25 map fit * .5 export unit size.
	if got := creatureSelectionScale(c, 56*.25*.5); math.Abs(got-21.0/72) > 1e-9 {
		t.Fatalf("native scale applied twice: %g", got)
	}
	if got := creatureSelectionScale(c, 56*.25); math.Abs(got-42.0/72) > 1e-9 {
		t.Fatalf("unit export scale ignored: %g", got)
	}
	c.Model.DisplayScale = .01
	c.Model.BoundingRadius = .01
	if got := creatureSelectionScale(c, 56*.25*.5); math.Abs(got-21.0/72) > 1e-9 {
		t.Fatal("circle depends on collision or double-applied native scale")
	}
	fields := fieldValues(t, creatureUnitFields(c, "x.mdx", "Selection", 1, 56*.25*.5, 56*.25))
	if math.Abs(float64(fields["ussc"].(float32))-21.0/72) > 1e-6 {
		t.Fatalf("WE selection field uses old .5 floor: %v", fields["ussc"])
	}
	if math.Abs(creatureSelectionScale(azerothcore.Creature{}, 36)-1.2) > 1e-9 {
		t.Fatal("missing-box client fallback changed")
	}
}

type selectionTestClient struct {
	client.Client
	calls   int
	options casc.NpcSelectionOptions
}

func (c *selectionTestClient) ResolveNpcSelection(ctx context.Context, id int, o casc.NpcSelectionOptions) (float64, error) {
	c.calls++
	c.options = o
	return 1.25, nil
}

func TestPrepareSelectionDeduplicatesIdenticalNPCsAndRecordsSourceRadius(t *testing.T) {
	m := common.NewWowObjectManager(config.DefaultConfig(), nil, nil)
	root := &common.WowObject{Type: common.WowObjectADT}
	for range 3 {
		root.Children = append(root.Children, &common.WowObject{Type: common.WowObjectUnit, Creature: &azerothcore.Creature{
			Template: azerothcore.CreatureTemplate{MaxLevel: 30}, Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 42, DisplayScale: .75}, Gameplay: &azerothcore.CreatureGameplay{Family: 2}}})
	}
	m.Roots = []*common.WowObject{root}
	resolver := &selectionTestClient{}
	e := &MapExporter{WowObjectManager: m, wowClient: resolver}
	if err := e.prepareCreatureSelection(context.Background()); err != nil {
		t.Fatal(err)
	}
	if resolver.calls != 1 || resolver.options != (casc.NpcSelectionOptions{Family: 2, Level: 30, ObjectScale: .75}) {
		t.Fatalf("lookup inputs/cache: %d %+v", resolver.calls, resolver.options)
	}
	for _, obj := range root.Children {
		c := common.ObjectCreature(obj)
		if c.Model.SelectionRadius == nil || *c.Model.SelectionRadius != 1.25 {
			t.Fatal("source radius not retained")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.prepareCreatureSelection(ctx); err == nil {
		t.Fatal("ignored cancellation")
	}
}
