import sharp from 'sharp';

import {
  HD_DESATURATION_FRAMES, SIZE_MAPPING,
} from './constants';
import {
  loadFrameImage,
  optimalCropMargin,
  removeColorsFromAlphaPixels,
  resolveFramePath,
} from './icon-utils';
import type {
  IconExtras,
  IconFrame,
  IconSize,
  IconStyle,
  MergedIconConversionOptions,
} from './schemas';
import { IconExtrasSchema } from './schemas';
import { getCustomFrameData } from './utils';

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
 * Process PNG image with frames, styles, and extras
 */
export async function processIconImage(
  inputPng: string | Buffer,
  options: MergedIconConversionOptions,
): Promise<Buffer> {
  // Load input image
  let image = typeof inputPng === 'string' ? sharp(inputPng) : sharp(inputPng);
  const metadata = await image.metadata();
  const originalWidth = metadata.width ?? 0;
  const originalHeight = metadata.height ?? 0;

  if (originalWidth === 0 || originalHeight === 0) {
    throw new Error('Invalid image dimensions');
  }

  // Defaults are already applied via mergeIconOptions
  const { size, style, frame } = options;

  // Determine frame size: if size is 'original', derive from image dimensions
  let frameSize: { width: number; height: number };
  let canvasSize: { width: number; height: number };

  if (size === 'original') {
    // If frame is 'none', use original image dimensions
    if (frame === 'none') {
      frameSize = { width: originalWidth, height: originalHeight };
      canvasSize = frameSize;
    } else {
      // Use closest standard size for frame (64, 128, or 256)
      const imageSize = Math.min(originalWidth, originalHeight);
      if (imageSize <= 64) {
        frameSize = SIZE_MAPPING['64x64'];
      } else if (imageSize <= 128) {
        frameSize = SIZE_MAPPING['128x128'];
      } else {
        frameSize = SIZE_MAPPING['256x256'];
      }
      canvasSize = frameSize;
    }
  } else {
    frameSize = SIZE_MAPPING[size];
    canvasSize = frameSize;
  }

  // Determine effective size for custom frame data lookup
  // If size is 'original', use the frame size we determined (or closest standard size if frame is 'none')
  const effectiveSizeForFrame: Exclude<IconSize, 'original'> = size === 'original'
    ? (frame === 'none'
      ? (originalWidth <= 64 && originalHeight <= 64 ? '64x64'
        : originalWidth <= 128 && originalHeight <= 128 ? '128x128'
          : '256x256')
      : (frameSize.width === 64 ? '64x64' : frameSize.width === 128 ? '128x128' : '256x256'))
    : size;

  // Check for custom frame positioning/sizing
  const customFrameData = getCustomFrameData(frame, effectiveSizeForFrame, style);
  let targetSize = canvasSize;
  let customPosition: readonly [number, number] | null = null;

  if (customFrameData) {
    // Use raw absolute values in native frame space
    targetSize = {
      width: customFrameData.im_size[0],
      height: customFrameData.im_size[1],
    };
    customPosition = customFrameData.im_pos;
  }

  // Apply defaults for extras using Zod
  const extras: IconExtras = IconExtrasSchema.parse(options.extras ?? {});

  // Apply crop if enabled
  if (extras.crop) {
    image = await cropImage(image, 0.1);
  }

  // Resize to target size (skip if size is 'original' and frame is 'none' - already at original size)
  if (!(size === 'original' && frame === 'none')) {
    image = image.resize(targetSize.width, targetSize.height, {
      fit: 'fill',
      kernel: 'lanczos3',
    });
  }

  // Ensure RGBA
  image = image.ensureAlpha();

  // Load and composite frame if not 'none'
  if (frame !== 'none') {
    const frameBuffer = await getCachedFrame(effectiveSizeForFrame, style, frame);
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

  // Apply desaturation/contrast for disabled frames in HD style
  if (style === 'reforged-hd' && HD_DESATURATION_FRAMES.has(frame)) {
    image = applyDisabledFrameEffects(image);
  }

  // Handle alpha channel - always keep alpha and remove colors from transparent pixels
  // This cleanup step prevents color bleeding artifacts in transparent areas that can occur
  // during compositing operations (frames, resizing, etc.). Reforgerator also performs this
  // cleanup when preserving alpha channel (extras_alpha=True).
  image = await removeColorsFromAlphaPixels(image);

  // Export as PNG buffer
  return image.png().toBuffer();
}
