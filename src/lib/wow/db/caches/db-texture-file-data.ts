/**
 * TextureFileData.db2 cache, ported from wow.export (src/js/db/caches/DBTextureFileData.js).
 */
import { write } from '@/lib/wow/log';

import { doOnce } from '../../formats/generics';
import { WDCReader } from '../wdc-reader';

const matResIDToFileDataID = new Map<number, number[]>();
const fileDataIDs = new Set<number>();

/** Initialize texture file data ID from TextureFileData.db2 */
export const initializeTextureFileData = doOnce('initializeTextureFileData', async () => {
  write('Loading texture mapping...');
  const textureFileData = new WDCReader('DBFilesClient/TextureFileData.db2');
  await textureFileData.parse();

  // Using the texture mapping, map all model fileDataIDs to used textures.
  for (const [textureFileDataID, textureFileDataRow] of textureFileData.getAllRows()) {
    // Keep a list of all FIDs for listfile unknowns.
    fileDataIDs.add(textureFileDataID);

    // TODO: Need to remap this to support other UsageTypes
    if (textureFileDataRow.UsageType !== 0) continue;

    const materialResourcesID = textureFileDataRow.MaterialResourcesID as number;
    if (matResIDToFileDataID.has(materialResourcesID)) matResIDToFileDataID.get(materialResourcesID)!.push(textureFileDataID);
    else matResIDToFileDataID.set(materialResourcesID, [textureFileDataID]);
  }
  write('Loaded texture mapping for %d materials', matResIDToFileDataID.size);
});

/** Retrieves texture file data IDs by a material resource ID. */
export function getTextureFDIDsByMatID(matResID: number): number[] | undefined {
  return matResIDToFileDataID.get(matResID);
}

/** Ensure texture file data is initialized. Call this before using other functions. */
export async function ensureInitialized(): Promise<void> {
  if (matResIDToFileDataID.size === 0) await initializeTextureFileData();
}

/** Retrieve a list of all file data IDs cached from TextureFileData.db2 */
export function getFileDataIDs(): Set<number> {
  return fileDataIDs;
}

export function getTextureFileDataCacheStats(): { matResIDs: number; fileDataIDs: number } {
  return { matResIDs: matResIDToFileDataID.size, fileDataIDs: fileDataIDs.size };
}

export default {
  initializeTextureFileData, ensureInitialized, getTextureFDIDsByMatID, getFileDataIDs,
};
