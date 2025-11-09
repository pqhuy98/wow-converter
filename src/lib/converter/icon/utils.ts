import { DEFAULT_ICON_OPTIONS } from './constants';
import type {
  IconConversionOptions, IconFrame, IconSize, IconStyle, RequiredIconConversionOptions,
} from './schemas';

export function mergeIconOptions(
  userOptions: IconConversionOptions,
): RequiredIconConversionOptions {
  return {
    size: userOptions.size ?? DEFAULT_ICON_OPTIONS.size,
    style: userOptions.style ?? DEFAULT_ICON_OPTIONS.style,
    frame: userOptions.frame ?? DEFAULT_ICON_OPTIONS.frame,
    extras: {
      ...DEFAULT_ICON_OPTIONS.extras,
      ...userOptions.extras,
    },
  };
}

export function resolveEffectiveSize(
  size: IconSize,
  originalWidth: number,
  originalHeight: number,
): Exclude<IconSize, 'original'> {
  if (size !== 'original') {
    return size;
  }
  const maxDim = Math.max(originalWidth, originalHeight);
  if (maxDim <= 96) return '64x64';
  if (maxDim <= 192) return '128x128';
  return '256x256';
}

/**
 * Custom frame positioning and sizing data from .ini files
 * Format: { size: { style: { im_pos: [x, y], im_size: [w, h] } } }
 */
type CustomFrameSizeData = Readonly<Record<IconStyle, { im_pos: readonly [number, number]; im_size: readonly [number, number] }>>;
type CustomFrameData = Readonly<Record<Exclude<IconSize, 'original'>, CustomFrameSizeData>>;

const CUSTOM_FRAME_DATA: Readonly<Partial<Record<IconFrame, CustomFrameData>>> = {
  att: {
    '64x64': {
      'classic-sd': { im_pos: [4, 4] as const, im_size: [48, 48] as const },
      'reforged-hd': { im_pos: [2, 2] as const, im_size: [51, 51] as const },
      'classic-hd-2.0': { im_pos: [4, 4] as const, im_size: [48, 48] as const },
    },
    '128x128': {
      'classic-sd': { im_pos: [8, 8] as const, im_size: [96, 96] as const },
      'reforged-hd': { im_pos: [5, 5] as const, im_size: [101, 101] as const },
      'classic-hd-2.0': { im_pos: [8, 8] as const, im_size: [96, 96] as const },
    },
    '256x256': {
      'classic-sd': { im_pos: [16, 16] as const, im_size: [192, 192] as const },
      'reforged-hd': { im_pos: [10, 10] as const, im_size: [202, 202] as const },
      'classic-hd-2.0': { im_pos: [16, 16] as const, im_size: [192, 192] as const },
    },
  },
  upg: {
    '64x64': {
      'classic-sd': { im_pos: [4, 4] as const, im_size: [48, 48] as const },
      'reforged-hd': { im_pos: [2, 2] as const, im_size: [51, 51] as const },
      'classic-hd-2.0': { im_pos: [4, 4] as const, im_size: [48, 48] as const },
    },
    '128x128': {
      'classic-sd': { im_pos: [8, 8] as const, im_size: [96, 96] as const },
      'reforged-hd': { im_pos: [5, 5] as const, im_size: [101, 101] as const },
      'classic-hd-2.0': { im_pos: [8, 8] as const, im_size: [96, 96] as const },
    },
    '256x256': {
      'classic-sd': { im_pos: [16, 16] as const, im_size: [192, 192] as const },
      'reforged-hd': { im_pos: [10, 10] as const, im_size: [202, 202] as const },
      'classic-hd-2.0': { im_pos: [16, 16] as const, im_size: [192, 192] as const },
    },
  },
  ssh: {
    '64x64': {
      'classic-sd': { im_pos: [2, 16] as const, im_size: [32, 32] as const },
      'reforged-hd': { im_pos: [2, 16] as const, im_size: [32, 32] as const },
      'classic-hd-2.0': { im_pos: [2, 16] as const, im_size: [32, 32] as const },
    },
    '128x128': {
      'classic-sd': { im_pos: [4, 32] as const, im_size: [64, 64] as const },
      'reforged-hd': { im_pos: [4, 32] as const, im_size: [64, 64] as const },
      'classic-hd-2.0': { im_pos: [4, 32] as const, im_size: [64, 64] as const },
    },
    '256x256': {
      'classic-sd': { im_pos: [8, 64] as const, im_size: [128, 128] as const },
      'reforged-hd': { im_pos: [8, 64] as const, im_size: [128, 128] as const },
      'classic-hd-2.0': { im_pos: [8, 64] as const, im_size: [128, 128] as const },
    },
  },
  ssp: {
    '64x64': {
      'classic-sd': { im_pos: [2, 16] as const, im_size: [32, 32] as const },
      'reforged-hd': { im_pos: [2, 16] as const, im_size: [32, 32] as const },
      'classic-hd-2.0': { im_pos: [2, 16] as const, im_size: [32, 32] as const },
    },
    '128x128': {
      'classic-sd': { im_pos: [4, 32] as const, im_size: [64, 64] as const },
      'reforged-hd': { im_pos: [4, 32] as const, im_size: [64, 64] as const },
      'classic-hd-2.0': { im_pos: [4, 32] as const, im_size: [64, 64] as const },
    },
    '256x256': {
      'classic-sd': { im_pos: [8, 64] as const, im_size: [128, 128] as const },
      'reforged-hd': { im_pos: [8, 64] as const, im_size: [128, 128] as const },
      'classic-hd-2.0': { im_pos: [8, 64] as const, im_size: [128, 128] as const },
    },
  },
} as const;

/**
 * Get custom frame positioning and sizing data
 * Returns null if frame doesn't have custom positioning
 */
export function getCustomFrameData(
  frame: IconFrame,
  size: Exclude<IconSize, 'original'>,
  style: IconStyle,
): { im_pos: readonly [number, number]; im_size: readonly [number, number] } | null {
  const frameData = CUSTOM_FRAME_DATA[frame];
  if (!frameData || Object.keys(frameData).length === 0) {
    return null;
  }
  const sizeData = frameData[size];
  if (!sizeData) {
    return null;
  }
  const styleData = sizeData[style];
  if (!styleData) {
    return null;
  }
  return styleData;
}
