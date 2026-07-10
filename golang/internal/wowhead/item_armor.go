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

// FetchItemMeta fetches item or armor meta JSON from zam modelviewer.
func FetchItemMeta(client *HTTPClient, expansion Expansion, displayID int, slotID *int) (ItemData, error) {
	effectiveSlot := 0
	if slotID != nil {
		effectiveSlot = *slotID
	}
	if effectiveSlot > 0 && !containsArmorSlot(EquipmentSlot(effectiveSlot)) {
		effectiveSlot = 0
	}
	path := fmt.Sprintf("meta/item/%d.json", displayID)
	if effectiveSlot > 0 {
		path = fmt.Sprintf("meta/armor/%d/%d.json", effectiveSlot, displayID)
	}
	if expansion == ExpansionLatestAvailable {
		var err error
		expansion, err = GetLatestExpansionHavingURL(client, path)
		if err != nil {
			return ItemData{}, err
		}
	}
	url := GetZamBaseURL(expansion) + "/" + path
	text, err := FetchWithCache(client, url)
	if err != nil {
		if effectiveSlot > 0 {
			if backup, ok := slotBackup[effectiveSlot]; ok {
				backupPtr := backup
				return FetchItemMeta(client, expansion, displayID, &backupPtr)
			}
		}
		if expansion != ExpansionLatestAvailable {
			if fallback, err2 := GetLatestExpansionHavingURL(client, path); err2 == nil && fallback != expansion {
				return FetchItemMeta(client, fallback, displayID, slotID)
			}
		}
		return ItemData{}, err
	}
	var data ItemData
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return ItemData{}, err
	}
	return data, nil
}

func containsArmorSlot(slot EquipmentSlot) bool {
	for _, s := range armorSlots {
		if s == slot {
			return true
		}
	}
	return false
}
