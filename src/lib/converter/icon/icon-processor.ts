import sharp from 'sharp';

import { HD_DESATURATION_FRAMES, SIZE_MAPPING } from './constants';
import {
  loadFrameImage,
  optimalCropMargin,
  removeColorsFromAlphaPixels,
  resolveExtraFramePath,
  resolveFramePath,
} from './icon-utils';
import type {
  IconFrame,
  IconSize,
  IconStyle,
  RequiredIconConversionOptions,
} from './schemas';
import { getCustomFrameData, resolveEffectiveSize } from './utils';

// Frame cache: key -> Buffer
const frameCache = new Map<string, Buffer>();

/**
 * Load and cache frame image
 */
async function getCachedFrame(
  size: Exclude<IconSize, 'original'>,
  style: IconStyle,
  frame: IconFrame,
): Promise<Buffer> {
  const cacheKey = `${size}-${style}-${frame}`;
  if (frameCache.has(cacheKey)) {
    return frameCache.get(cacheKey)!;
  }

  const framePath = resolveFramePath(size, style, frame);
  const frameBuffer = await loadFrameImage(framePath);
  frameCache.set(cacheKey, frameBuffer);
  return frameBuffer;
}

/**
 * Crop image symmetrically by percentage
 */
async function cropImage(
  image: sharp.Sharp,
  cropPercent: number,
): Promise<sharp.Sharp> {
  const metadata = await image.metadata();
  const width = metadata.width ?? 0;
  const height = metadata.height ?? 0;

  if (cropPercent <= 0 || cropPercent >= 1 || width === 0 || height === 0) {
    return image;
  }

  const cropMarginW = optimalCropMargin(width, cropPercent);
  const cropMarginH = optimalCropMargin(height, cropPercent);

  const newWidth = width - 2 * cropMarginW;
  const newHeight = height - 2 * cropMarginH;

  if (newWidth <= 0 || newHeight <= 0) {
    return image;
  }

  return image.extract({
    left: cropMarginW,
    top: cropMarginH,
    width: newWidth,
    height: newHeight,
  });
}

/**
 * Apply desaturation and contrast for disabled frames in HD style
 */
function applyDisabledFrameEffects(image: sharp.Sharp): sharp.Sharp {
  // Default values from Reforgerator
  const saturation = 0.5;
  const contrast = 0.82;

  return image
    .modulate({
      saturation,
    })
    .linear(contrast, -(128 * contrast) + 128);
}

/**
 * Clear alpha channel (composite with black background and convert to RGB)
 */
async function clearAlpha(image: sharp.Sharp, width: number, height: number): Promise<sharp.Sharp> {
  const blackBackground = sharp({
    create: {
      width,
      height,
      channels: 4,
      background: {
        r: 0, g: 0, b: 0, alpha: 1,
      },
    },
  });

  const imageBuffer = await image.toBuffer();
  const compositeBuffer = await blackBackground.composite([{ input: imageBuffer, blend: 'over' }]).toBuffer();
  // Convert to RGB (remove alpha channel) - matching Reforgerator behavior
  return sharp(compositeBuffer).removeAlpha();
}

/**
 * Process PNG image with frames, styles, and extras
 */
export async function processIconImage(
  inputPng: string | Buffer,
  options: RequiredIconConversionOptions,
): Promise<Buffer> {
  // Load input image
  let image = typeof inputPng === 'string' ? sharp(inputPng) : sharp(inputPng);
  const metadata = await image.metadata();
  const originalWidth = metadata.width ?? 0;
  const originalHeight = metadata.height ?? 0;

  if (originalWidth === 0 || originalHeight === 0) {
    throw new Error('Invalid image dimensions');
  }

  // Determine effective size and target dimensions
  const effectiveSize = options.size === 'original'
    ? resolveEffectiveSize(options.size, originalWidth, originalHeight)
    : options.size;

  const isSizeOriginal = options.size === 'original';
  const frameSize = SIZE_MAPPING[effectiveSize];
  const canvasSize = isSizeOriginal
    ? { width: originalWidth, height: originalHeight }
    : frameSize;

  // Check for custom frame positioning/sizing
  const customFrameData = getCustomFrameData(options.frame, effectiveSize, options.style);
  let targetSize = canvasSize;
  let customPosition: readonly [number, number] | null = null;

  if (customFrameData) {
    if (isSizeOriginal) {
      // Scale custom values from native frame space -> canvas space
      const sx = canvasSize.width / frameSize.width;
      const sy = canvasSize.height / frameSize.height;
      targetSize = {
        width: Math.max(1, Math.round(customFrameData.im_size[0] * sx)),
        height: Math.max(1, Math.round(customFrameData.im_size[1] * sy)),
      };
      customPosition = [
        Math.round(customFrameData.im_pos[0] * sx),
        Math.round(customFrameData.im_pos[1] * sy),
      ] as const;
    } else {
      // Use raw absolute values in native frame space
      targetSize = {
        width: customFrameData.im_size[0],
        height: customFrameData.im_size[1],
      };
      customPosition = customFrameData.im_pos;
    }
  }

  // Apply crop if enabled
  if (options.extras.crop) {
    image = await cropImage(image, 0.1);
  }

  // Resize to target size
  image = image.resize(targetSize.width, targetSize.height, {
    fit: 'fill',
    kernel: 'lanczos3',
  });

  // Ensure RGBA
  image = image.ensureAlpha();

  // Load and composite frame if not 'none'
  if (options.frame !== 'none') {
    const frameBuffer = await getCachedFrame(effectiveSize, options.style, options.frame);
    const frameImage = sharp(frameBuffer);

    // Resize frame to canvas size if needed
    const frameMetadata = await frameImage.metadata();
    const frameNeedsResize = frameMetadata.width !== canvasSize.width
      || frameMetadata.height !== canvasSize.height;

    const finalFrameImage = frameNeedsResize
      ? frameImage.resize(canvasSize.width, canvasSize.height, {
        fit: 'fill',
        kernel: 'lanczos3',
      })
      : frameImage;

    // Handle custom frame positioning
    if (customPosition) {
      // Create transparent canvas at canvas size
      const canvas = sharp({
        create: {
          width: canvasSize.width,
          height: canvasSize.height,
          channels: 4,
          background: {
            r: 0,
            g: 0,
            b: 0,
            alpha: 0,
          },
        },
      });

      // Composite: canvas -> image at custom position -> frame on top
      const imageBuffer = await image.toBuffer();
      const frameBufferData = await finalFrameImage.toBuffer();
      image = canvas.composite([
        {
          input: imageBuffer,
          left: customPosition[0],
          top: customPosition[1],
          blend: 'over',
        },
        {
          input: frameBufferData,
          left: 0,
          top: 0,
          blend: 'over',
        },
      ]);
    } else {
      // Regular frame compositing: image first, then frame on top
      const imageBuffer = await image.toBuffer();
      image = sharp(imageBuffer).composite([
        {
          input: await finalFrameImage.toBuffer(),
          blend: 'over',
        },
      ]);
    }
  }

  // Apply black frame overlay if enabled
  if (options.extras.blackFrame) {
    const blackFramePath = resolveExtraFramePath(effectiveSize, 'blackFrame');
    const blackFrameBuffer = await loadFrameImage(blackFramePath);
    const blackFrameImage = sharp(blackFrameBuffer);

    // Resize black frame to target size if needed
    const blackFrameMetadata = await blackFrameImage.metadata();
    const blackFrameNeedsResize = blackFrameMetadata.width !== targetSize.width
      || blackFrameMetadata.height !== targetSize.height;

    const finalBlackFrame = blackFrameNeedsResize
      ? blackFrameImage.resize(targetSize.width, targetSize.height, {
        fit: 'fill',
        kernel: 'lanczos3',
      })
      : blackFrameImage;

    const imageBuffer = await image.toBuffer();
    image = sharp(imageBuffer).composite([
      {
        input: await finalBlackFrame.toBuffer(),
        blend: 'over',
      },
    ]);
  }

  // Apply hero frame overlay if enabled
  if (options.extras.heroFrame) {
    const heroFramePath = resolveExtraFramePath(effectiveSize, 'heroFrame');
    const heroFrameBuffer = await loadFrameImage(heroFramePath);
    const heroFrameImage = sharp(heroFrameBuffer);

    const imageBuffer = await image.toBuffer();
    image = sharp(imageBuffer).composite([
      {
        input: await heroFrameImage.toBuffer(),
        blend: 'over',
      },
    ]);
  }

  // Apply desaturation/contrast for disabled frames in HD style
  if (options.style === 'reforged-hd' && HD_DESATURATION_FRAMES.has(options.frame)) {
    image = applyDisabledFrameEffects(image);
  }

  // Handle alpha channel
  if (options.extras.alpha) {
    image = await removeColorsFromAlphaPixels(image);
  } else {
    // Clear alpha: composite with black background and convert to RGB
    const finalMetadata = await image.metadata();
    image = await clearAlpha(image, finalMetadata.width ?? canvasSize.width, finalMetadata.height ?? canvasSize.height);
  }

  // Export as PNG buffer
  return image.png().toBuffer();
}
