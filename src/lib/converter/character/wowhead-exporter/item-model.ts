import { MDL } from '@/lib/objmdl/mdl/mdl';
import { fetchItemMeta, ItemData } from '@/lib/wowhead-client/item-armor';
import { ItemZamUrl } from '@/lib/wowhead-client/zam-url';

import { ExportContext, exportModelFileIdAsMdl } from '../utils';

export interface ItemMetata {
  modelFiles: number[];
  textureFiles: number[];
  geosetIds?: number[];
  hideGeosetIds?: number[];
}

// Slots enum used by orchestration and attachments
export enum EquipmentSlot {
  Head = 1,
  Shoulder = 3,
  Chest = 5,
  Belt = 6,
  Legs = 7,
  Boots = 8,
  Wrist = 9,
  Gloves = 10,
  Back = 16,
  Tabard = 19,
}

export function getEquipmentSlotName(slotId: number) {
  return Object.entries(EquipmentSlot).find(([_, v]) => v === slotId)?.[0];
}

const GEOSET_GROUPS: Record<number, number[]> = {
  [EquipmentSlot.Chest]: [800, 0, 1300],
  [EquipmentSlot.Belt]: [1800],
  [EquipmentSlot.Legs]: [1100, 0, 1300],
  [EquipmentSlot.Boots]: [500, 2000 + 1],
  [EquipmentSlot.Gloves]: [400],
  [EquipmentSlot.Back]: [1500],
  [EquipmentSlot.Tabard]: [1200],
};

function resolveGeosetId(slotId: number, itemData: ItemData) {
  // Accept any 'ItemData-like' object from wowhead meta
  const result = new Set<number>();
  (itemData?.Item?.GeosetGroup || []).forEach((value: number, i: number) => {
    if (!GEOSET_GROUPS[slotId]?.[i] || (value === 0 && (GEOSET_GROUPS[slotId][i] % 100) === 0)) return;
    const geosetId = (GEOSET_GROUPS[slotId][i]) + value + 1;
    result.add(geosetId);
  });
  return Array.from(result);
}

function resolveHideGeosetId(itemData: ItemData, targetRace: number, targetGender: number) {
  const result = new Set<number>();
  let hideGeosets = itemData?.Item?.HideGeosetMale;
  if (targetGender === 1) hideGeosets = itemData?.Item?.HideGeosetFemale;
  (hideGeosets ?? []).forEach((value) => {
    if (value.RaceId === targetRace) {
      const band = value.GeosetGroup;
      for (let i = 1; i < 100; i++) result.add(band * 100 + i);
    }
  });
  return Array.from(result);
}

export async function processItemData(url: ItemZamUrl, targetRace: number, targetGender: number): Promise<ItemMetata> {
  const itemData = await fetchItemMeta(url);
  return {
    modelFiles: filterFilesByRaceGender(itemData.ModelFiles || {}, itemData.ComponentModels || {}, targetRace, targetGender),
    textureFiles: [
      ...filterFilesByRaceGender(itemData.TextureFiles || {}, itemData.ComponentTextures || {}, targetRace, targetGender),
      ...[itemData.Textures, itemData.Textures2].filter((obj) => obj !== null)
        .flatMap((obj) => Object.entries(obj).flatMap(([, value]) => value)),
    ],
    geosetIds: url.slotId ? resolveGeosetId(url.slotId, itemData) : [],
    hideGeosetIds: resolveHideGeosetId(itemData, targetRace, targetGender),
  };
}

function filterFilesByRaceGender(
  files: ItemData['ModelFiles'] | ItemData['TextureFiles'],
  components: ItemData['ComponentModels'] | ItemData['ComponentTextures'],
  targetRace: number,
  targetGender: number,
): number[] {
  const filteredFiles: number[] = [];
  const componentEntries = Object.entries(components || {}).sort((a, b) => Number(a[0]) - Number(b[0]));
  for (let i = 0; i < componentEntries.length; i++) {
    const [_componentId, id] = componentEntries[i];
    const entries = files[id] || [];
    const matchingFiles = entries.filter((fileEntry) => (
      // Check if this entry matches our race and gender OR is universal (Race: 0, Gender: 2)
      (fileEntry.Race === targetRace || fileEntry.Race === 0)
        && (fileEntry.Gender === targetGender || fileEntry.Gender > 1)));
    matchingFiles.sort((a, b) => a.ExtraData - b.ExtraData);
    let matchingFile = matchingFiles[0];
    if (matchingFiles.length > 1 && matchingFiles[i]) {
      matchingFile = matchingFiles[i];
    }
    filteredFiles.push(matchingFile.FileDataId);
  }
  return filteredFiles;
}

export async function exportZamItemAsMdl({
  ctx,
  zam,
  targetRace,
  targetGender,
}: {
  ctx: ExportContext;
  zam: ItemZamUrl;
  targetRace: number;
  targetGender: number;
}): Promise<MDL> {
  const result = await processItemData(zam, targetRace, targetGender);
  const modelId = result.modelFiles[0];
  const allTextureIds = result.textureFiles;
  return exportModelFileIdAsMdl(ctx, modelId, allTextureIds);
}
