/**
 * Model skin lookups, ported from wow.export (src/js/ui/tab-models.js,
 * headless parts only: getModelDisplays / getAllSkinsForModel). Used by the
 * /rest/getModelSkins endpoint for the converter-side direct pipeline.
 */
import path from 'path';

import { getCreatureDisplaysByFileDataID, ModelDisplay } from '@/lib/wow/db/caches/db-creatures';
import { getItemDisplaysByFileDataID } from '@/lib/wow/db/caches/db-item-displays';

import * as listfile from '../../archive/casc/listfile';

export interface ModelSkin {
  id: string;
  label: string;
  displayID: number;
  extraGeosets?: number[];
  textures: number[];
}

/** Lookup model displays for items/creatures. */
export function getModelDisplays(fileDataID: number): ModelDisplay[] {
  let displays = getCreatureDisplaysByFileDataID(fileDataID);

  if (displays === undefined) displays = getItemDisplaysByFileDataID(fileDataID);

  return displays ?? [];
}

function getSkinForDisplay(modelName: string, display: ModelDisplay): ModelSkin {
  const texture = display.textures[0];

  let cleanSkinName = '';
  let skinName = listfile.getByID(texture);
  if (skinName !== undefined) {
    // Display the texture name without path/extension.
    skinName = path.basename(skinName, '.blp');
    cleanSkinName = skinName.replace(modelName, '').replace('_', '');
  } else {
    // Handle unknown textures.
    skinName = `unknown_${texture}`;
  }

  if (cleanSkinName.length === 0) cleanSkinName = 'base';

  if (display.extraGeosets && display.extraGeosets.length > 0) skinName += display.extraGeosets.join(',');

  cleanSkinName += ` (${display.ID})`;

  return {
    id: skinName,
    label: cleanSkinName,
    displayID: display.ID,
    extraGeosets: display.extraGeosets,
    textures: display.textures,
  };
}

export function getAllSkinsForModel(fileDataID: number): ModelSkin[] {
  let modelName = listfile.getByID(fileDataID)!;
  modelName = path.basename(modelName, 'm2');

  const displays = getModelDisplays(fileDataID);
  if (!displays) return [];
  const skinList: ModelSkin[] = [];
  for (const display of displays) {
    if (display.textures.length === 0) continue;

    skinList.push(getSkinForDisplay(modelName, display));
  }

  return skinList;
}

export default {
  getModelDisplays, getAllSkinsForModel,
};
