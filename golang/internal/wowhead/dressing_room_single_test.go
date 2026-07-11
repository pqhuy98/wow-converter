package wowhead

import "testing"

func TestDecodeDressingRoomSingleEquipment(t *testing.T) {
	if err := initDressingRoomData(); err != nil {
		t.Fatalf("init dressing room data: %v", err)
	}
	hash := "fa80o0zN89c8z18nw8zZ8jY8z28nj8z38n18z48yo8aM8z5P8Mz8yt8MM8yC8sW8z3g8zYv8dLv8Mtr808og8yh8M08yL877g3MZ8Maz87r"
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
	t.Logf("equipment: %#v", data.Equipment)
	t.Logf("settings: %#v", data.Settings)
	if len(data.Equipment) != 1 {
		t.Fatalf("expected 1 equipment entry, got %d: %#v", len(data.Equipment), data.Equipment)
	}
}
