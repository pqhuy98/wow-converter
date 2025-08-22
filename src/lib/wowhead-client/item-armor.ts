import { ZamUrl, getZamBaseUrl } from './zam-url';

export interface ItemData {
  Textures: { [k: string]: number } | null;
  Textures2: { [k: string]: number } | null;
  ModelFiles: { [modelId: string]: { FileDataId: number; Race: number; Gender: number }[] };
  TextureFiles: { [textureId: string]: { FileDataId: number; Race: number; Gender: number }[] };
  Item: {
    GeosetGroup: number[];
    HideGeosetMale?: { RaceId: number; GeosetGroup: number; RaceBitSelection: number }[];
    HideGeosetFemale?: { RaceId: number; GeosetGroup: number; RaceBitSelection: number }[];
  };
}

export async function fetchItemMeta(zam: ZamUrl): Promise<ItemData> {
  if (zam.type !== 'item') throw new Error('fetchItemMeta expects a ZamUrl of type item');
  const base = getZamBaseUrl(zam.expansion);
  const url = zam.slotId ? `${base}/meta/armor/${zam.slotId}/${zam.displayId}.json` : `${base}/meta/item/${zam.displayId}.json`;
  const res = await fetch(url);
  if (!res.ok) throw new Error(`Failed to fetch item meta: ${res.status} ${res.statusText}`);
  return await res.json() as unknown as ItemData;
}
