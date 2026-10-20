package mapexporter

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/pqhuy98/wow-converter/internal/wc3/data"
)

func TestMapUnitCreationMatchesPlacementsAndReplacesStaleCalls(t *testing.T) {
	script := []byte("function main()\nInitGlobals()\nCreateAllUnits()\nInitCustomTriggers()\nend\n")
	units := []data.Unit{{Type: "sloc"}}
	for i := 0; i < 205; i++ {
		units = append(units, data.Unit{Type: "a001", Skin: "a002", Player: 3, Position: [3]float32{float32(i), -12.5, 42}, Rotation: 1.5707964,
			Scale: [3]float32{1, 1, 1}, Hitpoints: 75, Mana: 20, Color: 23, TargetAcquisition: 600})
	}
	got, err := injectUnitCreation(script, units)
	if err != nil {
		t.Fatal(err)
	}
	if count := bytes.Count(got, []byte("u = BlzCreateUnitWithSkin(")); count != 205 {
		t.Fatalf("created %d units, want 205", count)
	}
	for _, fragment := range []string{
		"Player(3), FourCC(\"a001\"), 0, -12.5, 90, FourCC(\"a002\")",
		"SetUnitColor(u, ConvertPlayerColor(23))", "UNIT_STATE_MAX_LIFE) * 75 / 100",
		"SetUnitState(u, UNIT_STATE_MANA, 20)", "SetUnitAcquireRange(u, 600)",
		"InitGlobals()\nWowConverterCreateUnits()\nInitCustomTriggers()",
	} {
		if !strings.Contains(string(got), fragment) {
			t.Fatalf("missing unit state/startup: %s", fragment)
		}
	}
	for i := 0; i < 3; i++ {
		if bytes.Count(got, []byte(fmt.Sprintf("function WowConverterCreateUnits%d()", i))) != 1 {
			t.Fatalf("missing or duplicate batch %d", i)
		}
	}
	if strings.Contains(string(got), "\nCreateAllUnits()") || strings.Contains(string(got), "FourCC(\"sloc\")") {
		t.Fatal("template unit creation or start-location unit retained")
	}
	again, err := injectUnitCreation(got, units)
	if err != nil || !bytes.Equal(got, again) {
		t.Fatalf("repeated unit injection must be stable: %v", err)
	}
	empty, err := injectUnitCreation(got, nil)
	if err != nil || bytes.Contains(empty, []byte("BlzCreateUnitWithSkin(")) {
		t.Fatalf("opt-out must remove stale runtime placements: %v", err)
	}
}

func TestMapUnitCreationRejectsMissingStartupAnchor(t *testing.T) {
	for _, script := range []string{"", "function main()\nend\n", unitCreationStart} {
		if _, err := injectUnitCreation([]byte(script), nil); err == nil {
			t.Fatal("malformed startup accepted")
		}
	}
}
