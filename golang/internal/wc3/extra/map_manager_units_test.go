package extra_test

import (
	"reflect"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
	"github.com/pqhuy98/wow-converter/internal/wc3/extra"
)

func TestSaveRepairsDuplicateUnitCreationIDsWithoutChangingPlacements(t *testing.T) {
	m := extra.NewMapManager()
	m.SetTerrain(data.Terrain{Tileset: "L", Map: data.MapSize{Width: 1, Height: 1}})
	for i, id := range []int{-1, 0, 0, 5, 5, 1} {
		m.Units = append(m.Units, extra.UnitInstance{Type: "hfoo", Unit: data.Unit{
			ID: id, Type: "hfoo", Skin: "hfoo", Player: i, Position: [3]float32{float32(i * 20), 40, 50},
			Scale: [3]float32{1, 1, 1}, Rotation: 1.2, Hitpoints: 100, Mana: 0, RandomItemSetPtr: -1,
		}})
	}
	before := append([]extra.UnitInstance(nil), m.Units...)
	dir := t.TempDir()
	for pass := 0; pass < 2; pass++ {
		if err := m.Save(dir); err != nil {
			t.Fatal(err)
		}
		saved := extra.NewMapManager()
		if err := saved.Load(dir); err != nil {
			t.Fatal(err)
		}
		if len(saved.Units) != len(before) {
			t.Fatal("unit count changed")
		}
		seen := map[int]bool{}
		for i, u := range saved.Units {
			if u.ID < 0 || seen[u.ID] {
				t.Fatalf("placement %d has invalid or duplicate creation ID %d", i, u.ID)
			}
			seen[u.ID] = true
			if u.ID != m.Units[i].ID {
				t.Fatal("manager and serialized IDs differ")
			}
			want := before[i].Unit
			want.ID = u.ID
			if !reflect.DeepEqual(u.Unit, want) {
				t.Fatalf("placement %d changed outside creation ID: %+v vs %+v", i, u.Unit, want)
			}
		}
		if saved.Units[1].ID != 0 || saved.Units[3].ID != 5 || saved.Units[5].ID != 1 {
			t.Fatal("existing valid creation IDs must remain stable")
		}
		if pass == 0 {
			before = append([]extra.UnitInstance(nil), m.Units...)
		}
	}
}
