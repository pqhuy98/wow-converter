import { Geoset } from '@/lib/objmdl/mdl/components/geoset';
import { MDL } from '@/lib/objmdl/mdl/mdl';
import { fetchItemMeta, ItemData } from '@/lib/wowhead-client/item-armor';
import { ItemZamUrl } from '@/lib/wowhead-client/zam-url';

import { ExportContext, exportModelFileIdAsMdl } from '../utils';

export interface ItemMetata {
  slotId: number | null;
  displayId: number;
  modelFiles: number[];
  textureFiles: number[];
  geosetIds?: number[];
  hideGeosetIds?: number[];
  originalData: ItemData;
}

// Slots enum used by orchestration and attachments
export enum EquipmentSlot {
  Head = 1,
  Shoulder = 3,
  Shirt = 4,
  Chest = 5,
  Belt = 6,
  Legs = 7,
  Feet = 8,
  Wrist = 9,
  Gloves = 10,
  Back = 16,
  Tabard = 19,
}

export function getEquipmentSlotName(slotId: number) {
  return Object.entries(EquipmentSlot).find(([_, v]) => v === slotId)?.[0];
}

export const SUBMESH_GROUPS = {
  Hair: 0,
  FacialA: 100,
  FacialB: 200,
  FacialC: 300,
  Gloves: 400,
  Boots: 500,
  Tail: 600,
  Ears: 700,
  Wrists: 800,
  Kneepads: 900,
  Chest: 1000,
  Pants: 1100,
  Tabard: 1200,
  Trousers: 1300,
  Cloak: 1500,
  Chins: 1600,
  Eyeglow: 1700,
  Belt: 1800,
  'Bone/Tail': 1900,
  Feet: 2000,
  Torso: 2200,
  HandAttach: 2300,
  HeadAttach: 2400,
  DHBlindfolds: 2500,
  Head: 2700,
  Chest2: 2800,
  MechagnomeArms: 2900,
  MechagnomeLegs: 3000,
  MechagnomeFeet: 3100,
  Face: 3200,
  Eyes: 3300,
  Eyebrows: 3400,
  Earrings: 3500,
  Necklace: 3600,
  Headdress: 3700,
  Tails: 3800,
  Vines: 3900,
  'Chins/Tusk': 4000,
  Noses: 4100,
  HairDecoA: 4200,
  HairDecoB: 4300,
  BodySize: 4400,
  EyeGlowB: 5100,
} as const;

export function getSubmeshName(idx: number) {
  const group = Math.floor(idx / 100) * 100;
  const name = Object.entries(SUBMESH_GROUPS).find(([_, v]) => v === group)?.[0];
  return `${name}${idx % 100}`;
}

// Reverse engineered from https://wow.zamimg.com/modelviewer/live/viewer/viewer.min.js
// rr[] defaults: per geoset group default variant
const ZAM_GROUP_BASE_OFFSET: ReadonlyArray<number> = [
  1, 1, 1, 1, 1, 1, 1, 2, 1, 1, // 0-9
  1, 1, 1, 1, 0, 1, 0, 0, 1, 1, // 10-19
  1, 1, 1, 1, 0, 0, 1, 0, 1, 0, // 20-29
  0, 0, 2, 1, 1, 0, 0, 0, 1, 0, // 30-39
  1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // 40-49
  1, 1, // 50-51
];

// Slot -> geoset groups order expected by AttachGeosetGroup offsets
// Search for "i.k(t.b[1], 21)) : 3 == t.C ? i.k(t.b[0], 26) : 4 == t.C ? (i.k(t.b[0], 8),"
// in https://wow.zamimg.com/modelviewer/live/viewer/viewer.min.js
const ZAM_SLOT_TO_GROUPS: Readonly<Record<number, ReadonlyArray<number>>> = {
  // Head
  1: [27, 21],
  // Shoulder
  3: [26],
  // Chest (shirt)
  4: [8, 10],
  // Chest (armor) / Robe
  5: [8, 10, 13, 22, 28],
  20: [8, 10, 13, 22, 28],
  // Waist
  6: [18],
  // Legs
  7: [11, 9, 13],
  // Feet
  8: [5, 20],
  // Wrist
  9: [23],
  // Hands
  10: [4, 23],
  // Back (cloak)
  16: [15],
  // Tabard
  19: [12],
};

function computeZamMeshId(group: number, offset: number | undefined): number {
  const base = ZAM_GROUP_BASE_OFFSET[group] ?? 1;
  const variant = base + (offset ?? 0);
  return group * 100 + variant;
}

const debug = true;

// Geosets to show on the item model itself (viewer applies groups 27/21 and also 26 for some)
function resolveCharacterGeosetIds(slotId: number, itemData: ItemData) {
  // Use Item.GeosetGroup (viewer stores this as k) to equip onto character model groups
  const offsets = itemData?.Item?.GeosetGroup ?? [];
  const groups = ZAM_SLOT_TO_GROUPS[slotId] ?? [];
  const result = new Set<number>();
  debug && console.log('resolveGeosetId', slotId, getEquipmentSlotName(slotId), groups, offsets);
  for (let i = 0; i < groups.length; i++) {
    const group = groups[i];

    // special case for boots per reverse engineering
    if (slotId === EquipmentSlot.Feet && group === SUBMESH_GROUPS.Feet / 100) {
      result.add(2002);
      continue;
    }

    const off = offsets[i];
    const geosetId = computeZamMeshId(group, off);
    result.add(geosetId);
  }
  const geosetIds = Array.from(result);
  debug && console.log(geosetIds.map((id) => getSubmeshName(id)));
  return geosetIds;
}

// Geosets to apply when equipping the item on a character (attach to character groups)
export function filterCollectionGeosets(slotId: number, itemData: ItemData, model: MDL) {
  const submeshIds = new Set(resolveCharacterGeosetIds(slotId, itemData));
  debug && console.log('submeshIds', submeshIds);
  const chosenGeosets = new Set<Geoset>();
  const enabledGroups = new Set<number>();
  // multiple geosets can share same submeshId, we need to include all of them
  model.geosets.forEach((g) => {
    if (submeshIds.has(g.wowData.submeshId)) {
      chosenGeosets.add(g);
      enabledGroups.add(Math.floor(g.wowData.submeshId / 100));
    }
  });

  debug && console.log('enabledGroups 1', enabledGroups);

  submeshIds.forEach((id) => {
    const group = Math.floor(id / 100);
    if (enabledGroups.has(group)) return;
    // we reach here which means the group is needed, but zam submeshId doesn't exist in the model
    // fallback to the model's first geoset in the group
    const defaultGeoset = model.geosets.find((g) => Math.floor(g.wowData.submeshId / 100) === group);
    if (defaultGeoset) {
      // there can be multiple geosets with the same default submeshId, we need to add all of them
      model.geosets.filter((g) => g.wowData.submeshId === defaultGeoset.wowData.submeshId).forEach((g) => {
        chosenGeosets.add(g);
      });
      enabledGroups.add(group);
    }
  });

  debug && console.log('enabledGroups 1', enabledGroups);

  return Array.from(chosenGeosets);
}

function resolveHideGeosetIds(itemData: ItemData, targetRace: number, targetGender: number) {
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
  const result: ItemMetata = {
    slotId: url.slotId,
    displayId: url.displayId,
    modelFiles: filterFilesByRaceGender(itemData.ModelFiles || {}, itemData.ComponentModels || {}, targetRace, targetGender),
    textureFiles: [
      ...filterFilesByRaceGender(itemData.TextureFiles || {}, itemData.ComponentTextures || {}, targetRace, targetGender),
      ...[itemData.Textures, itemData.Textures2].filter((obj) => obj !== null)
        .flatMap((obj) => Object.entries(obj).flatMap(([, value]) => value)),
    ],
    geosetIds: url.slotId ? resolveCharacterGeosetIds(url.slotId, itemData) : [],
    hideGeosetIds: resolveHideGeosetIds(itemData, targetRace, targetGender),
    originalData: itemData,
  };
  return result;
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
  return exportModelFileIdAsMdl(ctx, modelId, { textureIds: allTextureIds });
}
