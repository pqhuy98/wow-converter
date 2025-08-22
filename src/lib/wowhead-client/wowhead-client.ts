import { LRUCache } from 'lru-cache';

import { ExportCharacterParams, wowExportClient } from '@/lib/wowexport-client/wowexport-client';

interface FileEntry {
  FileDataId: number;
  Gender: number;
  Class: number;
  Race: number;
  ExtraData: number;
}

interface Customization {
  optionId: number;
  choiceId: number;
}

interface Character {
  Race: number;
  Gender: number;
  ChrModelId: number;
}

interface Creature {
  CreatureCustomizations: Customization[];
}

interface Equipment {
  [slotId: string]: number;
}

interface TextureFiles {
  [textureId: string]: FileEntry[];
}

interface ModelFiles {
  [modelId: string]: FileEntry[];
}

interface NPCData {
  Model: number;
  Textures: {[k: string]: number};
  Character: Character;
  Creature?: Creature;
  Equipment: Equipment;
  TextureFiles: TextureFiles;
}

interface ItemData {
  Textures: {[k: string]: number};
  Textures2: {[k: string]: number};
  ModelFiles: ModelFiles;
  TextureFiles: TextureFiles;
  Item: {
    GeosetGroup: number[];
    HideGeosetMale?: {
      RaceId: number;
      GeosetGroup: number;
      RaceBitSelection: number;
    }[];
    HideGeosetFemale?: {
      RaceId: number;
      GeosetGroup: number;
      RaceBitSelection: number;
    }[];
  }
}

interface CharacterData {
  character: Character;
  equipment: Equipment;
  customizations: Customization[];
  textureFiles: TextureFiles;
  race: number;
  gender: number;
  raceId: number;
  genderId: number;
}

interface ItemResult {
  modelFiles: FilteredFile[];
  textureFiles: FilteredFile[];
  geosetIds?: number[];
  hideGeosetIds?: number[];
}

interface FilteredFile {
  fileDataId: number;
}

interface EquipmentSlotData {
  slotId: string;
  hasModel: boolean;
  data: ItemResult;
}

interface ExportResult {
  exportCharRpcParams: ExportCharacterParams;
  npcTextureFile: number | null;
  equipmentSlots: EquipmentSlotData[];
}

// ===== New types to support model-only NPCs =====

// Model export preparation result (for NPC definitions that only provide a model ID)
export interface ModelExportPreparation {
  type: 'model';
  baseModel: { fileDataID: number; skinName?: string };
  equipmentSlots: EquipmentSlotData[];
  npcTextureFile: number | null;
}

// Character export preparation result (existing path)
export interface CharacterExportPreparation extends ExportResult {
  type: 'character';
}

export type NpcExportPreparation = ModelExportPreparation | CharacterExportPreparation;

// Equipment slot enum
enum EquipmentSlot {
  Head = 1,
  Shoulder = 3,
  Chest = 5,
  Belt = 6,
  Legs = 7,
  Boots = 8,
  Wrist = 9,
  Gloves = 10,
  Back = 16,
  Tabard = 19,
}

// wow.export\src\js\3D\GeosetMapper.js
const GEOSET_GROUPS = {
  [EquipmentSlot.Chest]: [800, 0, 1300],
  [EquipmentSlot.Belt]: [1800],
  [EquipmentSlot.Legs]: [1100, 0, 1300],
  [EquipmentSlot.Boots]: [
    500,
    2000 + 1, // If has boot, autochoose the 2nd feet
  ],
  [EquipmentSlot.Gloves]: [400],
  [EquipmentSlot.Back]: [1500],
  [EquipmentSlot.Tabard]: [1200],
};

// Slots that have model files (not just textures)
const MODEL_SLOTS = new Set([EquipmentSlot.Head.toString(), EquipmentSlot.Shoulder.toString()]);

export const baseUrls = [
  'https://wow.zamimg.com/modelviewer/classic',
  'https://wow.zamimg.com/modelviewer/tbc',
  'https://wow.zamimg.com/modelviewer/wrath',
  'https://wow.zamimg.com/modelviewer/cata',
  'https://wow.zamimg.com/modelviewer/mists',
  'https://wow.zamimg.com/modelviewer/live',
  'https://wow.zamimg.com/modelviewer/ptr',
  'https://wow.zamimg.com/modelviewer/ptr2',
  'https://wow.zamimg.com/modelviewer/live',
].reverse();

const debug = false;

const fetchCache = new LRUCache<string, object>({ max: 1000 });

async function fetchJson(url: string) {
  const cached = fetchCache.get(url);
  if (cached) return cached;

  for (const baseUrl of baseUrls) {
    try {
      const response = await fetch(`${baseUrl}${url}`);
      const json = await response.json();
      fetchCache.set(url, json as object);
      return json;
    } catch (error) {
      // swallow error
    }
  }
  throw new Error(`No valid response from ${url}`);
}

function filterFilesByRaceGender(
  files: ModelFiles | TextureFiles,
  targetRace: number,
  targetGender: number,
): FilteredFile[] {
  const filteredFiles: FilteredFile[] = [];

  for (const entry of Object.entries(files)) {
    for (const fileEntry of entry[1]) {
      // Check if this entry matches our race and gender OR is universal (Race: 0, Gender: 2)
      if ((fileEntry.Race === targetRace || fileEntry.Race === 0)
          && (fileEntry.Gender === targetGender || fileEntry.Gender > 1)) {
        filteredFiles.push({
          fileDataId: fileEntry.FileDataId,
        });
      }
    }
  }

  return filteredFiles;
}

function resolveGeosetId(slotId: number, itemData: ItemData) {
  const result = new Set<number>();
  itemData.Item.GeosetGroup.forEach((value, i) => {
    if (
      !GEOSET_GROUPS[slotId]?.[i]
      || value === 0 && (GEOSET_GROUPS[slotId][i] % 100) === 0
    ) return;
    const geosetId = (GEOSET_GROUPS[slotId][i]) + value + 1;
    result.add(geosetId);
  });
  return Array.from(result);
}

function resolveHideGeosetId(itemData: ItemData, targetRace: number, targetGender: number) {
  const result = new Set<number>();
  let hideGeosets = itemData.Item.HideGeosetMale;
  if (targetGender === 1) {
    hideGeosets = itemData.Item.HideGeosetFemale;
  }
  (hideGeosets ?? []).forEach((value) => {
    if (value.RaceId === targetRace) {
      const band = value.GeosetGroup;
      for (let i = 1; i < 100; i++) {
        result.add(band * 100 + i);
      }
    }
  });
  return Array.from(result);
}

export async function processItemData(
  slotId: number,
  itemDisplayId: number,
  targetRace: number,
  targetGender: number,
): Promise<ItemResult> {
  let url = `/meta/armor/${slotId}/${itemDisplayId}.json`;
  if (slotId === -1) {
    url = `/meta/item/${itemDisplayId}.json`;
  }
  debug && console.log('Fetching item data for', url);
  const armorData = await fetchJson(url) as ItemData;
  debug && console.log('Fetched item data successfully for', itemDisplayId);
  return {
    modelFiles: filterFilesByRaceGender(armorData.ModelFiles || {}, targetRace, targetGender),
    textureFiles: [
      ...filterFilesByRaceGender(armorData.TextureFiles || {}, targetRace, targetGender),
      ...[armorData.Textures, armorData.Textures2].filter((obj) => obj !== null)
        .flatMap((obj) => Object.entries(obj).flatMap(([_, value]) => (<FilteredFile>{
          fileDataId: value,
        }))),
    ],
    geosetIds: resolveGeosetId(slotId, armorData),
    hideGeosetIds: resolveHideGeosetId(armorData, targetRace, targetGender),
  };
}

async function getExportCharacterRpcParams(characterData: CharacterData): Promise<ExportCharacterParams> {
  const customizations: { [optionId: string]: number } = {};
  for (const cust of characterData.customizations) { customizations[cust.optionId] = cust.choiceId; }

  const geosetIds = new Set<number>();
  const hideGeosetIds = new Set<number>();
  const slotIds = Object.values(EquipmentSlot).filter((value) => typeof value === 'number') as number[];
  for (const slotId of slotIds) {
    if (!characterData.equipment || !characterData.equipment[slotId.toString()]) continue;
    const itemId = characterData.equipment[slotId.toString()];
    const slotData = await processItemData(slotId, itemId, characterData.raceId, characterData.genderId);
    if ([
      EquipmentSlot.Head,
      EquipmentSlot.Shoulder,
      EquipmentSlot.Legs,
      EquipmentSlot.Boots,
      EquipmentSlot.Chest,
      EquipmentSlot.Gloves,
      EquipmentSlot.Belt,
      EquipmentSlot.Back,
      EquipmentSlot.Tabard,
    ].includes(slotId)) {
      slotData?.geosetIds?.forEach((geosetId) => geosetIds.add(geosetId));
      slotData?.hideGeosetIds?.forEach((hideGeosetId) => hideGeosetIds.add(hideGeosetId));
    }
  }

  const rpcParams: ExportCharacterParams = {
    race: characterData.raceId,
    gender: characterData.genderId,
    chrModelId: characterData.character.ChrModelId,
    customizations,
    format: 'obj',
    include_animations: true,
    include_base_clothing: false,
    geosetIds: Array.from(geosetIds),
    hideGeosetIds: Array.from(hideGeosetIds),
  };

  return rpcParams;
}

export { EquipmentSlot };

export async function prepareNpcExport(npcId: number): Promise<NpcExportPreparation> {
  // Fetch NPC metadata only ONCE.
  const npcMetaUrl = `/meta/npc/${npcId}.json`;
  debug && console.log('Fetching NPC metadata for', npcMetaUrl);
  const npcData = await fetchJson(npcMetaUrl) as NPCData;
  debug && console.log('Fetched NPC metadata successfully for', npcId);

  // ==== Character-based NPC ====
  if (npcData.Character) {
    debug && console.log('Processing character-based NPC', npcId);
    const character = npcData.Character;
    const characterData = {
      character,
      equipment: npcData.Equipment,
      customizations: npcData.Creature?.CreatureCustomizations || [],
      textureFiles: npcData.TextureFiles,
      race: character.Race,
      gender: character.Gender,
      raceId: character.Race,
      genderId: character.Gender,
    };

    // Process all equipment slots dynamically
    const equipmentSlots: EquipmentSlotData[] = [];
    const slotIds = Object.values(EquipmentSlot).filter((value) => typeof value === 'number') as number[];
    for (const slotId of slotIds) {
      if (!characterData.equipment || !characterData.equipment[slotId.toString()]) continue;
      const itemId = characterData.equipment[slotId.toString()];
      const slotData = await processItemData(slotId, itemId, characterData.raceId, characterData.genderId);
      equipmentSlots.push({
        slotId: slotId.toString(),
        hasModel: MODEL_SLOTS.has(slotId.toString()),
        data: slotData,
      });
    }

    // Generate RPC parameters
    const rpcParams = await getExportCharacterRpcParams(characterData);

    // Extract NPC base texture
    const npcTextureFile = characterData.textureFiles
      ? Object.values(characterData.textureFiles)[0]?.[0]?.FileDataId : null;

    return {
      type: 'character',
      exportCharRpcParams: rpcParams,
      npcTextureFile,
      equipmentSlots,
    };
  }

  // ==== Model-only NPC ====
  if (npcData.Model) {
    debug && console.log('Processing model-only NPC', npcId);

    const modelId: number = npcData.Model;
    const textureIds: number[] = npcData.Textures ? Object.values(npcData.Textures) : [];

    let skinName: string | undefined;
    if (textureIds.length > 0) {
      const skins = await wowExportClient.getModelSkins(modelId);
      const matches = skins.filter((s: { textureIDs: number[] }) => textureIds.every((id) => s.textureIDs.includes(id)));
      debug && console.log('Matched skins:', matches);
      skinName = (matches.length > 0 ? matches[0] : skins[0])?.id;
      debug && console.log('Selected skin:', skinName);
    }

    // ===== Process equipment (if any) =====
    const equipmentSlots: EquipmentSlotData[] = [];

    if (npcData.Equipment) {
      // Assume universal race/gender when none provided (0/2)
      const targetRace = 0;
      const targetGender = 2;

      for (const [slotIdStr, itemId] of Object.entries(npcData.Equipment as Record<string, number>)) {
        const slotId = parseInt(slotIdStr, 10);
        const slotData = await processItemData(slotId, itemId, targetRace, targetGender);

        equipmentSlots.push({
          slotId: slotId.toString(),
          hasModel: MODEL_SLOTS.has(slotId.toString()),
          data: slotData,
        });
      }
    }

    debug && console.log('PrepareNpcExport done', { modelId, skinName });

    return {
      type: 'model',
      baseModel: { fileDataID: modelId, skinName },
      equipmentSlots,
      npcTextureFile: null,
    };
  }

  throw new Error(`Unsupported NPC metadata format for NPC ID ${npcId}`);
}

export async function getDisplayIdFromUrl(url: string) {
  debug && console.log('getDisplayIdFromUrl start', url);
  const text = await fetch(url).then((res) => res.text());
  // data-mv-type-id="42928"
  const displayId = text.match(/data-mv-display-id="(\d+)"/)?.[1];
  if (!displayId) {
    if (/npc\/\d+.json/.test(url)) {
      return parseInt(url.match(/npc\/(\d+).json/)?.[1] ?? '0', 10);
    }
    throw new Error(`Cannot find display id for ${url}`);
  }
  return parseInt(displayId, 10);
}

export async function getItemFileModelIdFromUrl(url: string) {
  const displayId = await getDisplayIdFromUrl(url);
  const itemData = await fetchJson(`/meta/item/${displayId}.json`) as ItemData;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  return Object.values(itemData.ModelFiles)[0][0].FileDataId;
}
