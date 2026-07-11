package wowhead

import (
	"encoding/json"
	"fmt"
)

// EquipmentSlot identifies wow equipment slots.
type EquipmentSlot int

const (
	SlotHead EquipmentSlot = 1
	SlotShoulder EquipmentSlot = 3
	SlotShirt EquipmentSlot = 4
	SlotChest EquipmentSlot = 5
	SlotWaist EquipmentSlot = 6
	SlotLegs EquipmentSlot = 7
	SlotFeet EquipmentSlot = 8
	SlotWrist EquipmentSlot = 9
	SlotHands EquipmentSlot = 10
	SlotMainHand EquipmentSlot = 12
	SlotOffHand EquipmentSlot = 13
	SlotShield EquipmentSlot = 14
	SlotRanged EquipmentSlot = 15
	SlotCloak EquipmentSlot = 16
	SlotTabard EquipmentSlot = 19
	SlotRobe EquipmentSlot = 20
	SlotHoldable EquipmentSlot = 23
	SlotRangedRight EquipmentSlot = 26
)

var armorSlots = []EquipmentSlot{
	SlotHead, SlotShoulder, SlotShirt, SlotChest, SlotWaist, SlotLegs, SlotFeet,
	SlotWrist, SlotHands, SlotCloak, SlotTabard, SlotRobe,
}

var slotBackup = map[int]int{5: 20}

// ItemFile is a race/gender/class keyed file entry.
type ItemFile struct {
	FileDataID int `json:"FileDataId"`
	Race       int `json:"Race"`
	Gender     int `json:"Gender"`
	Class      int `json:"Class"`
	ExtraData  int `json:"ExtraData"`
}

// ItemData is zam item meta JSON.
type ItemData struct {
	Textures         map[string]int            `json:"Textures"`
	Textures2        map[string]int            `json:"Textures2"`
	ModelFiles       map[string][]ItemFile     `json:"ModelFiles"`
	TextureFiles     map[string][]ItemFile     `json:"TextureFiles"`
	ComponentModels  map[string]int            `json:"ComponentModels"`
	ComponentTextures map[string]int           `json:"ComponentTextures"`
	Item             ItemInfo                    `json:"Item"`
}

// ItemInfo holds item metadata fields.
type ItemInfo struct {
	Flags            int `json:"Flags"`
	InventoryType    int `json:"InventoryType"`
	ItemClass        int `json:"ItemClass"`
	ItemSubClass     int `json:"ItemSubClass"`
	GeosetGroup      []int `json:"GeosetGroup"`
	HideGeosetMale   []HideGeosetEntry `json:"HideGeosetMale"`
	HideGeosetFemale []HideGeosetEntry `json:"HideGeosetFemale"`
}

// HideGeosetEntry is a hide-geoset rule for a race.
type HideGeosetEntry struct {
	RaceID      int `json:"RaceId"`
	GeosetGroup int `json:"GeosetGroup"`
}

func itemMetaCandidatePaths(displayID, preferredSlot int) []string {
	paths := make([]string, 0, len(armorSlots)+2)
	seen := map[int]bool{}
	addSlot := func(slot int) {
		if slot <= 0 || seen[slot] {
			return
		}
		seen[slot] = true
		paths = append(paths, fmt.Sprintf("meta/armor/%d/%d.json", slot, displayID))
	}
	addSlot(preferredSlot)
	if backup, ok := slotBackup[preferredSlot]; ok {
		addSlot(backup)
	}
	for _, slot := range armorSlots {
		addSlot(int(slot))
	}
	paths = append(paths, fmt.Sprintf("meta/item/%d.json", displayID))
	return paths
}

func fetchItemMetaAtPath(client *HTTPClient, expansion Expansion, path string) (ItemData, error) {
	exp := expansion
	if exp == ExpansionLatestAvailable {
		var err error
		exp, err = GetLatestExpansionHavingURL(client, path)
		if err != nil {
			return ItemData{}, err
		}
	}
	url := GetZamBaseURL(exp) + "/" + path
	text, err := FetchWithCache(client, url)
	if err != nil && expansion != ExpansionLatestAvailable {
		if fallback, err2 := GetLatestExpansionHavingURL(client, path); err2 == nil && fallback != exp {
			url = GetZamBaseURL(fallback) + "/" + path
			text, err = FetchWithCache(client, url)
		}
	}
	if err != nil {
		return ItemData{}, err
	}
	var data ItemData
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return ItemData{}, err
	}
	return data, nil
}

// FetchItemMeta fetches item or armor meta JSON from zam modelviewer.
func FetchItemMeta(client *HTTPClient, expansion Expansion, displayID int, slotID *int) (ItemData, error) {
	preferredSlot := 0
	if slotID != nil {
		preferredSlot = *slotID
		if preferredSlot > 0 && !containsArmorSlot(EquipmentSlot(preferredSlot)) {
			preferredSlot = 0
		}
	}
	var lastErr error
	for _, path := range itemMetaCandidatePaths(displayID, preferredSlot) {
		data, err := fetchItemMetaAtPath(client, expansion, path)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return ItemData{}, lastErr
	}
	return ItemData{}, fmt.Errorf("item meta not found for display %d", displayID)
}

func containsArmorSlot(slot EquipmentSlot) bool {
	for _, s := range armorSlots {
		if s == slot {
			return true
		}
	}
	return false
}
