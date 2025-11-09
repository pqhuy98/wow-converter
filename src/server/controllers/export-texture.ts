import { createHash } from 'crypto';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';
import { z } from 'zod';

import {
  type IconConversionOptions,
  IconConversionOptionsSchema,
  IconExtrasSchema,
  IconOptionsSchema,
} from '@/lib/converter/icon';
import { IconExporter } from '@/lib/converter/icon';

import { isSharedHosting, outputDir } from '../config';

// Re-export types for API consistency
export type { IconConversionOptions } from '@/lib/converter/icon';

// Zod schema for icon query parameters (from URL query string)
// extras comes as string from query params, needs transformation
export const IconQuerySchema = IconConversionOptionsSchema.extend({
  mode: z.literal('icon'),
  extras: z.string().optional().transform((val) => {
    if (!val) return undefined;
    try {
      return JSON.parse(val) as z.infer<typeof IconExtrasSchema>;
    } catch {
      return undefined;
    }
  }),
});

// Global exporter instance
const iconExporter = new IconExporter();

function getErrorStatus(error: Error): number {
  const message = error.message;
  if (message.includes('not found')) return 404;
  if (message.includes('Access denied')) return 403;
  return 500;
}

function isCacheValid(req: express.Request, res: express.Response, etag: string): boolean {
  if (req.headers['if-none-match'] === etag) {
    return true;
  }
  res.setHeader('Content-Type', 'image/png');
  res.setHeader('Cache-Control', 'public, max-age=86400');
  res.setHeader('ETag', etag);
  return false;
}

function queryToIconOptions(query: z.infer<typeof IconQuerySchema>): IconConversionOptions {
  const { mode: _mode, ...options } = query;
  return options;
}

export function ControllerExportTexture(router: express.Router) {
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

      // Get or export PNG path
      let finalPath: string;
      try {
        finalPath = await iconExporter.getOrExportPngPath(normalizedPath);
      } catch (error) {
        const err = error instanceof Error ? error : new Error(String(error));
        return res.status(getErrorStatus(err)).json({ error: err.message });
      }

      // Check if icon mode is requested
      const mode = req.query.mode as string | undefined;
      if (mode === 'icon') {
        const iconQueryResult = IconQuerySchema.safeParse(req.query);
        if (!iconQueryResult.success) {
          return res.status(400).json({
            error: 'Invalid icon query parameters',
            details: iconQueryResult.error.issues,
          });
        }

        const iconBuffer = await iconExporter.convertPngToIconBuffer(
          finalPath,
          queryToIconOptions(iconQueryResult.data),
        );
        const stats = await fsExtra.stat(finalPath);
        const etag = createHash('md5')
          .update(`${finalPath}-${JSON.stringify(iconQueryResult.data)}-${stats.mtime.getTime()}`)
          .digest('hex');

        if (isCacheValid(req, res, etag)) {
          return res.status(304).end();
        }

        return res.send(iconBuffer);
      }

      // Normal PNG serving
      const stats = await fsExtra.stat(finalPath);
      const etag = createHash('md5')
        .update(`${finalPath}-${stats.mtime.getTime()}-${stats.size}`)
        .digest('hex');

      if (isCacheValid(req, res, etag)) {
        return res.status(304).end();
      }

      return res.sendFile(finalPath);
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });

  // Batch convert textures to BLP files
  router.post('/texture/blp', async (req, res) => {
    try {
      const requestSchema = z.object({
        items: z.array(z.object({
          texturePath: z.string(),
          options: IconOptionsSchema.optional(),
          outputPath: z.string().optional(),
        })),
      });

      const parsedRequest = requestSchema.parse(req.body);

      if (parsedRequest.items.length === 0) {
        return res.status(400).json({ error: 'No items provided' });
      }

      const result = await iconExporter.exportToBlp(parsedRequest.items, outputDir);

      return res.json({
        count: result.count,
        paths: result.paths,
        outputDirectory: !isSharedHosting ? path.resolve(outputDir) : undefined,
      });
    } catch (error) {
      console.error('Error exporting textures:', error);
      return res.status(400).json({
        error: error instanceof Error ? error.message : String(error),
      });
    }
  });
}
