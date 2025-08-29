import sharp from 'sharp';

// We need to resize the PNG but RGB and alpha are separate, since wow use alpha as mask
export async function resizePng(fromPath: string, targetWidth: number, targetHeight: number) {
  const src = sharp(fromPath);
  const meta = await src.metadata();

  console.log('Original image metadata', fromPath, meta);

  // If no alpha, resize normally
  if (meta.channels !== 4) {
    console.log('No alpha, resizing normally', fromPath);
    return src
      .resize({ width: targetWidth, height: targetHeight, fit: 'outside' })
      .png()
      .toBuffer();
  }

  // Split channels
  console.log('Alpha, resizing with separate channels', fromPath);
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
