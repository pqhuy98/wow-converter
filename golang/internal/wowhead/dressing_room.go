package wowhead

import (
	"fmt"
	"sort"
	"strings"
)

// ItemEffect describes an item visual effect attachment.
type ItemEffect struct {
	Slot     int     `json:"Slot"`
	SubClass int     `json:"SubClass"`
	Model    int     `json:"Model"`
	Scale    float64 `json:"Scale"`
}

// DecodeDressingRoom decodes a dressing room hash into character meta.
func DecodeDressingRoom(client *HTTPClient, expansion Expansion, hash string) (CharacterData, error) {
	if err := initDressingRoomData(); err != nil {
		return CharacterData{}, err
	}
	clean := strings.TrimPrefix(hash, "#")
	if clean == "" {
		return CharacterData{}, nil
	}

	latestVersion := getLatestTemplateVersion()
	latestCfg := prepareDecodeConfig(latestVersion)
	pre := decompress(latestCfg, clean)

	detectedVersion := charValue(latestCfg, pre[:1])
	version := latestVersion
	if detectedVersion > 0 {
		if _, ok := hashTemplates[detectedVersion]; ok {
			version = detectedVersion
		}
	}
	tpl := hashTemplates[version]
	if tpl.Version == 0 {
		tpl = hashTemplates[latestVersion]
	}
	cfg := prepareDecodeConfig(tpl.Version)

	raw := decodeWithTemplate(cfg, tpl, pre)
	data := mapToDecoded(raw)

	for k, v := range data.CustChoices {
		if v.OptionID == 0 || v.ChoiceID == 0 {
			delete(data.CustChoices, k)
		}
	}
	for k, v := range data.Equipment {
		if v.ItemID == 0 {
			delete(data.Equipment, k)
		}
	}

	itemsWithBonus := make([]GatherItemInput, 0, len(data.Equipment))
	for _, item := range data.Equipment {
		itemsWithBonus = append(itemsWithBonus, GatherItemInput{ItemID: item.ItemID, ItemBonus: item.ItemBonus})
	}
	items, err := GatherItems(client, expansion, itemsWithBonus)
	if err != nil {
		return CharacterData{}, err
	}

	equipments := map[string]int{}
	for dollSlot, item := range data.Equipment {
		slotID := paperdollSlots[toInt(dollSlot)]
		if slotID == 0 {
			continue
		}
		for _, gathered := range items {
			if gathered.ItemID == item.ItemID {
				equipments[fmt.Sprintf("%d", slotID)] = gathered.DisplayID
				break
			}
		}
	}

	itemVisuals := map[string]int{}
	for dollSlot, item := range data.Equipment {
		slotID := paperdollSlots[toInt(dollSlot)]
		if slotID == 0 {
			continue
		}
		if entry, ok := itemEnchantVisual[fmt.Sprintf("%d", item.Enchant)]; ok && entry.Visual > 0 {
			itemVisuals[fmt.Sprintf("%d", slotID)] = entry.Visual
		}
	}

	var itemEffects []ItemEffect
	for slotStr, visualID := range itemVisuals {
		meta, err := FetchItemVisualMeta(client, expansion, visualID)
		if err != nil {
			continue
		}
		model := 0
		if meta.Model != nil {
			model = *meta.Model
		} else if len(meta.ItemEffects) > 0 {
			model = meta.ItemEffects[0].Model
		}
		if model <= 0 {
			continue
		}
		slotID := 0
		fmt.Sscanf(slotStr, "%d", &slotID)
		itemEffects = append(itemEffects, ItemEffect{
			Slot: slotID, SubClass: 0, Model: model, Scale: 1,
		})
	}

	customizations := make([]Customization, 0, len(data.CustChoices))
	choiceKeys := make([]string, 0, len(data.CustChoices))
	for key := range data.CustChoices {
		choiceKeys = append(choiceKeys, key)
	}
	sort.Slice(choiceKeys, func(i, j int) bool {
		return toInt(choiceKeys[i]) < toInt(choiceKeys[j])
	})
	for _, key := range choiceKeys {
		v := data.CustChoices[key]
		if v.OptionID > 0 {
			customizations = append(customizations, Customization{OptionID: v.OptionID, ChoiceID: v.ChoiceID})
		}
	}

	race := data.Settings["race"]
	gender := data.Settings["gender"]
	class := data.Settings["class"]

	return CharacterData{
		Character: &CharacterMeta{
			Class: class, Race: race, Gender: gender,
			ChrModel: chrModelIDFor(race, gender),
		},
		Creature: &CreatureMeta{
			CreatureCustomizations: customizations,
			CreatureGeosetData:     nil,
		},
		Equipment:   equipments,
		ItemEffects: itemEffects,
	}, nil
}
