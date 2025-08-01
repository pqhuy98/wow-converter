import archiver from 'archiver';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';
import z from 'zod';

import { LocalRefValueSchema } from '@/lib/converter/character-exporter';

import { ceOutputPath } from '../config';

const DownloadRequestSchema = z.object({
  files: z.array(LocalRefValueSchema).min(1),
});

export function ControllerDownload(app: express.Application) {
  app.post('/download', (req, res) => {
    try {
      const { files } = DownloadRequestSchema.parse(req.body);

      // Attach headers so browsers treat the response as a downloadable file
      res.setHeader('Content-Type', 'application/zip');
      res.setHeader('Content-Disposition', 'attachment; filename="assets.zip"');

      const archive = archiver('zip', { zlib: { level: 9 } });
      archive.pipe(res);

      for (const relativePath of files) {
        const diskPath = path.resolve(ceOutputPath, relativePath);
        // Prevent directory-traversal attacks
        if (!diskPath.startsWith(path.resolve(ceOutputPath))) {
          throw new Error('Invalid path');
        }

        if (!fsExtra.existsSync(diskPath)) {
          throw new Error('File not found');
        }

        // If this is a model file that carries a version suffix ( __<32hex> ), strip it from the name
        // inside the ZIP but keep the on-disk path intact.
        let archiveName = relativePath;
        const ext = path.extname(relativePath).toLowerCase();
        if ((ext === '.mdx' || ext === '.mdl')) {
          archiveName = archiveName.replace(/__([0-9a-fA-F]{32})(?=\.(?:mdx|mdl)$)/, '');
        }

        archive.file(diskPath, { name: archiveName });
      }

      archive.finalize().catch((err) => {
        console.error(err);
        if (!res.headersSent) {
          res.status(500).json({ error: 'Failed to create archive' });
        }
      });
    } catch (err) {
      res.status(400).json({ error: err instanceof Error ? err.message : String(err) });
    }
  });
}
