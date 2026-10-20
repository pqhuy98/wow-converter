package azerothcore

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCreatureTileQueryKeepsBaseWorldQuestGivers(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "world.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE creature(
 guid INTEGER, id1 INTEGER, map INTEGER, position_x REAL, position_y REAL, position_z REAL,
 orientation REAL, phaseMask INTEGER, curhealth INTEGER, curmana INTEGER);
 CREATE TABLE creature_template(entry INTEGER, name TEXT, subname TEXT, maxlevel INTEGER, flags_extra INTEGER);
 CREATE TABLE creature_template_model(CreatureID INTEGER, Idx INTEGER, CreatureDisplayID INTEGER, DisplayScale REAL);
 CREATE TABLE creature_equip_template(CreatureID INTEGER, ID INTEGER, ItemID1 INTEGER, ItemID2 INTEGER, ItemID3 INTEGER);
 CREATE TABLE item_template(entry INTEGER, InventoryType INTEGER);
 CREATE TABLE game_event_creature(eventEntry INTEGER, guid INTEGER, PRIMARY KEY(guid, eventEntry));
 INSERT INTO creature_template VALUES
  (197,'Marshal McBride',NULL,20,2),
  (26012,'Arena Organizer','',70,0),
  (26747,'[DND] TAR Pedestal - Arena Organizer','',1,0),
  (1642,'Northshire Guard',NULL,5,0);
 INSERT INTO creature_template_model VALUES
  (197,0,1859,1),(26012,0,24023,1),(26747,0,24019,1),(1642,0,3167,1);
 INSERT INTO creature VALUES
  (79970,197,0,-8902.59,-162.606,82.0223,0,1,100,0),
  (96194,26012,0,-8918.51,-135.146,84.2136,0,1,100,0),
  (96190,26747,0,-8918.51,-135.146,80.5303,0,1,100,0),
  (79926,1642,0,-9047.58,-101.24,88.3071,0,1,100,0),
  (10,1642,0,-8900,-160,82,0,1,100,0),
  (11,1642,0,-8901,-161,82,0,1,100,0);
 INSERT INTO game_event_creature VALUES(31,96194),(31,96190),(-31,10),(31,11),(-12,11);`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := queryCreatures(db, 0, [2]int{32, 48})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]int{}
	for _, c := range got {
		names[c.Template.Name]++
	}
	if names["Marshal McBride"] != 1 || names["Northshire Guard"] != 3 {
		t.Fatalf("base-world NPCs: %+v", names)
	}
	if names["Arena Organizer"] != 0 || names["[DND] TAR Pedestal - Arena Organizer"] != 0 {
		t.Fatalf("Arena Tournament event NPCs leaked into the base world: %+v", names)
	}
	guards := map[int64]bool{}
	for _, c := range got {
		if c.Template.Entry == 1642 {
			guards[c.Creature.GUID] = true
		}
	}
	if !guards[79926] || !guards[10] || !guards[11] || len(guards) != 3 {
		t.Fatalf("base world must omit positive-only event spawns and retain negative links once: %v", guards)
	}
}
