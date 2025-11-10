import type {
  IconConversionOptions, IconFrame, IconSize, IconStyle, MergedIconConversionOptions,
} from './schemas';
import { IconConversionOptionsSchema } from './schemas';

export function mergeIconOptions(
  userOptions: IconConversionOptions,
): MergedIconConversionOptions {
  // Use Zod to parse and apply defaults
  // Note: Zod's type inference doesn't understand that .default() makes fields required in output,
  // but at runtime Zod guarantees size, style, and frame will always be present after parsing.
  const parsed = IconConversionOptionsSchema.parse(userOptions);
  // Type assertion is safe because Zod applies defaults at runtime
  return parsed as MergedIconConversionOptions;
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
  size: IconSize,
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
