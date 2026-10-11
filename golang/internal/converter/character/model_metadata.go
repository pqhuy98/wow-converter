package character

import (
	"context"
	"fmt"
	"strings"

	"github.com/pqhuy98/wow-converter/internal/converter/common"
	animmap "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/animation"
	directm2 "github.com/pqhuy98/wow-converter/internal/converter/wowmodel/direct/m2"
	"github.com/pqhuy98/wow-converter/internal/wowhead"
)

// ModelMetadata resolves the base through the export resolver, without conversion.
func (e *CharacterExporter) ModelMetadata(ctx context.Context, char Character, skinID string) (directm2.ModelMetadata, error) {
	e.HTTP = e.HTTP.WithContext(ctx)
	exportCtx := e.newExportContext("", char, skinID)
	exportCtx.ctx = ctx
	if err := e.WowClient.InitModelCaches(ctx); err != nil {
		return directm2.ModelMetadata{}, err
	}
	var fileDataID int
	var weaponTypes [2]int
	switch char.Base.Type {
	case "local":
		file, err := searchModelWithSkin(ctx, e.WowClient, common.NormalizeLocalModelRef(char.Base.Value))
		if err != nil {
			return directm2.ModelMetadata{}, err
		}
		if file == nil {
			return directm2.ModelMetadata{}, fmt.Errorf("model not found")
		}
		if strings.HasSuffix(strings.ToLower(file.FileName), ".wmo") {
			return directm2.ModelMetadata{}, fmt.Errorf("sequence selection is available for M2 models")
		}
		fileDataID = file.FileDataID
	case "wowhead", "displayID":
		var zam wowhead.ZamURL
		var err error
		if char.Base.Type == "wowhead" {
			zam, err = wowhead.GetZamURLFromWowheadURL(e.HTTP, char.Base.Value)
		} else {
			fmt.Sscanf(char.Base.Value, "%d", &zam.DisplayID)
			zam.Type, zam.Expansion = wowhead.ZamTypeNPC, wowhead.ExpansionLatestAvailable
		}
		if err != nil {
			return directm2.ModelMetadata{}, err
		}
		if zam.Type == wowhead.ZamTypeItem && zam.SlotID != nil && *zam.SlotID > 0 {
			item, err := ProcessItemData(exportCtx.WowheadHTTP(), zam.Expansion, zam, 0, 2, 0)
			if err != nil {
				return directm2.ModelMetadata{}, err
			}
			if len(item.ModelFiles) == 0 {
				return directm2.ModelMetadata{}, fmt.Errorf("item has no base model")
			}
			fileDataID = item.ModelFiles[0].FileDataID
			break
		}
		meta, err := e.resolveBaseMetadata(&exportCtx, zam)
		if err != nil {
			return directm2.ModelMetadata{}, err
		}
		if meta.Character != nil {
			prep, err := prepareCharacterExport(&exportCtx, meta, exportCtx.ResolveMetaExpansion(zam.Expansion))
			if err != nil {
				return directm2.ModelMetadata{}, err
			}
			resolved, _, err := resolveCharMeta(&exportCtx, prep.RPCParams)
			if err != nil {
				return directm2.ModelMetadata{}, err
			}
			fileDataID = resolved.FileDataID
			for _, slot := range prep.EquipmentSlots {
				if !isWeaponEquipmentSlot(slot) {
					continue
				}
				if slot.SlotID == wowhead.SlotMainHand {
					weaponTypes[0] = slot.Data.InventoryType
				}
				if slot.SlotID == wowhead.SlotOffHand {
					weaponTypes[1] = slot.Data.InventoryType
				}
			}
		} else if hasResolvedModel(meta.Model) {
			fileDataID = *meta.Model
		}
	default:
		return directm2.ModelMetadata{}, fmt.Errorf("unsupported model reference")
	}
	if fileDataID <= 0 {
		return directm2.ModelMetadata{}, fmt.Errorf("base M2 model could not be resolved")
	}
	tag := char.AttackTag
	if tag == animmap.AttackTagAuto {
		for _, attachment := range []struct {
			key  string
			hand int
		}{{"1", 0}, {"2", 1}, {"0", 1}} {
			item, ok := char.AttachItems[attachment.key]
			if !ok || weaponTypes[attachment.hand] != 0 || item.Path.Type != "wowhead" {
				continue
			}
			zam, err := wowhead.GetZamURLFromWowheadURL(e.HTTP, item.Path.Value)
			if err != nil {
				continue
			}
			if zam.Type != wowhead.ZamTypeItem {
				continue
			}
			data, err := ProcessItemData(exportCtx.WowheadHTTP(), zam.Expansion, zam, 0, 2, 0)
			if err == nil {
				weaponTypes[attachment.hand] = data.InventoryType
			}
		}
		tag = GuessAttackTag(weaponTypes[0], weaponTypes[1])
	}
	return directm2.ReadModelMetadata(ctx, CascFileSource{Client: e.WowClient}, fileDataID, char.KeepCinematic, tag)
}

func (e *CharacterExporter) resolveBaseMetadata(exportCtx *ExportContext, zam wowhead.ZamURL) (CharacterData, error) {
	var meta wowhead.CharacterData
	var err error
	expansion := exportCtx.ResolveMetaExpansion(zam.Expansion)
	switch zam.Type {
	case wowhead.ZamTypeItem:
		meta, err = wowhead.FetchNpcMeta(e.HTTP, wowhead.ExpansionLatestAvailable, zam.DisplayID)
	case wowhead.ZamTypeNPC:
		meta, err = wowhead.FetchNpcMeta(e.HTTP, expansion, zam.DisplayID)
	case wowhead.ZamTypeObject:
		meta, err = wowhead.FetchObjectMeta(e.HTTP, expansion, zam.DisplayID)
	case wowhead.ZamTypeDressingRoom:
		meta, err = wowhead.DecodeDressingRoom(e.HTTP, expansion, zam.Hash)
	default:
		return CharacterData{}, fmt.Errorf("unallowed base zam type: %s", zam.Type)
	}
	if err != nil {
		return CharacterData{}, err
	}
	result := convertWowheadCharacterData(meta)
	if zam.Type == wowhead.ZamTypeNPC {
		if dbMeta, ok := resolveNpcMetaFromDB(exportCtx.Context(), exportCtx, zam.DisplayID); ok {
			result = mergeNpcMeta(result, dbMeta, exportCtx.IsClassicCASC(exportCtx.Context()))
		}
	}
	return result, nil
}
