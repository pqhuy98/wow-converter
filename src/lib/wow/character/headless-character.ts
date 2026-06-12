/**
 * Headless character metadata, ported from wow.export (src/js/ui/headless-character.js).
 * Resolves DB2-derived character lookups (model fileDataID, geosets, bake
 * layers) for the converter-side direct pipeline (/rest/charMeta).
 */
import { getFileDataIDByDisplayID, initializeCreatureData } from '@/lib/wow/db/caches/db-creatures';
import { WDCReader } from '@/lib/wow/db/wdc-reader';
import { write } from '@/lib/wow/log';

import * as listfile from '../archive/casc/listfile';
import { doOnce } from '../formats/generics';
import {
  CharComponentTextureSection,
  ChrCustomizationMaterialEntry,
  ChrModelMaterialRow,
  ChrModelTextureLayerRow,
} from './char-material-renderer';

interface ChoiceMaterialRef {
  ChrCustomizationMaterialID: number;
  RelatedChrCustomizationChoiceID: number;
}

export interface CharacterLookups {
  chrRaceXChrModelMap: Map<number, Map<number, number>>;
  chrModelIDToFileDataID: Map<number, number | undefined>;
  chrModelIDToTextureLayoutID: Map<number, number>;
  choiceToGeoset: Map<number, number[]>;
  choiceToChrCustMaterialID: Map<number, ChoiceMaterialRef[]>;
  choiceToSkinnedModel: Map<number, number>;
  unsupportedChoices: number[];
  geosetMap: Map<number, number>;
  chrCustMatMap: Map<number, ChrCustomizationMaterialEntry>;
  chrModelTextureLayerMap: Map<string, ChrModelTextureLayerRow>;
  charComponentTextureSectionMap: Map<number, CharComponentTextureSection[]>;
  chrModelMaterialMap: Map<string, ChrModelMaterialRow>;
  chrCustSkinnedModelMap: Map<number, unknown>;
}

let lookupsCache: CharacterLookups | null = null;

export const initializeCharacterCaches = doOnce('initializeCharacterCaches', async (): Promise<CharacterLookups> => {
  if (lookupsCache) return lookupsCache;
  write('[headless] Loading DB2s and building lookup maps...');

  const tfdDB = new WDCReader('DBFilesClient/TextureFileData.db2');
  const chrModelDB = new WDCReader('DBFilesClient/ChrModel.db2');
  const chrCustElementDB = new WDCReader('DBFilesClient/ChrCustomizationElement.db2');
  const chrCustMatDB = new WDCReader('DBFilesClient/ChrCustomizationMaterial.db2');
  const chrCustChoiceDB = new WDCReader('DBFilesClient/ChrCustomizationChoice.db2');
  const chrCustGeosetDB = new WDCReader('DBFilesClient/ChrCustomizationGeoset.db2');
  const chrModelTextureLayerDB = new WDCReader('DBFilesClient/ChrModelTextureLayer.db2');
  const charComponentTextureSectionDB = new WDCReader('DBFilesClient/CharComponentTextureSections.db2');
  const chrModelMaterialDB = new WDCReader('DBFilesClient/ChrModelMaterial.db2');
  const chrRaceXChrModelDB = new WDCReader('DBFilesClient/ChrRaceXChrModel.db2');
  await Promise.all([
    initializeCreatureData(),
    tfdDB.parse(),
    chrModelDB.parse(),
    chrCustElementDB.parse(),
    chrCustGeosetDB.parse(),
    chrCustMatDB.parse(),
    chrModelTextureLayerDB.parse(),
    chrModelMaterialDB.parse(),
    chrRaceXChrModelDB.parse(),
    chrCustChoiceDB.parse(),
  ]);

  // Lookup maps
  const chrRaceXChrModelMap = new Map<number, Map<number, number>>();
  const chrModelIDToFileDataID = new Map<number, number | undefined>();
  const chrModelIDToTextureLayoutID = new Map<number, number>();
  const choiceToGeoset = new Map<number, number[]>();
  const choiceToChrCustMaterialID = new Map<number, ChoiceMaterialRef[]>();
  const choiceToSkinnedModel = new Map<number, number>();
  const unsupportedChoices: number[] = [];
  const geosetMap = new Map<number, number>();
  const chrCustMatMap = new Map<number, ChrCustomizationMaterialEntry>();
  const chrModelTextureLayerMap = new Map<string, ChrModelTextureLayerRow>();
  const charComponentTextureSectionMap = new Map<number, CharComponentTextureSection[]>();
  const chrModelMaterialMap = new Map<string, ChrModelMaterialRow>();
  const chrCustSkinnedModelMap = new Map<number, unknown>();

  const tfdMap = new Map<number, number>();
  for (const tfdRow of tfdDB.getAllRows().values()) {
    if (tfdRow.UsageType !== 0) continue;
    tfdMap.set(tfdRow.MaterialResourcesID as number, tfdRow.FileDataID as number);
  }

  // ChrModel.db2
  for (const [chrModelID, chrModelRow] of chrModelDB.getAllRows()) {
    const fileDataID = getFileDataIDByDisplayID(chrModelRow.DisplayID as number);
    chrModelIDToFileDataID.set(chrModelID, fileDataID);
    chrModelIDToTextureLayoutID.set(chrModelID, chrModelRow.CharComponentTextureLayoutID as number);
  }

  // ChrCustomizationElement.db2
  for (const row of chrCustElementDB.getAllRows().values()) {
    const choiceID = row.ChrCustomizationChoiceID as number;
    if (row.ChrCustomizationGeosetID !== 0) {
      if (!choiceToGeoset.has(choiceID)) choiceToGeoset.set(choiceID, []);
      choiceToGeoset.get(choiceID)!.push(row.ChrCustomizationGeosetID as number);
    }
    if (row.ChrCustomizationSkinnedModelID !== 0) choiceToSkinnedModel.set(choiceID, row.ChrCustomizationSkinnedModelID as number);
    if (row.ChrCustomizationBoneSetID !== 0) unsupportedChoices.push(choiceID);
    if (row.ChrCustomizationCondModelID !== 0) unsupportedChoices.push(choiceID);
    if (row.ChrCustomizationDisplayInfoID !== 0) unsupportedChoices.push(choiceID);
    if (row.ChrCustomizationMaterialID !== 0) {
      const ref: ChoiceMaterialRef = {
        ChrCustomizationMaterialID: row.ChrCustomizationMaterialID as number,
        RelatedChrCustomizationChoiceID: row.RelatedChrCustomizationChoiceID as number,
      };
      if (choiceToChrCustMaterialID.has(choiceID)) choiceToChrCustMaterialID.get(choiceID)!.push(ref);
      else choiceToChrCustMaterialID.set(choiceID, [ref]);
    }
  }

  // ChrCustomizationMaterial.db2
  for (const row of chrCustMatDB.getAllRows().values()) {
    chrCustMatMap.set(row.ID as number, {
      ChrModelTextureTargetID: row.ChrModelTextureTargetID as number,
      FileDataID: tfdMap.get(row.MaterialResourcesID as number)!,
    });
  }

  // ChrCustomizationChoice.db2
  for (const row of chrCustChoiceDB.getAllRows().values()) {
    if (row.ChrCustomizationGeosetID) {
      const choiceID = row.ChrCustomizationChoiceID as number;
      if (!choiceToGeoset.has(choiceID)) choiceToGeoset.set(choiceID, []);
      choiceToGeoset.get(choiceID)!.push(row.ChrCustomizationGeosetID as number);
    }
    if (!choiceToChrCustMaterialID.has(row.ID as number)) choiceToChrCustMaterialID.set(row.ID as number, []);
  }

  // ChrCustomizationGeoset.db2
  for (const [id, row] of chrCustGeosetDB.getAllRows()) {
    const geoset = (row.GeosetType as number).toString().padStart(2, '0') + (row.GeosetID as number).toString().padStart(2, '0');
    geosetMap.set(id, Number(geoset));
  }

  // ChrModelTextureLayer.db2
  for (const row of chrModelTextureLayerDB.getAllRows().values()) {
    chrModelTextureLayerMap.set(`${String(row.CharComponentTextureLayoutsID)}-${(row.ChrModelTextureTargetID as number[])[0]}`, row as unknown as ChrModelTextureLayerRow);
  }

  // CharComponentTextureSections.db2
  await charComponentTextureSectionDB.parse();
  for (const row of charComponentTextureSectionDB.getAllRows().values()) {
    const layoutID = row.CharComponentTextureLayoutID as number;
    if (!charComponentTextureSectionMap.has(layoutID)) charComponentTextureSectionMap.set(layoutID, []);
    charComponentTextureSectionMap.get(layoutID)!.push(row as unknown as CharComponentTextureSection);
  }

  // ChrModelMaterial.db2
  for (const row of chrModelMaterialDB.getAllRows().values()) {
    chrModelMaterialMap.set(`${String(row.CharComponentTextureLayoutsID)}-${String(row.TextureType)}`, row as unknown as ChrModelMaterialRow);
  }

  // ChrRaceXChrModel.db2
  for (const row of chrRaceXChrModelDB.getAllRows().values()) {
    const racesID = row.ChrRacesID as number;
    if (!chrRaceXChrModelMap.has(racesID)) chrRaceXChrModelMap.set(racesID, new Map());
    chrRaceXChrModelMap.get(racesID)!.set(row.Sex as number, row.ChrModelID as number);
  }

  lookupsCache = {
    chrRaceXChrModelMap,
    chrModelIDToFileDataID,
    chrModelIDToTextureLayoutID,
    choiceToGeoset,
    choiceToChrCustMaterialID,
    choiceToSkinnedModel,
    unsupportedChoices,
    geosetMap,
    chrCustMatMap,
    chrModelTextureLayerMap,
    charComponentTextureSectionMap,
    chrModelMaterialMap,
    chrCustSkinnedModelMap,
  };
  write('[headless] DB2s loaded and lookups built.');
  return lookupsCache;
});

export function getCharacterCacheStats(): {
  initialized: boolean;
  chrRaceXChrModel: number;
  chrModelIDToFileDataID: number;
  choiceToGeoset: number;
  chrCustMat: number;
  chrModelTextureLayer: number;
  charComponentTextureSection: number;
  } {
  if (!lookupsCache) {
    return {
      initialized: false,
      chrRaceXChrModel: 0,
      chrModelIDToFileDataID: 0,
      choiceToGeoset: 0,
      chrCustMat: 0,
      chrModelTextureLayer: 0,
      charComponentTextureSection: 0,
    };
  }
  return {
    initialized: true,
    chrRaceXChrModel: lookupsCache.chrRaceXChrModelMap.size,
    chrModelIDToFileDataID: lookupsCache.chrModelIDToFileDataID.size,
    choiceToGeoset: lookupsCache.choiceToGeoset.size,
    chrCustMat: lookupsCache.chrCustMatMap.size,
    chrModelTextureLayer: lookupsCache.chrModelTextureLayerMap.size,
    charComponentTextureSection: lookupsCache.charComponentTextureSectionMap.size,
  };
}

export interface CharacterMetaParams {
  race: number;
  gender: number | string;
  fileDataIdOverride?: number;
  customizations: Record<string, number>;
}

export interface CharMetaChoiceMaterial {
  custMaterial: ChrCustomizationMaterialEntry;
  textureLayer: ChrModelTextureLayerRow;
  material: ChrModelMaterialRow;
  section: CharComponentTextureSection | null;
  filename: string | null;
}

export interface CharacterMetaResult {
  fileDataID: number;
  fileName: string;
  textureLayoutID: number;
  /** Per-choice geoset IDs (mapped) and resolved bake layers. */
  choices: Record<number, { geosets: number[]; materials: CharMetaChoiceMaterial[] }>;
}

/**
 * Resolve all DB2-derived character metadata needed by the converter-side
 * direct pipeline: model fileDataID + per-choice geosets and bake layers.
 * Mirrors the lookups in exportCharacterModelHeadless without exporting.
 */
export async function getCharacterMeta({
  race, gender, fileDataIdOverride, customizations,
}: CharacterMetaParams): Promise<CharacterMetaResult> {
  const lookups = await initializeCharacterCaches();

  const modelMap = lookups.chrRaceXChrModelMap.get(race);
  if (!modelMap) throw new Error('Invalid race');
  const genderNum = typeof gender === 'string' ? parseInt(gender, 10) : gender;
  const modelID = modelMap.get(genderNum);
  if (!modelID) throw new Error('Invalid gender for race');
  const fileDataID = fileDataIdOverride || lookups.chrModelIDToFileDataID.get(modelID);
  if (!fileDataID) throw new Error(`No fileDataID for model (modelID: ${modelID})`);
  const textureLayoutID = lookups.chrModelIDToTextureLayoutID.get(modelID);
  if (!textureLayoutID) throw new Error('No textureLayoutID for model');

  const choiceIDs = new Set(Object.values(customizations || {}).map(Number));
  const allChoiceValues = Object.values(customizations || {}).map(Number);

  const choices: CharacterMetaResult['choices'] = {};
  for (const choiceID of choiceIDs) {
    const geosets: number[] = [];
    for (const chrCustGeoID of lookups.choiceToGeoset.get(choiceID) || []) {
      const geosetId = lookups.geosetMap.get(chrCustGeoID);
      if (geosetId !== undefined) geosets.push(geosetId);
    }

    const materials: CharMetaChoiceMaterial[] = [];
    for (const chrCustMatID of lookups.choiceToChrCustMaterialID.get(choiceID) ?? []) {
      if (chrCustMatID.RelatedChrCustomizationChoiceID !== 0
        && !allChoiceValues.includes(chrCustMatID.RelatedChrCustomizationChoiceID)) continue;
      const chrCustMat = lookups.chrCustMatMap.get(chrCustMatID.ChrCustomizationMaterialID)!;
      const chrModelTextureTarget = chrCustMat.ChrModelTextureTargetID;
      const chrModelTextureLayer = lookups.chrModelTextureLayerMap.get(`${textureLayoutID}-${chrModelTextureTarget}`);
      if (!chrModelTextureLayer) continue;
      const chrModelMaterial = lookups.chrModelMaterialMap.get(`${textureLayoutID}-${chrModelTextureLayer.TextureType}`);
      if (!chrModelMaterial) continue;

      let charComponentTextureSection: CharComponentTextureSection | null = null;
      if (chrModelTextureLayer.TextureSectionTypeBitMask === -1) {
        charComponentTextureSection = {
          X: 0, Y: 0, Width: chrModelMaterial.Width, Height: chrModelMaterial.Height,
        };
      } else {
        const charComponentTextureSectionResults = lookups.charComponentTextureSectionMap.get(textureLayoutID)!;
        for (const charComponentTextureSectionRow of charComponentTextureSectionResults) {
          if ((1 << charComponentTextureSectionRow.SectionType!) & chrModelTextureLayer.TextureSectionTypeBitMask) {
            charComponentTextureSection = charComponentTextureSectionRow;
            break;
          }
        }
      }

      materials.push({
        custMaterial: chrCustMat,
        textureLayer: chrModelTextureLayer,
        material: chrModelMaterial,
        section: charComponentTextureSection,
        filename: listfile.getByID(chrCustMat.FileDataID) ?? null,
      });
    }

    choices[choiceID] = { geosets, materials };
  }

  return {
    fileDataID,
    fileName: listfile.getByID(fileDataID)!,
    textureLayoutID,
    choices,
  };
}

export default { initializeCharacterCaches };
