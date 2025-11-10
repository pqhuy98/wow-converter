import fsExtra from 'fs-extra';
import path from 'path';
import sharp from 'sharp';

import { pngsToBlps } from '@/lib/formats/blp/blp';
import { getPngDimensions } from '@/lib/formats/png';
import { getDefaultConfig } from '@/lib/global-config';
import { waitUntil } from '@/lib/utils';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { getListFiles } from '../../../server/controllers/shared';
import { AiResizer } from './ai-resizer';
import { processIconImage } from './icon-processor';
import type { IconConversionOptions, IconFrame, IconSize } from './schemas';
import { mergeIconOptions } from './utils';
import { getWc3Path } from './wc3.utils';

export interface IconExportItem {
  texturePath: string;
  options?: IconConversionOptions;
  outputPath?: string;
}

export class IconExporter {
  private readonly fileNameToFileDataID = new Map<string, number>();

  private assetDir = '';

  private initialized = false;

  private aiResizer: AiResizer | null = null;

  /**
   * Initialize the exporter by loading file mappings and asset directory
   * Idempotent - safe to call multiple times
   */
  private async initialize(): Promise<void> {
    if (this.initialized) return;

    await waitUntil(() => wowExportClient.isReady);
    const config = await getDefaultConfig();
    this.assetDir = config.wowExportAssetDir;

    const listFiles = await getListFiles();
    for (const file of listFiles) {
      this.fileNameToFileDataID.set(file.fileName, file.fileDataID);
    }

    this.aiResizer = new AiResizer();

    this.initialized = true;
  }

  /**
   * AI Resize PNG and cache the result
   * Caches AI-upscaled images in the same directory as the source with __ai{size} suffix
   * Returns the path to the cached upscaled image
   */
  private async resizeAiPngWithCache(pngPath: string, targetSize: IconSize): Promise<string> {
    await this.initialize();
    if (!this.aiResizer) {
      throw new Error('AI resizer not initialized');
    }

    // Extract numeric size from IconSize ('64x64' -> 64, '128x128' -> 128, '256x256' -> 256)
    const sizeNum = targetSize === '64x64' ? 64 : targetSize === '128x128' ? 128 : 256;
    // Generate cache filename: same directory, append __ai{size}.png after original filename
    const cachePath = `${pngPath}__ai${sizeNum}.png`;

    // Check if cached file exists
    if (await fsExtra.pathExists(cachePath)) {
      return cachePath;
    }

    // Perform AI resize to cache path
    return this.aiResizer.resizePng(pngPath, targetSize, cachePath);
  }

  /**
   * Exports PNG texture from WoW data by path, returning the absolute path to the PNG file
   * @returns The absolute path to the PNG file
   * @throws Error if texture is not found or path is invalid
   */
  async exportPngByPath(wowTexturePath: string): Promise<string> {
    await this.initialize();

    // Find fileDataID from cached map
    const fileDataID = this.fileNameToFileDataID.get(wowTexturePath);
    if (!fileDataID) {
      throw new Error(`Texture not found: ${wowTexturePath}`);
    }

    // Construct expected PNG path (replace .blp with .png, or add .png if no extension)
    const pngPath = `${wowTexturePath.replace(/\.(blp|png|tga|dds)$/i, '')}.png`;
    const resolvedPngPath = path.join(this.assetDir, pngPath);

    // Verify resolved path is within asset directory (security check)
    const resolvedAssetDir = path.resolve(this.assetDir);
    const resolvedPath = path.resolve(resolvedPngPath);
    if (!resolvedPath.startsWith(resolvedAssetDir)) {
      throw new Error('Access denied: path outside asset directory');
    }

    // Check if PNG already exists, if not export it
    let finalPath = resolvedPath;
    if (!await fsExtra.pathExists(resolvedPath)) {
      // Export texture if not already exported
      const textures = await wowExportClient.exportTextures([fileDataID]);
      if (textures.length === 0) {
        throw new Error(`Failed to export texture: ${wowTexturePath}`);
      }

      const exportedPngPath = textures.find((t) => /\.png$/i.test(t.file))?.file;
      if (!exportedPngPath) {
        throw new Error('Failed to export texture as PNG');
      }

      // Verify exported path is within asset directory
      const resolvedExportedPath = path.resolve(exportedPngPath);
      if (!resolvedExportedPath.startsWith(resolvedAssetDir)) {
        throw new Error('Access denied: exported path outside asset directory');
      }

      finalPath = resolvedExportedPath;

      // Check if file exists after export
      if (!await fsExtra.pathExists(finalPath)) {
        throw new Error('Texture file not found after export');
      }
    }

    return finalPath;
  }

  /**
   * Resize PNG using normal algorithm (shift 1px up-left to match AI resize alignment)
   */
  private async resizePngNormal(pngPath: string, targetSize: number): Promise<Buffer> {
    const resizedBuffer = await sharp(pngPath)
      .resize(targetSize + 1, targetSize + 1, {
        fit: 'fill',
        kernel: 'lanczos3',
      })
      .toBuffer();
    return sharp(resizedBuffer)
      .extract({
        left: 1,
        top: 1,
        width: targetSize,
        height: targetSize,
      })
      .png()
      .toBuffer();
  }

  /**
   * Convert PNG to icon buffer
   */
  async convertPngToIconBuffer(
    pngPath: string,
    options: IconConversionOptions,
  ): Promise<Buffer> {
    let pngBuffer: Buffer;

    // Parse merged options to get defaults applied
    const mergedOptions = mergeIconOptions(options);
    const { size, resizeMode } = mergedOptions;

    // If size is "original", use original image without resizing
    if (size === 'original') {
      pngBuffer = await fsExtra.readFile(pngPath);
    } else {
      // Extract target size from size option ('64x64' -> 64, '128x128' -> 128, '256x256' -> 256)
      const targetSize = size === '64x64' ? 64
        : size === '128x128' ? 128
          : 256;

      // Get source dimensions to check if resize is needed
      const sourceDims = await getPngDimensions(pngPath);
      const sourceSize = Math.min(sourceDims.width, sourceDims.height);

      // Only resize if source is different from target
      if (sourceSize === targetSize) {
        pngBuffer = await fsExtra.readFile(pngPath);
      } else if (resizeMode === 'ai' && sourceSize < targetSize) {
        // AI resize - only if source is smaller than target
        const finalPngPath = await this.resizeAiPngWithCache(pngPath, size);
        pngBuffer = await fsExtra.readFile(finalPngPath);
      } else {
        // Normal resize (or AI requested but source >= target, so fallback to normal)
        pngBuffer = await this.resizePngNormal(pngPath, targetSize);
      }
    }

    return processIconImage(pngBuffer, mergedOptions);
  }

  /**
   * Get default BLP filename from texture path
   */
  private getDefaultBlpFilename(wowTexturePath: string): string {
    const filename = wowTexturePath.split('/').pop() ?? wowTexturePath;
    return filename.replace(/\.(blp|png|tga|dds)$/i, '.blp');
  }

  /**
   * Generate Warcraft 3 path for an icon based on frame type
   */
  private getWc3Path(wowTexturePath: string, frame: IconFrame): string {
    return getWc3Path(wowTexturePath, frame);
  }

  /**
   * Export textures to BLP files
   * Returns the count and paths of all exported files
   */
  async exportToBlp(
    items: IconExportItem[],
    outputDir: string,
  ): Promise<{ count: number; paths: string[] }> {
    await this.initialize();

    const conversionTasks: Array<{ png: Buffer; blpPath: string }> = [];

    for (const item of items) {
      try {
        const finalPngPath = await this.exportPngByPath(item.texturePath);

        const pngBuffer = item.options
          ? await this.convertPngToIconBuffer(finalPngPath, item.options)
          : await fsExtra.readFile(finalPngPath);

        const blpPath = item.outputPath
          ? (() => {
            // Validate output path doesn't escape output directory
            const joinedPath = path.join(outputDir, item.outputPath);
            const resolvedOutputDir = path.resolve(outputDir);
            const resolvedBlpPath = path.resolve(joinedPath);

            // Ensure the resolved path is within the output directory
            if (!resolvedBlpPath.startsWith(resolvedOutputDir)) {
              throw new Error('Access denied: output path outside output directory');
            }

            return joinedPath;
          })()
          : item.options?.frame
            ? path.join(outputDir, this.getWc3Path(item.texturePath, item.options.frame).replace(/\\/g, '/'))
            : path.join(outputDir, this.getDefaultBlpFilename(item.texturePath));

        await fsExtra.ensureDir(path.dirname(blpPath));
        conversionTasks.push({ png: pngBuffer, blpPath });
      } catch (error) {
        console.error(`Error processing ${item.texturePath}:`, error);
      }
    }

    if (conversionTasks.length === 0) {
      throw new Error('No valid items to export');
    }

    // Convert all PNGs to BLP files
    await pngsToBlps(conversionTasks);

    // Return relative paths from outputDir
    const relativePaths = conversionTasks.map((task) => {
      const relativePath = path.relative(outputDir, task.blpPath);
      return relativePath.replace(/\\/g, '/');
    });

    return {
      count: conversionTasks.length,
      paths: relativePaths,
    };
  }
}
