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
  Class: number;
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

const debug = false;

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

function itemMetaCandidatePaths(displayId: number, preferredSlot: number | null): string[] {
  const paths: string[] = [];
  const seen = new Set<number>();
  const addSlot = (slot: number) => {
    if (slot <= 0 || seen.has(slot)) return;
    seen.add(slot);
    paths.push(`meta/armor/${slot}/${displayId}.json`);
  };
  if (preferredSlot) {
    addSlot(preferredSlot);
    const backup = slotBackup[preferredSlot as keyof typeof slotBackup];
    if (backup) addSlot(backup);
  }
  for (const slot of ArmorSlots) {
    addSlot(slot);
  }
  paths.push(`meta/item/${displayId}.json`);
  return paths;
}

async function fetchItemMetaAtPath(expansion: ZamUrl['expansion'], path: string): Promise<ItemData> {
  let exp = expansion;
  if (exp === 'latest-available') {
    exp = await getLatestExpansionHavingUrl(path);
  }
  const url = `${getZamBaseUrl(exp)}/${path}`;
  try {
    const res = await fetchWithCache(url);
    return JSON.parse(res) as unknown as ItemData;
  } catch (e) {
    if (expansion !== 'latest-available') {
      try {
        const fallback = await getLatestExpansionHavingUrl(path);
        if (fallback !== exp) {
          const fallbackUrl = `${getZamBaseUrl(fallback)}/${path}`;
          const res = await fetchWithCache(fallbackUrl);
          return JSON.parse(res) as unknown as ItemData;
        }
      } catch {
        // fall through
      }
    }
    throw e;
  }
}

export async function fetchItemMeta(zam: ZamUrl): Promise<ItemData> {
  if (zam.type !== 'item') throw new Error('fetchItemMeta expects a ZamUrl of type item');
  let slotId: number | null = zam.slotId;
  if (slotId && !ArmorSlots.includes(slotId)) slotId = null;

  let lastError: unknown;
  for (const path of itemMetaCandidatePaths(zam.displayId, slotId)) {
    try {
      return await fetchItemMetaAtPath(zam.expansion, path);
    } catch (e) {
      lastError = e;
    }
  }
  throw lastError;
}
