import fsExtra from 'fs-extra';
import path from 'path';

import { pngsToBlps } from '@/lib/formats/blp/blp';
import { getDefaultConfig } from '@/lib/global-config';
import { waitUntil } from '@/lib/utils';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { getListFiles } from '../../../server/controllers/shared';
import { processIconImage } from './icon-processor';
import type { IconConversionOptions, IconFrame } from './schemas';
import { mergeIconOptions } from './utils';

export interface IconExportItem {
  texturePath: string;
  options?: IconConversionOptions;
  outputPath?: string;
}

export class IconExporter {
  private readonly fileNameToFileDataID = new Map<string, number>();

  private assetDir = '';

  private initialized = false;

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

    this.initialized = true;
  }

  /**
   * Get or export PNG texture path
   * Returns the resolved PNG file path, or throws an error if not found
   */
  async getOrExportPngPath(texturePath: string): Promise<string> {
    await this.initialize();

    // Find fileDataID from cached map
    const fileDataID = this.fileNameToFileDataID.get(texturePath);
    if (!fileDataID) {
      throw new Error(`Texture not found: ${texturePath}`);
    }

    // Construct expected PNG path (replace .blp with .png, or add .png if no extension)
    const pngPath = `${texturePath.replace(/\.(blp|png|tga|dds)$/i, '')}.png`;
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
        throw new Error(`Failed to export texture: ${texturePath}`);
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
   * Convert PNG to icon buffer
   */
  async convertPngToIconBuffer(
    pngPath: string,
    options: IconConversionOptions,
  ): Promise<Buffer> {
    const pngBuffer = await fsExtra.readFile(pngPath);
    const mergedOptions = mergeIconOptions(options);
    return processIconImage(pngBuffer, mergedOptions);
  }

  /**
   * Get default BLP filename from texture path
   */
  private getDefaultBlpFilename(texturePath: string): string {
    const filename = texturePath.split('/').pop() ?? texturePath;
    return filename.replace(/\.(blp|png|tga|dds)$/i, '.blp');
  }

  /**
   * Generate Warcraft 3 path for an icon based on frame type
   */
  getWc3Path(texturePath: string, frame: IconFrame): string {
    const filename = texturePath.split('/').pop() ?? texturePath;
    const baseName = filename.replace(/\.(blp|png|jpg|jpeg)$/i, '');

    switch (frame) {
      case 'btn':
        return `ReplaceableTextures\\CommandButtons\\BTN_${baseName}.blp`;
      case 'disbtn':
        return `ReplaceableTextures\\CommandButtonsDisabled\\DISBTN_${baseName}.blp`;
      case 'pas':
        return `ReplaceableTextures\\PassiveButtons\\PAS_${baseName}.blp`;
      case 'dispas':
        return `ReplaceableTextures\\CommandButtonsDisabled\\DISPAS_${baseName}.blp`;
      case 'atc':
        return `ReplaceableTextures\\CommandButtons\\ATC_${baseName}.blp`;
      case 'disatc':
        return `ReplaceableTextures\\CommandButtonsDisabled\\DISATC_${baseName}.blp`;
      case 'upg':
        return `ReplaceableTextures\\CommandButtons\\UPG_${baseName}.blp`;
      case 'att':
        return `ReplaceableTextures\\CommandButtons\\ATT_${baseName}.blp`;
      case 'ssh':
        return `scorescreen-hero-${baseName}.blp`;
      case 'ssp':
        return `scorescreen-player-${baseName}.blp`;
      case 'none':
        return filename;
      default:
        return filename;
    }
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
        const finalPngPath = await this.getOrExportPngPath(item.texturePath);

        const pngBuffer = item.options
          ? await this.convertPngToIconBuffer(finalPngPath, item.options)
          : await fsExtra.readFile(finalPngPath);

        const blpPath = item.outputPath
          ? path.join(outputDir, item.outputPath)
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
