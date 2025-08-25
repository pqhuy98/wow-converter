import chalk from 'chalk';

import {
  fetchWowZaming, getLatestExpansionHavingUrl, getZamBaseUrl, NpcZamUrl,
} from './zam-url';

export interface FileEntry {
  FileDataId: number;
  Gender: number;
  Class: number;
  Race: number;
  ExtraData: number;
}

export interface Customization {
  optionId: number;
  choiceId: number;
}

export interface CharacterMeta {
  Race: number;
  Gender: number;
  ChrModelId: number;
}

export interface CreatureMeta {
  CreatureCustomizations: Customization[];
  CreatureGeosetData: {
    GeosetIndex: number;
    GeosetValue: number;
  }[];
}

export interface EquipmentMap {
  [slotId: string]: number;
}

export interface TextureFilesMap {
  [textureId: string]: FileEntry[];
}

export interface ModelFilesMap {
  [modelId: string]: FileEntry[];
}

export interface NPCData {
  Model?: number;
  Textures?: { [k: string]: number };
  Character?: CharacterMeta;
  Creature?: CreatureMeta;
  Equipment?: EquipmentMap;
  TextureFiles?: TextureFilesMap;
}

const debug = false;

export async function fetchNpcMeta(zam: NpcZamUrl): Promise<NPCData> {
  if (zam.type !== 'npc') throw new Error('fetchNpcMeta expects a ZamUrl of type npc');
  const path = `meta/npc/${zam.displayId}.json`;

  let expansion = zam.expansion;
  if (expansion === 'latest-available') {
    expansion = await getLatestExpansionHavingUrl(path);
  }
  const base = getZamBaseUrl(expansion);
  const url = `${base}/${path}`;
  debug && console.log('Get NPC meta from', chalk.blue(url));
  try {
    const res = await fetchWowZaming(url);
    return JSON.parse(res) as unknown as NPCData;
  } catch (e) {
    console.log(
      chalk.red('Failed to fetch NPC meta from'),
      chalk.blue(url),
      chalk.red(e),
      chalk.red('falling back to latest available expansion'),
    );
    const base2 = getZamBaseUrl(await getLatestExpansionHavingUrl(path));
    const url2 = `${base2}/${path}`;
    const res2 = await fetchWowZaming(url2);
    return JSON.parse(res2) as unknown as NPCData;
  }
}
