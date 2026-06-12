/**
 * ModelFileData.db2 cache, ported from wow.export (src/js/db/caches/DBModelFileData.js).
 */
import { write } from '@/lib/wow/log';

import { doOnce } from '../../formats/generics';
import { WDCReader } from '../wdc-reader';

const modelResIDToFileDataID = new Map<number, number[]>();
const fileDataIDs = new Set<number>();

/** Initialize model file data from ModelFileData.db2 */
export const initializeModelFileData = doOnce('initializeModelFileData', async () => {
  if (modelResIDToFileDataID.size > 0) return;
  write('Loading model mapping...');
  const modelFileData = new WDCReader('DBFilesClient/ModelFileData.db2');
  await modelFileData.parse();

  // Using the texture mapping, map all model fileDataIDs to used textures.
  for (const [modelFileDataID, modelFileDataRow] of modelFileData.getAllRows()) {
    // Keep a list of all FIDs for listfile unknowns.
    fileDataIDs.add(modelFileDataID);

    const modelResourcesID = modelFileDataRow.ModelResourcesID as number;
    if (modelResIDToFileDataID.has(modelResourcesID)) modelResIDToFileDataID.get(modelResourcesID)!.push(modelFileDataID);
    else modelResIDToFileDataID.set(modelResourcesID, [modelFileDataID]);
  }
  write('Loaded model mapping for %d models', modelResIDToFileDataID.size);
});

/** Retrieve model file data IDs by a model resource ID. */
export function getModelFileDataID(modelResID: number): number[] | undefined {
  return modelResIDToFileDataID.get(modelResID);
}

/** Retrieve a list of all file data IDs cached from ModelFileData.db2 */
export function getFileDataIDs(): Set<number> {
  return fileDataIDs;
}

export function getModelFileDataCacheStats(): { modelResIDs: number; fileDataIDs: number } {
  return { modelResIDs: modelResIDToFileDataID.size, fileDataIDs: fileDataIDs.size };
}

export function resetModelFileDataCache(): void {
  modelResIDToFileDataID.clear();
  fileDataIDs.clear();
}

export default { initializeModelFileData, getModelFileDataID, getFileDataIDs };
