/** WoW expansion IDs (matches Map.db2 ExpansionID / wow.export constants). */
export const WOW_EXPANSION_ALL = -1 as const;

export interface WowExpansion {
  id: number;
  name: string;
  shortName: string;
}

export const WOW_EXPANSIONS: WowExpansion[] = [
  { id: 0, name: 'Classic', shortName: 'Classic' },
  { id: 1, name: 'The Burning Crusade', shortName: 'TBC' },
  { id: 2, name: 'Wrath of the Lich King', shortName: 'WotLK' },
  { id: 3, name: 'Cataclysm', shortName: 'Cataclysm' },
  { id: 4, name: 'Mists of Pandaria', shortName: 'MoP' },
  { id: 5, name: 'Warlords of Draenor', shortName: 'WoD' },
  { id: 6, name: 'Legion', shortName: 'Legion' },
  { id: 7, name: 'Battle for Azeroth', shortName: 'BfA' },
  { id: 8, name: 'Shadowlands', shortName: 'SL' },
  { id: 9, name: 'Dragonflight', shortName: 'DF' },
  { id: 10, name: 'The War Within', shortName: 'TWW' },
  { id: 11, name: 'Midnight', shortName: 'Midnight' },
];

export function getWowExpansion(id: number): WowExpansion | undefined {
  return WOW_EXPANSIONS.find((e) => e.id === id);
}

/** Expansions that appear in the current map list (for a compact filter bar). */
export function expansionsPresentInMaps(maps: { expansionID?: number }[]): WowExpansion[] {
  const ids = new Set<number>();
  for (const m of maps) {
    if (typeof m.expansionID === 'number' && m.expansionID >= 0) ids.add(m.expansionID);
  }
  return WOW_EXPANSIONS.filter((e) => ids.has(e.id));
}
