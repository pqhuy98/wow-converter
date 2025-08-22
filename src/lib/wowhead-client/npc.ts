import { NpcZamUrl, ZamUrl, getZamBaseUrl } from './zam-url';

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
  const base = getZamBaseUrl(zam.expansion);
  const res = await fetch(`${base}/meta/npc/${zam.displayId}.json`);
  if (!res.ok) throw new Error(`Failed to fetch NPC meta: ${res.status} ${res.statusText}`);
  return await res.json() as unknown as NPCData;
}

// retrieve npc data from wowhead