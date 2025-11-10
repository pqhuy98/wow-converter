import fs from 'fs';
import path from 'path';
import sharp from 'sharp';

import { FRAME_FILE_MAP, STYLE_FOLDER_MAP } from './constants';
import type { IconFrame, IconSize, IconStyle } from './schemas';

const REMOVE_COLORS_THRESHOLD = 1;

/**
 * Resolve path to frame PNG asset in resources/icon-frames/
 */
export function resolveFramePath(
  size: Exclude<IconSize, 'original'>,
  style: IconStyle,
  frame: IconFrame,
): string {
  const projectRoot = process.cwd();
  const sizeFolder = size;
  const styleFolder = STYLE_FOLDER_MAP[style];
  const frameFile = FRAME_FILE_MAP[frame];

  // Custom frames (ATT, UPG, SSH, SSP) are stored in custom_frames directory
  const isCustomFrame = frame === 'att' || frame === 'upg' || frame === 'ssh' || frame === 'ssp';

  if (isCustomFrame) {
    // SSH and SSP are stored in misc folder (only available in 64x64)
    if (frame === 'ssh' || frame === 'ssp') {
      return path.join(
        projectRoot,
        'resources',
        'icon-frames',
        'custom_frames',
        'misc',
        `${frameFile}.png`,
      );
    }
    // ATT and UPG are stored in custom_frames/{size}/{style}/
    return path.join(
      projectRoot,
      'resources',
      'icon-frames',
      'custom_frames',
      sizeFolder,
      styleFolder,
      `${frameFile}.png`,
    );
  }

  // Regular frames are in the main directory
  return path.join(
    projectRoot,
    'resources',
    'icon-frames',
    sizeFolder,
    styleFolder,
    `${frameFile}.png`,
  );
}

/**
 * Compute the optimal integer margin to crop from a given dimension
 * so that the cropped size is as close as possible to (1 - crop_percent) of the original.
 */
export function optimalCropMargin(dim: number, cropPercent: number): number {
  if (cropPercent <= 0 || cropPercent >= 1) {
    return 0;
  }

  const target = dim * (cropPercent / 2);
  let mFloor = Math.floor(target);
  let mCeil = Math.ceil(target);

  // Ensure at least 1 pixel is cropped if possible
  if (mFloor < 1) mFloor = 1;
  if (mCeil < 1) mCeil = 1;

  // Safety check: cropping should not make the dimension non-positive
  if (dim - 2 * mFloor <= 0) mFloor = 1;
  if (dim - 2 * mCeil <= 0) mCeil = 1;

  // The target remaining ratio should be (1 - crop_percent)
  const errorFloor = Math.abs((dim - 2 * mFloor) / dim - (1 - cropPercent));
  const errorCeil = Math.abs((dim - 2 * mCeil) / dim - (1 - cropPercent));

  return errorFloor <= errorCeil ? mFloor : mCeil;
}

/**
 * Remove colors from fully transparent pixels (set RGB to 0 where alpha <= threshold)
 */
export async function removeColorsFromAlphaPixels(
  image: sharp.Sharp,
): Promise<sharp.Sharp> {
  const { data, info } = await image
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });

  const channels = info.channels;
  const width = info.width;
  const height = info.height;

  // Process pixel data
  for (let i = 0; i < data.length; i += channels) {
    const alpha = data[i + channels - 1]; // Last channel is alpha
    if (alpha <= REMOVE_COLORS_THRESHOLD) {
      // Set RGB to 0
      data[i] = 0; // R
      if (channels >= 3) data[i + 1] = 0; // G
      if (channels >= 3) data[i + 2] = 0; // B
    }
  }

  return sharp(data, {
    raw: {
      width,
      height,
      channels,
    },
  });
}

/**
 * Load frame image from file system
 */
export async function loadFrameImage(framePath: string): Promise<Buffer> {
  if (!fs.existsSync(framePath)) {
    throw new Error(`Frame image not found: ${framePath}`);
  }
  return fs.promises.readFile(framePath);
}

/**
 * Check if file exists
 */
export function fileExists(filePath: string): boolean {
  return fs.existsSync(filePath);
}
