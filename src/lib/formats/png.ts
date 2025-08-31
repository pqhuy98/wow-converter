import sharp, { OverlayOptions } from 'sharp';

const debug = false;

// We need to resize the PNG but RGB and alpha are separate, since wow use alpha as mask
export async function resizePng(fromPath: string, targetWidth: number, targetHeight: number) {
  const src = sharp(fromPath);
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
  const rgbBuffer = src.clone().removeAlpha();
  const alphaChan = src.clone().extractChannel('alpha');

  // Resize RGB without alpha-weighting
  const resizedRgb = rgbBuffer
    .resize({ width: targetWidth, height: targetHeight, fit: 'outside' });

  // Resize alpha separately; if alpha is data/mask
  const resizedAlpha = alphaChan
    .resize({ width: targetWidth, height: targetHeight, fit: 'outside' });

  // Rejoin
  return resizedRgb.joinChannel(await resizedAlpha.toBuffer()).png().toBuffer();
}

export interface PngDraw {
  pngPath: string;
  x: number;
  y: number;
  width: number;
  height: number;
}

// x, y, width, height are in percentage of the base texture
export async function drawPngsOnBasePng(
  basePngPath: string,
  draws: PngDraw[],
): Promise<Buffer> {
  const base = sharp(basePngPath);
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

    if (await isAbnormalTransparency(draw.pngPath)) {
      console.log('Abnormal transparency, removing alpha', draw.pngPath);
      input = await sharp(
        // cannot chain sharp operations otherwise RGB will turn to 0
        await sharp(draw.pngPath).removeAlpha().toBuffer(),
      ).resize({ width: targetWidth, height: targetHeight, fit: 'outside' })
        .toBuffer();
    } else {
      input = await resizePng(draw.pngPath, targetWidth, targetHeight);
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

async function isAbnormalTransparency(pngPath: string): Promise<boolean> {
  const png = sharp(pngPath);
  const metadata = await png.metadata();
  if (!metadata.width || !metadata.height) {
    throw new Error('PNG must have width and height metadata');
  }
  const width = metadata.width;
  const height = metadata.height;

  // If no alpha channel, cannot have this abnormal transparency pattern
  if (metadata.channels && metadata.channels < 4) {
    return false;
  }

  const alphaBuffer = await png
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
