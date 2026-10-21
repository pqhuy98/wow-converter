package azerothcore

import (
	"database/sql"
	"math"
	"path/filepath"
	"testing"
)

func TestCreatureGameplayUsesMaximumStatsAndSpawnMovementOverrides(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "world.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
 CREATE TABLE creature(guid INTEGER PRIMARY KEY,id1 INTEGER);
 CREATE TABLE creature_template(entry INTEGER PRIMARY KEY,maxlevel INTEGER,unit_class INTEGER,type INTEGER,family INTEGER,rank INTEGER,faction INTEGER,npcflag INTEGER,unit_flags INTEGER,type_flags INTEGER,
 speed_walk REAL,speed_run REAL,speed_swim REAL,speed_flight REAL,detection_range REAL,HoverHeight REAL,RegenHealth INTEGER,dmgschool INTEGER,exp INTEGER,
 HealthModifier REAL,ManaModifier REAL,ArmorModifier REAL,DamageModifier REAL,BaseAttackTime INTEGER,RangeAttackTime INTEGER,BaseVariance REAL,RangeVariance REAL);
 CREATE TABLE creature_classlevelstats(level INTEGER,class INTEGER,basehp0 REAL,basehp1 REAL,basehp2 REAL,basemana REAL,basearmor REAL,attackpower REAL,rangedattackpower REAL,damage_base REAL,damage_exp1 REAL,damage_exp2 REAL);
 CREATE TABLE creature_template_movement(CreatureId INTEGER,Ground INTEGER,Swim INTEGER,Flight INTEGER,Rooted INTEGER);
 CREATE TABLE creature_movement_override(SpawnId INTEGER,Ground INTEGER,Swim INTEGER,Flight INTEGER,Rooted INTEGER);
 CREATE TABLE factiontemplate_dbc(ID INTEGER,FactionGroup INTEGER,FriendGroup INTEGER,EnemyGroup INTEGER);
 CREATE TABLE creature_model_info(DisplayID INTEGER,BoundingRadius REAL,CombatReach REAL);
 CREATE TABLE item_template(entry INTEGER,class INTEGER,subclass INTEGER,SoundOverrideSubclass INTEGER,delay INTEGER);
 INSERT INTO creature VALUES(1,80);
 INSERT INTO creature_template VALUES(80,4,1,7,0,0,14,0,0,0,1,1,1,1,20,1,1,0,0,2,1.5,.5,2,2000,3000,1,2);
 INSERT INTO creature_classlevelstats VALUES(4,1,100,200,300,40,50,28,14,10,20,30);
 INSERT INTO creature_template_movement VALUES(80,1,1,0,0);
 INSERT INTO creature_movement_override VALUES(1,0,0,1,NULL);
 INSERT INTO creature_model_info VALUES(42,.75,2);
 INSERT INTO item_template VALUES(11,2,2,-1,2000);
 `)
	if err != nil {
		t.Fatal(err)
	}
	creatures := []Creature{{Creature: CreatureRecord{GUID: 1, ID1: 80, CurHealth: 1, CurMana: 0}, Template: CreatureTemplate{Entry: 80, MaxLevel: 4},
		Model: CreatureTemplateModel{CreatureDisplayID: 42, DisplayScale: 1}, Equipment: &Equipment{Item3: &ItemTemplate{Entry: 11, InventoryType: 15}}}}
	if err := enrichCreatures(db, creatures); err != nil {
		t.Fatal(err)
	}
	c := creatures[0]
	g := c.Gameplay
	if g == nil || g.Health != 200 || g.Mana != 60 || g.Armor != 25 || g.DamageMin != 48 || g.DamageMax != 68 || g.RangedMin != 72 || g.RangedMax != 102 {
		t.Fatalf("maximum/combat stats: %+v", g)
	}
	if g.Ground != 0 || g.Swim != 0 || g.Flight != 1 || g.Rooted != 0 {
		t.Fatalf("spawn movement override: %+v", g)
	}
	if c.Model.BoundingRadius != .75 || c.Model.CombatReach != 2 || c.Equipment.Item3.Class != 2 || c.Equipment.Item3.Subclass != 2 || c.Equipment.Item3.InventoryType != 15 {
		t.Fatal("physical/equipment metadata lost")
	}
	if c.Creature.CurHealth != 1 || c.Creature.CurMana != 0 {
		t.Fatal("source spawn data was mutated")
	}
	if math.IsNaN(g.Health) {
		t.Fatal("invalid source stat")
	}
	// Enriched schema failures must be surfaced, not mistaken for zero creatures.
	if _, err := db.Exec("DROP TABLE creature_classlevelstats"); err != nil {
		t.Fatal(err)
	}
	if err := enrichCreatures(db, creatures); err == nil {
		t.Fatal("missing enriched source table silently ignored")
	}
}

func TestBundledCreatureGameplayForSelectedMaps(t *testing.T) {
	db, err := sql.Open("sqlite", resolveDatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, selection := range []struct {
		mapID int
		tile  [2]int
	}{{0, [2]int{32, 48}}, {609, [2]int{42, 28}}, {631, [2]int{26, 23}}} {
		creatures, err := queryCreatures(db, selection.mapID, selection.tile)
		if err != nil {
			t.Fatal(err)
		}
		if len(creatures) == 0 {
			t.Fatalf("empty representative source selection %+v", selection)
		}
		for _, c := range creatures {
			if c.Gameplay == nil || c.Gameplay.Health < 1 || c.Gameplay.AttackSeconds <= 0 {
				t.Fatalf("missing source gameplay: %s", c.Template.Name)
			}
		}
	}
}
