import displayModByBonus from '../../../golang/internal/wowhead/display_mod_bonuses.json';
import { customFetch } from './http-client';
import { getWowheadPrefix, ZamExpansion } from './zam-url';

export interface GathererAppearancesMap {
  [index: string]: [number, string];
}

export interface GathererEntryJson {
  id: number;
  slot: number;
  displayid: number;
  appearances?: GathererAppearancesMap;
  name: string;
}

export interface GathererEntry {
  jsonequip?: GathererEntryJson;
  json: GathererEntryJson;
}

interface ItemData {
  itemId: number
  displayId: number
  name: string
}

export async function gatherItems(expansion: ZamExpansion, items: {itemId: number, itemBonus: number}[]): Promise<ItemData[]> {
  if (!items.length) {
    return [];
  }

  const url = buildGathererUrl(expansion, items.map((v) => v.itemId));
  console.log(url);
  const res = await customFetch(url);
  const body = await res.text();
  if (!body) {
    return [];
  }

  const payload = extractAddDataPayload(body);
  const data = parseGathererPayload(payload);

  const result: ItemData[] = [];
  for (const { itemId, itemBonus } of items) {
    const key = String(itemId);
    const entry = data[key];
    if (entry) {
      const displayId = selectDisplayIdForCharacter(entry, itemBonus);
      if (typeof displayId === 'number' && Number.isFinite(displayId)) {
        result.push({
          itemId,
          displayId,
          name: entry.json.name,
        });
      }
    }
  }
  return result;
}

function buildGathererUrl(expansion: ZamExpansion, itemIds: number[]): string {
  const items = itemIds.join(',');
  return `${getWowheadPrefix(expansion)}/gatherer?items=${items}`;
}

function extractAddDataPayload(scriptText: string): string {
  // Capture the 3rd argument (object literal) to WH.Gatherer.addData(a, b, {...});
  const match = scriptText.match(/WH\.Gatherer\.addData\([^,]+,[^,]+,\s*(\{[\s\S]*\})\);?/);
  if (!match || !match[1]) {
    console.log(scriptText);
    throw new Error('Failed to extract Gatherer payload');
  }
  return match[1];
}

function parseGathererPayload(payload: string): Record<string, GathererEntry> {
  const parsed: unknown = JSON.parse(payload);
  if (!isRecord(parsed)) {
    throw new Error('Invalid Gatherer payload: not an object');
  }
  // Narrow values to expected shape as best-effort
  return parsed as Record<string, GathererEntry>;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function selectDisplayIdForCharacter(entry: GathererEntry, bonus: number): number {
  const idx = displayModIndex(bonus);
  const appearances = entry.json.appearances ?? entry.jsonequip?.appearances ?? {};
  return appearances[idx]?.[0] ?? entry.json.displayid;
}

const displayModByBonusId = new Map<string, number>(Object.entries(displayModByBonus));

function displayModIndex(bonus: number): number {
  if (!bonus) return 0;
  return displayModByBonusId.get(String(bonus)) ?? 0;
}
