package character

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/azerothcore"
	"github.com/pqhuy98/wow-converter/internal/config"
	"github.com/pqhuy98/wow-converter/internal/formats/mdx"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

func TestCreatureAttackTagMatchesEquipment(t *testing.T) {
	for _, tt := range []struct {
		name      string
		equipment *azerothcore.Equipment
		want      string
	}{
		{name: "missing equipment", want: "Unarmed"},
		{name: "empty equipment", equipment: &azerothcore.Equipment{}, want: "Unarmed"},
		{name: "main hand", equipment: &azerothcore.Equipment{Item1: &azerothcore.ItemTemplate{InventoryType: InventoryWeapon}}, want: "1H"},
		{name: "off hand", equipment: &azerothcore.Equipment{Item2: &azerothcore.ItemTemplate{InventoryType: InventoryWeaponOffHand}}, want: "1H"},
		{name: "two handed", equipment: &azerothcore.Equipment{Item1: &azerothcore.ItemTemplate{InventoryType: InventoryTwoHanded}}, want: "2H"},
		{name: "bow", equipment: &azerothcore.Equipment{Item1: &azerothcore.ItemTemplate{InventoryType: InventoryRanged}}, want: "Bow"},
		{name: "rifle", equipment: &azerothcore.Equipment{Item1: &azerothcore.ItemTemplate{InventoryType: InventoryRangedRight}}, want: "Rifle"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ch := creatureCharacter(azerothcore.Creature{Equipment: tt.equipment})
			if string(ch.AttackTag) != tt.want {
				t.Fatalf("attack tag = %q, want %q", ch.AttackTag, tt.want)
			}
		})
	}
}

func TestCreatureAssetKeepsResolvedCharacterClassification(t *testing.T) {
	for _, isCharacter := range []bool{false, true} {
		model := 123
		c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 1}}
		_, assets, err := planCreatureExports(context.Background(), []azerothcore.Creature{c}, config.DefaultConfig(), "build", nil, func(azerothcore.Creature) (CharacterData, error) {
			meta := CharacterData{Model: &model}
			if isCharacter {
				meta.Character = &wowhead.CharacterMeta{}
			}
			return meta, nil
		})
		if err != nil || assets.For(c) == nil || assets.For(c).CharacterModel != isCharacter {
			t.Fatalf("character classification lost: %v, %+v", err, assets.For(c))
		}
	}
}

func TestCreatureAppearanceDedupBeforeConversion(t *testing.T) {
	model := 126487
	creatures := []azerothcore.Creature{}
	for i, id := range []int{604, 447, 380, 903} {
		creatures = append(creatures, azerothcore.Creature{Template: azerothcore.CreatureTemplate{Entry: i + 1, Name: []string{"Diseased Timber Wolf", "Diseased Young Wolf", "Gray Forest Wolf", "Mangy Wolf"}[i]}, Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: id}})
	}
	creatures = append(creatures, creatures[0])
	calls := 0
	jobs, assets, err := planCreatureExports(context.Background(), creatures, config.DefaultConfig(), "build", nil, func(c azerothcore.Creature) (CharacterData, error) {
		calls++
		return CharacterData{Model: &model, Textures: map[string]int{"1": 126498, "2": 126499}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || len(assets) != 4 || calls != 4 {
		t.Fatalf("jobs=%d inputs=%d resolutions=%d", len(jobs), len(assets), calls)
	}
	for _, c := range creatures {
		if a := assets.For(c); a != jobs[0].asset || a.ModelPath != "npcs/diseased-timber-wolf.mdx" {
			t.Fatalf("unshared appearance: %+v", a)
		}
	}
}

func TestCreatureNamesCollideOnlyForDifferentAppearances(t *testing.T) {
	model := 1
	creatures := []azerothcore.Creature{{Template: azerothcore.CreatureTemplate{Name: "Same! Name"}, Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 20}}, {Template: azerothcore.CreatureTemplate{Name: "same name"}, Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 30}}}
	jobs, _, err := planCreatureExports(context.Background(), creatures, config.DefaultConfig(), "build", nil, func(c azerothcore.Creature) (CharacterData, error) {
		return CharacterData{Model: &model, Textures: map[string]int{"1": c.Model.CreatureDisplayID}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || jobs[0].asset.ModelPath != "npcs/same-name_20.mdx" || jobs[1].asset.ModelPath != "npcs/same-name_30.mdx" {
		t.Fatalf("unstable collision paths: %+v", jobs)
	}
	if creatureSlug("../ King\tMograine's (Death_Knight)!") != "king-mograine-s-death-knight" || creatureSlug("???") != "npc" {
		t.Fatal("unsafe model name normalization")
	}
}

func TestCreatureAppearanceKeyIncludesVisualSettingsNotIdentity(t *testing.T) {
	id := 1
	meta := CharacterData{Model: &id, Textures: map[string]int{"1": 2}}
	cfg := config.DefaultConfig()
	ch := Character{Base: DisplayRef(1), InGameMovespeed: 270}
	key, _ := creatureAppearanceKey(meta, ch, cfg, "a")
	ch.Base = DisplayRef(999)
	cfg.ExportAssetDir = "elsewhere"
	cfg.OverrideModels = !cfg.OverrideModels
	same, _ := creatureAppearanceKey(meta, ch, cfg, "a")
	if same != key {
		t.Fatal("identity/output cache switches changed appearance")
	}
	capped := cfg
	capped.MaxTextureSize = 512
	cappedKey, _ := creatureAppearanceKey(meta, ch, capped, "a")
	if cappedKey == key {
		t.Fatal("texture cap missing from appearance cache key")
	}
	variants := []CharacterData{{Model: &id, Textures: map[string]int{"1": 3}}, {Model: &id, Textures: meta.Textures, Creature: &CreatureMeta{CreatureGeosetData: []GeosetEntry{{GeosetIndex: 1, GeosetValue: 2}}}}, {Model: &id, Textures: meta.Textures, Scale: 2}}
	for _, variant := range variants {
		k, _ := creatureAppearanceKey(variant, ch, cfg, "a")
		if k == key {
			t.Fatal("different visual configuration collapsed")
		}
	}
	otherBuild, _ := creatureAppearanceKey(meta, ch, cfg, "b")
	if key == otherBuild {
		t.Fatal("build missing from fingerprint")
	}
	ch.AttachItems = map[string]AttachItem{"1": {Path: WowheadRef("item=1")}}
	withItem, _ := creatureAppearanceKey(meta, ch, cfg, "a")
	if withItem == key {
		t.Fatal("equipment missing from fingerprint")
	}
}

func TestCreaturePlanRefreshesRaceFromResolvedMetadata(t *testing.T) {
	id := 1
	c := azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 1}}
	jobs, assets, err := planCreatureExports(context.Background(), []azerothcore.Creature{c}, config.DefaultConfig(), "build", nil, func(azerothcore.Creature) (CharacterData, error) {
		return CharacterData{Model: &id, Character: &wowhead.CharacterMeta{Race: 1}}, nil
	})
	if err != nil || len(jobs) != 1 || assets.For(c) == nil || assets.For(c).WowRace != 1 {
		t.Fatalf("resolved character race missing: %v, %v", assets.For(c), err)
	}
}

func TestCreatureCacheReusesKnownAppearanceAndCleansOnlyOwnedModels(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "npcs"), 0755)
	raw, err := mdx.ConvertMdlToMdx(`Version { FormatVersion 1000, } Model "old" { BlendTime 150, }`)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "creature-604.mdx"), raw, 0644)
	os.WriteFile(filepath.Join(root, "npcs", "old-wolf.mdx"), raw, 0644)
	os.WriteFile(filepath.Join(root, "handmade.mdx"), raw, 0644)
	job := &creatureExport{creature: azerothcore.Creature{Model: azerothcore.CreatureTemplateModel{CreatureDisplayID: 604}}, asset: &CreatureAsset{ModelPath: "npcs/wolf.mdx", ConfigKey: "key"}}
	cfg := config.DefaultConfig()
	cfg.OverrideModels = false
	old := map[string]*CreatureAsset{"key": {ModelPath: "npcs/old-wolf.mdx"}}
	cleanup, err := stageCreatureCache(context.Background(), root, []*creatureExport{job}, cfg, old)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err = exportCreatureAppearance(context.Background(), job, root, cfg, nil); err != nil {
		t.Fatal(err)
	}
	cache := map[string]*CreatureAsset{"key": job.asset}
	if err = writeCreatureAssetCache(root, cache); err != nil {
		t.Fatal(err)
	}
	if readCreatureAssetCache(root)["key"].ModelPath != job.asset.ModelPath {
		t.Fatal("cache mapping lost")
	}
	if err = cleanCreatureAssets(root, nil, cache); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(root, "creature-604.mdx")); !os.IsNotExist(err) {
		t.Fatal("legacy root model retained")
	}
	if _, err = os.Stat(filepath.Join(root, "handmade.mdx")); err != nil {
		t.Fatal("unrelated model touched")
	}
	for _, path := range []string{"npcs/../outside.mdx", "../npcs/x.mdx", `npcs\x.mdx`} {
		if validCreatureAssetPath(path) {
			t.Fatalf("unsafe path %s", path)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err = exportCreatureAppearance(ctx, job, root, cfg, nil); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatal("ignored cancellation")
	}
}

func TestCachedCreatureFingerprintIgnoresNameAndPreservesRealDifferences(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "npcs"), 0755)
	var hashes []string
	for i, source := range []string{`Version { FormatVersion 1000, } Model "first" { BlendTime 150, } Textures 1 { Bitmap { Image "skin.blp", } }`, `Version { FormatVersion 1000, } Model "second" { BlendTime 150, } Textures 1 { Bitmap { Image "skin.blp", } }`, `Version { FormatVersion 1000, } Model "third" { BlendTime 200, } Textures 1 { Bitmap { Image "skin.blp", } }`} {
		raw, err := mdx.ConvertMdlToMdx(source)
		if err != nil {
			t.Fatal(err)
		}
		name := []string{"first", "second", "third"}[i]
		asset := &CreatureAsset{ModelPath: "npcs/" + name + ".mdx"}
		os.WriteFile(filepath.Join(root, filepath.FromSlash(asset.ModelPath)), raw, 0644)
		if err = fingerprintCreatureModel(context.Background(), root, asset); err != nil {
			t.Fatal(err)
		}
		if len(asset.Textures) != 1 || asset.Textures[0] != "skin.blp" {
			t.Fatal("texture dependencies not retained")
		}
		hashes = append(hashes, asset.ContentKey)
	}
	if hashes[0] != hashes[1] || hashes[0] == hashes[2] {
		t.Fatal("cached model content fingerprint is wrong")
	}
}

func TestCreatureCacheStagesBeforeRenamingAndRejectsMissingTextures(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "npcs"), 0755)
	a, _ := mdx.ConvertMdlToMdx(`Version { FormatVersion 1000, } Model "a" { BlendTime 150, }`)
	b, _ := mdx.ConvertMdlToMdx(`Version { FormatVersion 1000, } Model "b" { BlendTime 250, }`)
	os.WriteFile(filepath.Join(root, "npcs", "a.mdx"), a, 0644)
	os.WriteFile(filepath.Join(root, "npcs", "b.mdx"), b, 0644)
	first := &creatureExport{asset: &CreatureAsset{ConfigKey: "a", ModelPath: "npcs/b.mdx"}}
	second := &creatureExport{asset: &CreatureAsset{ConfigKey: "b", ModelPath: "npcs/a.mdx"}}
	old := map[string]*CreatureAsset{"a": {ModelPath: "npcs/a.mdx"}, "b": {ModelPath: "npcs/b.mdx"}}
	cfg := config.DefaultConfig()
	cfg.OverrideModels = false
	cleanup, err := stageCreatureCache(context.Background(), root, []*creatureExport{first, second}, cfg, old)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, job := range []*creatureExport{first, second} {
		if err = exportCreatureAppearance(context.Background(), job, root, cfg, nil); err != nil {
			t.Fatal(err)
		}
	}
	actualA, _ := os.ReadFile(filepath.Join(root, "npcs", "a.mdx"))
	actualB, _ := os.ReadFile(filepath.Join(root, "npcs", "b.mdx"))
	if string(actualA) != string(b) || string(actualB) != string(a) {
		t.Fatal("cache renaming overwrote another appearance")
	}
	first.cachedPath = ""
	old["a"].Textures = []string{"missing.blp"}
	cleanup2, err := stageCreatureCache(context.Background(), root, []*creatureExport{first}, cfg, old)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup2()
	if first.cachedPath != "" {
		t.Fatal("reused model with missing texture")
	}
}
