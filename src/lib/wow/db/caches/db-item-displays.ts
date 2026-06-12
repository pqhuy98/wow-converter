/**
 * Item display cache, ported from wow.export (src/js/db/caches/DBItemDisplays.js).
 */
import { write } from '@/lib/wow/log';

import { doOnce } from '../../formats/generics';
import { WDCReader } from '../wdc-reader';
import type { ModelDisplay } from './db-creatures';
import { getModelFileDataID } from './db-model-file-data';
import { ensureInitialized, getTextureFDIDsByMatID } from './db-texture-file-data';

const itemDisplays = new Map<number, ModelDisplay[]>();

/** Initialize item displays from ItemDisplayInfo.db2 */
export const initializeItemDisplays = doOnce('initializeItemDisplays', async () => {
  if (itemDisplays.size > 0) return;
  await ensureInitialized();

  write('Loading item textures...');
  const itemDisplayInfo = new WDCReader('DBFilesClient/ItemDisplayInfo.db2');
  await itemDisplayInfo.parse();

  // Using the texture mapping, map all model fileDataIDs to used textures.
  for (const [itemDisplayInfoID, itemDisplayInfoRow] of itemDisplayInfo.getAllRows()) {
    const modelResIDs = (itemDisplayInfoRow.ModelResourcesID as number[]).filter((e) => e > 0);
    if (modelResIDs.length === 0) continue;

    const matResIDs = (itemDisplayInfoRow.ModelMaterialResourcesID as number[]).filter((e) => e > 0);
    if (matResIDs.length === 0) continue;

    const modelFileDataIDs = getModelFileDataID(modelResIDs[0]);
    const textureFileDataIDs = getTextureFDIDsByMatID(matResIDs[0]);

    if (modelFileDataIDs !== undefined && textureFileDataIDs !== undefined) {
      for (const modelFileDataID of modelFileDataIDs) {
        const display: ModelDisplay = { ID: itemDisplayInfoID, textures: textureFileDataIDs };

        if (itemDisplays.has(modelFileDataID)) itemDisplays.get(modelFileDataID)!.push(display);
        else itemDisplays.set(modelFileDataID, [display]);
      }
    }
  }

  write('Loaded textures for %d items', itemDisplays.size);
});

/** Gets item skins from a given file data ID. */
export function getItemDisplaysByFileDataID(fileDataID: number): ModelDisplay[] | undefined {
  return itemDisplays.get(fileDataID);
}

export function getItemDisplayCacheStats(): { itemDisplays: number } {
  return { itemDisplays: itemDisplays.size };
}

export function resetItemDisplayCache(): void {
  itemDisplays.clear();
}

export default { initializeItemDisplays, getItemDisplaysByFileDataID };
