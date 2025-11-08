import express from 'express';

import { FileEntry } from '@/lib/wowexport-client/wowexport-client';

import { getListFiles } from './shared';

let allFiles: FileEntry[] | null = null;
let modelFiles: FileEntry[] | null = null;
let textureFiles: FileEntry[] | null = null;

const m2WmoRegex = /\.(m2|wmo)$/i;
const badWmoRegex = /_([0-9]{3}|lod\d)\.wmo$/;
const textureRegex = /\.(blp|png|tga|dds)$/i;

export function ControllerBrowse(router: express.Router) {
  async function fetchAllFiles() {
    allFiles = await getListFiles();
    allFiles = allFiles.filter((f) => !badWmoRegex.test(f.fileName));

    modelFiles = allFiles.filter((f) => m2WmoRegex.test(f.fileName))
      .sort((a, b) => a.fileName.localeCompare(b.fileName));
    console.log('Total M2/WMO files:', modelFiles.length);

    textureFiles = allFiles.filter((f) => textureRegex.test(f.fileName))
      .sort((a, b) => a.fileName.localeCompare(b.fileName));

    console.log('Total texture files:', textureFiles.length);
  }
  void fetchAllFiles();

  // Search files in wow.export listfile; default to all m2 if no search
  router.get('/browse', async (req, res) => {
    try {
      if (!allFiles) {
        await fetchAllFiles();
      }
      const { q } = req.query as { q?: string };
      if (!q) {
        return res.status(400).json({ error: 'q is required' });
      }
      if (!['model', 'texture'].includes(q)) {
        return res.status(400).json({ error: 'q must be "model" or "texture"' });
      }

      let result: FileEntry[] = [];
      if (q === 'model') {
        result = modelFiles ?? [];
      } else if (q === 'texture') {
        result = textureFiles ?? [];
      }

      return res.header('Cache-Control', 'public, max-age=60').json(result);
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });
}
