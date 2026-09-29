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

func TestDisplayModIndexForOutfitColorBonuses(t *testing.T) {
	cases := map[int]int{
		0: 0, 6806: 1, 7309: 3, 12282: 4, 7980: 1,
		8999: 159, 9434: 160, 13473: 3, 11981: 1, 12265: 4,
		999999: 0,
	}
	for bonus, want := range cases {
		if got := displayModIndexForBonus(bonus); got != want {
			t.Fatalf("bonus %d: got appearance %d, want %d", bonus, got, want)
		}
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
