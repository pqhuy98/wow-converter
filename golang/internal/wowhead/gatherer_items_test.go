package wowhead

import "testing"

func TestSelectDisplayIDForCharacterUsesDefaultAppearance(t *testing.T) {
	entry := gathererEntry{}
	entry.JSON.DisplayID = 0
	entry.JSON.Appearances = map[string][]any{
		"0": {float64(12345), "default"},
	}
	if got := selectDisplayIDForCharacter(entry, 0); got != 12345 {
		t.Fatalf("expected default appearance display id, got %d", got)
	}
}

func TestSelectDisplayIDForCharacterUsesDisplayModBonusIndex(t *testing.T) {
	entry := gathererEntry{}
	entry.JSON.DisplayID = 100
	entry.JSON.Appearances = map[string][]any{
		"0": {float64(100), "default"},
		"3": {float64(300), "variant"},
	}
	if got := selectDisplayIDForCharacter(entry, 7309); got != 300 {
		t.Fatalf("expected bonus appearance display id, got %d", got)
	}
}

func TestSelectDisplayIDForCharacterFallsBackToJSONEquipAppearances(t *testing.T) {
	entry := gathererEntry{}
	entry.JSON.DisplayID = 0
	entry.JSONEquip = &struct {
		Appearances map[string][]any `json:"appearances"`
	}{
		Appearances: map[string][]any{"1": {float64(111), "equip"}},
	}
	if got := selectDisplayIDForCharacter(entry, 6806); got != 111 {
		t.Fatalf("expected jsonequip appearance display id, got %d", got)
	}
}
