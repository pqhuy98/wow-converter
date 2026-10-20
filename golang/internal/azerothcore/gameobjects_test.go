package azerothcore

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestGameObjectTileQueryPreservesOverlapsAndBoundaries(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "world.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE gameobject(guid INTEGER,id INTEGER,map INTEGER,position_x REAL,position_y REAL,position_z REAL,orientation REAL,rotation0 REAL,rotation1 REAL,rotation2 REAL,rotation3 REAL,spawnMask INTEGER,phaseMask INTEGER,state INTEGER,animprogress INTEGER);
 CREATE TABLE gameobject_template(entry INTEGER,type INTEGER,displayId INTEGER,name TEXT,size REAL);
 CREATE TABLE gameobject_addon(guid INTEGER,invisibilityType INTEGER,invisibilityValue INTEGER);
 CREATE TABLE game_event_gameobject(eventEntry INTEGER,guid INTEGER,PRIMARY KEY(guid,eventEntry));
 INSERT INTO gameobject_addon VALUES(150349,0,0),(150345,9,1000);
 INSERT INTO gameobject_template VALUES(201613,0,9200,'Orange gate',1),(201614,0,9201,'Green gate',1),(201616,10,9202,'Valve',5.25),(1,0,0,'Invisible',1);
 INSERT INTO gameobject VALUES(150319,201613,631,4357.06,3071.33,354.362,-1.57,0,0,1,0,15,1,0,100),(150349,201614,631,4357.06,3071.33,354.362,-1.57,0,0,1,0,15,1,0,100),(150345,201616,631,4280.84,3090.88,362.335,1.57,0,0,1,0,15,-1,1,100),(9,1,631,4300,3000,350,0,0,0,0,1,15,1,1,100);`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := queryGameObjects(context.Background(), db, 631, [2]int{26, 23})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d spawns, want 4", len(got))
	}
	if got[2].Scale != 5.25 || got[2].PhaseMask != 0xffffffff || got[1].Rotation != [4]float64{0, 0, 1, 0} {
		t.Fatalf("metadata lost: %+v", got)
	}
	if got[1].UseQuaternion || !got[3].UseQuaternion || got[2].UseQuaternion {
		t.Fatal("rotation policy does not match GameObject::Create")
	}
	const boundary = (32 - 23) * (1600.0 / 3)
	if _, err := db.Exec(`UPDATE gameobject SET position_x=? WHERE guid=150319`, boundary); err != nil {
		t.Fatal(err)
	}
	current, err := queryGameObjects(context.Background(), db, 631, [2]int{26, 23})
	if err != nil {
		t.Fatal(err)
	}
	adjacent, err := queryGameObjects(context.Background(), db, 631, [2]int{26, 22})
	if err != nil {
		t.Fatal(err)
	}
	if len(current) != 3 || len(adjacent) != 1 || adjacent[0].GUID != 150319 {
		t.Fatalf("boundary duplication: %v %v", current, adjacent)
	}
	// Event-only invisible barriers must not become permanent map obstacles.
	// Negative links are present while their event is inactive. EXISTS also
	// prevents duplicate placements for spawns assigned to multiple events.
	_, err = db.Exec(`INSERT INTO gameobject VALUES
 (29600,1,631,4300,3000,350,0,0,0,0,1,15,1,1,100),
 (10,1,631,4300,3000,350,0,0,0,0,1,15,1,1,100),
 (11,1,631,4300,3000,350,0,0,0,0,1,15,1,1,100);
 INSERT INTO game_event_gameobject VALUES(31,29600),(-31,10),(31,11),(-12,11);`)
	if err != nil {
		t.Fatal(err)
	}
	base, err := queryGameObjects(context.Background(), db, 631, [2]int{26, 23})
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != 5 || base[1].GUID != 10 || base[2].GUID != 11 {
		t.Fatalf("base world must omit positive-only event spawns and retain negative links once: %+v", base)
	}
}
