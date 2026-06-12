import { LRUCache } from 'lru-cache';

import { customFetch } from './http-client';

export type ZamExpansion = 'classic' | 'tbc' | 'wrath' | 'cata' | 'mists' | 'live' | 'ptr' | 'ptr2' | 'latest-available';
export type ZamType = 'npc' | 'object' | 'item' | 'dressing-room' | 'character-customization' | 'itemvisual';

export type BaseZamUrl = {
  expansion: ZamExpansion;
}

export type NpcZamUrl = BaseZamUrl & { type: 'npc', displayId: number };
export type ObjectZamUrl = BaseZamUrl & { type: 'object', displayId: number };
export type DressingRoomZamUrl = BaseZamUrl & { type: 'dressing-room', hash: string };
export type ItemZamUrl = BaseZamUrl & { type: 'item', displayId: number, slotId: number | null };
export type CharacterCustomizationZamUrl = BaseZamUrl & { type: 'character-customization', chrModelId: number };
export type ItemVisualZamUrl = BaseZamUrl & { type: 'itemvisual', visualId: number };
export type ZamUrl = NpcZamUrl | ObjectZamUrl | ItemZamUrl | DressingRoomZamUrl | CharacterCustomizationZamUrl | ItemVisualZamUrl;

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
  const type = getTypeFromWowheadUrl(url);
  if (!type) throw new Error(`Cannot infer type from wowhead url: ${url}`);
  if (type === 'character-customization') {
    throw new Error('Cannot get character customization from wowhead url');
  }
  if (type === 'itemvisual') {
    throw new Error('Cannot get item visual from wowhead url');
  }

  const expansion = getExpansionFromUrl(url) || 'live';
  if (type === 'dressing-room') return { expansion, type, hash: url.split('#')[1].split('?')[0] };

  // npc or item
  const { displayId, slotId } = await getDisplayIdFromUrl(url);
  if (type === 'item') {
    return {
      expansion, type, displayId, slotId,
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
      const res = await customFetch(`${base}/${path}`);
      if (!res.ok) throw new Error(`Failed to fetch latest available npc: ${res.status} ${res.statusText}`);
      return zamEx;
    } catch (e) {
      // continue
    }
  }
  throw new Error(`Invalid Wowhead URL ${path}`);
}

function getTypeFromWowheadUrl(url: string): ZamType | undefined {
  if (/\/npc[=/]/i.test(url)) return 'npc';
  if (/\/spell[=/]/i.test(url)) return 'npc';
  if (/\/object[=/]/i.test(url)) return 'object';
  if (/\/item[=/]/i.test(url)) return 'item';
  if (/\/dressing-room(\?.+)?[#]/i.test(url)) return 'dressing-room';
  return undefined;
}

function getEntityIdFromWowheadUrl(url: string): number | null {
  const m = url.match(/\/(?:item|npc|object)[=/](\d+)/i);
  return m ? parseInt(m[1], 10) : null;
}

function parseDisplayIdFromGathererHtml(html: string, entityId: number): number | null {
  const match = html.match(/WH\.Gatherer\.addData\([^,]+,[^,]+,\s*(\{[\s\S]*\})\);?/);
  if (!match?.[1]) return null;
  try {
    const data = JSON.parse(match[1]) as Record<string, {
      json?: { displayid?: number };
      jsonequip?: { displayid?: number };
    }>;
    const entry = data[String(entityId)];
    const displayid = entry?.json?.displayid ?? entry?.jsonequip?.displayid;
    return typeof displayid === 'number' ? displayid : null;
  } catch {
    return null;
  }
}

async function fetchItemDisplayIdViaGatherer(expansion: ZamExpansion, itemId: number): Promise<number | null> {
  const gathererUrl = `${getWowheadPrefix(expansion)}/gatherer?items=${itemId}`;
  const res = await customFetch(gathererUrl);
  if (!res.ok) return null;
  return parseDisplayIdFromGathererHtml(await res.text(), itemId);
}

async function getDisplayIdFromUrl(url: string): Promise<{ displayId: number, slotId: number | null }> {
  const byJson = url.match(/\/(?:npc|item)\/(\d+)\.json/i)?.[1];
  if (byJson) return { displayId: parseInt(byJson, 10), slotId: null };

  const html = await fetchWithCache(url);
  const m = html.match(/data-mv-display-id="(\d+)"/);
  if (m) {
    const slotM = html.match(/data-mv-slot="(\d+)"/);
    return { displayId: parseInt(m[1], 10), slotId: slotM ? parseInt(slotM[1], 10) : null };
  }

  const type = getTypeFromWowheadUrl(url);
  const entityId = getEntityIdFromWowheadUrl(url);
  if (type === 'item' && entityId !== null) {
    const fromPage = parseDisplayIdFromGathererHtml(html, entityId);
    if (fromPage !== null) {
      return { displayId: fromPage, slotId: null };
    }
    const expansion = getExpansionFromUrl(url) || 'live';
    const fromGatherer = await fetchItemDisplayIdViaGatherer(expansion, entityId);
    if (fromGatherer !== null) {
      return { displayId: fromGatherer, slotId: null };
    }
  }

  throw new Error(`Cannot extract displayId from wowhead url: ${url}`);
}

const respCache = new LRUCache<string, string>({ max: 200 });
export async function fetchWithCache(url: string): Promise<string> {
  const cached = respCache.get(url);
  if (cached) return cached;
  const res = await customFetch(url);
  if (!res.ok) {
    throw new Error(`Failed to fetch wowhead url: ${url} ${res.status} ${res.statusText} ${await res.text()}`);
  }
  const text = await res.text();
  respCache.set(url, text);
  return text;
}
