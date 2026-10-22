package reportshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFocusedTerrainChecksPreserveOtherRegions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checks.json")
	previous := []map[string]any{{"region": "Azeroth", "focus": "8K ADT", "passed": true}, {"region": "Voidstorm", "passed": false}, {"region": "Northrend", "passed": true}}
	if err := saveTerrainChecks(path, "", previous); err != nil {
		t.Fatal(err)
	}
	updated := []map[string]any{{"region": "Voidstorm", "passed": true}, {"region": "Voidstorm", "focus": "8K ADT", "passed": true}}
	// Repeated focused runs must replace stale failures without duplicating rows
	// or discarding the other regions' overview and full-resolution evidence.
	for run := 0; run < 2; run++ {
		if err := saveTerrainChecks(path, "Voidstorm", updated); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var got []map[string]any
		if err = json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		want := []map[string]any{previous[0], previous[2], updated[0], updated[1]}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("focused checks lost or duplicated evidence: %v", got)
		}
	}
	if err := os.WriteFile(path, []byte("corrupt"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := saveTerrainChecks(path, "Voidstorm", updated); err == nil {
		t.Fatal("corrupted existing evidence was silently overwritten")
	}
}
