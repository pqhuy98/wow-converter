import { gatherItems } from './gatherer';
import { EquipmentSlot } from './item-armor';
import {
  CharacterData, EquipmentMap, fetchItemVisualMeta, ItemVisualMap,
} from './objects';
import { itemEnchants } from './snipped-data/dressing-room/item-enchants';
import { raceGenderMap } from './snipped-data/dressing-room/transmog-data';
import { ZamExpansion } from './zam-url';

const PAPERDOLL_SLOTS = {
  1: EquipmentSlot.Head,
  2: EquipmentSlot.Shoulder,
  3: EquipmentSlot.Cloak,
  4: EquipmentSlot.Chest,
  5: EquipmentSlot.Shirt,
  6: EquipmentSlot.Tabard,
  7: EquipmentSlot.Wrist,
  8: EquipmentSlot.Hands,
  9: EquipmentSlot.Waist,
  10: EquipmentSlot.Legs,
  11: EquipmentSlot.Feet,
  12: EquipmentSlot.MainHand,
  13: EquipmentSlot.OffHand,
};

export async function decodeDressingRoom(expansion: ZamExpansion, hash: string): Promise<CharacterData> {
  const clean = (hash || '').replace(/^#/, '');
  if (!clean) {
    return {};
  }

  const latestVersion = getLatestVersion();
  const latestCfg = prepareConfig(latestVersion);
  const pre = decompress(latestCfg, clean);

  const detectedVersion = charValue(latestCfg, pre.charAt(0));
  const version = isFiniteNumber(detectedVersion) && HASH_TEMPLATES[detectedVersion as number] ? (detectedVersion as number) : latestVersion;
  const tpl = HASH_TEMPLATES[version] || HASH_TEMPLATES[latestVersion];
  const cfg = prepareConfig(tpl.version);

  const data = decodeWithTemplate(cfg, tpl, pre) as {
    settings: {
      race: number,
      gender: number,
      mount: number,
    },
    custChoices: {
      [key: string]: { optionId: number, choiceId: number };
    }
    equipment: Record<string, { itemId: number; itemBonus: number, enchant: number }>;
  };
  data.equipment ||= {};
  data.custChoices ||= {};

  Object.entries(data.custChoices).forEach(([k, v]) => {
    if (v.optionId === 0 || v.choiceId === 0) {
      delete data.custChoices[k];
    }
  });
  Object.keys(data.equipment).forEach((k) => {
    if (data.equipment[k].itemId === 0) {
      delete data.equipment[k];
    }
  });

  console.log(JSON.stringify(data, null, 2));
  const itemsWithBonus = Object.values(data.equipment);
  const items = await gatherItems(expansion, itemsWithBonus);

  const equipments: EquipmentMap = Object.fromEntries(Object.entries(data.equipment).map(([dollSlotId, item]) => {
    const slotId = PAPERDOLL_SLOTS[dollSlotId];
    return [slotId, items.find((v) => v.itemId === item.itemId)?.displayId];
  }).filter(([k, v]) => k !== undefined && v !== undefined));

  const itemVisuals: ItemVisualMap = Object.fromEntries(Object.entries(data.equipment).map(([dollSlotId, item]) => {
    const slotId = PAPERDOLL_SLOTS[dollSlotId];
    return [slotId, itemEnchants[item.enchant]?.visual];
  }).filter(([k, v]) => k !== undefined && v !== undefined));

  return {
    Character: {
      Race: data.settings.race,
      Gender: data.settings.gender,
      ChrModelId: raceGenderMap[data.settings.race][data.settings.gender],
    },
    Creature: {
      CreatureCustomizations: Object.entries(data.custChoices)
        .map(([_k, v]) => ({
          optionId: Number(v.optionId),
          choiceId: Number(v.choiceId),
        })).filter((v) => v.optionId > 0),
      CreatureGeosetData: [],
    },
    Equipment: equipments,
    ItemEffects: (await Promise.all(Object.entries(itemVisuals).map(async ([slotId, visualId]) => {
      const itemVisual = await fetchItemVisualMeta({ expansion, type: 'itemvisual', visualId });
      const model = itemVisual.Model || itemVisual.ItemEffects?.[0].Model;
      return {
        Slot: Number(slotId),
        SubClass: 0,
        Model: model,
        Scale: 1,
      };
    }))).filter((v) => (v.Model ?? 0) > 0) as CharacterData['ItemEffects'],
  };
}

type DelimiterIndex = number | boolean | undefined;

interface DecodeConfig {
  encoding: string;
  encodingLength: number;
  delimiters: string[];
  zeroDelimiterCompression: number | false;
}

interface TemplateSegment {
  key?: (string | number)[];
  keyLong?: (string | number)[];
  delimiter?: boolean | number;
  calculatorValue?: string;
  calculatorLongValue?: string;
  buildKey?: (string | number)[];
}

interface HashTemplate {
  version: number;
  data: TemplateSegment[];
  increaseDelimiters?: number;
  decreaseDelimiters?: number;
  modifyEncodingLength?: number;
  zeroDelimiterCompression?: number | false;
}

function getLatestVersion(): number {
  return Object.keys(HASH_TEMPLATES)
    .map((k) => parseInt(k, 10))
    .reduce((a, b) => (isNaN(a) ? b : isNaN(b) ? a : Math.max(a, b)));
}

function prepareConfig(version: number): DecodeConfig {
  const tpl = HASH_TEMPLATES[version] as HashTemplate;
  const encoding = DEFAULT_ENCODING;
  let encodingLength = DEFAULT_ENCODING_LENGTH;
  const delimiters: string[] = [...DEFAULT_DELIMITERS];

  const inc = (tpl && typeof tpl.increaseDelimiters === 'number') ? tpl.increaseDelimiters as number : 0;
  for (let i = 0; i < inc; i++) {
    const ch = encoding.charAt(encodingLength - 1);
    delimiters.push(ch);
    encodingLength -= 1;
  }

  if (tpl && typeof tpl.decreaseDelimiters === 'number') {
    for (let i = 0; i < (tpl.decreaseDelimiters as number); i++) {
      delimiters.pop();
      encodingLength += 1;
    }
  }

  if (tpl && typeof tpl.modifyEncodingLength === 'number') {
    encodingLength += (tpl.modifyEncodingLength as number);
  }

  const zeroDelimiterCompression = (tpl && typeof tpl.zeroDelimiterCompression !== 'undefined')
    ? (tpl.zeroDelimiterCompression as number | false)
    : DEFAULT_ZERO_DELIMITER_COMPRESSION;

  return {
    encoding,
    encodingLength,
    delimiters,
    zeroDelimiterCompression,
  };
}

const DEFAULT_ENCODING = '0zMcmVokRsaqbdrfwihuGINALpTjnyxtgevElBCDFHJKOPQSUWXYZ123456789';
const DEFAULT_ENCODING_LENGTH = 60;
const DEFAULT_DELIMITERS = ['9', '8'];
const DEFAULT_ZERO_DELIMITER_COMPRESSION: number | false = false;

function getDelimiter(cfg: DecodeConfig, idx?: DelimiterIndex): string {
  let i: number = 1;
  if (typeof idx === 'number' && !isNaN(idx)) {
    i = idx;
  }
  const d = cfg.delimiters[i];
  if (typeof d === 'undefined') {
    throw new Error(`Requested undefined delimiter: ${String(idx)}`);
  }
  return d;
}

function getZeroDelimiterCompressionIndicator(cfg: DecodeConfig): string | undefined {
  if (typeof cfg.zeroDelimiterCompression !== 'undefined' && cfg.zeroDelimiterCompression !== false) {
    return getDelimiter(cfg, cfg.zeroDelimiterCompression);
  }
  return undefined;
}

function maxEncodingIndex(cfg: DecodeConfig): number {
  return cfg.encodingLength - 1;
}

function charValue(cfg: DecodeConfig, ch: string): number {
  return cfg.encoding.indexOf(ch);
}

function longValue(cfg: DecodeConfig, s: string): number {
  if (s.length < 2) {
    return charValue(cfg, s);
  }
  const digits = s.split('').reverse();
  let acc = 0;
  for (let pos = 0; pos < digits.length; pos++) {
    let v = charValue(cfg, digits[pos]);
    for (let a = 0; a < pos; a++) {
      v *= maxEncodingIndex(cfg);
    }
    acc += v;
  }
  return acc;
}

function decompress(cfg: DecodeConfig, s: string): string {
  let out = decodeZeroes(cfg, s);
  out = decodeZeroDelimiters(cfg, out);
  return out;
}

function decodeZeroes(cfg: DecodeConfig, s: string): string {
  const chars = s.split('');
  let result = '';
  let run: number | false = false;
  for (let i = 0; i < chars.length; i++) {
    const c = chars[i];
    if (run && c === getDelimiter(cfg, 0)) {
      run = (run as number) + 1;
    } else if (run) {
      const count = cfg.encoding.indexOf(c) + ((run as number) - 1) * maxEncodingIndex(cfg);
      for (let n = 1; n <= count; n++) {
        result += '0';
      }
      run = false;
    } else {
      if (c === getDelimiter(cfg, 0)) {
        run = 1;
      } else {
        result += c;
      }
    }
  }
  return result;
}

function decodeZeroDelimiters(cfg: DecodeConfig, s: string): string {
  const indicator = getZeroDelimiterCompressionIndicator(cfg);
  if (!indicator) {
    return s;
  }
  const chars = s.split('');
  let result = '';
  let run: number | false = false;
  for (let i = 0; i < chars.length; i++) {
    const c = chars[i];
    if (run && c === indicator) {
      run = (run as number) + 1;
    } else if (run) {
      const count = cfg.encoding.indexOf(c) + ((run as number) - 1) * maxEncodingIndex(cfg);
      for (let n = 1; n <= count; n++) {
        result += `0${getDelimiter(cfg)}`;
      }
      run = false;
    } else {
      if (c === indicator) {
        run = 1;
      } else {
        result += c;
      }
    }
  }
  return result;
}

function setValueOnObject(target: Record<string, unknown>, path: (string | number)[], value: number): Record<string, unknown> {
  let obj: Record<string, unknown> = target;
  for (let i = 0; i < path.length; i++) {
    const key = path[i];
    const isLast = i === path.length - 1;
    if (isLast) {
      if (typeof key === 'number') {
        const arr = (obj as unknown as unknown[]);
        (arr as unknown as number[])[key] = value;
      } else {
        obj[key] = value as unknown;
      }
      break;
    }
    if (typeof key === 'number') {
      const arr = (obj as unknown as unknown[]);
      if (!arr[key] || typeof arr[key] !== 'object') {
        arr[key] = {} as unknown;
      }
      obj = arr[key] as Record<string, unknown>;
    } else {
      if (!obj[key] || typeof obj[key] !== 'object') {
        obj[key] = {} as unknown;
      }
      obj = obj[key] as Record<string, unknown>;
    }
  }
  return target;
}

function getHashPieces(cfg: DecodeConfig, hash: string, current: TemplateSegment, next?: TemplateSegment): [string, string] {
  let parts: string[] = [hash];
  const r = next;
  if (
    r
    && !(r as unknown as { collection: unknown }).collection
    && !(r as unknown as { collectionKey: unknown }).collectionKey
    && (typeof (current as { delimiter?: number | string }).delimiter !== 'undefined' || r.delimiter === true)
  ) {
    const a = r.delimiter === true ? getDelimiter(cfg) : getDelimiter(cfg, r.delimiter as number);
    parts = hash.split(a);
    const first = parts.shift() ?? '';
    const rest = a + parts.join(a);
    return [first, rest];
  }
  return [hash, ''];
}

function isFiniteNumber(n: unknown): n is number {
  return typeof n === 'number' && Number.isFinite(n);
}

function decodeWithTemplate(cfg: DecodeConfig, tpl: HashTemplate, rawHash: string): Record<string, unknown> {
  let hash = String(rawHash);
  const build: Record<string, unknown> = {};
  let index = 0;
  while (hash.length && index < tpl.data.length) {
    const seg = tpl.data[index];
    const next = tpl.data[index + 1];

    if (seg.key && Array.isArray(seg.key)) {
      const val = charValue(cfg, hash.substr(0, 1));
      setValueOnObject(build, seg.key, val);
      hash = hash.substr(1);
      index++;
      continue;
    }

    if (seg.keyLong && Array.isArray(seg.keyLong)) {
      const [lhs, rhs] = getHashPieces(cfg, hash, seg, next);
      const val = longValue(cfg, lhs);
      setValueOnObject(build, seg.keyLong, val);
      hash = rhs || '';
      index++;
      continue;
    }

    if (typeof seg.delimiter !== 'undefined') {
      const ch = seg.delimiter === true ? getDelimiter(cfg) : getDelimiter(cfg, seg.delimiter);
      if (hash.startsWith(ch)) {
        hash = hash.substr(1);
      }
      index++;
      continue;
    }

    if (seg.buildKey && (typeof seg.calculatorValue === 'string' || typeof seg.calculatorValue === 'function')) {
      const val = charValue(cfg, hash.substr(0, 1));
      setValueOnObject(build, seg.buildKey, val);
      hash = hash.substr(1);
      index++;
      continue;
    }

    if (seg.buildKey && (typeof seg.calculatorLongValue === 'string' || typeof seg.calculatorLongValue === 'function')) {
      const [lhs, rhs] = getHashPieces(cfg, hash, seg, next);
      const val = longValue(cfg, lhs);
      setValueOnObject(build, seg.buildKey, val);
      hash = rhs || '';
      index++;
      continue;
    }

    // If segment type is unhandled, consume nothing and move on to avoid infinite loop
    index++;
  }
  return build;
}

const HASH_TEMPLATES = {
  1: {
    version: 1,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    modifyEncodingLength: 1,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }],
  },
  2: {
    version: 2,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    modifyEncodingLength: 1,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }],
  },
  3: {
    version: 3,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    modifyEncodingLength: 1,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      key: ['settings', 'blindfolds'],
    }, {
      key: ['settings', 'hornstyle'],
    }, {
      key: ['settings', 'tattoos'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }],
  },
  4: {
    version: 4,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    modifyEncodingLength: 1,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      key: ['settings', 'blindfolds'],
    }, {
      key: ['settings', 'hornstyle'],
    }, {
      key: ['settings', 'tattoos'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMod'],
    }],
  },
  5: {
    version: 5,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      key: ['settings', 'blindfolds'],
    }, {
      key: ['settings', 'hornstyle'],
    }, {
      key: ['settings', 'tattoos'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMod'],
    }],
  },
  6: {
    version: 6,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      key: ['settings', 'blindfolds'],
    }, {
      key: ['settings', 'hornstyle'],
    }, {
      key: ['settings', 'tattoos'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMod'],
    }],
  },
  7: {
    version: 7,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'uprightPosture'],
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      key: ['settings', 'blindfolds'],
    }, {
      key: ['settings', 'hornstyle'],
    }, {
      key: ['settings', 'tattoos'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMod'],
    }],
  },
  8: {
    version: 8,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'uprightPosture'],
    }, {
      key: ['settings', 'pepe'],
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      key: ['settings', 'blindfolds'],
    }, {
      key: ['settings', 'hornstyle'],
    }, {
      key: ['settings', 'tattoos'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMod'],
    }],
  },
  9: {
    version: 9,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'uprightPosture'],
    }, {
      key: ['settings', 'pepe'],
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      key: ['settings', 'blindfolds'],
    }, {
      key: ['settings', 'hornstyle'],
    }, {
      key: ['settings', 'tattoos'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMod'],
    }],
  },
  10: {
    version: 10,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'uprightPosture'],
    }, {
      key: ['settings', 'pepe'],
    }, {
      key: ['settings', 'skincolor'],
    }, {
      key: ['settings', 'hairstyle'],
    }, {
      key: ['settings', 'haircolor'],
    }, {
      key: ['settings', 'facetype'],
    }, {
      key: ['settings', 'features'],
    }, {
      key: ['settings', 'blindfolds'],
    }, {
      key: ['settings', 'hornstyle'],
    }, {
      key: ['settings', 'tattoos'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMod'],
    }],
  },
  11: {
    version: 11,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'pepe'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMod'],
    }],
  },
  12: {
    version: 12,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'pepe'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMainHand'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceOffHand'],
    }],
  },
  13: {
    version: 13,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'pepe'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 15, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 15, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 16, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 16, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 17, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 17, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 18, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 18, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 19, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 19, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 20, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 20, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 21, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 21, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 22, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 22, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 23, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 23, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 24, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 24, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 25, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 25, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 26, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 26, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 27, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 27, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 28, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 28, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 29, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 29, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMainHand'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceOffHand'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'separateShoulders'],
    }],
  },
  14: {
    version: 14,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      key: ['settings', 'race'],
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'pepe'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 15, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 15, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 16, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 16, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 17, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 17, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 18, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 18, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 19, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 19, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 20, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 20, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 21, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 21, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 22, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 22, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 23, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 23, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 24, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 24, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 25, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 25, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 26, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 26, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 27, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 27, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 28, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 28, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 29, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 29, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 30, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 30, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 31, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 31, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 32, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 32, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 33, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 33, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 34, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 34, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 35, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 35, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 36, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 36, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 37, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 37, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 38, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 38, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 39, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 39, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 40, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 40, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 41, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 41, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 42, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 42, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 43, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 43, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 44, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 44, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 45, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 45, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 46, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 46, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 47, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 47, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 48, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 48, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 49, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 49, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMainHand'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceOffHand'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'separateShoulders'],
    }],
  },
  15: {
    version: 15,
    build: 'getCharacterForHash',
    decodingPostProcess: 'decodingPostProcess',
    increaseDelimiters: 1,
    zeroDelimiterCompression: 2,
    data: [{
      calculatorValue: 'getHashVersion',
      buildKey: 'version',
    }, {
      keyLong: ['settings', 'race'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'gender'],
    }, {
      key: ['settings', 'class'],
    }, {
      key: ['settings', 'specialization'],
    }, {
      keyLong: ['settings', 'level'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'npcOptions'],
    }, {
      key: ['settings', 'pepe'],
    }, {
      keyLong: ['settings', 'mount'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 0, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 1, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 2, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 3, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 4, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 5, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 6, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 7, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 8, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 9, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 10, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 11, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 12, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 13, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 14, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 15, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 15, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 16, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 16, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 17, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 17, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 18, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 18, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 19, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 19, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 20, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 20, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 21, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 21, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 22, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 22, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 23, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 23, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 24, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 24, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 25, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 25, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 26, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 26, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 27, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 27, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 28, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 28, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 29, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 29, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 30, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 30, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 31, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 31, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 32, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 32, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 33, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 33, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 34, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 34, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 35, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 35, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 36, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 36, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 37, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 37, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 38, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 38, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 39, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 39, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 40, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 40, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 41, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 41, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 42, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 42, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 43, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 43, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 44, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 44, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 45, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 45, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 46, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 46, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 47, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 47, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 48, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 48, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 49, 'optionId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['custChoices', 49, 'choiceId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 1, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 2, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 3, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 4, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 5, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 6, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 7, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 8, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 9, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 10, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 11, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 12, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 13, 'enchant'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemId'],
    }, {
      delimiter: true,
    }, {
      keyLong: ['equipment', 14, 'itemBonus'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceMainHand'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'artifactAppearanceOffHand'],
    }, {
      delimiter: true,
    }, {
      key: ['settings', 'separateShoulders'],
    }],
  },
};
