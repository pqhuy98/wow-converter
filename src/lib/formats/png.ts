import sharp, { OverlayOptions } from 'sharp';

import { sharpFromExportAsset } from '@/lib/export-asset-store';

const debug = false;

/**
 * Get PNG image dimensions
 * @returns The width and height of the PNG image
 * @throws Error if image dimensions are invalid
 */
export async function getPngDimensions(pngPath: string): Promise<{ width: number; height: number }> {
  const metadata = await sharpFromExportAsset(pngPath).metadata();
  if (!metadata.width || !metadata.height) {
    throw new Error('Invalid image dimensions');
  }
  return { width: metadata.width, height: metadata.height };
}

// We need to resize the PNG but RGB and alpha are separate, since wow use alpha as mask
export async function resizePng(from: string | Buffer, targetWidth: number, targetHeight: number) {
  const fromPath = typeof from === 'string' ? from : '<buffer>';
  const src = typeof from === 'string' ? sharpFromExportAsset(from) : sharp(from);
  const meta = await src.metadata();

  debug && console.log('Original image metadata', fromPath, meta);

  // If no alpha, resize normally
  if (meta.channels !== 4) {
    console.log('No alpha, resizing normally', fromPath);
    return src
      .resize({ width: targetWidth, height: targetHeight, fit: 'outside' })
      .png()
      .toBuffer();
  }

  // Split channels
  debug && console.log('Alpha, resizing with separate channels', fromPath);
  const rgbBuffer = await src.clone().removeAlpha().toBuffer();
  const alphaChan = await src.clone().extractChannel('alpha').toBuffer();

  // Resize RGB without alpha-weighting
  const resizedRgb = await sharp(rgbBuffer)
    .resize({ width: targetWidth, height: targetHeight, fit: 'outside' })
    .toBuffer();

  // Resize alpha separately; if alpha is data/mask
  const resizedAlpha = await sharp(alphaChan)
    .resize({ width: targetWidth, height: targetHeight, fit: 'outside' })
    .toBuffer();

  // Rejoin
  return sharp(resizedRgb).joinChannel(resizedAlpha).png().toBuffer();
}

export interface PngDraw {
  png: Buffer;
  /** Relative path key for logging/hash only. */
  pngPath?: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

// x, y, width, height are in percentage of the base texture
export async function drawPngsOnBasePng(
  basePng: string | Buffer,
  draws: PngDraw[],
): Promise<Buffer> {
  const base = typeof basePng === 'string' ? sharpFromExportAsset(basePng) : sharp(basePng);
  const meta = await base.metadata();

  if (!meta.width || !meta.height) {
    throw new Error('Base PNG must have width and height metadata');
  }

  const baseBuffer = await base.toBuffer();

  if (!draws.length) {
    return baseBuffer;
  }

  const overlays = await Promise.all(draws.map(async (draw): Promise<OverlayOptions> => {
    const targetWidth = Math.max(1, Math.round(meta.width! * draw.width));
    const targetHeight = Math.max(1, Math.round(meta.height! * draw.height));
    const left = Math.round(meta.width! * draw.x);
    const top = Math.round(meta.height! * draw.y);

    let input: Buffer;

    if (await isAbnormalTransparency(draw.png)) {
      console.log('Abnormal transparency, removing alpha', draw.pngPath ?? '<buffer>');
      input = await sharp(
        // cannot chain sharp operations otherwise RGB will turn to 0
        await sharp(draw.png).removeAlpha().toBuffer(),
      ).resize({ width: targetWidth, height: targetHeight, fit: 'outside' })
        .toBuffer();
    } else {
      input = await resizePng(draw.png, targetWidth, targetHeight);
    }

    return {
      input, left, top,
    };
  }));

  return sharp(baseBuffer)
    .composite(overlays)
    .png()
    .toBuffer();
}

async function isAbnormalTransparency(png: string | Buffer): Promise<boolean> {
  const img = typeof png === 'string' ? sharpFromExportAsset(png) : sharp(png);
  const metadata = await img.metadata();
  if (!metadata.width || !metadata.height) {
    throw new Error('PNG must have width and height metadata');
  }
  const width = metadata.width;
  const height = metadata.height;

  // If no alpha channel, cannot have this abnormal transparency pattern
  if (metadata.channels && metadata.channels < 4) {
    return false;
  }

  const alphaBuffer = await img
    .ensureAlpha()
    .extractChannel('alpha')
    .raw()
    .toBuffer();

  // Abnormal if every pixel in 0-based odd columns (1, 3, 5...) are all fully transparent
  for (let i = 1; i < width; i += 2) {
    for (let j = 0; j < height; j++) {
      const idx = j * width + i;
      if (alphaBuffer[idx] !== 0) {
        return false;
      }
    }
  }
  return true;
}
