package character

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	archivecasc "github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

type choiceMaterialRef struct {
	ChrCustomizationMaterialID      uint32
	RelatedChrCustomizationChoiceID uint32
}

type characterLookups struct {
	chrRaceXChrModelMap            map[uint32]map[uint32]uint32
	chrModelIDToFileDataID         map[uint32]uint32
	chrModelIDToTextureLayoutID    map[uint32]uint32
	choiceToGeoset                 map[uint32][]uint32
	choiceToChrCustMaterialID      map[uint32][]choiceMaterialRef
	geosetMap                      map[uint32]uint32
	chrCustMatMap                  map[uint32]ChrCustomizationMaterialEntry
	chrModelTextureLayerMap        map[string]ChrModelTextureLayerRow
	charComponentTextureSectionMap map[uint32][]CharComponentTextureSection
	chrModelMaterialMap            map[string]ChrModelMaterialRow
}

var (
	lookupsOnce  sync.Once
	lookupsCache *characterLookups
	lookupsErr   error
)

// MetaParams are inputs for character metadata resolution.
type MetaParams struct {
	Race               int
	Gender             int
	FileDataIDOverride *int
	Customizations     map[string]int
}

// MetaResult is the resolved character metadata payload.
type MetaResult struct {
	FileDataID      int
	FileName        string
	TextureLayoutID int
	Choices         map[int]ChoiceMeta
}

// InitializeCharacterCaches loads DB2 tables and builds lookup maps.
func InitializeCharacterCaches(ctx context.Context) (*characterLookups, error) {
	lookupsOnce.Do(func() {
		log.Write("[headless] Loading DB2s and building lookup maps...")
		if err := caches.InitializeCreatureData(ctx); err != nil {
			lookupsErr = err
			return
		}

		tfdDB := db.NewWDCReader("DBFilesClient/TextureFileData.db2", nil)
		chrModelDB := db.NewWDCReader("DBFilesClient/ChrModel.db2", nil)
		chrCustElementDB := db.NewWDCReader("DBFilesClient/ChrCustomizationElement.db2", nil)
		chrCustMatDB := db.NewWDCReader("DBFilesClient/ChrCustomizationMaterial.db2", nil)
		chrCustChoiceDB := db.NewWDCReader("DBFilesClient/ChrCustomizationChoice.db2", nil)
		chrCustGeosetDB := db.NewWDCReader("DBFilesClient/ChrCustomizationGeoset.db2", nil)
		chrModelTextureLayerDB := db.NewWDCReader("DBFilesClient/ChrModelTextureLayer.db2", nil)
		charComponentTextureSectionDB := db.NewWDCReader("DBFilesClient/CharComponentTextureSections.db2", nil)
		chrModelMaterialDB := db.NewWDCReader("DBFilesClient/ChrModelMaterial.db2", nil)
		chrRaceXChrModelDB := db.NewWDCReader("DBFilesClient/ChrRaceXChrModel.db2", nil)

		tables := []*db.WDCReader{
			tfdDB, chrModelDB, chrCustElementDB, chrCustGeosetDB, chrCustMatDB,
			chrModelTextureLayerDB, chrModelMaterialDB, chrRaceXChrModelDB, chrCustChoiceDB,
		}
		for _, table := range tables {
			if err := table.Parse(ctx, nil); err != nil {
				lookupsErr = err
				return
			}
		}
		if err := charComponentTextureSectionDB.Parse(ctx, nil); err != nil {
			lookupsErr = err
			return
		}

		tfdMap := make(map[uint32]uint32)
		for _, tfdRow := range tfdDB.GetAllRows() {
			if rowUint32(tfdRow["UsageType"]) != 0 {
				continue
			}
			tfdMap[rowUint32(tfdRow["MaterialResourcesID"])] = rowUint32(tfdRow["FileDataID"])
		}

		lookups := &characterLookups{
			chrRaceXChrModelMap:            make(map[uint32]map[uint32]uint32),
			chrModelIDToFileDataID:         make(map[uint32]uint32),
			chrModelIDToTextureLayoutID:    make(map[uint32]uint32),
			choiceToGeoset:                 make(map[uint32][]uint32),
			choiceToChrCustMaterialID:      make(map[uint32][]choiceMaterialRef),
			geosetMap:                      make(map[uint32]uint32),
			chrCustMatMap:                  make(map[uint32]ChrCustomizationMaterialEntry),
			chrModelTextureLayerMap:        make(map[string]ChrModelTextureLayerRow),
			charComponentTextureSectionMap: make(map[uint32][]CharComponentTextureSection),
			chrModelMaterialMap:            make(map[string]ChrModelMaterialRow),
		}

		for chrModelID, chrModelRow := range chrModelDB.GetAllRows() {
			if fileDataID, ok := caches.GetFileDataIDByDisplayID(rowUint32(chrModelRow["DisplayID"])); ok {
				lookups.chrModelIDToFileDataID[chrModelID] = fileDataID
			}
			lookups.chrModelIDToTextureLayoutID[chrModelID] = rowUint32(chrModelRow["CharComponentTextureLayoutID"])
		}

		for _, row := range chrCustElementDB.GetAllRows() {
			choiceID := rowUint32(row["ChrCustomizationChoiceID"])
			if rowUint32(row["ChrCustomizationGeosetID"]) != 0 {
				lookups.choiceToGeoset[choiceID] = append(lookups.choiceToGeoset[choiceID], rowUint32(row["ChrCustomizationGeosetID"]))
			}
			if rowUint32(row["ChrCustomizationMaterialID"]) != 0 {
				ref := choiceMaterialRef{
					ChrCustomizationMaterialID:      rowUint32(row["ChrCustomizationMaterialID"]),
					RelatedChrCustomizationChoiceID: rowUint32(row["RelatedChrCustomizationChoiceID"]),
				}
				lookups.choiceToChrCustMaterialID[choiceID] = append(lookups.choiceToChrCustMaterialID[choiceID], ref)
			}
		}

		for _, row := range chrCustMatDB.GetAllRows() {
			lookups.chrCustMatMap[rowUint32(row["ID"])] = ChrCustomizationMaterialEntry{
				ChrModelTextureTargetID: int(rowUint32(row["ChrModelTextureTargetID"])),
				FileDataID:              int(tfdMap[rowUint32(row["MaterialResourcesID"])]),
			}
		}

		for _, row := range chrCustChoiceDB.GetAllRows() {
			choiceID := rowUint32(row["ChrCustomizationChoiceID"])
			if rowUint32(row["ChrCustomizationGeosetID"]) != 0 {
				lookups.choiceToGeoset[choiceID] = append(lookups.choiceToGeoset[choiceID], rowUint32(row["ChrCustomizationGeosetID"]))
			}
			if _, ok := lookups.choiceToChrCustMaterialID[rowUint32(row["ID"])]; !ok {
				lookups.choiceToChrCustMaterialID[rowUint32(row["ID"])] = nil
			}
		}

		for id, row := range chrCustGeosetDB.GetAllRows() {
			geoset := fmt.Sprintf("%02d%02d", rowUint32(row["GeosetType"]), rowUint32(row["GeosetID"]))
			parsed, _ := strconv.ParseUint(geoset, 10, 32)
			lookups.geosetMap[id] = uint32(parsed)
		}

		for _, row := range chrModelTextureLayerDB.GetAllRows() {
			targets := rowIntSlice(row["ChrModelTextureTargetID"])
			target := 0
			if len(targets) > 0 {
				target = targets[0]
			}
			key := fmt.Sprintf("%d-%d", rowUint32(row["CharComponentTextureLayoutsID"]), target)
			lookups.chrModelTextureLayerMap[key] = ChrModelTextureLayerRow{
				ID:                            int(rowUint32(row["ID"])),
				CharComponentTextureLayoutsID: int(rowUint32(row["CharComponentTextureLayoutsID"])),
				TextureType:                   int(rowUint32(row["TextureType"])),
				Layer:                         int(rowUint32(row["Layer"])),
				Flags:                         int(rowUint32(row["Flags"])),
				BlendMode:                     int(rowUint32(row["BlendMode"])),
				TextureSectionTypeBitMask:     int(rowInt32(row["TextureSectionTypeBitMask"])),
				TextureSectionTypeBitMask2:    int(rowInt32(row["TextureSectionTypeBitMask2"])),
				ChrModelTextureTargetID:       targets,
				Field90134365006:              rowIntSlice(row["Field_9_0_1_34365_006"]),
			}
		}

		for id, row := range charComponentTextureSectionDB.GetAllRows() {
			layoutID := rowUint32(row["CharComponentTextureLayoutID"])
			sectionType := int(rowUint32(row["SectionType"]))
			rowID := int(id)
			layoutIDInt := int(layoutID)
			overlap := int(rowUint32(row["OverlapSectionMask"]))
			lookups.charComponentTextureSectionMap[layoutID] = append(lookups.charComponentTextureSectionMap[layoutID], CharComponentTextureSection{
				ID:                           &rowID,
				CharComponentTextureLayoutID: &layoutIDInt,
				SectionType:                  &sectionType,
				X:                            int(rowUint32(row["X"])),
				Y:                            int(rowUint32(row["Y"])),
				Width:                        int(rowUint32(row["Width"])),
				Height:                       int(rowUint32(row["Height"])),
				OverlapSectionMask:           &overlap,
			})
		}

		for id, row := range chrModelMaterialDB.GetAllRows() {
			key := fmt.Sprintf("%d-%d", rowUint32(row["CharComponentTextureLayoutsID"]), rowUint32(row["TextureType"]))
			lookups.chrModelMaterialMap[key] = ChrModelMaterialRow{
				ID:                            int(id),
				CharComponentTextureLayoutsID: int(rowUint32(row["CharComponentTextureLayoutsID"])),
				TextureType:                   int(rowUint32(row["TextureType"])),
				Width:                         int(rowUint32(row["Width"])),
				Height:                        int(rowUint32(row["Height"])),
				Flags:                         int(rowUint32(row["Flags"])),
				Field90134615006:              int(rowInt32(row["Field_9_0_1_34615_006"])),
			}
		}

		for _, row := range chrRaceXChrModelDB.GetAllRows() {
			raceID := rowUint32(row["ChrRacesID"])
			if lookups.chrRaceXChrModelMap[raceID] == nil {
				lookups.chrRaceXChrModelMap[raceID] = make(map[uint32]uint32)
			}
			lookups.chrRaceXChrModelMap[raceID][rowUint32(row["Sex"])] = rowUint32(row["ChrModelID"])
		}

		lookupsCache = lookups
		log.Write("[headless] DB2s loaded and lookups built.")
	})
	return lookupsCache, lookupsErr
}

// GetCharacterMeta resolves model fileDataID and per-choice geosets/materials.
func GetCharacterMeta(ctx context.Context, params MetaParams) (MetaResult, error) {
	lookups, err := InitializeCharacterCaches(ctx)
	if err != nil {
		return MetaResult{}, err
	}
	if lookups == nil {
		return MetaResult{}, fmt.Errorf("character lookups unavailable")
	}

	modelMap, ok := lookups.chrRaceXChrModelMap[uint32(params.Race)]
	if !ok {
		return MetaResult{}, fmt.Errorf("Invalid race")
	}
	modelID, ok := modelMap[uint32(params.Gender)]
	if !ok {
		return MetaResult{}, fmt.Errorf("Invalid gender for race")
	}

	var fileDataID uint32
	if params.FileDataIDOverride != nil {
		fileDataID = uint32(*params.FileDataIDOverride)
	} else {
		var okFDID bool
		fileDataID, okFDID = lookups.chrModelIDToFileDataID[modelID]
		if !okFDID || fileDataID == 0 {
			return MetaResult{}, fmt.Errorf("No fileDataID for model (modelID: %d)", modelID)
		}
	}

	textureLayoutID, ok := lookups.chrModelIDToTextureLayoutID[modelID]
	if !ok || textureLayoutID == 0 {
		return MetaResult{}, fmt.Errorf("No textureLayoutID for model")
	}

	choiceIDs := make(map[int]struct{})
	allChoiceValues := make([]int, 0, len(params.Customizations))
	for _, v := range params.Customizations {
		choiceIDs[v] = struct{}{}
		allChoiceValues = append(allChoiceValues, v)
	}

	choices := make(map[int]ChoiceMeta)
	for choiceID := range choiceIDs {
		geosets := make([]int, 0)
		for _, chrCustGeoID := range lookups.choiceToGeoset[uint32(choiceID)] {
			if geosetID, ok := lookups.geosetMap[chrCustGeoID]; ok {
				geosets = append(geosets, int(geosetID))
			}
		}

		materials := make([]CharMetaChoiceMaterial, 0)
		for _, chrCustMatID := range lookups.choiceToChrCustMaterialID[uint32(choiceID)] {
			if chrCustMatID.RelatedChrCustomizationChoiceID != 0 &&
				!containsInt(allChoiceValues, int(chrCustMatID.RelatedChrCustomizationChoiceID)) {
				continue
			}
			chrCustMat, ok := lookups.chrCustMatMap[chrCustMatID.ChrCustomizationMaterialID]
			if !ok {
				continue
			}
			layerKey := fmt.Sprintf("%d-%d", textureLayoutID, chrCustMat.ChrModelTextureTargetID)
			chrModelTextureLayer, ok := lookups.chrModelTextureLayerMap[layerKey]
			if !ok {
				continue
			}
			matKey := fmt.Sprintf("%d-%d", textureLayoutID, chrModelTextureLayer.TextureType)
			chrModelMaterial, ok := lookups.chrModelMaterialMap[matKey]
			if !ok {
				continue
			}

			var section *CharComponentTextureSection
			if chrModelTextureLayer.TextureSectionTypeBitMask == -1 {
				section = &CharComponentTextureSection{
					X: 0, Y: 0, Width: chrModelMaterial.Width, Height: chrModelMaterial.Height,
				}
			} else {
				for _, candidate := range lookups.charComponentTextureSectionMap[textureLayoutID] {
					if candidate.SectionType != nil &&
						(1<<*candidate.SectionType)&chrModelTextureLayer.TextureSectionTypeBitMask != 0 {
						copyCandidate := candidate
						section = &copyCandidate
						break
					}
				}
			}

			var filename *string
			if name, ok := archivecasc.GetByID(chrCustMat.FileDataID); ok {
				filename = &name
			}

			materials = append(materials, CharMetaChoiceMaterial{
				CustMaterial: chrCustMat,
				TextureLayer: chrModelTextureLayer,
				Material:     chrModelMaterial,
				Section:      section,
				Filename:     filename,
			})
		}
		choices[choiceID] = ChoiceMeta{Geosets: geosets, Materials: materials}
	}

	fileName, _ := archivecasc.GetByID(int(fileDataID))
	return MetaResult{
		FileDataID:      int(fileDataID),
		FileName:        fileName,
		TextureLayoutID: int(textureLayoutID),
		Choices:         choices,
	}, nil
}

// CacheStats returns character cache sizes for diagnostics.
func CacheStats() map[string]any {
	if lookupsCache == nil {
		return map[string]any{"initialized": false}
	}
	return map[string]any{
		"initialized":                 true,
		"chrRaceXChrModel":            len(lookupsCache.chrRaceXChrModelMap),
		"chrModelIDToFileDataID":      len(lookupsCache.chrModelIDToFileDataID),
		"choiceToGeoset":              len(lookupsCache.choiceToGeoset),
		"chrCustMat":                  len(lookupsCache.chrCustMatMap),
		"chrModelTextureLayer":        len(lookupsCache.chrModelTextureLayerMap),
		"charComponentTextureSection": len(lookupsCache.charComponentTextureSectionMap),
	}
}

// ResetCharacterCaches clears cached lookups (for tests / CASC unload).
func ResetCharacterCaches() {
	lookupsCache = nil
	lookupsErr = nil
	lookupsOnce = sync.Once{}
}

func init() {
	runtimecache.RegisterWowDataServerClearHook(ResetCharacterCaches)
}

func containsInt(values []int, target int) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func rowUint32(v db.DB2FieldValue) uint32 {
	switch x := v.(type) {
	case uint32:
		return x
	case int32:
		return uint32(x)
	case int64:
		return uint32(x)
	case uint64:
		return uint32(x)
	case int:
		return uint32(x)
	case float32:
		return uint32(x)
	case float64:
		return uint32(x)
	default:
		return 0
	}
}

func rowInt32(v db.DB2FieldValue) int32 {
	switch x := v.(type) {
	case int32:
		return x
	case uint32:
		return int32(x)
	case int64:
		return int32(x)
	case int:
		return int32(x)
	case float64:
		return int32(x)
	default:
		return 0
	}
}

func rowIntSlice(v db.DB2FieldValue) []int {
	switch x := v.(type) {
	case []int:
		return x
	case []int32:
		out := make([]int, len(x))
		for i, n := range x {
			out[i] = int(n)
		}
		return out
	case []int64:
		out := make([]int, len(x))
		for i, n := range x {
			out[i] = int(n)
		}
		return out
	case []uint32:
		out := make([]int, len(x))
		for i, n := range x {
			out[i] = int(n)
		}
		return out
	default:
		return nil
	}
}
