package mapexporter

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func fieldValues(t *testing.T, mods []data.Modification) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	for _, m := range mods {
		if _, exists := out[m.ID]; exists {
			t.Fatalf("duplicate WE field %s", m.ID)
		}
		out[m.ID] = m.Value
	}
	return out
}

func TestCreatureFieldsPreserveSourceStatsAndClearStockDefaults(t *testing.T) {
	weak := azerothcore.Creature{Template: azerothcore.CreatureTemplate{Entry: 1, Name: "Weak", MaxLevel: 1},
		Model:    azerothcore.CreatureTemplateModel{CreatureDisplayID: 1, DisplayScale: 1, BoundingRadius: .5, CombatReach: 1.5},
		Creature: azerothcore.CreatureRecord{CurHealth: 1},
		Gameplay: &azerothcore.CreatureGameplay{Type: 7, Ground: 1, RunSpeed: 1, Health: 100, DamageMin: 5, DamageMax: 6, AttackSeconds: 2}}
	strong := weak
	strong.Template.Entry = 2
	g := *weak.Gameplay
	g.Health = 10000000
	g.Mana = 200000
	g.Armor = 10000
	g.DamageMin = 20000
	g.DamageMax = 30000
	strong.Gameplay = &g
	a := fieldValues(t, creatureUnitFields(weak, "creature-1.mdx", "Weak", 1, 40, 40))
	b := fieldValues(t, creatureUnitFields(strong, "creature-2.mdx", "Strong", 1, 40, 40))
	if a["uhpm"] != 100 || b["uhpm"] != 10000000 || a["umpm"] != 0 || b["umpm"] != 200000 || b["umpi"] != 200000 || a["udef"] != 0 || b["udef"] != 50 {
		t.Fatalf("source resources/armor: weak=%v/%v/%v strong=%v/%v/%v", a["uhpm"], a["umpm"], a["udef"], b["uhpm"], b["umpm"], b["udef"])
	}
	for _, id := range []string{"uabi", "uhab", "upgr", "utra", "ures", "uupt", "ubui"} {
		if a[id] != "" {
			t.Fatalf("inherited command %s", id)
		}
	}
	if a["umdl"] != a["upor"] || a["uaen"] != 1 || a["ua2d"] != 0 || a["ua2b"] != 0 || a["umvs"] != 280 || a["ucol"] != float32(8) {
		t.Fatal("model, physics or inherited attacks wrong")
	}
	for i, f := range []map[string]interface{}{a, b} {
		minDamage := f["ua1b"].(int) + f["ua1d"].(int)
		maxDamage := f["ua1b"].(int) + f["ua1d"].(int)*f["ua1s"].(int)
		want := [][2]int{{5, 6}, {20000, 30000}}[i]
		if minDamage != want[0] || maxDamage != want[1] {
			t.Fatalf("damage endpoints: got %d..%d want %d..%d", minDamage, maxDamage, want[0], want[1])
		}
	}
	for _, id := range []string{"uico", "usnd"} {
		if _, exists := a[id]; exists {
			t.Fatalf("excluded field %s was overridden", id)
		}
	}
}

func TestCreatureArmorMatchesPhysicalEffectiveHealth(t *testing.T) {
	for _, tc := range []struct {
		name        string
		armor       float64
		level, want int
		// Independently calculated AC armor denominator at the level breakpoint.
		k float64
	}{
		{"unarmored", 0, 80, 0, 15232.5},
		{"negative", -100, 80, 0, 15232.5},
		{"level one", 100, 1, 3, 485},
		{"before breakpoint", 3234, 59, 10, 5415},
		{"after breakpoint", 3234, 60, 9, 5882.5},
		{"screenshot at 80", 3234, 80, 4, 15232.5},
		{"boss", 10643, 83, 11, 16635},
		{"cap", 1e9, 80, 50, 15232.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := azerothcore.Creature{Template: azerothcore.CreatureTemplate{MaxLevel: tc.level}, Gameplay: &azerothcore.CreatureGameplay{Health: 10000, Armor: tc.armor}}
			fields := fieldValues(t, creatureUnitFields(c, "x.mdx", "Armor", 1, 40, 40))
			got := fields["udef"].(int)
			if got != tc.want || fields["uhpm"] != 10000 {
				t.Fatalf("armor=%d want %d, HP=%v", got, tc.want, fields["uhpm"])
			}
			acReduction := math.Min(.75, math.Max(0, tc.armor)/(math.Max(0, tc.armor)+tc.k))
			acMultiplier := 1 / (1 - acReduction)
			wc3Multiplier := 1 + .06*float64(got)
			// Nearest integer WC3 armor introduces at most .03 EHP/HP error.
			if math.Abs(acMultiplier-wc3Multiplier) > .030001 {
				t.Fatalf("physical EHP multipliers: AC=%g WC3=%g", acMultiplier, wc3Multiplier)
			}
		})
	}
}

func TestCreatureFieldsRangedCritterFlightAndClassification(t *testing.T) {
	c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{DisplayScale: 1}, Gameplay: &azerothcore.CreatureGameplay{
		Type: 6, Health: 100, Mana: 100, Ground: 0, Flight: 1, FlightSpeed: 1, DamageMin: 1, DamageMax: 2, RangedMin: 10, RangedMax: 20, AttackSeconds: 2, RangedSeconds: 3},
		Equipment: &azerothcore.Equipment{Item3: &azerothcore.ItemTemplate{Class: 2, Subclass: 2}}}
	f := fieldValues(t, creatureUnitFields(c, "x.mdx", "Undead Archer", 1, 40, 40))
	if f["umvt"] != "fly" || f["utar"] != "air,organic" || f["utyp"] != "undead" || f["ua1w"] != "missile" || f["ua1t"] != "pierce" || f["ua1c"] != float32(3) || f["ua1m"] == "" {
		t.Fatal("source capabilities/ranged attack lost")
	}
	if f["ua1b"].(int)+f["ua1d"].(int) != 10 || f["ua1b"].(int)+f["ua1d"].(int)*f["ua1s"].(int) != 20 {
		t.Fatal("source ranged damage endpoints lost")
	}
	c.Equipment.Item1 = c.Equipment.Item3
	c.Equipment.Item3 = nil
	if creatureRangedWeapon(c) == nil {
		t.Fatal("main-hand bow was not recognized")
	}
	c.Equipment.Item3 = c.Equipment.Item1
	c.Equipment.Item1 = &azerothcore.ItemTemplate{Class: 2, Subclass: 7}
	if creatureRangedWeapon(c) != nil {
		t.Fatal("unused ranged equipment overrides main-hand melee animations")
	}
	c.Gameplay.Type = 8
	c.Gameplay.Flight = 0
	c.Gameplay.Ground = 1
	f = fieldValues(t, creatureUnitFields(c, "x.mdx", "Rabbit", 1, 40, 40))
	if f["uaen"] != 0 || f["ua1d"] != 0 || f["ua1b"] != 0 || f["uacq"] != float32(0) {
		t.Fatal("critter inherited combat")
	}
	c.Gameplay.Rooted = 1
	f = fieldValues(t, creatureUnitFields(c, "x.mdx", "Rooted", 1, 40, 40))
	if f["umvs"] != 0 || f["umvt"] != "" {
		t.Fatal("rooted actor moves")
	}
}

func TestCachedCreatureModelMetadataEnrichesWEFields(t *testing.T) {
	source := `Version { FormatVersion 800, }
Model "fixture" { BlendTime 150, }
Sequences 3 {
 Anim "Death" { Interval { 0, 5000 }, NonLooping, }
 Anim "Attack" { Interval { 6000, 8000 }, }
 Anim "Walk" { Interval { 9000, 10000 }, MoveSpeed 270, }
}
EventObject "SNDxAXXX" { ObjectId 0, EventTrack 1 { 6600, } }
Attachment "weapon" { ObjectId 1, AttachmentID 0, }
PivotPoints 2 { { 0, 0, 0 }, { 10, 20, 100 }, }
`
	raw, err := mdx.ConvertMdlToMdx(source)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "creature-42.mdx")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	p, err := readCreatureModelFields(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(p.Death-5.2) > 1e-5 || p.Attack != 2 || math.Abs(p.Release-.6) > 1e-5 || !p.AttackSound || p.WalkSpeed != 270 || p.Launch != [3]float64{10, 20, 100} {
		t.Fatalf("decoded metadata: %+v", p)
	}
	c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 42, DisplayScale: 1}}
	mm := extra.NewMapManager()
	ut := mm.AddUnitType(false, "hfoo", creatureUnitFields(c, "creature-42.mdx", "Fixture", 1, 40, 40))
	if err := enrichCreatureModelFields(context.Background(), dir, mm, []PlacedUnit{{Creature: c, TypeCode: ut.Code}}, nil); err != nil {
		t.Fatal(err)
	}
	f := fieldValues(t, mm.UnitTypes[0].Data)
	if f["udtm"] != float32(5.2) || f["udp1"] != float32(.6) || f["ucs1"] != "" || f["ulpz"] != float32(100) {
		t.Fatalf("model fields: %+v", f)
	}
	if f["ussc"] != float32(2*1.2*40/72) {
		t.Fatal("empty geometry must retain native selection sizing")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := readCreatureModelFields(ctx, path); err == nil {
		t.Fatal("metadata reader ignores cancellation")
	}
}
