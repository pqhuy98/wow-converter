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

/** Unique visual variant: primary skin texture + enabled geosets. */
function skinVariantKey(display: ModelDisplay): string {
  const geosets = (display.extraGeosets ?? []).slice().sort((a, b) => a - b).join(',');
  return `${display.textures[0]}|${geosets}`;
}

function getSkinForDisplay(display: ModelDisplay): ModelSkin {
  const texture = display.textures[0];

  let skinName = listfile.getByID(texture);
  if (skinName !== undefined) {
    skinName = path.basename(skinName, '.blp');
  } else {
    skinName = `unknown_${texture}`;
  }

  let label = skinName;
  if (display.extraGeosets && display.extraGeosets.length > 0) {
    skinName += display.extraGeosets.join(',');
    label += ` [${display.extraGeosets.join(', ')}]`;
  }

  return {
    id: skinName,
    label,
    displayID: display.ID,
    extraGeosets: display.extraGeosets,
    textures: display.textures,
  };
}

/** Prefer the display row with the richest texture override list. */
function preferSkin(a: ModelSkin, b: ModelSkin): ModelSkin {
  if (b.textures.length !== a.textures.length) {
    return b.textures.length > a.textures.length ? b : a;
  }
  const aTexKey = a.textures.join(',');
  const bTexKey = b.textures.join(',');
  if (aTexKey !== bTexKey) {
    return bTexKey.localeCompare(aTexKey) > 0 ? b : a;
  }
  return b.displayID > a.displayID ? b : a;
}

export function getAllSkinsForModel(fileDataID: number): ModelSkin[] {
  const displays = getModelDisplays(fileDataID);
  if (!displays) return [];

  const byVariant = new Map<string, ModelSkin>();
  for (const display of displays) {
    if (display.textures.length === 0) continue;

    const key = skinVariantKey(display);
    const skin = getSkinForDisplay(display);
    const existing = byVariant.get(key);
    byVariant.set(key, existing ? preferSkin(existing, skin) : skin);
  }

  const skinList = [...byVariant.values()];
  skinList.sort((a, b) => a.label.localeCompare(b.label));
  return skinList;
}

export default {
  getModelDisplays, getAllSkinsForModel,
};
