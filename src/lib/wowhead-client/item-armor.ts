import chalk from 'chalk';

import {
  fetchWithCache, getLatestExpansionHavingUrl, getZamBaseUrl, ZamUrl,
} from './zam-url';

// Slots enum used by orchestration and attachments
export enum EquipmentSlot {
  Head = 1,
  Shoulder = 3,
  Shirt = 4,
  Chest = 5,
  Waist = 6,
  Legs = 7,
  Feet = 8,
  Wrist = 9,
  Hands = 10,
  MainHand = 12,
  OffHand = 13,
  Shield = 14,
  Ranged = 15,
  Cloak = 16,
  Tabard = 19,
  Robe = 20,
  Holdable = 23,
  RangedRight = 26,
}

interface ItemFile {
  FileDataId: number;
  Race: number;
  Gender: number;
  ExtraData: number;
}

export interface ItemData {
  Textures: { [k: string]: number } | null;
  Textures2: { [k: string]: number } | null;
  ModelFiles: { [modelId: string]: ItemFile[] };
  TextureFiles: { [textureId: string]: ItemFile[] };
  Item: {
    Flags: number;
    InventoryType: number;
    ItemClass: number;
    ItemSubClass: number;
    GeosetGroup: number[];
    AttachGeosetGroup?: number[];
    GeosetGroupOverride?: number[];
    HideGeosetMale?: { RaceId: number; GeosetGroup: number; RaceBitSelection: number }[];
    HideGeosetFemale?: { RaceId: number; GeosetGroup: number; RaceBitSelection: number }[];
  };
  ComponentModels: { [componentId: string]: number };
  ComponentTextures: { [componentId: string]: number };
}

const debug = true;

const ArmorSlots = [
  EquipmentSlot.Head,
  EquipmentSlot.Shoulder,
  EquipmentSlot.Shirt,
  EquipmentSlot.Chest,
  EquipmentSlot.Waist,
  EquipmentSlot.Legs,
  EquipmentSlot.Feet,
  EquipmentSlot.Wrist,
  EquipmentSlot.Hands,
  EquipmentSlot.Cloak,
  EquipmentSlot.Tabard,
  EquipmentSlot.Robe,
];

const slotBackup = {
  5: 20,
};

export async function fetchItemMeta(zam: ZamUrl): Promise<ItemData> {
  if (zam.type !== 'item') throw new Error('fetchItemMeta expects a ZamUrl of type item');
  let slotId: number | null = zam.slotId;
  if (slotId && !ArmorSlots.includes(slotId)) slotId = null;

  const path = slotId
    ? `meta/armor/${slotId}/${zam.displayId}.json`
    : `meta/item/${zam.displayId}.json`;

  let expansion = zam.expansion;
  if (expansion === 'latest-available') {
    expansion = await getLatestExpansionHavingUrl(path);
  }
  const base = getZamBaseUrl(expansion);
  const url = `${base}/${path}`;
  debug && console.log('Get item meta from', chalk.blue(url));
  try {
    const res = await fetchWithCache(url);
    return JSON.parse(res) as unknown as ItemData;
  } catch (e) {
    if (slotId && slotBackup[slotId]) {
      return fetchItemMeta({ ...zam, slotId: slotBackup[slotId] });
    }
    throw e;
  }
}
