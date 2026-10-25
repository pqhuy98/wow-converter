package creature

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func fixtureCreatureAssetKey(gameplay ...*azerothcore.CreatureGameplay) string {
	var source *azerothcore.CreatureGameplay
	if len(gameplay) > 0 {
		source = gameplay[0]
	}
	raw, _ := json.Marshal(struct {
		character.Character
		Gameplay *azerothcore.CreatureGameplay `json:"gameplay,omitempty"`
	}{character.Character{Base: character.DisplayRef(1), AttackTag: character.GuessAttackTag(0, 0), InGameMovespeed: 270}, source})
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func TestCreatureAssetReferencesAndIconFailureFallback(t *testing.T) {
	c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 1}}
	mm := extra.NewMapManager()
	u := mm.AddUnitType(false, "hfoo", []data.Modification{
		{ID: "umdl", Type: data.ModificationString, Value: "creature-1.mdx"},
		{ID: "upor", Type: data.ModificationString, Value: "creature-1.mdx"},
	})
	d := mm.AddDoodadType([]data.Modification{{ID: "bfil", Type: data.ModificationString, Value: "old.mdx"}, {ID: "bgpm", Type: data.ModificationString, Value: "creature-1.mdx"}}, true)
	units := []PlacedUnit{{Creature: c, TypeCode: u.Code}, {Creature: c, TypeCode: d.Code}, {Creature: c, ExcludedReason: "height"}}
	// Obtain the public lookup key without duplicating the character export-input schema.
	assets := character.CreatureAssets{}
	// fixtureCreatureAsset uses the same serialized Character identity as the catalog.
	assets[fixtureCreatureAssetKey()] = &character.CreatureAsset{ModelPath: "npcs/shared-wolf.mdx"}
	ApplyCreatureAssets(mm, units, assets, nil)
	if fieldValues(t, mm.UnitTypes[0].Data)["uico"] != `ReplaceableTextures\CommandButtons\BTNFootman.blp` {
		t.Fatal("icon failure must retain Footman")
	}
	if fieldValues(t, mm.UnitTypes[0].Data)["umdl"] != `npcs\shared-wolf.mdx` || fieldValues(t, mm.DestructibleTypes[0].Data)["bfil"] != `npcs\shared-wolf.mdx` {
		t.Fatal("unit/destructible did not share model")
	}
	if fieldValues(t, mm.UnitTypes[0].Data)["upor"] != `npcs\shared-wolf.mdx` || fieldValues(t, mm.DestructibleTypes[0].Data)["bgpm"] != `npcs\shared-wolf.mdx` {
		t.Fatal("unit and destructible portraits retained legacy model paths")
	}
	icon := "ReplaceableTextures/CommandButtons/BTN_shared-wolf.blp"
	ApplyCreatureAssets(mm, units, assets, map[string]string{"npcs/shared-wolf.mdx": icon})
	if fieldValues(t, mm.UnitTypes[0].Data)["uico"] != `ReplaceableTextures\CommandButtons\BTN_shared-wolf.blp` || units[2].ModelPath != "npcs/shared-wolf.mdx" {
		t.Fatal("shared icon/model audit missing")
	}
}

func TestDestructibleNPCsSkipIconGeneration(t *testing.T) {
	for _, force := range []bool{false, true} {
		root := t.TempDir()
		cfg := config.DefaultConfig()
		cfg.OverrideModels = force
		exported, err := ExportCreatureAssets(context.Background(), nil, root, cfg, nil, 1, true, nil)
		if err != nil || len(exported.Icons) != 0 {
			t.Fatalf("destructibles should skip icons: %v, %v", exported.Icons, err)
		}
		files, err := os.ReadDir(root)
		if err != nil || len(files) != 1 || files[0].Name() != "npcs" {
			t.Fatalf("destructible mode created icon/cache files: %v, %v", files, err)
		}
		cacheDir := filepath.Join(root, "npcs")
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			t.Fatal(err)
		}
		cache := filepath.Join(cacheDir, "icons.json")
		const original = `{"existing-unit.mdx":"keep cached icon"}`
		if err := os.WriteFile(cache, []byte(original), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := ExportCreatureAssets(context.Background(), nil, root, cfg, nil, 1, true, nil); err != nil {
			t.Fatal(err)
		}
		if raw, err := os.ReadFile(cache); err != nil || string(raw) != original {
			t.Fatal("destructible mode changed an existing unit icon cache")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ExportCreatureAssets(ctx, nil, t.TempDir(), config.DefaultConfig(), nil, 1, true, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("destructible mode must preserve job cancellation")
	}
}

func TestStandReadyCombatEligibilityIgnoresFaction(t *testing.T) {
	for _, tc := range []struct {
		name     string
		gameplay *azerothcore.CreatureGameplay
		want     bool
	}{
		{"Defias Thug", &azerothcore.CreatureGameplay{Type: 7, Faction: 7, DamageMax: 4.973}, true},
		{"friendly guard", &azerothcore.CreatureGameplay{Type: 7, Faction: 12, DamageMax: 10}, true},
		{"hostile fighter", &azerothcore.CreatureGameplay{Type: 7, Faction: 17, DamageMax: 10}, true},
		{"vendor with attacks", &azerothcore.CreatureGameplay{Type: 7, NPCFlags: 128, DamageMax: 10}, true},
		{"ranged fighter", &azerothcore.CreatureGameplay{Type: 7, RangedMax: 10}, true},
		{"pacified", &azerothcore.CreatureGameplay{Type: 7, UnitFlags: 0x00020000, DamageMax: 10}, false},
		{"critter", &azerothcore.CreatureGameplay{Type: 8, DamageMax: 10}, false},
		{"noncombat pet", &azerothcore.CreatureGameplay{Type: 12, DamageMax: 10}, false},
		{"zero damage", &azerothcore.CreatureGameplay{Type: 7}, false},
		{"legacy gameplay fallback", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := azerothcore.Creature{Gameplay: tc.gameplay}
			if creatureCanAttack(c) != tc.want {
				t.Fatal("wrong combat eligibility")
			}
			fields := fieldValues(t, creatureUnitFields(c, "fixture.mdx", tc.name, 1, 56, 56))
			if (fields["uaen"] != 0) != tc.want {
				t.Fatal("idle policy disagrees with exported attacks")
			}
		})
	}
	combat := azerothcore.Creature{Gameplay: &azerothcore.CreatureGameplay{Faction: 7, DamageMax: 5}}
	noncombat := combat
	noncombat.Gameplay = &azerothcore.CreatureGameplay{Faction: 7}
	if creatureUnitKey(combat) == creatureUnitKey(noncombat) {
		t.Fatal("combat/noncombat variants must not share a unit type")
	}
}

func TestCreatureRaceUsesResolvedSourceWithoutChangingCampaignOrClassification(t *testing.T) {
	for _, tc := range []struct {
		name               string
		race, creatureType int
		want               string
	}{
		{"human", 1, 7, "human"}, {"dwarf", 3, 7, "human"},
		{"gnome", 7, 7, "human"}, {"blood elf", 10, 7, "human"},
		{"orc", 2, 7, "orc"}, {"tauren", 6, 7, "orc"},
		{"troll", 8, 7, "orc"}, {"goblin", 9, 7, "orc"},
		{"night elf", 4, 7, "nightelf"}, {"forsaken", 5, 7, "undead"},
		{"human zombie", 1, 6, "undead"}, {"demon", 4, 3, "demon"},
		{"critter", 0, 8, "critters"}, {"beast", 0, 1, "creeps"},
		{"unknown race", 999, 7, "other"}, {"draenei", 11, 7, "other"},
		{"naga", 13, 7, "naga"}, {"undead naga", 13, 6, "undead"},
		{"broken", 14, 7, "other"}, {"vrykul", 16, 7, "other"},
		{"tuskarr", 17, 7, "other"}, {"pandaren", 24, 7, "other"},
		{"fel orc", 12, 7, "orc"}, {"forest troll", 18, 7, "orc"},
		{"taunka", 19, 7, "orc"}, {"ice troll", 21, 7, "orc"},
		{"skeleton", 15, 7, "undead"}, {"northrend skeleton", 20, 7, "undead"},
		{"noncombat pet", 0, 12, "critters"}, {"elemental", 0, 4, "other"},
		{"mechanical", 0, 9, "other"}, {"totem", 0, 11, "other"},
		{"missing race", 0, 7, "creeps"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 1}, Gameplay: &azerothcore.CreatureGameplay{Type: tc.creatureType}}
			mm := extra.NewMapManager()
			u := mm.AddUnitType(false, "hfoo", creatureUnitFields(c, "x.mdx", tc.name, 1, 56, 56))
			before := fieldValues(t, u.Data)
			assets := character.CreatureAssets{fixtureCreatureAssetKey(c.Gameplay): {ModelPath: "npcs/shared.mdx", WowRace: tc.race}}
			ApplyCreatureAssets(mm, []PlacedUnit{{Creature: c, TypeCode: u.Code}}, assets, nil)
			got := fieldValues(t, mm.UnitTypes[0].Data)
			if got["urac"] != tc.want || got["ucam"] != 1 || got["utyp"] != before["utyp"] {
				t.Fatalf("race/campaign/classification = %v/%v/%v", got["urac"], got["ucam"], got["utyp"])
			}
		})
	}
}

func TestCreatureCommonerFallbackRequiresExplicitHumanoidCivilian(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		race, creatureType, flags int
		want                      string
	}{
		{"human civilian", 1, 7, 2, "human"},
		{"dwarf civilian", 3, 7, 2, "human"},
		{"orc civilian", 2, 7, 2, "orc"},
		{"night elf civilian", 4, 7, 2, "nightelf"},
		{"draenei civilian", 11, 7, 2, "other"},
		{"civilian without character race", 0, 7, 2, "commoner"},
		{"unflagged human", 1, 7, 0, "human"},
		{"civilian beast", 0, 1, 2, "creeps"},
		{"undead civilian", 1, 6, 2, "undead"},
		{"forsaken civilian", 5, 7, 2, "undead"},
		{"naga civilian", 13, 7, 2, "naga"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := azerothcore.Creature{Template: azerothcore.CreatureTemplate{FlagsExtra: tc.flags}, Gameplay: &azerothcore.CreatureGameplay{Type: tc.creatureType}}
			if got := creatureRace(c, tc.race); got != tc.want {
				t.Fatalf("race = %q, want %q", got, tc.want)
			}
		})
	}
}
