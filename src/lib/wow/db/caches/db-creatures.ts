/**
 * Creature display cache, ported from wow.export (src/js/db/caches/DBCreatures.js).
 */
import { write } from '@/lib/wow/log';

import { doOnce } from '../../formats/generics';
import { WDCReader } from '../wdc-reader';

/** A creature/item display entry mapped to a model fileDataID. */
export interface ModelDisplay {
  ID: number;
  modelID?: number;
  textures: number[];
  extraGeosets?: number[];
}

const creatureDisplays = new Map<number, ModelDisplay[]>();
const displayIDToFileDataID = new Map<number, number>();
let isInitialized = false;

/** Initialize creature data. */
export const initializeCreatureData = doOnce('initializeCreatureData', async () => {
  if (isInitialized) return;

  write('Loading creature textures...');

  const creatureDisplayInfo = new WDCReader('DBFilesClient/CreatureDisplayInfo.db2');
  await creatureDisplayInfo.parse();

  const creatureModelData = new WDCReader('DBFilesClient/CreatureModelData.db2');
  await creatureModelData.parse();

  const creatureGeosetMap = new Map<number, number[]>();

  const creatureDisplayInfoGeosetData = new WDCReader('DBFilesClient/CreatureDisplayInfoGeosetData.db2');
  await creatureDisplayInfoGeosetData.parse();
  // CreatureDisplayInfoID => Array of geosets to enable which should only be used if CreatureModelData.CreatureDisplayInfoGeosetData != 0
  for (const geosetRow of creatureDisplayInfoGeosetData.getAllRows().values()) {
    const displayInfoID = geosetRow.CreatureDisplayInfoID as number;
    if (!creatureGeosetMap.has(displayInfoID)) creatureGeosetMap.set(displayInfoID, []);

    creatureGeosetMap.get(displayInfoID)!.push(((geosetRow.GeosetIndex as number) + 1) * 100 + (geosetRow.GeosetValue as number));
  }

  const creatureDisplayInfoMap = new Map<number, ModelDisplay>();
  const modelIDToDisplayInfoMap = new Map<number, number[]>();

  // Map all available texture fileDataIDs to model IDs.
  for (const [displayID, displayRow] of creatureDisplayInfo.getAllRows()) {
    const modelID = displayRow.ModelID as number;
    creatureDisplayInfoMap.set(displayID, {
      ID: displayID,
      modelID,
      textures: (displayRow.TextureVariationFileDataID as number[]).filter((e) => e > 0),
    });

    if (modelIDToDisplayInfoMap.has(modelID)) modelIDToDisplayInfoMap.get(modelID)!.push(displayID);
    else modelIDToDisplayInfoMap.set(modelID, [displayID]);
  }

  // Using the texture mapping, map all model fileDataIDs to used textures.
  for (const [modelID, modelRow] of creatureModelData.getAllRows()) {
    if (modelIDToDisplayInfoMap.has(modelID)) {
      const fileDataID = modelRow.FileDataID as number;
      const displayIDs = modelIDToDisplayInfoMap.get(modelID)!;
      const modelIDHasExtraGeosets = (modelRow.CreatureGeosetDataID as number) > 0;

      for (const displayID of displayIDs) {
        displayIDToFileDataID.set(displayID, fileDataID);

        const display = creatureDisplayInfoMap.get(displayID)!;

        if (modelIDHasExtraGeosets) {
          display.extraGeosets = [];
          if (creatureGeosetMap.has(displayID)) display.extraGeosets = creatureGeosetMap.get(displayID)!;
        }

        if (creatureDisplays.has(fileDataID)) creatureDisplays.get(fileDataID)!.push(display);
        else creatureDisplays.set(fileDataID, [display]);
      }
    }
  }

  write('Loaded textures for %d creatures', creatureDisplays.size);
  isInitialized = true;
});

/** Gets creature skins from a given file data ID. */
export function getCreatureDisplaysByFileDataID(fileDataID: number): ModelDisplay[] | undefined {
  return creatureDisplays.get(fileDataID);
}

/** Gets the file data ID for a given display ID. */
export function getFileDataIDByDisplayID(displayID: number): number | undefined {
  return displayIDToFileDataID.get(displayID);
}

export function getCreatureCacheStats(): { initialized: boolean; creatureDisplays: number; displayIDToFileDataID: number } {
  return {
    initialized: isInitialized,
    creatureDisplays: creatureDisplays.size,
    displayIDToFileDataID: displayIDToFileDataID.size,
  };
}

export default { initializeCreatureData, getCreatureDisplaysByFileDataID, getFileDataIDByDisplayID };
