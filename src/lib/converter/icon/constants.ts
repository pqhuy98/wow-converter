import type {
  IconFrame,
  IconSize,
  IconStyle,
  RequiredIconConversionOptions,
} from './schemas';

export const SIZE_MAPPING: Readonly<Record<Exclude<IconSize, 'original'>, { width: number; height: number }>> = {
  '64x64': { width: 64, height: 64 },
  '128x128': { width: 128, height: 128 },
  '256x256': { width: 256, height: 256 },
} as const;

export const STYLE_FOLDER_MAP: Readonly<Record<IconStyle, string>> = {
  'classic-sd': 'ClassicSD',
  'reforged-hd': 'ReforgedHD',
  'classic-hd-2.0': 'ClassicHD2.0',
} as const;

export const FRAME_FILE_MAP: Readonly<Record<IconFrame, string>> = {
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
  none: 'NONE',
} as const;

export const HD_DESATURATION_FRAMES: ReadonlySet<IconFrame> = new Set([
  'disbtn',
  'dispas',
  'disatc',
]);

export const DEFAULT_ICON_OPTIONS: RequiredIconConversionOptions = {
  size: '256x256',
  style: 'classic-sd',
  frame: 'btn',
  extras: {
    crop: false,
    blackFrame: false,
    heroFrame: false,
    alpha: true,
  },
} as const;
