package creature

import (
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestBundledCreatureVoices(t *testing.T) {
	voices, err := LoadCreatureVoices()
	if err != nil {
		t.Fatal(err)
	}
	if len(voices) != 24183 {
		t.Fatalf("unexpected Wrath display coverage: %d", len(voices))
	}
	// All skins/displays of a base model must use the same name-based choices.
	humans, rabbits := 0, 0
	for display, names := range voices {
		if names["Marshal McBride"] == "Footman" {
			humans++
			if names["Defias Thug"] != "Bandit" || names["Stormwind City Guard"] != "Footman" {
				t.Fatalf("shared human model lost role-specific voices: %d", display)
			}
		}
		if names["Rabbit"] == "Rabbit" {
			rabbits++
		}
	}
	if humans == 0 || rabbits == 0 {
		t.Fatal("missing known name/model pairs")
	}
}

const voiceFixture = `{"version":1,"pairs":4,"sounds":["","Footman","Bandit"],"models":[
{"path":"humanmale.m2","displays":[1,2],"voices":{"Guard":1,"Bandit":2,"Silent":0}},
{"path":"humanfemale.m2","displays":[3],"voices":{"Guard":0}}]}`

func TestCreatureVoicesKeepModelRoleAndSilenceSeparate(t *testing.T) {
	voices, err := decodeCreatureVoices(strings.NewReader(voiceFixture))
	if err != nil {
		t.Fatal(err)
	}
	mm := extra.NewMapManager()
	unrelated := mm.AddUnitType(false, "hfoo", []data.Modification{{ID: "usnd", Type: data.ModificationString, Value: "Peasant"}}).Code
	destructible := mm.AddDoodadType([]data.Modification{{ID: "bnam", Type: data.ModificationString, Value: "Guard"}}, true)
	var units []PlacedUnit
	for i, tc := range []struct {
		name    string
		display int
		flags   int
		want    string
		found   bool
	}{
		{"Guard", 1, 0, "Footman", true},
		{"Bandit", 2, 0, "Bandit", true}, // Same base model, different voice.
		{"Silent", 1, 0, "", true},
		{"Guard", 3, 0, "", true}, // Same name, different base model.
		{"Guard", 1, 128, "", true},
		{"Unknown", 1, 0, "", false},
		{"Guard", 999, 0, "", false},
	} {
		c := azerothcore.Creature{Template: azerothcore.CreatureTemplate{Entry: i + 1, Name: tc.name, FlagsExtra: tc.flags}, Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: tc.display}}
		if sound, found := voices.lookup(c); sound != tc.want || found != tc.found {
			t.Fatalf("lookup %v: %q/%v", tc, sound, found)
		}
		u := mm.AddUnitType(false, "hfoo", []data.Modification{
			{ID: "umdl", Type: data.ModificationString, Value: "npcs/shared.mdx"},
			{ID: "usnd", Type: data.ModificationString, Value: "OldVoice"},
		})
		units = append(units, PlacedUnit{Creature: c, TypeCode: u.Code})
	}
	// An excluded spawn must not overwrite the type chosen by an actual placement.
	units = append(units, PlacedUnit{Creature: azerothcore.Creature{}, ExcludedReason: "height"})
	for range 2 { // Applying again replaces the field instead of duplicating it.
		ApplyCreatureVoices(mm, units, voices)
	}
	want := []string{"Peasant", "Footman", "Bandit", "", "", "", "", ""}
	for i, u := range mm.UnitTypes {
		f := fieldValues(t, u.Data)
		if f["usnd"] != want[i] || (u.Code != unrelated && f["umdl"] != "npcs/shared.mdx") {
			t.Fatalf("unit %d: unexpected fields %v", i, f)
		}
		count := 0
		for _, field := range u.Data {
			if field.ID == "usnd" {
				count++
			}
		}
		if count != 1 {
			t.Fatal("duplicate voice fields")
		}
	}
	if len(destructible.Data) != 1 {
		t.Fatal("NPC destructible was modified")
	}
}

func TestCreatureVoiceResourceRejectsCorruption(t *testing.T) {
	for _, raw := range []string{
		strings.Replace(voiceFixture, `"version":1`, `"version":2`, 1),
		strings.Replace(voiceFixture, `"pairs":4`, `"pairs":5`, 1),
		strings.Replace(voiceFixture, `"Bandit":2`, `"Bandit":99`, 1),
		strings.Replace(voiceFixture, `"displays":[3]`, `"displays":[1]`, 1),
		voiceFixture + `{}`,
		voiceFixture[:len(voiceFixture)-1],
	} {
		if _, err := decodeCreatureVoices(strings.NewReader(raw)); err == nil {
			t.Fatal("accepted corrupt resource")
		}
	}
}
