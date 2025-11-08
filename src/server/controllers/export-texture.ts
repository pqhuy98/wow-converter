import { createHash } from 'crypto';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';

import { getDefaultConfig } from '@/lib/global-config';
import { waitUntil } from '@/lib/utils';
import { wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { getListFiles } from './shared';

export async function ControllerExportTexture(router: express.Router) {
  await waitUntil(() => wowExportClient.isReady);
  const config = await getDefaultConfig();
  const assetDir = config.wowExportAssetDir;

  // Create Map for fast fileName -> fileDataID lookups
  const listFiles = await getListFiles();
  const fileNameToFileDataID = new Map<string, number>();
  for (const file of listFiles) {
    fileNameToFileDataID.set(file.fileName, file.fileDataID);
  }

  // Serve PNG texture, exporting if missing
  router.get(/^\/texture\/png\/(.+)$/, async (req, res) => {
    try {
      // Extract texture path from URL (captured by regex group)
      const match = req.path.match(/^\/texture\/png\/(.+)$/);
      if (!match || !match[1]) {
        return res.status(400).json({ error: 'Texture path is required' });
      }

      // Normalize path (handle URL encoding)
      const normalizedPath = decodeURIComponent(match[1]);

      // Find fileDataID from cached map
      const fileDataID = fileNameToFileDataID.get(normalizedPath);
      if (!fileDataID) {
        return res.status(404).json({ error: `Texture not found: ${normalizedPath}` });
      }

      // Construct expected PNG path (replace .blp with .png, or add .png if no extension)
      const pngPath = `${normalizedPath.replace(/\.(blp|png|tga|dds)$/i, '')}.png`;
      const resolvedPngPath = path.join(assetDir, pngPath);

      // Verify resolved path is within asset directory (security check)
      const resolvedAssetDir = path.resolve(assetDir);
      const resolvedPath = path.resolve(resolvedPngPath);
      if (!resolvedPath.startsWith(resolvedAssetDir)) {
        return res.status(403).json({ error: 'Access denied: path outside asset directory' });
      }

      // Check if PNG already exists, if not export it
      let finalPath = resolvedPath;
      if (!await fsExtra.pathExists(resolvedPath)) {
        // Export texture if not already exported
        const textures = await wowExportClient.exportTextures([fileDataID]);
        if (textures.length === 0) {
          return res.status(404).json({ error: `Failed to export texture: ${normalizedPath}` });
        }

        const exportedPngPath = textures.find((t) => /\.png$/i.test(t.file))?.file;
        if (!exportedPngPath) {
          return res.status(500).json({ error: 'Failed to export texture as PNG' });
        }

        // Verify exported path is within asset directory
        const resolvedExportedPath = path.resolve(exportedPngPath);
        if (!resolvedExportedPath.startsWith(resolvedAssetDir)) {
          return res.status(403).json({ error: 'Access denied: exported path outside asset directory' });
        }

        finalPath = resolvedExportedPath;

        // Check if file exists after export
        if (!await fsExtra.pathExists(finalPath)) {
          return res.status(404).json({ error: 'Texture file not found after export' });
        }
      }

      // Generate ETag for caching
      const stats = await fsExtra.stat(finalPath);
      const etag = createHash('md5').update(`${finalPath}-${stats.mtime.getTime()}-${stats.size}`).digest('hex');

      // Check if client has cached version
      const ifNoneMatch = req.headers['if-none-match'];
      if (ifNoneMatch === etag) {
        return res.status(304).end();
      }

      res.setHeader('Content-Type', 'image/png');
      res.setHeader('Cache-Control', 'public, max-age=86400');
      res.setHeader('ETag', etag);
      return res.sendFile(finalPath);
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });
}
