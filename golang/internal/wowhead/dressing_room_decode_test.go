package wowhead

import "testing"

func TestDecodeDressingRoomSingleEquipmentSlots(t *testing.T) {
	if err := initDressingRoomData(); err != nil {
		t.Fatalf("init dressing room data: %v", err)
	}
	hash := "fa80o0zN89c8zZ8jY8z18nw8zZ8jY8z28nj8z38n18z48yo8aM8z5P8Mz8yt8MM8yC8sW8z3g8zYv8dLv8Mtr808og8yh8M08yL877g3MZ8Maz87r"
	latestVersion := getLatestTemplateVersion()
	cfg := prepareDecodeConfig(latestVersion)
	pre := decompress(cfg, hash)
	tpl := hashTemplates[latestVersion]
	data := mapToDecoded(decodeWithTemplate(cfg, tpl, pre))
	for k, item := range data.Equipment {
		if item.ItemID == 0 {
			delete(data.Equipment, k)
		}
	}
	for dollSlot, item := range data.Equipment {
		slotID := paperdollSlots[toInt(dollSlot)]
		t.Logf("doll %q -> equip slot %d item %d bonus %d", dollSlot, slotID, item.ItemID, item.ItemBonus)
	}
}

func TestDecodeDressingRoomFrostPrinceEquipment(t *testing.T) {
	if err := initDressingRoomData(); err != nil {
		t.Fatalf("init dressing room data: %v", err)
	}
	hash := "fa80o0zN89c8zZ8jY8z18nw8z28nj8z38n18z48yo8M08yL8Mz8yt8MM8yC8og8yh8sW8z3g8aM8z5P8zYv8dLv8Mtr877inNk808zMaD808zMal808zMaO808zVpW87Vzzei8MIv83MZ8Maz8zVDJ808zMnO8MIv8zoFW87MzoFW87o"
	latestVersion := getLatestTemplateVersion()
	latestCfg := prepareDecodeConfig(latestVersion)
	pre := decompress(latestCfg, hash)
	version := latestVersion
	if detectedVersion := charValue(latestCfg, pre[:1]); detectedVersion > 0 {
		if _, ok := hashTemplates[detectedVersion]; ok {
			version = detectedVersion
		}
	}
	tpl := hashTemplates[version]
	cfg := prepareDecodeConfig(tpl.Version)
	data := mapToDecoded(decodeWithTemplate(cfg, tpl, pre))
	for k, item := range data.Equipment {
		if item.ItemID == 0 {
			delete(data.Equipment, k)
		}
	}

	expected := map[string]int{
		"1": 95475, "2": 202459, "3": 202456, "4": 202464, "5": 213431,
		"8": 200407, "9": 185188, "10": 214236, "11": 203508, "12": 217665, "13": 217665,
	}
	if len(data.Equipment) != len(expected) {
		t.Fatalf("expected %d equipment entries, got %d: %#v", len(expected), len(data.Equipment), data.Equipment)
	}
	for slot, itemID := range expected {
		if got := data.Equipment[slot].ItemID; got != itemID {
			t.Fatalf("slot %s item mismatch: expected %d, got %d", slot, itemID, got)
		}
	}
}

func TestToIntParsesStringMapKeys(t *testing.T) {
	if got := toInt("12"); got != 12 {
		t.Fatalf("expected string map key to parse as 12, got %d", got)
	}
}
