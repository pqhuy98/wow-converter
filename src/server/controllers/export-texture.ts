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
import { getWc3Path } from '@/lib/converter/icon/wc3.utils';

import { isSharedHosting, outputDir } from '../config';

// Re-export types for API consistency
export type { IconConversionOptions } from '@/lib/converter/icon';

/**
 * Validate and sanitize a path to prevent path traversal attacks
 * @param inputPath - The path to validate
 * @param allowedPrefix - Optional required prefix (e.g., 'interface/icons/')
 * @returns Normalized path if valid, throws error if invalid
 */
function validatePath(inputPath: string, allowedPrefix?: string): string {
  // Check for path traversal attempts before decoding/normalization
  if (inputPath.includes('..') || inputPath.includes('../') || inputPath.includes('..\\')) {
    throw new Error('Invalid path: path traversal detected');
  }

  // Decode URL encoding
  const decoded = decodeURIComponent(inputPath);

  // Check again after decoding (in case of encoded ..)
  if (decoded.includes('..') || decoded.includes('../') || decoded.includes('..\\')) {
    throw new Error('Invalid path: path traversal detected');
  }

  // Normalize the path (resolves . and .. sequences)
  // Note: path.normalize() may convert forward slashes to backslashes on Windows
  const normalized = path.normalize(decoded);

  // Check for absolute paths
  if (path.isAbsolute(normalized)) {
    throw new Error('Invalid path: absolute path not allowed');
  }

  // Normalize both paths to forward slashes for comparison (cross-platform)
  const normalizedForComparison = normalized.replace(/\\/g, '/');

  // Ensure path starts with allowed prefix if provided (case-insensitive)
  if (allowedPrefix) {
    const normalizedPrefix = allowedPrefix.replace(/\\/g, '/');
    if (!normalizedForComparison.toLowerCase().startsWith(normalizedPrefix.toLowerCase())) {
      throw new Error(`Path must start with ${allowedPrefix}`);
    }
  }

  // Return path with forward slashes (consistent format)
  return normalizedForComparison;
}

/**
 * Validate output path to prevent path traversal attacks
 * @param outputPath - The output path to validate
 * @returns Normalized path if valid, throws error if invalid
 */
function validateOutputPath(outputPath: string): string {
  // Check for path traversal attempts before normalization
  if (outputPath.includes('..') || outputPath.includes('../') || outputPath.includes('..\\')) {
    throw new Error('Invalid output path: path traversal detected');
  }

  // Normalize the path
  const normalized = path.normalize(outputPath);

  // Check for absolute paths
  if (path.isAbsolute(normalized)) {
    throw new Error('Invalid output path: absolute path not allowed');
  }

  // Ensure path doesn't start with / or \
  if (normalized.startsWith('/') || normalized.startsWith('\\')) {
    throw new Error('Invalid output path: absolute path not allowed');
  }

  return normalized;
}

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

      // Check if icon mode is requested
      const mode = req.query.mode as string | undefined;
      const isIconMode = mode === 'icon';

      // Normalize path (handle URL encoding)
      // Only enforce interface/icons/ prefix if icon options are provided
      let normalizedPath: string;
      try {
        normalizedPath = validatePath(match[1], isIconMode ? 'interface/icons/' : undefined);
      } catch (error) {
        return res.status(400).json({
          error: error instanceof Error ? error.message : 'Invalid path',
        });
      }

      // Get or export PNG path
      let finalPath: string;
      try {
        finalPath = await iconExporter.exportPngByPath(normalizedPath);
      } catch (error) {
        const err = error instanceof Error ? error : new Error(String(error));
        return res.status(getErrorStatus(err)).json({ error: err.message });
      }

      if (isIconMode) {
        // Only allow icon conversion for files in interface/icons directory
        if (!normalizedPath.toLowerCase().startsWith('interface/icons/')) {
          return res.status(400).json({
            error: 'Icon conversion is only available for files in interface/icons directory',
          });
        }

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

      // Only allow icon conversion for files in interface/icons directory
      // Validate all paths before processing
      try {
        for (const item of parsedRequest.items) {
          // Validate texture path - only enforce prefix if icon options are provided
          const validatedTexturePath = validatePath(
            item.texturePath,
            item.options ? 'interface/icons/' : undefined,
          );

          // Validate output path if provided
          let validatedOutputPath: string | undefined;
          if (item.outputPath) {
            validatedOutputPath = validateOutputPath(item.outputPath);
          }

          // Replace with validated paths
          item.texturePath = validatedTexturePath;
          if (validatedOutputPath) {
            item.outputPath = validatedOutputPath;
          }
        }
      } catch (error) {
        return res.status(400).json({
          error: error instanceof Error ? error.message : 'Invalid path',
        });
      }

      // Filter duplicates: only keep first occurrence of each Wc3 output path
      // Use outputPath if provided, otherwise generate from texturePath and frame

      const seenKeys = new Set<string>();
      const filteredItems = parsedRequest.items.filter((item) => {
        const frame = item.options?.frame ?? 'none';
        const wc3Path = item.outputPath ?? getWc3Path(item.texturePath, frame);
        if (seenKeys.has(wc3Path)) {
          return false;
        }
        seenKeys.add(wc3Path);
        return true;
      });

      const result = await iconExporter.exportToBlp(filteredItems, outputDir);

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
