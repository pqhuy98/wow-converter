import { LRUCache } from 'lru-cache';

export type ZamExpansion = 'classic' | 'tbc' | 'wrath' | 'cata' | 'mists' | 'live' | 'ptr' | 'ptr2';
export type ZamType = 'npc' | 'item';

export type BaseZamUrl = {
  expansion: ZamExpansion;
}

export type NpcZamUrl = BaseZamUrl & { type: 'npc', displayId: number };
export type ItemZamUrl = BaseZamUrl & { type: 'item', displayId: number, slotId: number | null };
export type CharacterModelZamUrl = BaseZamUrl & { type: 'character', modelId: number };
export type ZamUrl = NpcZamUrl | ItemZamUrl | CharacterModelZamUrl;

export function getZamBaseUrl(expansion: ZamExpansion): string {
  return `https://wow.zamimg.com/modelviewer/${expansion}`;
}

export async function parseWowheadUrlToZam(url: string, fallbackType?: ZamType, fallbackExpansion?: ZamExpansion): Promise<ZamUrl> {
  const type = getTypeFromUrl(url) || fallbackType;
  if (!type) throw new Error(`Cannot infer type from wowhead url: ${url}`);

  const displayId = await getDisplayIdFromUrl(url);
  const expansion = getExpansionFromUrl(url) || fallbackExpansion || 'live';
  if (type === 'item') {
    return { expansion, type, displayId, slotId: null };
  }
  return { expansion, type, displayId };
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

export function getExpansionFromUrl(url: string): ZamExpansion | undefined {
  for (const [wowheadEx, zamEx] of Object.entries(expansionMap)) {
    const wowheadPrefix = `https://www.wowhead.com/${wowheadEx ? `${wowheadEx}/` : ''}`
    const wowZamPrefix = `https://wow.zamimg.com/modelviewer/${zamEx}`
    if (url.startsWith(wowheadPrefix) || url.startsWith(wowZamPrefix)) return zamEx;
  }
  return undefined;
}

function getTypeFromUrl(url: string): ZamType | undefined {
  if (/\/npc[=\/]/i.test(url)) return 'npc';
  if(/\/item[=\/]/i.test(url)) return 'item';
  return undefined;
}

async function getDisplayIdFromUrl(url: string): Promise<number> {
  const byJson = url.match(/\/(?:npc|item)\/(\d+)\.json/i)?.[1];
  if (byJson) return parseInt(byJson, 10);
  // Fallback to parsing embedded data attributes
  const html = await fetchHtml(url);
  const m = html.match(/data-mv-display-id="(\d+)"/);
  if (!m) {
    throw new Error(`Cannot extract displayId from wowhead url: ${url}`);
  }
  return parseInt(m[1], 10);
}

const htmlCache = new LRUCache<string, string>({ max: 200 });
async function fetchHtml(url: string): Promise<string> {
  const cached = htmlCache.get(url);
  if (cached) return cached;
  const res = await fetch(url);
  const text = await res.text();
  htmlCache.set(url, text);
  return text;
}



