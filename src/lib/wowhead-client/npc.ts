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

export async function fetchNpcMeta(zam: NpcZamUrl): Promise<NPCData> {
  if (zam.type !== 'npc') throw new Error('fetchNpcMeta expects a ZamUrl of type npc');
  const path = `meta/npc/${zam.displayId}.json`;

  let expansion = zam.expansion;
  if (expansion === 'latest-available') {
    expansion = await getLatestExpansionHavingUrl(path);
  }
  const base = getZamBaseUrl(expansion);
  const url = `${base}/${path}`;
  console.log('Get NPC meta from', chalk.blue(url));
  const res = await fetchWowZaming(url);
  return JSON.parse(res) as unknown as NPCData;
}
