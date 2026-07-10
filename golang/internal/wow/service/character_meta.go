package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/pqhuy98/wow-converter/internal/converter/runtimecache"
	"github.com/pqhuy98/wow-converter/internal/wow/archive/casc"
	apicasc "github.com/pqhuy98/wow-converter/internal/wow/casc"
	"github.com/pqhuy98/wow-converter/internal/wow/character"
	"github.com/pqhuy98/wow-converter/internal/wow/db"
	"github.com/pqhuy98/wow-converter/internal/wow/db/caches"
	"github.com/pqhuy98/wow-converter/internal/wow/log"
)

// CharacterMetaService resolves DB2-derived character metadata for /rest/charMeta.
type CharacterMetaService struct{}

type choiceMaterialRef struct {
	ChrCustomizationMaterialID      int
	RelatedChrCustomizationChoiceID int
}

type characterLookups struct {
	chrRaceXChrModelMap            map[int]map[int]int
	chrModelIDToFileDataID         map[int]int
	chrModelIDToTextureLayoutID    map[int]int
	choiceToGeoset                 map[int][]int
	choiceToChrCustMaterialID      map[int][]choiceMaterialRef
	geosetMap                      map[int]int
	chrCustMatMap                  map[int]character.ChrCustomizationMaterialEntry
	chrModelTextureLayerMap        map[string]character.ChrModelTextureLayerRow
	charComponentTextureSectionMap map[int][]character.CharComponentTextureSection
	chrModelMaterialMap            map[string]character.ChrModelMaterialRow
}

var (
	charLookups     *characterLookups
	charLookupsOnce sync.Once
	charLookupsErr  error
)

func (CharacterMetaService) GetCharacterMeta(ctx context.Context, params apicasc.CharacterMetaParams) (apicasc.CharacterMetaResponse, error) {
	lookups, err := initializeCharacterCaches(ctx)
	if err != nil {
		return apicasc.CharacterMetaResponse{}, err
	}

	modelMap, ok := lookups.chrRaceXChrModelMap[params.Race]
	if !ok {
		return apicasc.CharacterMetaResponse{}, fmt.Errorf("invalid race")
	}
	modelID, ok := modelMap[params.Gender]
	if !ok {
		return apicasc.CharacterMetaResponse{}, fmt.Errorf("invalid gender for race")
	}

	fileDataID := 0
	if params.FileDataIDOverride != nil {
		fileDataID = *params.FileDataIDOverride
	} else {
		fileDataID = lookups.chrModelIDToFileDataID[modelID]
	}
	if fileDataID == 0 {
		return apicasc.CharacterMetaResponse{}, fmt.Errorf("no fileDataID for model (modelID: %d)", modelID)
	}
	textureLayoutID := lookups.chrModelIDToTextureLayoutID[modelID]
	if textureLayoutID == 0 {
		return apicasc.CharacterMetaResponse{}, fmt.Errorf("no textureLayoutID for model")
	}

	choiceIDs := map[int]struct{}{}
	allChoiceValues := make([]int, 0, len(params.Customizations))
	for _, v := range params.Customizations {
		id, _ := strconv.Atoi(fmt.Sprint(v))
		choiceIDs[id] = struct{}{}
		allChoiceValues = append(allChoiceValues, id)
	}

	choices := map[string]any{}
	for choiceID := range choiceIDs {
		geosets := make([]int, 0)
		for _, chrCustGeoID := range lookups.choiceToGeoset[choiceID] {
			if geosetID, ok := lookups.geosetMap[chrCustGeoID]; ok {
				geosets = append(geosets, geosetID)
			}
		}

		materials := make([]character.CharMetaChoiceMaterial, 0)
		for _, chrCustMatID := range lookups.choiceToChrCustMaterialID[choiceID] {
			if chrCustMatID.RelatedChrCustomizationChoiceID != 0 &&
				!containsInt(allChoiceValues, chrCustMatID.RelatedChrCustomizationChoiceID) {
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

			var section *character.CharComponentTextureSection
			if chrModelTextureLayer.TextureSectionTypeBitMask == -1 {
				section = &character.CharComponentTextureSection{
					X: 0, Y: 0, Width: chrModelMaterial.Width, Height: chrModelMaterial.Height,
				}
			} else if sections, ok := lookups.charComponentTextureSectionMap[textureLayoutID]; ok {
				for i := range sections {
					s := sections[i]
					if s.SectionType != nil && (1<<*s.SectionType)&chrModelTextureLayer.TextureSectionTypeBitMask != 0 {
						sec := s
						section = &sec
						break
					}
				}
			}

			var filename *string
			if name, ok := casc.GetByID(chrCustMat.FileDataID); ok {
				filename = &name
			}
			materials = append(materials, character.CharMetaChoiceMaterial{
				CustMaterial: chrCustMat,
				TextureLayer: chrModelTextureLayer,
				Material:     chrModelMaterial,
				Section:      section,
				Filename:     filename,
			})
		}
		choices[strconv.Itoa(choiceID)] = character.ChoiceMeta{Geosets: geosets, Materials: materials}
	}

	fileName, _ := casc.GetByID(fileDataID)
	if fileName == "" {
		fileName = fmt.Sprintf("unknown/%d.m2", fileDataID)
	}

	return apicasc.CharacterMetaResponse{
		FileDataID:      fileDataID,
		FileName:        fileName,
		TextureLayoutID: textureLayoutID,
		Choices:         choices,
	}, nil
}

func initializeCharacterCaches(ctx context.Context) (*characterLookups, error) {
	charLookupsOnce.Do(func() {
		log.Write("[headless] Loading DB2s and building lookup maps...")
		if err := caches.InitializeCreatureData(ctx); err != nil {
			charLookupsErr = err
			return
		}
		source := db.RuntimeFileSource{}
		readers := []*db.WDCReader{
			db.NewWDCReader("DBFilesClient/TextureFileData.db2", source),
			db.NewWDCReader("DBFilesClient/ChrModel.db2", source),
			db.NewWDCReader("DBFilesClient/ChrCustomizationElement.db2", source),
			db.NewWDCReader("DBFilesClient/ChrCustomizationMaterial.db2", source),
			db.NewWDCReader("DBFilesClient/ChrCustomizationChoice.db2", source),
			db.NewWDCReader("DBFilesClient/ChrCustomizationGeoset.db2", source),
			db.NewWDCReader("DBFilesClient/ChrModelTextureLayer.db2", source),
			db.NewWDCReader("DBFilesClient/CharComponentTextureSections.db2", source),
			db.NewWDCReader("DBFilesClient/ChrModelMaterial.db2", source),
			db.NewWDCReader("DBFilesClient/ChrRaceXChrModel.db2", source),
		}
		for _, r := range readers {
			if err := r.Parse(ctx, nil); err != nil {
				charLookupsErr = err
				return
			}
		}

		lookups := &characterLookups{
			chrRaceXChrModelMap:            map[int]map[int]int{},
			chrModelIDToFileDataID:         map[int]int{},
			chrModelIDToTextureLayoutID:    map[int]int{},
			choiceToGeoset:                 map[int][]int{},
			choiceToChrCustMaterialID:      map[int][]choiceMaterialRef{},
			geosetMap:                      map[int]int{},
			chrCustMatMap:                  map[int]character.ChrCustomizationMaterialEntry{},
			chrModelTextureLayerMap:        map[string]character.ChrModelTextureLayerRow{},
			charComponentTextureSectionMap: map[int][]character.CharComponentTextureSection{},
			chrModelMaterialMap:            map[string]character.ChrModelMaterialRow{},
		}

		tfdMap := map[int]int{}
		for _, row := range readers[0].GetAllRows() {
			if toInt(row["UsageType"]) != 0 {
				continue
			}
			tfdMap[toInt(row["MaterialResourcesID"])] = toInt(row["FileDataID"])
		}

		for chrModelID, row := range readers[1].GetAllRows() {
			displayID := uint32(toInt(row["DisplayID"]))
			if fileDataID, ok := caches.GetFileDataIDByDisplayID(displayID); ok {
				lookups.chrModelIDToFileDataID[int(chrModelID)] = int(fileDataID)
			}
			lookups.chrModelIDToTextureLayoutID[int(chrModelID)] = toInt(row["CharComponentTextureLayoutID"])
		}

		for _, row := range sortedDBRows(readers[2].GetAllRows()) {
			choiceID := toInt(row["ChrCustomizationChoiceID"])
			if geosetID := toInt(row["ChrCustomizationGeosetID"]); geosetID != 0 {
				lookups.choiceToGeoset[choiceID] = append(lookups.choiceToGeoset[choiceID], geosetID)
			}
			if matID := toInt(row["ChrCustomizationMaterialID"]); matID != 0 {
				ref := choiceMaterialRef{
					ChrCustomizationMaterialID:      matID,
					RelatedChrCustomizationChoiceID: toInt(row["RelatedChrCustomizationChoiceID"]),
				}
				lookups.choiceToChrCustMaterialID[choiceID] = append(lookups.choiceToChrCustMaterialID[choiceID], ref)
			}
		}

		for _, id := range sortedDBRowIDs(readers[3].GetAllRows()) {
			row := readers[3].GetAllRows()[id]
			lookups.chrCustMatMap[int(id)] = character.ChrCustomizationMaterialEntry{
				ChrModelTextureTargetID: toInt(row["ChrModelTextureTargetID"]),
				FileDataID:              tfdMap[toInt(row["MaterialResourcesID"])],
			}
		}

		for _, row := range sortedDBRows(readers[4].GetAllRows()) {
			choiceID := toInt(row["ChrCustomizationChoiceID"])
			if geosetID := toInt(row["ChrCustomizationGeosetID"]); geosetID != 0 {
				lookups.choiceToGeoset[choiceID] = append(lookups.choiceToGeoset[choiceID], geosetID)
			}
			id := toInt(row["ID"])
			if _, ok := lookups.choiceToChrCustMaterialID[id]; !ok {
				lookups.choiceToChrCustMaterialID[id] = nil
			}
		}

		for _, id := range sortedDBRowIDs(readers[5].GetAllRows()) {
			row := readers[5].GetAllRows()[id]
			geoset := fmt.Sprintf("%02d%02d", toInt(row["GeosetType"]), toInt(row["GeosetID"]))
			if n, err := strconv.Atoi(geoset); err == nil {
				lookups.geosetMap[int(id)] = n
			}
		}

		for _, row := range readers[6].GetAllRows() {
			layoutID := toInt(row["CharComponentTextureLayoutsID"])
			targets := toIntSlice(row["ChrModelTextureTargetID"])
			if len(targets) == 0 {
				continue
			}
			key := fmt.Sprintf("%d-%d", layoutID, targets[0])
			lookups.chrModelTextureLayerMap[key] = character.ChrModelTextureLayerRow{
				ID:                            toInt(row["ID"]),
				CharComponentTextureLayoutsID: layoutID,
				TextureType:                   toInt(row["TextureType"]),
				Layer:                         toInt(row["Layer"]),
				Flags:                         toInt(row["Flags"]),
				BlendMode:                     toInt(row["BlendMode"]),
				TextureSectionTypeBitMask:     toInt(row["TextureSectionTypeBitMask"]),
				TextureSectionTypeBitMask2:    toInt(row["TextureSectionTypeBitMask2"]),
				ChrModelTextureTargetID:       targets,
				Field90134365006:              toIntSlice(row["Field_9_0_1_34365_006"]),
			}
		}

		for id, row := range readers[7].GetAllRows() {
			layoutID := toInt(row["CharComponentTextureLayoutID"])
			secType := toInt(row["SectionType"])
			rowID := int(id)
			overlap := toInt(row["OverlapSectionMask"])
			lookups.charComponentTextureSectionMap[layoutID] = append(
				lookups.charComponentTextureSectionMap[layoutID],
				character.CharComponentTextureSection{
					ID:                           &rowID,
					CharComponentTextureLayoutID: &layoutID,
					SectionType:                  &secType,
					X:                            toInt(row["X"]),
					Y:                            toInt(row["Y"]),
					Width:                        toInt(row["Width"]),
					Height:                       toInt(row["Height"]),
					OverlapSectionMask:           &overlap,
				},
			)
		}

		for id, row := range readers[8].GetAllRows() {
			layoutID := toInt(row["CharComponentTextureLayoutsID"])
			texType := toInt(row["TextureType"])
			key := fmt.Sprintf("%d-%d", layoutID, texType)
			lookups.chrModelMaterialMap[key] = character.ChrModelMaterialRow{
				ID:                            int(id),
				CharComponentTextureLayoutsID: layoutID,
				TextureType:                   texType,
				Width:                         toInt(row["Width"]),
				Height:                        toInt(row["Height"]),
				Flags:                         toInt(row["Flags"]),
				Field90134615006:              toInt(row["Field_9_0_1_34615_006"]),
			}
		}

		for _, row := range readers[9].GetAllRows() {
			raceID := toInt(row["ChrRacesID"])
			if lookups.chrRaceXChrModelMap[raceID] == nil {
				lookups.chrRaceXChrModelMap[raceID] = map[int]int{}
			}
			lookups.chrRaceXChrModelMap[raceID][toInt(row["Sex"])] = toInt(row["ChrModelID"])
		}

		charLookups = lookups
		log.Write("[headless] DB2s loaded and lookups built.")
	})
	return charLookups, charLookupsErr
}

// ResetCharacterMetaCaches clears character lookup maps (e.g. after CASC unload / build switch).
func ResetCharacterMetaCaches() {
	charLookups = nil
	charLookupsErr = nil
	charLookupsOnce = sync.Once{}
}

func init() {
	runtimecache.RegisterWowDataServerClearHook(ResetCharacterMetaCaches)
}

func toInt(v any) int {
	switch x := v.(type) {
	case int:
		return x
	case int8:
		return int(x)
	case int16:
		return int(x)
	case int32:
		return int(x)
	case int64:
		return int(x)
	case uint:
		return int(x)
	case uint8:
		return int(x)
	case uint16:
		return int(x)
	case uint32:
		return int(x)
	case uint64:
		return int(x)
	case float32:
		return int(x)
	case float64:
		return int(x)
	case []int64:
		if len(x) > 0 {
			return int(x[0])
		}
	case []uint32:
		if len(x) > 0 {
			return int(x[0])
		}
	}
	return 0
}

func toIntSlice(v any) []int {
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

func sortedDBRows(rows map[uint32]db.DB2Row) []db.DB2Row {
	ids := sortedDBRowIDs(rows)
	out := make([]db.DB2Row, 0, len(ids))
	for _, id := range ids {
		out = append(out, rows[id])
	}
	return out
}

func sortedDBRowIDs(rows map[uint32]db.DB2Row) []uint32 {
	ids := make([]uint32, 0, len(rows))
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func containsInt(slice []int, v int) bool {
	for _, n := range slice {
		if n == v {
			return true
		}
	}
	return false
}
