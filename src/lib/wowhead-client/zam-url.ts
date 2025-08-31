import { LRUCache } from 'lru-cache';

export type ZamExpansion = 'classic' | 'tbc' | 'wrath' | 'cata' | 'mists' | 'live' | 'ptr' | 'ptr2' | 'latest-available';
export type ZamType = 'npc' | 'item' | 'dressing-room' | 'character-customization';

export type BaseZamUrl = {
  expansion: ZamExpansion;
}

export type NpcZamUrl = BaseZamUrl & { type: 'npc', displayId: number };
export type DressingRoomZamUrl = BaseZamUrl & { type: 'dressing-room', hash: string };
export type ItemZamUrl = BaseZamUrl & { type: 'item', displayId: number, slotId: number | null };
export type CharacterCustomizationZamUrl = BaseZamUrl & { type: 'character-customization', chrModelId: number };
export type ZamUrl = NpcZamUrl | ItemZamUrl | DressingRoomZamUrl | CharacterCustomizationZamUrl;

export function getZamBaseUrl(expansion: ZamExpansion): string {
  return `https://wow.zamimg.com/modelviewer/${expansion}`;
}

const expansionMap: Record<string, ZamExpansion> = {
  // wowhead expansion -> zam expansion
  classic: 'classic',
  tbc: 'tbc',
  wotlk: 'wrath',
  cata: 'cata',
  'mop-classic': 'mists',
  retail: 'live',
  ptr: 'ptr',
  'ptr-2': 'ptr2',
  '': 'live',
} as const;
const expansions = [...Object.entries(expansionMap)];
const expansionsReverse = [...expansions].reverse();

export async function getZamUrlFromWowheadUrl(url: string): Promise<ZamUrl> {
  const type = getTypeFromUrl(url);
  if (!type) throw new Error(`Cannot infer type from wowhead url: ${url}`);
  if (type === 'character-customization') {
    throw new Error('Cannot get character customization from wowhead url');
  }

  const expansion = getExpansionFromUrl(url) || 'live';
  if (type === 'dressing-room') return { expansion, type, hash: url.split('#')[1].split('?')[0] };

  // npc or item
  const displayId = await getDisplayIdFromUrl(url);
  if (type === 'item') {
    return {
      expansion, type, displayId, slotId: null,
    };
  }
  return { expansion, type, displayId };
}

export function getExpansionFromUrl(url: string): ZamExpansion | undefined {
  for (const [wowheadEx, zamEx] of expansions) {
    const wowheadPrefix = `https://www.wowhead.com/${wowheadEx ? `${wowheadEx}/` : ''}`;
    const wowZamPrefix = `https://wow.zamimg.com/modelviewer/${zamEx}`;
    if (url.startsWith(wowheadPrefix) || url.startsWith(wowZamPrefix)) return zamEx;
  }
  return undefined;
}

export function getWowheadPrefix(expansion: ZamExpansion): string {
  const [wowheadEx] = expansionsReverse.find(([_, zamEx]) => zamEx === expansion) || [];
  return `https://www.wowhead.com${wowheadEx ? `/${wowheadEx}` : ''}`;
}

export async function getLatestExpansionHavingUrl(path: string): Promise<ZamExpansion> {
  for (const [_, zamEx] of expansionsReverse) {
    try {
      const base = getZamBaseUrl(zamEx);
      const res = await fetch(`${base}/${path}`);
      if (!res.ok) throw new Error(`Failed to fetch latest available npc: ${res.status} ${res.statusText}`);
      return zamEx;
    } catch (e) {
      // continue
    }
  }
  throw new Error('No expansion found');
}

function getTypeFromUrl(url: string): ZamType | undefined {
  if (/\/npc[=/]/i.test(url)) return 'npc';
  if (/\/item[=/]/i.test(url)) return 'item';
  if (/\/dressing-room(\?.+)?[#]/i.test(url)) return 'dressing-room';
  return undefined;
}

async function getDisplayIdFromUrl(url: string): Promise<number> {
  const byJson = url.match(/\/(?:npc|item)\/(\d+)\.json/i)?.[1];
  if (byJson) return parseInt(byJson, 10);
  // Fallback to parsing embedded data attributes
  const html = await fetchWithCache(url);
  const m = html.match(/data-mv-display-id="(\d+)"/);
  if (!m) {
    throw new Error(`Cannot extract displayId from wowhead url: ${url}`);
  }
  return parseInt(m[1], 10);
}

const respCache = new LRUCache<string, string>({ max: 200 });
export async function fetchWithCache(url: string): Promise<string> {
  const cached = respCache.get(url);
  if (cached) return cached;
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`Failed to fetch wowhead url: ${url} ${res.status} ${res.statusText} ${await res.text()}`);
  }
  const text = await res.text();
  respCache.set(url, text);
  return text;
}
