// Icon conversion types - shared between frontend and backend
// Keep in sync with wow-converter/src/lib/converter/icon/index.ts

// Size options
export type IconSize = '64x64' | '128x128' | '256x256' | 'original';

// Style options
export type IconStyle = 'classic-sd' | 'reforged-hd' | 'classic-hd-2.0';

// Frame/border types
export type IconFrame = 'btn' | 'disbtn' | 'pas' | 'dispas' | 'atc' | 'disatc' | 'att' | 'upg' | 'ssh' | 'ssp' | 'none';

// Extras configuration
export interface IconExtras {
  readonly crop?: boolean; // Apply 10% symmetric crop
}

// Resize mode options
export type IconResizeMode = 'normal' | 'ai';

// Main conversion options
export interface IconConversionOptions {
  readonly size: IconSize; // Output size (required)
  readonly style?: IconStyle; // Frame style, default: 'classic-hd-2.0'
  readonly frame?: IconFrame; // Frame type, default: 'none'
  readonly extras?: IconExtras; // Extra processing options
  readonly resizeMode?: IconResizeMode; // Resize mode, default: 'normal'
}

// Frame label mapping for display
export const FRAME_LABEL_MAP: Readonly<Record<IconFrame, string>> = {
  btn: 'BTN',
  disbtn: 'DISBTN',
  pas: 'PAS',
  dispas: 'DISPAS',
  atc: 'ATC',
  disatc: 'DISATC',
  att: 'ATT',
  upg: 'UPG',
  ssh: 'SSH',
  ssp: 'SSP',
  none: 'Raw',
} as const;

// Style label mapping for display
export const STYLE_LABEL_MAP: Readonly<Record<IconStyle, string>> = {
  'classic-sd': 'Classic SD',
  'reforged-hd': 'Reforged HD',
  'classic-hd-2.0': 'Classic HD 2.0',
} as const;
