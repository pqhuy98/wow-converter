package mapexporter

import (
	"context"
	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/common"
	"github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/client"
	"math"
	"testing"
)

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
