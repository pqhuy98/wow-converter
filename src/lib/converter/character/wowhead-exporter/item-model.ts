import { Geoset } from '@/lib/formats/mdl/components/geoset';
import { MDL } from '@/lib/formats/mdl/mdl';
import { EquipmentSlot, fetchItemMeta, ItemData } from '@/lib/wowhead-client/item-armor';
import { ItemZamUrl } from '@/lib/wowhead-client/zam-url';

import { Model } from '../../common/models';
import { applyReplaceableTextures, ExportContext, exportModelFileIdAsMdl } from '../utils';

interface FileWithComponent {
  fileDataId: number;
  componentId: number;
}

export interface ItemMetata {
  slotId: number | null;
  inventoryType: number;
  itemClass: number;
  itemSubClass: number;
  displayId: number;
  flags: number;
  modelFiles: FileWithComponent[];
  modelTextureFiles: [FileWithComponent[], FileWithComponent[]];
  bodyTextureFiles: FileWithComponent[];
  hideGeosetIds?: number[];
  zamGeosetGroup?: number[];
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
const _ZAM_SLOT_TO_GROUPS: Readonly<Record<number, ReadonlyArray<number>>> = {
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

export function computeZamMeshId(group: number, offset: number | undefined): number {
  const base = ZAM_GROUP_BASE_OFFSET[group] ?? 1;
  const variant = base + (offset ?? 0);
  return group * 100 + variant;
}

export type EquipmentSlotData = {
  slotId: EquipmentSlot;
  data: ItemMetata;
}

export function getGeosetIdsFromEquipments(equipments: EquipmentSlotData[], chosenEquipments: EquipmentSlotData[] = equipments) {
  let geosetIds: number[] = [];
  const hideGeosetIds: number[] = [];

  const head = equipments.find((s) => s.slotId === EquipmentSlot.Head);
  const shoulders = equipments.find((s) => s.slotId === EquipmentSlot.Shoulder);
  const shirt = equipments.find((s) => s.slotId === EquipmentSlot.Shirt);
  const chest = equipments.find((s) => s.slotId === EquipmentSlot.Chest);
  const waist = equipments.find((s) => s.slotId === EquipmentSlot.Waist);
  const legs = equipments.find((s) => s.slotId === EquipmentSlot.Legs);
  const feet = equipments.find((s) => s.slotId === EquipmentSlot.Feet);
  const wrist = equipments.find((s) => s.slotId === EquipmentSlot.Wrist);
  const hands = equipments.find((s) => s.slotId === EquipmentSlot.Hands);
  const tabard = equipments.find((s) => s.slotId === EquipmentSlot.Tabard);
  const cloak = equipments.find((s) => s.slotId === EquipmentSlot.Cloak);

  const removeGroup = (group: number, hideDefault: boolean) => {
    geosetIds = geosetIds.filter((g) => Math.floor(g / 100) !== group);
    hideDefault && hideGeosetIds.push(group * 100 + 1);
  };
  const hasGeoset = (equiment: EquipmentSlotData | undefined, i: number) => (equiment?.data.zamGeosetGroup?.[i] ?? 0) > 0;
  const addGeoset = (equiment: EquipmentSlotData | undefined, group: number, i: number) => {
    if (equiment?.data.zamGeosetGroup?.[i] != null) {
      removeGroup(group, false);
      geosetIds.push(computeZamMeshId(group, equiment?.data.zamGeosetGroup?.[i]));
    }
  };

  const hasChestTrouser = hasGeoset(chest, 2);
  const hasLegsTrouser = hasGeoset(legs, 2);

  chosenEquipments.forEach((s) => {
    switch (s.slotId) {
      case EquipmentSlot.Head:
        addGeoset(head, 27, 0);
        addGeoset(head, 21, 1);
        break;
      case EquipmentSlot.Shoulder:
        addGeoset(shoulders, 26, 0);
        break;
      case EquipmentSlot.Shirt:
        !hasGeoset(hands, 0) && addGeoset(shirt, 8, 0);
        addGeoset(shirt, 10, 1);
        break;
      case EquipmentSlot.Chest:
      case EquipmentSlot.Robe:
        !hasGeoset(hands, 0) && addGeoset(s, 8, 0);
        addGeoset(s, 10, 1);
        addGeoset(s, 13, 2);
        addGeoset(s, 22, 3);
        addGeoset(s, 28, 4);
        break;

      case EquipmentSlot.Waist:
        addGeoset(waist, 18, 0);
        break;
      case EquipmentSlot.Legs:
        addGeoset(legs, 11, 0);
        addGeoset(legs, 9, 1);
        addGeoset(legs, 13, 2);
        break;
      case EquipmentSlot.Feet:
        addGeoset(feet, 5, 0);
        geosetIds.push(
          hasGeoset(feet, 1)
            ? (2000 + (feet?.data.zamGeosetGroup?.[1] ?? 0))
            : (!feet || (feet.data.flags & 1048576)) ? 2001 : 2002,
        );
        break;
      case EquipmentSlot.Hands: {
        const chestHasPalms = hasGeoset(chest, 0);
        const handsHasPalms = hasGeoset(hands, 0);
        (handsHasPalms || !chestHasPalms) && addGeoset(hands, 4, 0);
        addGeoset(hands, 23, 1);
        break;
      }
      case EquipmentSlot.Cloak:
        addGeoset(cloak, 15, 0);
        break;
      case EquipmentSlot.Tabard:
        addGeoset(tabard, 12, 0);
        break;
      case EquipmentSlot.Wrist: {
        const handsHasGlove = hasGeoset(hands, 0);
        const chestHasWristsTrousers = hasGeoset(chest, 2) && hasGeoset(chest, 0);
        !handsHasGlove && !chestHasWristsTrousers && addGeoset(wrist, 23, 0);
        break;
      }
      default:
        break;
    }
    hideGeosetIds.push(...s.data.hideGeosetIds ?? []);
  });

  if (hasChestTrouser) {
    removeGroup(5, true);
    removeGroup(9, true);
    removeGroup(11, true);
    removeGroup(13, true);
    addGeoset(chest, 13, 2);
  } else if (hasLegsTrouser) {
    removeGroup(5, true);
    removeGroup(9, true);
    removeGroup(11, true);
    removeGroup(13, true);
    addGeoset(legs, 13, 2);
  }

  const debug = false;
  debug && console.log('geosetIds', geosetIds);
  debug && console.log('hideGeosetIds', hideGeosetIds);

  return { geosetIds, hideGeosetIds };
}

// Geosets to apply when equipping the item on a character (attach to character groups)
export function filterCollectionGeosets(equipmentSlots: EquipmentSlotData[], slotData: EquipmentSlotData, model: MDL) {
  const debug = false;
  const submeshIds = new Set(getGeosetIdsFromEquipments(equipmentSlots, [slotData]).geosetIds);
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
      let start = 1;
      if ([1, 2, 3].includes(band)) start = 2;
      for (let i = start; i < 100; i++) result.add(band * 100 + i);
      console.log('hideGeosets', { band });
    }
  });
  return Array.from(result).sort((a, b) => a - b);
}

export async function processItemData(url: ItemZamUrl, targetRace: number, targetGender: number): Promise<ItemMetata> {
  const itemData = await fetchItemMeta(url);
  const result: ItemMetata = {
    slotId: url.slotId,
    inventoryType: itemData.Item.InventoryType,
    itemClass: itemData.Item.ItemClass,
    itemSubClass: itemData.Item.ItemSubClass,
    displayId: url.displayId,
    flags: itemData.Item.Flags,
    modelFiles: filterFilesByRaceGender(itemData.ModelFiles || {}, itemData.ComponentModels || {}, targetRace, targetGender, false),
    modelTextureFiles: [
      Object.entries(itemData.Textures || {}).flatMap(([k, value]) => ({ fileDataId: value, componentId: Number(k) })),
      Object.entries(itemData.Textures2 || {}).flatMap(([k, value]) => ({ fileDataId: value, componentId: Number(k) })),
    ],
    bodyTextureFiles: filterFilesByRaceGender(itemData.TextureFiles || {}, itemData.ComponentTextures || {}, targetRace, targetGender, true),
    hideGeosetIds: resolveHideGeosetIds(itemData, targetRace, targetGender),
    zamGeosetGroup: itemData.Item.GeosetGroup,
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
}): Promise<{model: Model, itemData: ItemMetata}> {
  const result = await processItemData(zam, targetRace, targetGender);
  const modelId = result.modelFiles[0].fileDataId;
  const allTextureIds = result.modelTextureFiles[0].map((f) => f.fileDataId);
  const model = await exportModelFileIdAsMdl(ctx, modelId, { textureIds: allTextureIds });
  await applyReplaceableTextures(ctx, model.mdl, Object.fromEntries(result.modelTextureFiles[0].map((f) => [f.componentId, f.fileDataId])));
  return { model, itemData: result };
}
