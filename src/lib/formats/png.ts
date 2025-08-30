import sharp from 'sharp';

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

  const overlays = await Promise.all(draws.map(async (draw) => {
    const targetWidth = Math.max(1, Math.round(meta.width! * draw.width));
    const targetHeight = Math.max(1, Math.round(meta.height! * draw.height));
    const left = Math.round(meta.width! * draw.x);
    const top = Math.round(meta.height! * draw.y);

    const input = await resizePng(draw.pngPath, targetWidth, targetHeight);
    return { input, left, top } as const;
  }));

  return sharp(baseBuffer)
    .composite(overlays.map((o) => ({ input: o.input, left: o.left, top: o.top })))
    .png()
    .toBuffer();
}
