package character

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

// FileWithComponent pairs a CASC file with a component/section id.
type FileWithComponent struct {
	FileDataID  int
	ComponentID int
}

// ItemMetadata is processed zam item meta for character export.
type ItemMetadata struct {
	SlotID            *int
	InventoryType     int
	ItemClass         int
	ItemSubClass      int
	DisplayID         int
	Flags             int
	ModelFiles        []FileWithComponent
	ModelTextureFiles [2][]FileWithComponent
	BodyTextureFiles  []FileWithComponent
	HideGeosetIDs     []int
	ZamGeosetGroup    []int
	OriginalData      wowhead.ItemData
}

var equipmentSlotNames = map[wowhead.EquipmentSlot]string{
	wowhead.SlotHead: "Head", wowhead.SlotShoulder: "Shoulder", wowhead.SlotShirt: "Shirt",
	wowhead.SlotChest: "Chest", wowhead.SlotWaist: "Waist", wowhead.SlotLegs: "Legs",
	wowhead.SlotFeet: "Feet", wowhead.SlotWrist: "Wrist", wowhead.SlotHands: "Hands",
	wowhead.SlotMainHand: "MainHand", wowhead.SlotOffHand: "OffHand", wowhead.SlotShield: "Shield",
	wowhead.SlotRanged: "Ranged", wowhead.SlotCloak: "Cloak", wowhead.SlotTabard: "Tabard",
	wowhead.SlotRobe: "Robe", wowhead.SlotHoldable: "Holdable", wowhead.SlotRangedRight: "RangedRight",
}

// GetEquipmentSlotName returns the equipment slot name for a slot id.
func GetEquipmentSlotName(slotID int) string {
	if name, ok := equipmentSlotNames[wowhead.EquipmentSlot(slotID)]; ok {
		return name
	}
	return strconv.Itoa(slotID)
}

var submeshGroups = map[int]string{
	0: "Hair", 100: "FacialA", 200: "FacialB", 300: "FacialC", 400: "Gloves", 500: "Boots",
	600: "Tail", 700: "Ears", 800: "Wrists", 900: "Kneepads", 1000: "Chest", 1100: "Pants",
	1200: "Tabard", 1300: "Trousers", 1500: "Cloak", 1600: "Chins", 1700: "Eyeglow", 1800: "Belt",
	1900: "Bone/Tail", 2000: "Feet", 2200: "Torso", 2300: "HandAttach", 2400: "HeadAttach",
	2500: "DHBlindfolds", 2700: "Head", 2800: "Chest2", 2900: "MechagnomeArms", 3000: "MechagnomeLegs",
	3100: "MechagnomeFeet", 3200: "Face", 3300: "Eyes", 3400: "Eyebrows", 3500: "Earrings",
	3600: "Necklace", 3700: "Headdress", 3800: "Tails", 3900: "Vines", 4000: "Chins/Tusk",
	4100: "Noses", 4200: "HairDecoA", 4300: "HairDecoB", 4400: "BodySize", 5100: "EyeGlowB",
}

// GetSubmeshName returns a human-readable submesh name.
func GetSubmeshName(idx int) string {
	group := (idx / 100) * 100
	name := submeshGroups[group]
	if name == "" {
		name = "Unknown"
	}
	return fmt.Sprintf("%s%d", name, idx%100)
}

var zamGroupBaseOffset = []int{
	1, 1, 1, 1, 1, 1, 1, 2, 1, 1,
	1, 1, 1, 1, 0, 1, 0, 0, 1, 1,
	1, 1, 1, 1, 0, 0, 1, 0, 1, 0,
	0, 0, 2, 1, 1, 0, 0, 0, 1, 0,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1,
}

// ComputeZamMeshId computes zam submesh id from group and offset.
func ComputeZamMeshId(group int, offset *int) int {
	base := 1
	if group >= 0 && group < len(zamGroupBaseOffset) {
		base = zamGroupBaseOffset[group]
	}
	variant := base
	if offset != nil {
		variant += *offset
	}
	return group*100 + variant
}

// EquipmentSlotData pairs slot id with processed item metadata.
type EquipmentSlotData struct {
	SlotID wowhead.EquipmentSlot
	Data   ItemMetadata
}

// GetGeosetIdsFromEquipments derives geoset ids from equipped items.
func GetGeosetIdsFromEquipments(equipments []EquipmentSlotData, chosenEquipments ...[]EquipmentSlotData) (geosetIDs, hideGeosetIDs []int) {
	chosen := equipments
	if len(chosenEquipments) > 0 && chosenEquipments[0] != nil {
		chosen = chosenEquipments[0]
	}

	find := func(slot wowhead.EquipmentSlot) *EquipmentSlotData {
		for i := range equipments {
			if equipments[i].SlotID == slot {
				return &equipments[i]
			}
		}
		return nil
	}
	head := find(wowhead.SlotHead)
	shoulders := find(wowhead.SlotShoulder)
	shirt := find(wowhead.SlotShirt)
	chest := find(wowhead.SlotChest)
	waist := find(wowhead.SlotWaist)
	legs := find(wowhead.SlotLegs)
	feet := find(wowhead.SlotFeet)
	wrist := find(wowhead.SlotWrist)
	hands := find(wowhead.SlotHands)
	tabard := find(wowhead.SlotTabard)
	cloak := find(wowhead.SlotCloak)

	removeGroup := func(group int, hideDefault bool) {
		filtered := geosetIDs[:0]
		for _, g := range geosetIDs {
			if g/100 != group {
				filtered = append(filtered, g)
			}
		}
		geosetIDs = filtered
		if hideDefault {
			hideGeosetIDs = append(hideGeosetIDs, group*100+1)
		}
	}
	hasGeoset := func(equip *EquipmentSlotData, i int) bool {
		if equip == nil || i >= len(equip.Data.ZamGeosetGroup) {
			return false
		}
		return equip.Data.ZamGeosetGroup[i] > 0
	}
	addGeoset := func(equip *EquipmentSlotData, group, i int) {
		if equip == nil || i >= len(equip.Data.ZamGeosetGroup) {
			return
		}
		removeGroup(group, false)
		off := equip.Data.ZamGeosetGroup[i]
		geosetIDs = append(geosetIDs, ComputeZamMeshId(group, &off))
	}

	hasChestTrouser := hasGeoset(chest, 2)
	hasLegsTrouser := hasGeoset(legs, 2)

	for _, s := range chosen {
		switch s.SlotID {
		case wowhead.SlotHead:
			addGeoset(head, 27, 0)
			addGeoset(head, 21, 1)
		case wowhead.SlotShoulder:
			addGeoset(shoulders, 26, 0)
		case wowhead.SlotShirt:
			if !hasGeoset(hands, 0) {
				addGeoset(shirt, 8, 0)
			}
			addGeoset(shirt, 10, 1)
		case wowhead.SlotChest, wowhead.SlotRobe:
			if !hasGeoset(hands, 0) {
				addGeoset(&s, 8, 0)
			}
			addGeoset(&s, 10, 1)
			addGeoset(&s, 13, 2)
			addGeoset(&s, 22, 3)
			addGeoset(&s, 28, 4)
		case wowhead.SlotWaist:
			addGeoset(waist, 18, 0)
		case wowhead.SlotLegs:
			addGeoset(legs, 11, 0)
			addGeoset(legs, 9, 1)
			addGeoset(legs, 13, 2)
		case wowhead.SlotFeet:
			addGeoset(feet, 5, 0)
			if hasGeoset(feet, 1) {
				v := 2000 + feet.Data.ZamGeosetGroup[1]
				geosetIDs = append(geosetIDs, v)
			} else if feet == nil || (feet.Data.Flags&1048576) != 0 {
				geosetIDs = append(geosetIDs, 2001)
			} else {
				geosetIDs = append(geosetIDs, 2002)
			}
		case wowhead.SlotHands:
			chestHasPalms := hasGeoset(chest, 0)
			handsHasPalms := hasGeoset(hands, 0)
			if handsHasPalms || !chestHasPalms {
				addGeoset(hands, 4, 0)
			}
			addGeoset(hands, 23, 1)
		case wowhead.SlotCloak:
			addGeoset(cloak, 15, 0)
		case wowhead.SlotTabard:
			addGeoset(tabard, 12, 0)
		case wowhead.SlotWrist:
			handsHasGlove := hasGeoset(hands, 0)
			chestHasWristsTrousers := hasGeoset(chest, 2) && hasGeoset(chest, 0)
			if !handsHasGlove && !chestHasWristsTrousers {
				addGeoset(wrist, 23, 0)
			}
		}
		hideGeosetIDs = append(hideGeosetIDs, s.Data.HideGeosetIDs...)
	}

	if hasChestTrouser {
		removeGroup(5, true)
		removeGroup(9, true)
		removeGroup(11, true)
		removeGroup(13, true)
		addGeoset(chest, 13, 2)
	} else if hasLegsTrouser {
		removeGroup(5, true)
		removeGroup(9, true)
		removeGroup(11, true)
		removeGroup(13, true)
		addGeoset(legs, 13, 2)
	}
	return geosetIDs, hideGeosetIDs
}

// FilterCollectionGeosets returns geosets to enable on a collection armor model.
func FilterCollectionGeosets(equipmentSlots []EquipmentSlotData, slotData EquipmentSlotData, model *mdl.MDL) []*components.Geoset {
	submeshIDList, _ := GetGeosetIdsFromEquipments(equipmentSlots, []EquipmentSlotData{slotData})
	submeshIDs := map[int]struct{}{}
	for _, id := range submeshIDList {
		submeshIDs[id] = struct{}{}
	}

	chosen := map[*components.Geoset]struct{}{}
	var chosenOrder []*components.Geoset
	addChosen := func(g *components.Geoset) {
		if g == nil {
			return
		}
		if _, ok := chosen[g]; ok {
			return
		}
		chosen[g] = struct{}{}
		chosenOrder = append(chosenOrder, g)
	}
	enabledGroups := map[int]struct{}{}
	for _, g := range model.Geosets {
		if g == nil {
			continue
		}
		if _, ok := submeshIDs[g.WowData.SubmeshID]; ok {
			addChosen(g)
			enabledGroups[g.WowData.SubmeshID/100] = struct{}{}
		}
	}
	seenSubmeshID := map[int]struct{}{}
	for _, id := range submeshIDList {
		if _, ok := seenSubmeshID[id]; ok {
			continue
		}
		seenSubmeshID[id] = struct{}{}
		group := id / 100
		if _, ok := enabledGroups[group]; ok {
			continue
		}
		var defaultGeoset *components.Geoset
		for _, g := range model.Geosets {
			if g != nil && g.WowData.SubmeshID/100 == group {
				defaultGeoset = g
				break
			}
		}
		if defaultGeoset == nil {
			continue
		}
		for _, g := range model.Geosets {
			if g != nil && g.WowData.SubmeshID == defaultGeoset.WowData.SubmeshID {
				addChosen(g)
			}
		}
		enabledGroups[group] = struct{}{}
	}
	return chosenOrder
}

func resolveHideGeosetIDs(itemData wowhead.ItemData, targetRace, targetGender int) []int {
	result := map[int]struct{}{}
	hideGeosets := itemData.Item.HideGeosetMale
	if targetGender == 1 {
		hideGeosets = itemData.Item.HideGeosetFemale
	}
	for _, value := range hideGeosets {
		if value.RaceID != targetRace {
			continue
		}
		band := value.GeosetGroup
		start := 1
		if band == 1 || band == 2 || band == 3 {
			start = 2
		}
		for i := start; i < 100; i++ {
			result[band*100+i] = struct{}{}
		}
	}
	ids := make([]int, 0, len(result))
	for id := range result {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func itemReplaceableTextures(files [2][]FileWithComponent) map[string]int {
	replaceable := map[string]int{}
	for _, set := range files {
		for _, f := range set {
			replaceable[strconv.Itoa(f.ComponentID)] = f.FileDataID
		}
	}
	return replaceable
}

func itemModelTextureIDs(files [2][]FileWithComponent) []int {
	seen := map[int]struct{}{}
	out := make([]int, 0)
	for _, set := range files {
		for _, f := range set {
			if _, ok := seen[f.FileDataID]; ok {
				continue
			}
			seen[f.FileDataID] = struct{}{}
			out = append(out, f.FileDataID)
		}
	}
	return out
}

// ProcessItemData fetches and processes zam item meta.
func ProcessItemData(http *wowhead.HTTPClient, expansion wowhead.Expansion, zam wowhead.ZamURL, targetRace, targetGender, targetClass int) (ItemMetadata, error) {
	itemData, err := wowhead.FetchItemMeta(http, expansion, zam.DisplayID, zam.SlotID)
	if err != nil {
		return ItemMetadata{}, err
	}
	tex0 := make([]FileWithComponent, 0)
	for k, v := range itemData.Textures {
		cid, _ := strconv.Atoi(k)
		tex0 = append(tex0, FileWithComponent{FileDataID: v, ComponentID: cid})
	}
	sort.Slice(tex0, func(i, j int) bool { return tex0[i].ComponentID < tex0[j].ComponentID })
	tex1 := make([]FileWithComponent, 0)
	for k, v := range itemData.Textures2 {
		cid, _ := strconv.Atoi(k)
		tex1 = append(tex1, FileWithComponent{FileDataID: v, ComponentID: cid})
	}
	sort.Slice(tex1, func(i, j int) bool { return tex1[i].ComponentID < tex1[j].ComponentID })
	slotID := zam.SlotID
	return ItemMetadata{
		SlotID: slotID, InventoryType: itemData.Item.InventoryType,
		ItemClass: itemData.Item.ItemClass, ItemSubClass: itemData.Item.ItemSubClass,
		DisplayID: zam.DisplayID, Flags: itemData.Item.Flags,
		ModelFiles: filterFilesByRaceGenderClass(itemData.ModelFiles, itemData.ComponentModels,
			targetRace, targetGender, targetClass, false, slotID),
		ModelTextureFiles: [2][]FileWithComponent{tex0, tex1},
		BodyTextureFiles: filterFilesByRaceGenderClass(itemData.TextureFiles, itemData.ComponentTextures,
			targetRace, targetGender, targetClass, true, slotID),
		HideGeosetIDs:  resolveHideGeosetIDs(itemData, targetRace, targetGender),
		ZamGeosetGroup: itemData.Item.GeosetGroup,
		OriginalData:   itemData,
	}, nil
}

// ExportZamItemAsMdl exports a zam item as MDL.
func ExportZamItemAsMdl(ctx *ExportContext, zam wowhead.ZamURL, targetRace, targetGender, targetClass int) (*commonModel, ItemMetadata, error) {
	result, err := ProcessItemData(ctx.WowheadHTTP(), zam.Expansion, zam, targetRace, targetGender, targetClass)
	if err != nil {
		return nil, ItemMetadata{}, err
	}
	if len(result.ModelFiles) == 0 {
		return nil, ItemMetadata{}, fmt.Errorf("found no model for item %d", zam.DisplayID)
	}
	modelID := result.ModelFiles[0].FileDataID
	model, err := ExportModelFileIDAsMdl(ctx, modelID, ExportModelOptions{
		TextureIDs: itemModelTextureIDs(result.ModelTextureFiles),
	})
	if err != nil {
		return nil, ItemMetadata{}, err
	}
	if err := ApplyReplaceableTextures(ctx, model.MDL, itemReplaceableTextures(result.ModelTextureFiles)); err != nil {
		return nil, ItemMetadata{}, err
	}
	return model, result, nil
}

var raceGenderFallback = map[int][8]int{
	86: {4, 0, 4, 1, 4, 0, 4, 1}, 85: {84, 0, 84, 1, 84, 0, 84, 1},
	84: {3, 0, 3, 1, 3, 0, 3, 1}, 77: {5, 1, 0, -1, 5, 0, 0, -1},
	76: {10, 0, 1, 1, 10, 0, 1, 1}, 75: {10, 0, 1, 1, 10, 0, 1, 1},
	74: {5, 1, 0, -1, 5, 0, 0, -1}, 73: {5, 1, 0, -1, 5, 0, 0, -1},
	72: {5, 1, 0, -1, 5, 0, 0, -1}, 71: {5, 1, 0, -1, 5, 0, 0, -1},
	37: {7, 0, 7, 1, 7, 0, 7, 1}, 36: {2, 0, 2, 1, 2, 0, 2, 1},
	34: {3, 0, 3, 1, 3, 0, 3, 1}, 33: {5, 1, 0, -1, 5, 0, 0, -1},
	31: {0, -1, 8, 1, 0, -1, 8, 1}, 30: {11, 0, 11, 1, 11, 0, 11, 1},
	29: {10, 0, 10, 1, 10, 0, 10, 1}, 28: {6, 0, 6, 1, 6, 0, 6, 1},
	27: {4, 0, 4, 1, 4, 0, 4, 1}, 26: {24, 0, 24, 1, 24, 0, 24, 1},
	25: {24, 0, 24, 1, 24, 0, 24, 1}, 23: {1, 0, 1, 1, 1, 0, 1, 1},
	15: {5, 0, 5, 1, 5, 0, 5, 1}, 1: {0, -1, 0, -1, 0, -1, 0, 3},
}

func filterFilesByRaceGenderClass(
	files map[string][]wowhead.ItemFile,
	components map[string]int,
	targetRace, targetGender, targetClass int,
	isTexture bool,
	slotID *int,
) []FileWithComponent {
	keys := make([]string, 0, len(components))
	for k := range components {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ai, _ := strconv.Atoi(keys[i])
		aj, _ := strconv.Atoi(keys[j])
		return ai < aj
	})
	out := make([]FileWithComponent, 0)
	for _, componentID := range keys {
		id := components[componentID]
		entries := files[strconv.Itoa(id)]
		cid, _ := strconv.Atoi(componentID)
		if isTexture {
			best := wowheadSelectBestTexture(entries, targetGender, targetClass, targetRace)
			if best > 0 {
				out = append(out, FileWithComponent{FileDataID: best, ComponentID: cid})
			}
		} else {
			extraData := -1
			if slotID != nil && *slotID == int(wowhead.SlotShoulder) {
				if cid == 0 {
					extraData = 0
				} else if cid == 1 {
					extraData = 1
				}
			}
			fileDataID := wowheadSelectBestModel(entries, extraData, targetGender, targetClass, targetRace)
			if fileDataID > 0 {
				out = append(out, FileWithComponent{FileDataID: fileDataID, ComponentID: cid})
			}
		}
	}
	return out
}

func wowheadRemapRaceGender(gender, race int, isTexture bool) (int, int, bool) {
	row, ok := raceGenderFallback[race]
	if !ok {
		return 0, 0, false
	}
	base := 0
	if isTexture {
		base = 4
	}
	idx := base + 2*gender
	if idx+1 >= len(row) {
		return 0, 0, false
	}
	return row[idx], row[idx+1], true
}

func wowheadSelectBestTexture(entries []wowhead.ItemFile, gender, clazz, race int) int {
	if len(entries) == 0 {
		return 0
	}
	bucket := make([]int, 24)
	for _, a := range entries {
		o, h, l, u := a.Gender, a.Class, a.Race, a.ExtraData
		c := 0
		if gender > 1 || o != gender {
			if o < 2 {
				continue
			}
			c = 0
		} else {
			c = 2
		}
		d := 1
		if clazz > 0 && h == clazz {
			d = 0
		} else if h > 0 {
			continue
		}
		f := 1
		if race > 0 && l == race {
			f = 0
		} else if l > 0 {
			continue
		}
		index := u + 3*(f+2*(c+d))
		if index >= 0 && index < len(bucket) {
			bucket[index] = a.FileDataID
		}
	}
	for t := 0; t < 2; t++ {
		for e := 0; e < 2; e++ {
			for i := 0; i < 2; i++ {
				s := 3 * (t + 2*(e+2*i))
				if bucket[s] > 0 {
					return bucket[s]
				}
			}
		}
	}
	if nr, ng, ok := wowheadRemapRaceGender(gender, race, true); ok && nr != 0 {
		return wowheadSelectBestTexture(entries, ng, clazz, nr)
	}
	return 0
}

func wowheadSelectBestModel(entries []wowhead.ItemFile, extraData, gender, clazz, race int) int {
	bucket := make([]int, 16)
	for _, o := range entries {
		h, l, u, c := o.Gender, o.Class, o.Race, o.ExtraData
		d := 0
		if gender > 1 || h != gender {
			if h < 2 {
				continue
			}
			d = 0
		} else {
			d = 2
		}
		f := 1
		if clazz > 0 && l == clazz {
			f = 0
		} else if l > 0 {
			continue
		}
		g := 1
		if race > 0 && u == race {
			g = 0
		} else if u > 0 {
			continue
		}
		b := 1
		if extraData == -1 || c != extraData {
			if c != -1 && extraData != -1 {
				continue
			}
		} else {
			b = 0
		}
		index := b + 2*(g+2*(d+f))
		if index >= 0 && index < len(bucket) {
			bucket[index] = o.FileDataID
		}
	}
	for t := 0; t < 2; t++ {
		for e := 0; e < 2; e++ {
			for i := 0; i < 2; i++ {
				for s := 0; s < 2; s++ {
					r := s + 2*(t+2*(e+2*i))
					if bucket[r] > 0 {
						return bucket[r]
					}
				}
			}
		}
	}
	if nr, ng, ok := wowheadRemapRaceGender(gender, race, false); ok && nr != 0 {
		return wowheadSelectBestModel(entries, extraData, ng, clazz, nr)
	}
	return 0
}
