import chalk from 'chalk';

import {
  fetchWowZaming, getLatestExpansionHavingUrl, getZamBaseUrl, ZamUrl,
} from './zam-url';

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
    GeosetGroup: number[];
    HideGeosetMale?: { RaceId: number; GeosetGroup: number; RaceBitSelection: number }[];
    HideGeosetFemale?: { RaceId: number; GeosetGroup: number; RaceBitSelection: number }[];
  };
  ComponentModels: { [componentId: string]: number };
  ComponentTextures: { [componentId: string]: number };
}

const debug = false;

export async function fetchItemMeta(zam: ZamUrl): Promise<ItemData> {
  if (zam.type !== 'item') throw new Error('fetchItemMeta expects a ZamUrl of type item');
  const path = zam.slotId
    ? `meta/armor/${zam.slotId}/${zam.displayId}.json`
    : `meta/item/${zam.displayId}.json`;

  let expansion = zam.expansion;
  if (expansion === 'latest-available') {
    expansion = await getLatestExpansionHavingUrl(path);
  }
  const base = getZamBaseUrl(expansion);
  const url = `${base}/${path}`;
  debug && console.log('Get item meta from', chalk.blue(url));
  const res = await fetchWowZaming(url);
  return JSON.parse(res) as unknown as ItemData;
}
