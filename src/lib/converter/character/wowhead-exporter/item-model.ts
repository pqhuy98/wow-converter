import { Geoset } from '@/lib/formats/mdl/components/geoset';
import { MDL } from '@/lib/formats/mdl/mdl';
import { EquipmentSlot, fetchItemMeta, ItemData } from '@/lib/wowhead-client/item-armor';
import { ItemZamUrl } from '@/lib/wowhead-client/zam-url';

import { applyReplaceableTextures, ExportContext, exportModelFileIdAsMdl } from '../utils';

interface FileWithComponent {
  fileDataId: number;
  componentId: number;
}

export interface ItemMetata {
  slotId: number | null;
  displayId: number;
  flags: number;
  modelFiles: FileWithComponent[];
  modelTextureFiles: [FileWithComponent[], FileWithComponent[]];
  bodyTextureFiles: FileWithComponent[];
  geosetIds?: number[];
  hideGeosetIds?: number[];
  originalData: ItemData;
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

const debug = false;

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
    flags: itemData.Item.Flags,
    modelFiles: filterFilesByRaceGender(itemData.ModelFiles || {}, itemData.ComponentModels || {}, targetRace, targetGender, false),
    modelTextureFiles: [
      Object.entries(itemData.Textures || {}).flatMap(([k, value]) => ({ fileDataId: value, componentId: Number(k) })),
      Object.entries(itemData.Textures2 || {}).flatMap(([k, value]) => ({ fileDataId: value, componentId: Number(k) })),
    ],
    bodyTextureFiles: filterFilesByRaceGender(itemData.TextureFiles || {}, itemData.ComponentTextures || {}, targetRace, targetGender, true),
    geosetIds: url.slotId ? resolveCharacterGeosetIds(url.slotId, itemData) : [],
    hideGeosetIds: resolveHideGeosetIds(itemData, targetRace, targetGender),
    originalData: itemData,
  };
  return result;
}

// From viewer.min.js
const raceGenderFallback = {
  86: [4, 0, 4, 1, 4, 0, 4, 1],
  85: [84, 0, 84, 1, 84, 0, 84, 1],
  84: [3, 0, 3, 1, 3, 0, 3, 1],
  77: [5, 1, 0, -1, 5, 0, 0, -1],
  76: [10, 0, 1, 1, 10, 0, 1, 1],
  75: [10, 0, 1, 1, 10, 0, 1, 1],
  74: [5, 1, 0, -1, 5, 0, 0, -1],
  73: [5, 1, 0, -1, 5, 0, 0, -1],
  72: [5, 1, 0, -1, 5, 0, 0, -1],
  71: [5, 1, 0, -1, 5, 0, 0, -1],
  37: [7, 0, 7, 1, 7, 0, 7, 1],
  36: [2, 0, 2, 1, 2, 0, 2, 1],
  34: [3, 0, 3, 1, 3, 0, 3, 1],
  33: [5, 1, 0, -1, 5, 0, 0, -1],
  31: [0, -1, 8, 1, 0, -1, 8, 1],
  30: [11, 0, 11, 1, 11, 0, 11, 1],
  29: [10, 0, 10, 1, 10, 0, 10, 1],
  28: [6, 0, 6, 1, 6, 0, 6, 1],
  27: [4, 0, 4, 1, 4, 0, 4, 1],
  26: [24, 0, 24, 1, 24, 0, 24, 1],
  25: [24, 0, 24, 1, 24, 0, 24, 1],
  23: [1, 0, 1, 1, 1, 0, 1, 1],
  15: [5, 0, 5, 1, 5, 0, 5, 1],
  1: [0, -1, 0, -1, 0, -1, 0, 3],
};

function filterFilesByRaceGender(
  files: ItemData['ModelFiles'] | ItemData['TextureFiles'],
  components: ItemData['ComponentModels'] | ItemData['ComponentTextures'],
  targetRace: number,
  targetGender: number,
  isTexture: boolean,
): FileWithComponent[] {
  const filteredFiles: FileWithComponent[] = [];
  const componentEntries = Object.entries(components || {}).sort((a, b) => Number(a[0]) - Number(b[0]));
  for (let i = 0; i < componentEntries.length; i++) {
    const [componentId, id] = componentEntries[i];
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
    // Fallback: use raceGenderFallback map like Wowhead. If no direct match, remap race/gender and retry.
    if (!matchingFile) {
      // try one-step fallback
      const remap = (race: number, gender: number): [number, number] | null => {
        const row = (raceGenderFallback as Record<number, number[] | undefined>)[race];
        if (!row) return null;
        const base = isTexture ? 4 : 0;
        const idx = base + 2 * gender;
        const r = row[idx];
        const g = row[idx + 1];
        if (r === undefined || g === undefined) return null;
        return [r, g];
      };

      let cur: [number, number] | null = [targetRace, targetGender];
      const visited = new Set<string>();
      while (cur) {
        const key = cur.join(':');
        if (visited.has(key)) break;
        visited.add(key);
        const [r, g] = cur;
        const tryFiles = entries.filter((fileEntry) => (
          (fileEntry.Race === r || (r === 0 && fileEntry.Race === 0))
            && (g === -1 ? fileEntry.Gender > 1 : (fileEntry.Gender === g || fileEntry.Gender > 1))));
        tryFiles.sort((a, b) => a.ExtraData - b.ExtraData);
        if (tryFiles.length) {
          matchingFile = tryFiles[0];
          break;
        }
        cur = remap(r, g);
      }
    }
    if (matchingFile) {
      filteredFiles.push({ fileDataId: matchingFile.FileDataId, componentId: Number(componentId) });
    }
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
  const modelId = result.modelFiles[0].fileDataId;
  const allTextureIds = result.modelTextureFiles[0].map((f) => f.fileDataId);
  const mdl = await exportModelFileIdAsMdl(ctx, modelId, { textureIds: allTextureIds });
  await applyReplaceableTextures(ctx, mdl, Object.fromEntries(result.modelTextureFiles[0].map((f) => [f.componentId, f.fileDataId])));
  return mdl;
}
