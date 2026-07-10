package character

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pqhuy98/wow-converter/internal/ansi"
	"github.com/pqhuy98/wow-converter/internal/buffer"
	"github.com/pqhuy98/wow-converter/internal/converter/texturesource"
	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
	"github.com/pqhuy98/wow-converter/internal/formats/blp"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl"
	"github.com/pqhuy98/wow-converter/internal/formats/mdl/components"
	pngfmt "github.com/pqhuy98/wow-converter/internal/formats/png"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

// ExportCharacterAsMdl exports a player character with equipment and customizations.
func ExportCharacterAsMdl(ctx *ExportContext, metaData CharacterData, expansion wowhead.Expansion, keepCinematic bool, attackTag animmap.AttackTag) (*mdl.MDL, error) {
	start := time.Now()
	prep, err := prepareCharacterExport(ctx, metaData, expansion)
	if err != nil {
		return nil, err
	}
	if !ctx.Config.IsBulkExport {
		log.Printf("Character export - race: %d gender: %d", prep.RPCParams.Race, prep.RPCParams.Gender)
	}

	rpcBody := prep.RPCParams
	rpcBody.ExcludeAnimationIDs = getExcludedAnimIDs(keepCinematic, attackTag)

	charModel, err := ExportCharacterDirectAsModel(ctx, rpcBody)
	if err != nil {
		return nil, err
	}
	charMdl := charModel.MDL

	if err := applyPrebakedTexture(ctx, charMdl, prep); err != nil {
		return nil, err
	}
	if err := applyEquipmentsBodyTextures(ctx, charMdl, prep, expansion); err != nil {
		return nil, err
	}
	if err := applyCloakTexture(ctx, charMdl, prep.EquipmentSlots); err != nil {
		return nil, err
	}
	if err := attachEquipmentsWithModel(ctx, charMdl, prep.EquipmentSlots, metaData); err != nil {
		return nil, err
	}
	if err := applyCustomizationCollections(ctx, charMdl, metaData, prep, expansion); err != nil {
		return nil, err
	}

	hasTrousers := false
	for _, g := range charMdl.Geosets {
		if strings.HasPrefix(g.Name, "Trousers") && g.Name != "Trousers1" {
			hasTrousers = true
			break
		}
	}
	if hasTrousers {
		filtered := charMdl.Geosets[:0]
		for _, g := range charMdl.Geosets {
			if !strings.HasPrefix(g.Name, "Tabard") {
				filtered = append(filtered, g)
			}
		}
		charMdl.Geosets = filtered
	}

	if err := ApplyReplaceableTextures(ctx, charMdl, prep.ReplaceableTextures); err != nil {
		return nil, err
	}
	log.Printf("Character export took %s", ansi.Yellowf("%.2fs", time.Since(start).Seconds()))
	return charMdl, nil
}

type characterPrep struct {
	RPCParams           ExportCharacterParams
	PrebakedTexture     *int
	EquipmentSlots      []EquipmentSlotData
	ReplaceableTextures map[string]int
	ChrModelID          int
}

func prepareCharacterExport(ctx *ExportContext, metadata CharacterData, expansion wowhead.Expansion) (characterPrep, error) {
	if metadata.Character == nil {
		return characterPrep{}, fmt.Errorf("prepareCharacterExport: URL has no character metadata")
	}
	character := metadata.Character
	race, gender, clazz := character.Race, character.Gender, character.Class

	equipmentSlots := make([]EquipmentSlotData, 0)
	slotIDs := []wowhead.EquipmentSlot{
		wowhead.SlotHead, wowhead.SlotShoulder, wowhead.SlotShirt, wowhead.SlotChest,
		wowhead.SlotWaist, wowhead.SlotLegs, wowhead.SlotFeet, wowhead.SlotWrist,
		wowhead.SlotHands, wowhead.SlotMainHand, wowhead.SlotOffHand, wowhead.SlotShield,
		wowhead.SlotRanged, wowhead.SlotCloak, wowhead.SlotTabard, wowhead.SlotRobe,
		wowhead.SlotHoldable, wowhead.SlotRangedRight,
	}
	for _, slotID := range slotIDs {
		itemID := 0
		if metadata.Equipment != nil {
			itemID = metadata.Equipment[strconv.Itoa(int(slotID))]
		}
		if itemID == 0 {
			continue
		}
		slot := int(slotID)
		zam := wowhead.ZamURL{Expansion: expansion, Type: wowhead.ZamTypeItem, DisplayID: itemID, SlotID: &slot}
		itemData, err := ProcessItemData(ctx.WowheadHTTP(), expansion, zam, race, gender, clazz)
		if err != nil {
			log.Printf("Failed to process item %d for slot %d: %v", itemID, slotID, err)
			continue
		}
		equipmentSlots = append(equipmentSlots, EquipmentSlotData{SlotID: slotID, Data: itemData})
	}
	log.Printf("Equipments: %v", slotNames(equipmentSlots))

	geosetIDs, hideGeosetIDs := GetGeosetIdsFromEquipments(equipmentSlots)
	fileDataIDOverride, _ := checkCharacterFileDataIDOverride(ctx, metadata, expansion)

	customizations := map[string]int{}
	if metadata.Creature != nil {
		for _, c := range metadata.Creature.CreatureCustomizations {
			customizations[strconv.Itoa(c.OptionID)] = c.ChoiceID
		}
	}
	rpcParams := ExportCharacterParams{
		Race: race, Gender: gender,
		FileDataIDOverride: fileDataIDOverride,
		Customizations:     customizations,
		GeosetIDs:          geosetIDs,
		HideGeosetIDs:      hideGeosetIDs,
	}
	prebaked := selectPrebakedTextureID(metadata.TextureFiles)
	return characterPrep{
		RPCParams: rpcParams, PrebakedTexture: prebaked,
		EquipmentSlots: equipmentSlots, ReplaceableTextures: metadata.Textures,
		ChrModelID: character.ChrModel,
	}, nil
}

func selectPrebakedTextureID(textureFiles map[string][]wowhead.FileEntry) *int {
	if len(textureFiles) == 0 {
		return nil
	}
	keys := make([]string, 0, len(textureFiles))
	for k := range textureFiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if len(textureFiles[k]) > 0 {
			id := textureFiles[k][0].FileDataID
			return &id
		}
	}
	return nil
}

func slotNames(slots []EquipmentSlotData) []string {
	out := make([]string, len(slots))
	for i, s := range slots {
		out[i] = GetEquipmentSlotName(int(s.SlotID))
	}
	return out
}

func checkCharacterFileDataIDOverride(ctx *ExportContext, metadata CharacterData, expansion wowhead.Expansion) (*int, error) {
	if metadata.Character == nil || metadata.Character.ChrModel == 0 {
		return nil, nil
	}
	choices := map[string]struct{}{}
	if metadata.Creature != nil {
		for _, c := range metadata.Creature.CreatureCustomizations {
			choices[fmt.Sprintf("%d-%d", c.OptionID, c.ChoiceID)] = struct{}{}
		}
	}
	charCus, err := wowhead.FetchCharacterCustomization(ctx.WowheadHTTP(), expansion, metadata.Character.ChrModel)
	if err != nil {
		return nil, err
	}
	for _, option := range charCus.Options {
		for _, choice := range option.Choices {
			if _, ok := choices[fmt.Sprintf("%d-%d", option.ID, choice.ID)]; !ok {
				continue
			}
			for _, element := range choice.Elements {
				if element.CondModelFileDataId > 0 {
					id := element.CondModelFileDataId
					return &id, nil
				}
			}
		}
	}
	return nil, nil
}

func applyCustomizationCollections(ctx *ExportContext, charMdl *mdl.MDL, metadata CharacterData, prep characterPrep, expansion wowhead.Expansion) error {
	charCus, err := wowhead.FetchCharacterCustomization(ctx.WowheadHTTP(), expansion, prep.ChrModelID)
	if err != nil {
		return err
	}
	customizations := metadata.Creature.CreatureCustomizations
	if metadata.Creature == nil {
		customizations = nil
	}
	choiceIDs := map[int]struct{}{}
	for _, c := range customizations {
		choiceIDs[c.ChoiceID] = struct{}{}
	}
	type collectionEntry struct {
		model     *commonModel
		geosetIDs map[int]struct{}
	}
	collections := map[int]*collectionEntry{}
	var collectionOrder []int
	replaceableTextures := map[string]int{}

	for _, option := range charCus.Options {
		for _, choice := range option.Choices {
			var customization *wowhead.Customization
			for i := range customizations {
				if customizations[i].OptionID == option.ID && customizations[i].ChoiceID == choice.ID {
					customization = &customizations[i]
					break
				}
			}
			if customization == nil {
				continue
			}
			for _, element := range choice.Elements {
				if element.SkinnedModel != nil {
					_, hasVarChoice := choiceIDs[element.VariationChoiceID]
					if element.VariationChoiceID <= 0 || hasVarChoice {
						sm := element.SkinnedModel
						entry := collections[sm.CollectionFileDataID]
						if entry == nil {
							model, err := ExportModelFileIDAsMdl(ctx, sm.CollectionFileDataID, ExportModelOptions{})
							if err != nil {
								return err
							}
							entry = &collectionEntry{model: model, geosetIDs: map[int]struct{}{}}
							collections[sm.CollectionFileDataID] = entry
							collectionOrder = append(collectionOrder, sm.CollectionFileDataID)
						}
						geosetID := sm.GeosetType*100 + sm.GeosetID
						entry.geosetIDs[geosetID] = struct{}{}
					}
				}
				if element.Material != nil {
					texFiles := charCus.TextureFiles[strconv.Itoa(element.Material.MaterialResourcesID)]
					for _, t := range texFiles {
						if (t.Race == prep.RPCParams.Race || t.Race == 0) &&
							(t.Gender == prep.RPCParams.Gender || t.Gender > 1) {
							for _, layer := range charCus.TextureLayers {
								if layer.ChrModelTextureTargetID == element.Material.TextureTarget {
									replaceableTextures[strconv.Itoa(layer.TextureType)] = t.FileDataID
								}
							}
						}
					}
				}
			}
		}
	}

	textureTypeToImage := map[int]string{}
	if replaceableTextures["1"] != 0 {
		for _, t := range charMdl.Textures {
			if t.WowData.Type == 1 && t.Image != "" {
				textureTypeToImage[t.WowData.Type] = t.Image
				delete(replaceableTextures, "1")
			}
		}
	}

	for _, fileDataID := range collectionOrder {
		entry := collections[fileDataID]
		itemMdl := entry.model.MDL
		filtered := itemMdl.Geosets[:0]
		for _, g := range itemMdl.Geosets {
			if g == nil {
				continue
			}
			if _, ok := entry.geosetIDs[g.WowData.SubmeshID]; ok {
				filtered = append(filtered, g)
			}
		}
		itemMdl.Geosets = filtered
		for i := range itemMdl.Textures {
			if img, ok := textureTypeToImage[itemMdl.Textures[i].WowData.Type]; ok {
				itemMdl.Textures[i].Image = img
			}
		}
		if err := ApplyReplaceableTextures(ctx, itemMdl, replaceableTextures); err != nil {
			return err
		}
		charMdl.Modify.AddMdlCollectionItemToModel(itemMdl)
	}
	return nil
}

func attachEquipmentsWithModel(ctx *ExportContext, charMdl *mdl.MDL, equipmentSlots []EquipmentSlotData, metadata CharacterData) error {
	collectionTemplates := map[int]*commonModel{}
	attachmentList := map[wowhead.EquipmentSlot][]animmap.WoWAttachmentID{
		wowhead.SlotHead:        {animmap.WoWAttachmentHelm},
		wowhead.SlotShoulder:    {animmap.WoWAttachmentShoulderLeft, animmap.WoWAttachmentShoulderRight},
		wowhead.SlotWaist:       {animmap.WoWAttachmentBeltBuckle},
		wowhead.SlotCloak:       {animmap.WoWAttachmentBackpack},
		wowhead.SlotChest:       {},
		wowhead.SlotLegs:        {},
		wowhead.SlotFeet:        {},
		wowhead.SlotHands:       {},
		wowhead.SlotMainHand:    {animmap.WoWAttachmentHandRight},
		wowhead.SlotOffHand:     {animmap.WoWAttachmentHandLeft},
		wowhead.SlotShirt:       {},
		wowhead.SlotTabard:      {},
		wowhead.SlotWrist:       {},
		wowhead.SlotRobe:        {},
		wowhead.SlotShield:      {animmap.WoWAttachmentShield},
		wowhead.SlotRanged:      {animmap.WoWAttachmentHandLeft},
		wowhead.SlotHoldable:    {animmap.WoWAttachmentHandRight},
		wowhead.SlotRangedRight: {animmap.WoWAttachmentHandLeft},
	}
	attachmentSlotOrder := []wowhead.EquipmentSlot{
		wowhead.SlotHead, wowhead.SlotShoulder, wowhead.SlotShirt, wowhead.SlotChest,
		wowhead.SlotWaist, wowhead.SlotLegs, wowhead.SlotFeet, wowhead.SlotWrist,
		wowhead.SlotHands, wowhead.SlotMainHand, wowhead.SlotOffHand, wowhead.SlotShield,
		wowhead.SlotRanged, wowhead.SlotCloak, wowhead.SlotTabard, wowhead.SlotRobe,
		wowhead.SlotHoldable, wowhead.SlotRangedRight,
	}

	isWeapon := func(slot EquipmentSlotData) bool {
		switch slot.Data.InventoryType {
		case InventoryWeapon, InventoryShield, InventoryRanged, InventoryRangedRight,
			InventoryTwoHanded, InventoryWeaponMainHand, InventoryWeaponOffHand,
			InventoryHoldable, InventoryThrown, InventoryRelic:
			return true
		default:
			return false
		}
	}
	backSheath := ctx.ForceSheathed
	if backSheath {
		for _, s := range equipmentSlots {
			if s.Data.InventoryType == InventoryShield || s.Data.InventoryType == InventoryRanged {
				backSheath = true
				break
			}
		}
	}

	for _, slotID := range attachmentSlotOrder {
		attachmentIDs, ok := attachmentList[slotID]
		if !ok {
			continue
		}
		var slot *EquipmentSlotData
		for i := range equipmentSlots {
			if equipmentSlots[i].SlotID == slotID {
				slot = &equipmentSlots[i]
				break
			}
		}
		if slot == nil {
			continue
		}
		if isWeapon(*slot) {
			inv := slot.Data.InventoryType
			if slotID == wowhead.SlotMainHand && ctx.WeaponInventoryTypes[0] == nil {
				ctx.WeaponInventoryTypes[0] = &inv
			}
			if slotID == wowhead.SlotOffHand && ctx.WeaponInventoryTypes[1] == nil {
				ctx.WeaponInventoryTypes[1] = &inv
			}
		}
		for i := range slot.Data.ModelFiles {
			attachmentID := animmap.WoWAttachmentID(-1)
			if len(attachmentIDs) > 0 {
				attachmentID = attachmentIDs[0]
			}
			if i < len(attachmentIDs) {
				attachmentID = attachmentIDs[i]
			}
			if slotID == wowhead.SlotOffHand && slot.Data.InventoryType == InventoryShield {
				attachmentID = animmap.WoWAttachmentShield
			}
			if slotID == wowhead.SlotMainHand && slot.Data.InventoryType == InventoryRanged {
				attachmentID = animmap.WoWAttachmentHandLeft
			}
			if ctx.ForceSheathed {
				log.Printf("attachmentId before sheathing %s", animmap.GetWoWAttachmentName(int(attachmentID)))
				log.Printf("slot.data.inventoryType %d", slot.Data.InventoryType)
				if slot.Data.InventoryType == InventoryRanged || slot.Data.InventoryType == InventoryShield ||
					attachmentID == animmap.WoWAttachmentShield {
					attachmentID = animmap.WoWAttachmentSheathShield
				}
				if attachmentID == animmap.WoWAttachmentHandRight {
					if backSheath {
						attachmentID = animmap.WoWAttachmentHipWeaponLeft
					} else {
						attachmentID = animmap.WoWAttachmentSheathMainHand
					}
				}
				if attachmentID == animmap.WoWAttachmentHandLeft {
					if backSheath {
						attachmentID = animmap.WoWAttachmentHipWeaponRight
					} else {
						attachmentID = animmap.WoWAttachmentSheathOffHand
					}
				}
				log.Printf("attachmentId after sheathing %s", animmap.GetWoWAttachmentName(int(attachmentID)))
			}
			if err := attachItemModel(ctx, charMdl, equipmentSlots, *slot, i, attachmentID, collectionTemplates, metadata); err != nil {
				return err
			}
		}
	}
	return nil
}

func logAttachResult(attachmentID animmap.WoWAttachmentID, itemMdl *mdl.MDL, ok bool, fileDataID int) {
	attachmentName := "collection"
	if attachmentID >= 0 {
		attachmentName = animmap.GetWoWAttachmentName(int(attachmentID))
	}
	itemName := filepath.Base(itemMdl.Model.Name)
	if ok {
		log.Printf("Attach %s -> %s (%d)", attachmentName, itemName, fileDataID)
	} else {
		log.Printf("%s", ansi.Redf("Failed to attach %s -> %s", attachmentName, itemName))
	}
}

func attachItemModel(ctx *ExportContext, charMdl *mdl.MDL, equipmentSlots []EquipmentSlotData, slotData EquipmentSlotData, idx int, attachmentID animmap.WoWAttachmentID, templates map[int]*commonModel, metadata CharacterData) error {
	itemData := slotData.Data
	if idx >= len(itemData.ModelFiles) {
		return nil
	}
	texIdx := 0
	if idx < len(itemData.ModelTextureFiles) {
		texIdx = idx
	}
	_ = texIdx
	fileDataID := itemData.ModelFiles[idx].FileDataID
	replaceable := map[string]int{}
	if idx < len(itemData.ModelTextureFiles) && len(itemData.ModelTextureFiles[idx]) > 0 {
		for _, f := range itemData.ModelTextureFiles[idx] {
			replaceable[strconv.Itoa(f.ComponentID)] = f.FileDataID
		}
	} else if len(itemData.ModelTextureFiles[0]) > 0 {
		for _, f := range itemData.ModelTextureFiles[0] {
			replaceable[strconv.Itoa(f.ComponentID)] = f.FileDataID
		}
	}

	if template, ok := templates[fileDataID]; ok {
		enabled := FilterCollectionGeosets(equipmentSlots, slotData, template.MDL)
		forked := mdl.ForkCollectionModel(mdl.CollectionModel{RelativePath: template.RelativePath, MDL: template.MDL}, enabled)
		itemMdl := forked.MDL
		if err := ApplyReplaceableTextures(ctx, itemMdl, replaceable); err != nil {
			return err
		}
		charMdl.Modify.AddMdlCollectionItemToModel(itemMdl)
		logAttachResult(attachmentID, itemMdl, true, fileDataID)
		return nil
	}

	exported, err := ExportModelFileIDAsMdl(ctx, fileDataID, ExportModelOptions{})
	if err != nil {
		return err
	}
	isCollection := mdl.CanAddMdlCollectionItemToModel(charMdl, exported.MDL)
	var itemMdl *mdl.MDL
	if isCollection {
		templates[fileDataID] = exported
		enabled := FilterCollectionGeosets(equipmentSlots, slotData, exported.MDL)
		forked := mdl.ForkCollectionModel(mdl.CollectionModel{RelativePath: exported.RelativePath, MDL: exported.MDL}, enabled)
		itemMdl = forked.MDL
	} else {
		itemMdl = exported.MDL
	}
	if err := ApplyReplaceableTextures(ctx, itemMdl, replaceable); err != nil {
		return err
	}
	if isCollection {
		charMdl.Modify.AddMdlCollectionItemToModel(itemMdl)
		logAttachResult(attachmentID, itemMdl, true, fileDataID)
		return nil
	}
	if attachmentID < 0 {
		return nil
	}
	var attachment *components.WowAttachment
	for i := range charMdl.WowAttachments {
		if charMdl.WowAttachments[i].WowAttachmentID == int(attachmentID) {
			attachment = &charMdl.WowAttachments[i]
			break
		}
	}
	if attachment == nil {
		log.Printf("%s", ansi.Redf("Cannot find bone for wow attachment %d (%s)", attachmentID, animmap.GetWoWAttachmentName(int(attachmentID))))
		log.Printf("%s", ansi.Redf("No WoW attachments data found in this model %s", charMdl.Model.Name))
		logAttachResult(attachmentID, itemMdl, false, fileDataID)
		return nil
	}
	if attachmentID == animmap.WoWAttachmentHandLeft && (itemData.Flags&256) != 0 {
		itemMdl.Modify.FlipY()
	}
	for _, effect := range metadata.ItemEffects {
		if effect.Slot == int(slotData.SlotID) {
			log.Printf("Apply item visual %d", effect.Model)
			visual, err := ExportModelFileIDAsMdl(ctx, effect.Model, ExportModelOptions{})
			if err != nil {
				return err
			}
			for _, a := range itemMdl.WowAttachments {
				itemMdl.Modify.AddMdlItemToBone(cloneMDL(visual.MDL), a.Bone)
			}
		}
	}
	charMdl.Modify.AddMdlItemToBone(itemMdl, attachment.Bone)
	logAttachResult(attachmentID, itemMdl, true, fileDataID)
	return nil
}

func cloneMDL(src *mdl.MDL) *mdl.MDL {
	data, err := json.Marshal(src)
	if err != nil {
		return src
	}
	var out mdl.MDL
	if err := json.Unmarshal(data, &out); err != nil {
		return src
	}
	out.Modify = mdl.NewModify(&out)
	return &out
}

func applyCloakTexture(ctx *ExportContext, charMdl *mdl.MDL, slots []EquipmentSlotData) error {
	var cloakSlot *EquipmentSlotData
	for i := range slots {
		if slots[i].SlotID == wowhead.SlotCloak {
			cloakSlot = &slots[i]
			break
		}
	}
	if cloakSlot == nil || len(cloakSlot.Data.BodyTextureFiles) == 0 {
		return nil
	}
	texPath, err := ExportTexture(ctx, cloakSlot.Data.BodyTextureFiles[0].FileDataID)
	if err != nil {
		return err
	}
	ctx.AssetManager.AddPngTexture(texPath, false)
	tex := &components.Texture{
		ID: len(charMdl.Textures), Image: filepath.ToSlash(filepath.Join(ctx.Config.AssetPrefix, texPath)),
		WowData: components.TextureWowData{Type: 0, PngPath: texPath},
	}
	tex.Image = strings.Replace(tex.Image, ".png", ".blp", 1)
	charMdl.Textures = append(charMdl.Textures, tex)
	mat := &components.Material{
		ID: len(charMdl.Materials), ConstantColor: false, TwoSided: true,
		Layers: []components.Layer{{
			FilterMode: components.BlendTransparent, Texture: charMdl.Textures[len(charMdl.Textures)-1],
			TwoSided: true, Alpha: components.AnimatedOrStatic[float64]{Static: true, Value: 1},
		}},
	}
	charMdl.Materials = append(charMdl.Materials, mat)
	for i := range charMdl.Geosets {
		if strings.Contains(charMdl.Geosets[i].Name, "Cloak") {
			charMdl.Geosets[i].Material = charMdl.Materials[len(charMdl.Materials)-1]
		}
	}
	return nil
}

func applyPrebakedTexture(ctx *ExportContext, charMdl *mdl.MDL, prep characterPrep) error {
	if prep.PrebakedTexture == nil {
		return nil
	}
	texPath, err := ExportTexture(ctx, *prep.PrebakedTexture)
	if err != nil {
		return err
	}
	ctx.AssetManager.AddPngTexture(texPath, false)
	log.Printf("Character has prebaked texture %d %s", *prep.PrebakedTexture, texPath)
	newPath := filepath.ToSlash(filepath.Join(ctx.Config.AssetPrefix, texPath))
	newPath = strings.Replace(newPath, ".png", ".blp", 1)
	for i := range charMdl.Textures {
		if isPrebakedBaseTextureType(charMdl.Textures[i].WowData.Type) {
			charMdl.Textures[i].Image = newPath
			charMdl.Textures[i].WowData.PngPath = texPath
		}
	}
	for i := range charMdl.Geosets {
		for j := range charMdl.Geosets[i].Material.Layers {
			layer := &charMdl.Geosets[i].Material.Layers[j]
			if layer.Texture != nil && isPrebakedBaseTextureType(layer.Texture.WowData.Type) {
				layer.Texture.Image = newPath
				layer.Texture.WowData.PngPath = texPath
			}
		}
	}
	return nil
}

func isPrebakedBaseTextureType(textureType int) bool {
	return textureType == 1
}

func applyEquipmentsBodyTextures(ctx *ExportContext, charMdl *mdl.MDL, prep characterPrep, expansion wowhead.Expansion) error {
	if prep.PrebakedTexture != nil {
		return nil
	}
	nonCloak := 0
	for _, s := range prep.EquipmentSlots {
		if s.SlotID != wowhead.SlotCloak {
			nonCloak++
		}
	}
	if nonCloak == 0 {
		return nil
	}
	var baseTexture *components.Texture
	for i := range charMdl.Textures {
		if charMdl.Textures[i].WowData.Type == 1 {
			baseTexture = charMdl.Textures[i]
			break
		}
	}
	if baseTexture == nil {
		log.Printf("%s", ansi.Yellow("Character has no base texture with wowData.type === 1. Skipping body texture baking."))
		return nil
	}
	if baseTexture.Image == "" {
		return fmt.Errorf("cannot find the model's base texture (expansion %s may not match installed WoW build)", expansion)
	}
	log.Printf("Character has no prebaked texture. Using default texture: %s", baseTexture.WowData.PngPath)
	charCus, err := wowhead.FetchCharacterCustomization(ctx.WowheadHTTP(), expansion, prep.ChrModelID)
	if err != nil {
		return err
	}
	slotPriority := map[wowhead.EquipmentSlot]int{
		wowhead.SlotHead: 16, wowhead.SlotShoulder: 15, wowhead.SlotShirt: 1,
		wowhead.SlotChest: 7, wowhead.SlotWaist: 10, wowhead.SlotLegs: 5,
		wowhead.SlotFeet: 6, wowhead.SlotWrist: 6, wowhead.SlotHands: 8,
		wowhead.SlotMainHand: 17, wowhead.SlotOffHand: 18, wowhead.SlotTabard: 9,
	}
	type overlay struct {
		priority, componentID, fileDataID int
		section                           wowhead.TextureSectionEntry
	}
	var overlays []overlay
	for _, s := range prep.EquipmentSlots {
		if s.SlotID == wowhead.SlotCloak {
			continue
		}
		priority := slotPriority[s.SlotID]
		if s.SlotID == wowhead.SlotLegs && len(s.Data.ZamGeosetGroup) > 2 && s.Data.ZamGeosetGroup[2] > 0 {
			priority += 2
		}
		for _, f := range s.Data.BodyTextureFiles {
			overlays = append(overlays, overlay{priority: priority, componentID: f.ComponentID, fileDataID: f.FileDataID})
		}
	}
	if len(overlays) == 0 {
		return nil
	}
	sort.SliceStable(overlays, func(i, j int) bool {
		if overlays[i].priority != overlays[j].priority {
			return overlays[i].priority < overlays[j].priority
		}
		return overlays[i].componentID < overlays[j].componentID
	})
	for i := range overlays {
		found := false
		for _, sec := range charCus.TextureSections {
			if sec.SectionType == overlays[i].componentID {
				overlays[i].section = sec
				found = true
				break
			}
		}
		if !found {
			log.Printf("%s", ansi.Redf("Texture section not found for file %d component %d", overlays[i].fileDataID, overlays[i].componentID))
		}
	}
	draws := make([]pngfmt.Draw, 0, len(overlays))
	for _, t := range overlays {
		if t.section.Width == 0 && t.section.Height == 0 {
			continue
		}
		pngPath, err := ExportTexture(ctx, t.fileDataID)
		if err != nil {
			continue
		}
		absPng := filepath.Join(ctx.Config.ExportAssetDir, pngPath)
		if err := ensureTexturePNG(ctx, pngPath, absPng); err != nil {
			continue
		}
		draws = append(draws, pngfmt.Draw{
			PngPath: absPng,
			X:       t.section.X, Y: t.section.Y,
			Width: t.section.Width, Height: t.section.Height,
		})
	}
	basePng := filepath.Join(ctx.Config.ExportAssetDir, baseTexture.WowData.PngPath)
	if err := ensureTexturePNG(ctx, baseTexture.WowData.PngPath, basePng); err != nil {
		return err
	}
	newPng, err := pngfmt.DrawPngsOnBasePng(basePng, draws)
	if err != nil {
		return err
	}
	type textureDrawHash struct {
		PngPath string  `json:"pngPath"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		Width   float64 `json:"width"`
		Height  float64 `json:"height"`
	}
	textureDraws := make([]textureDrawHash, 0, len(draws))
	for _, draw := range draws {
		textureDraws = append(textureDraws, textureDrawHash{
			PngPath: draw.PngPath,
			X:       draw.X,
			Y:       draw.Y,
			Width:   draw.Width,
			Height:  draw.Height,
		})
	}
	hashInput, _ := json.Marshal(map[string]any{"basePng": basePng, "textureDraws": textureDraws})
	sum := md5.Sum(hashInput)
	newName := fmt.Sprintf("%s-%s.png", ctx.OutputFile, hex.EncodeToString(sum[:]))
	newPngPath := filepath.Join(ctx.Config.ExportAssetDir, newName)
	if err := osWriteExportAsset(newPngPath, newPng); err != nil {
		return err
	}
	rel, _ := filepath.Rel(ctx.Config.ExportAssetDir, newPngPath)
	newBlp := filepath.ToSlash(filepath.Join(ctx.Config.AssetPrefix, rel))
	newBlp = strings.Replace(newBlp, ".png", ".blp", 1)
	for i := range charMdl.Textures {
		if charMdl.Textures[i].WowData.Type == 1 {
			charMdl.Textures[i].Image = newBlp
			charMdl.Textures[i].WowData.PngPath = rel
		}
	}
	for i := range charMdl.Geosets {
		if charMdl.Geosets[i].Material == nil {
			continue
		}
		for j := range charMdl.Geosets[i].Material.Layers {
			layer := &charMdl.Geosets[i].Material.Layers[j]
			if layer.Texture != nil && layer.Texture.WowData.Type == 1 {
				layer.Texture.Image = newBlp
				layer.Texture.WowData.PngPath = rel
			}
		}
	}
	ctx.AssetManager.AddPngTexture(rel, true)
	return nil
}

func ensureTexturePNG(ctx *ExportContext, rel, absPath string) error {
	if _, err := os.Stat(absPath); err == nil {
		return nil
	}
	source, ok := texturesource.Get(rel)
	if !ok {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return err
	}
	switch source.Kind {
	case texturesource.KindPNG:
		return os.WriteFile(absPath, source.PNG, 0o644)
	case texturesource.KindBLP:
		if ctx.WowClient == nil {
			return nil
		}
		raw, err := ctx.WowClient.DownloadCascFile(context.Background(), source.FileDataID)
		if err != nil {
			return err
		}
		img, err := blp.NewBLPImage(buffer.From(raw))
		if err != nil {
			return err
		}
		pngBuf, err := img.ToPNG(0b1111, 0)
		if err != nil {
			return err
		}
		return os.WriteFile(absPath, pngBuf.Raw(), 0o644)
	default:
		return nil
	}
}

func osWriteExportAsset(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func getExcludedAnimIDs(keepCinematic bool, attackTag animmap.AttackTag) []int {
	var excluded []int
	for animID := 0; animID < animmap.AnimNameCount(); animID++ {
		wc3 := animmap.GetWc3AnimInfo(animmap.GetWowAnimName(animID))
		if !keepCinematic && strings.Contains(wc3.WC3Name, "Cinematic") {
			excluded = append(excluded, animID)
			continue
		}
		if attackTag != "" && attackTag != animmap.AttackTagAuto && wc3.AttackTag != "" && wc3.AttackTag != string(attackTag) {
			excluded = append(excluded, animID)
		}
	}
	return excluded
}
