package mapexporter

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
	"github.com/pqhuy98/wow-converter/internal/converter/character"
	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func fixtureCreatureAssetKey() string {
	raw, _ := json.Marshal(character.Character{Base: character.DisplayRef(1), AttackTag: character.GuessAttackTag(0, 0), InGameMovespeed: 270})
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
	applyCreatureAssets(mm, units, assets, nil)
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
	applyCreatureAssets(mm, units, assets, map[string]string{"npcs/shared-wolf.mdx": icon})
	if fieldValues(t, mm.UnitTypes[0].Data)["uico"] != `ReplaceableTextures\CommandButtons\BTN_shared-wolf.blp` || units[2].ModelPath != "npcs/shared-wolf.mdx" {
		t.Fatal("shared icon/model audit missing")
	}
}

func TestDestructibleNPCsSkipIconGeneration(t *testing.T) {
	for _, force := range []bool{false, true} {
		root := t.TempDir()
		assets := character.CreatureAssets{"test": {ModelPath: "npcs/not-exported-yet.mdx"}}
		icons, err := renderCreatureIcons(context.Background(), root, assets, true, force)
		if err != nil || len(icons) != 0 {
			t.Fatalf("destructibles should skip icons: %v, %v", icons, err)
		}
		files, err := os.ReadDir(root)
		if err != nil || len(files) != 0 {
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
		if _, err := renderCreatureIcons(context.Background(), root, assets, true, force); err != nil {
			t.Fatal(err)
		}
		if raw, err := os.ReadFile(cache); err != nil || string(raw) != original {
			t.Fatal("destructible mode changed an existing unit icon cache")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := renderCreatureIcons(ctx, t.TempDir(), nil, true, false); !errors.Is(err, context.Canceled) {
		t.Fatal("destructible mode must preserve job cancellation")
	}
}
