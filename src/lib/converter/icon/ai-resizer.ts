import fsExtra from 'fs-extra';
import os from 'os';
import path from 'path';
import sharp from 'sharp';
import { upscaler } from 'upscayl-node';

import { getPngDimensions } from '@/lib/formats/png';

import { IconSize } from './schemas';

const debug = false;

/**
 * Handles AI-powered image upscaling with job deduplication
 */
export class AiResizer {
  // Track in-progress AI resize jobs to prevent duplicates
  private readonly aiResizeJobs = new Map<string, Promise<string>>();

  /**
   * Resize PNG image using AI upscaling
   * Upscales the source image to the target size and saves it to the output path
   * @param pngPath Source PNG file path
   * @param targetSize Target size (128 or 256)
   * @param outputPath Path where the upscaled image will be saved
   * @returns Promise that resolves to the output path
   */
  async resizePng(pngPath: string, targetSize: IconSize, outputPath: string): Promise<string> {
    // Create a unique job key for this texture/size combination
    const jobKey = `${pngPath}-${targetSize}-${outputPath}`;

    // Check if there's already an in-progress job for this texture/size/output
    const existingJobPromise = this.aiResizeJobs.get(jobKey);
    if (existingJobPromise) {
      debug && console.log(`[AI Resize] Job already in progress for ${pngPath} -> ${targetSize}px, waiting for existing job...`);
      return existingJobPromise;
    }

    // Start new AI resize job
    console.log(`[AI Resize] Starting new job for ${pngPath} -> ${targetSize}px`);
    const jobPromise = this.performAiResize(pngPath, targetSize, outputPath);
    this.aiResizeJobs.set(jobKey, jobPromise);

    // Clean up job from map when done (success or error)
    jobPromise
      .then(() => {
        console.log(`[AI Resize] Job completed for ${pngPath} -> ${targetSize}px`);
        this.aiResizeJobs.delete(jobKey);
      })
      .catch((error) => {
        debug && console.error(`[AI Resize] Job failed for ${pngPath} -> ${targetSize}px:`, error);
        this.aiResizeJobs.delete(jobKey);
        throw error;
      });

    return jobPromise;
  }

  /**
   * Perform the actual AI resize operation
   * Upscales the source image to the target size and saves it to the output path
   */
  private async performAiResize(
    pngPath: string,
    targetSize: IconSize,
    outputPath: string,
  ): Promise<string> {
    // Get source dimensions
    const sourceDims = await getPngDimensions(pngPath);
    const sourceWidth = sourceDims.width;
    const sourceHeight = sourceDims.height;
    debug && console.log(`[AI Resize] Source dimensions: ${sourceWidth}x${sourceHeight} for ${pngPath}`);

    // Determine if we need to pre-resize to a standard size
    // Only pre-resize UP to preserve image details - never resize DOWN
    const standardSizes = [64, 128, 256];
    const isStandardSize = standardSizes.includes(sourceWidth) && standardSizes.includes(sourceHeight);

    let inputPath = pngPath;
    let inputWidth = sourceWidth;
    let inputHeight = sourceHeight;

    // If not standard size, normal resize UP to nearest standard size (preserve details)
    if (!isStandardSize) {
      const maxDim = Math.max(sourceWidth, sourceHeight);
      // Find nearest standard size that is >= current size (only resize up)
      let nearestStandard = 64;
      if (maxDim <= 64) {
        nearestStandard = 64;
      } else if (maxDim <= 128) {
        nearestStandard = 128;
      } else if (maxDim <= 256) {
        nearestStandard = 256;
      } else {
        // Source is larger than 256, use it directly (no pre-resize)
        nearestStandard = maxDim;
      }

      // Only pre-resize if we're scaling UP (preserve details by not downscaling)
      if (nearestStandard > maxDim) {
        debug && console.log(`[AI Resize] Pre-resizing ${sourceWidth}x${sourceHeight} UP to ${nearestStandard}x${nearestStandard} before AI upscale`);
        const tempPreResizePath = path.join(os.tmpdir(), `pre-resize-${Date.now()}-${Math.random().toString(36).substring(7)}.png`);
        await sharp(pngPath)
          .resize(nearestStandard, nearestStandard, {
            fit: 'fill',
            kernel: 'lanczos3',
          })
          .toFile(tempPreResizePath);

        inputPath = tempPreResizePath;
        inputWidth = nearestStandard;
        inputHeight = nearestStandard;
      } else {
        // Source is already at or above standard size, use it directly
        debug && console.log(`[AI Resize] Using source ${sourceWidth}x${sourceHeight} directly (already >= standard size)`);
      }
    }

    // Calculate AI scale factor
    let aiScale = 2;
    if (targetSize === '256x256') {
      if (inputWidth === 64) {
        aiScale = 4;
      } else if (inputWidth === 128) {
        aiScale = 2;
      }
    } else if (targetSize === '128x128') {
      if (inputWidth === 64) {
        aiScale = 2;
      }
    }

    debug && console.log(`[AI Resize] Using ${aiScale}x scale to reach ${targetSize}px from ${inputWidth}x${inputHeight}`);

    // Always use high quality model for best results
    // Models are paths like ".../realesrgan-x4fast" or ".../realesrgan-x4plus"
    const models = upscaler.getModels();
    const modelHighQuality = models.find((m) => m.includes('x4plus')) ?? models[models.length - 1];
    const model = modelHighQuality;
    debug && console.log(`[AI Resize] Using model: ${path.basename(model)} for ${aiScale}x scaling`);

    // Ensure output directory exists
    await fsExtra.ensureDir(path.dirname(outputPath));

    try {
      // Upscale directly to output path using the input file (source or pre-resized)
      debug && console.log(`[AI Resize] Starting upscayl process for ${pngPath} -> ${targetSize}px...`);
      const startTime = Date.now();
      await upscaler.upscaleImage(inputPath, outputPath, {
        model,
        scale: aiScale,
        compression: 0,
      });
      const duration = Date.now() - startTime;
      debug && console.log(`[AI Resize] Upscayl completed in ${duration}ms for ${pngPath} -> ${targetSize}px`);
      debug && console.log(`[AI Resize] Result saved at ${outputPath}`);

      return outputPath;
    } finally {
      // Clean up temp pre-resize file if it was created
      if (inputPath !== pngPath && inputPath.startsWith(os.tmpdir())) {
        await fsExtra.remove(inputPath).catch(() => {});
      }
    }
  }
}
