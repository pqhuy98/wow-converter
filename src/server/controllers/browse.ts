import express from 'express';

import { getModelSkinOptions } from '@/lib/converter/character/utils';
import { getListFiles, registerListfileClearHook } from '@/lib/wow/listfile-cache';
import { isDirectListfileClient, tryInProcessListfileClient } from '@/lib/wow-data-client/direct-listfile';
import { FileEntry, wowDataClient } from '@/lib/wow-data-client/wow-data-client';
import {
  applyCascBuildCache, etagFromParts, matchNotModified, writeNotModified,
} from '@/server/utils/casc-cache';

let allFiles: FileEntry[] | null = null;
let modelFiles: FileEntry[] | null = null;
let textureFiles: FileEntry[] | null = null;

const m2WmoRegex = /\.(m2|wmo)$/i;
const badWmoRegex = /_([0-9]{3}|lod\d)\.wmo$/;
const textureRegex = /\.(blp|png|tga|dds)$/i;

export function ControllerBrowse(router: express.Router) {
  async function fetchAllFilesFromFullListfile() {
    allFiles = await getListFiles();
    allFiles = allFiles.filter((f) => !badWmoRegex.test(f.fileName));

    modelFiles = allFiles.filter((f) => m2WmoRegex.test(f.fileName))
      .sort((a, b) => a.fileName.localeCompare(b.fileName));
    console.log('Total M2/WMO files:', modelFiles.length);

    textureFiles = allFiles.filter((f) => textureRegex.test(f.fileName))
      .sort((a, b) => a.fileName.localeCompare(b.fileName));

    console.log('Total texture files:', textureFiles.length);
  }

  async function fetchBrowseIndex() {
    const start = Date.now();
    const inProcess = tryInProcessListfileClient();
    if (inProcess) {
      console.log('Building browse index from listfile...');
      const { models, textures } = await inProcess.collectBrowseFileIndex();
      modelFiles = models;
      textureFiles = textures;
      allFiles = null;
      console.log(`Collected ${models.length} models and ${textures.length} textures in ${((Date.now() - start) / 1000).toFixed(1)}s`);
      console.log('Total M2/WMO files:', modelFiles.length);
      console.log('Total texture files:', textureFiles.length);
      return;
    }
    if (isDirectListfileClient(wowDataClient)) {
      console.log('Building browse index from listfile...');
      try {
        const { models, textures } = await wowDataClient.collectBrowseFileIndex();
        modelFiles = models;
        textureFiles = textures;
        allFiles = null;
        console.log(`Collected ${models.length} models and ${textures.length} textures in ${((Date.now() - start) / 1000).toFixed(1)}s`);
        console.log('Total M2/WMO files:', modelFiles.length);
        console.log('Total texture files:', textureFiles.length);
        return;
      } catch (e) {
        console.warn('Direct browse index unavailable, falling back to full listfile copy:', (e as Error).message);
      }
    }
    console.log('Building browse index (full listfile copy)...');
    await fetchAllFilesFromFullListfile();
  }

  async function preloadBrowseIndex() {
    for (;;) {
      try {
        await wowDataClient.waitUntilReady();
        await fetchBrowseIndex();
        if ((modelFiles?.length ?? 0) > 0 || (textureFiles?.length ?? 0) > 0) return;
      } catch (e) {
        console.warn('browse index preload waiting for CASC:', (e as Error).message);
      }
      await new Promise<void>((resolve) => { setTimeout(resolve, 2000); });
    }
  }

  registerListfileClearHook(() => {
    allFiles = null;
    modelFiles = null;
    textureFiles = null;
    void preloadBrowseIndex();
  });

  void preloadBrowseIndex();

  // Search files in the WoW listfile; default to all m2 if no search
  router.get('/browse', async (req, res) => {
    try {
      if (!modelFiles && !textureFiles) {
        await fetchBrowseIndex();
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

      const buildKey = wowDataClient.cascInfo?.buildKey ?? '';
      const etag = etagFromParts('browse', buildKey, q, String(result.length));
      if (matchNotModified(req, etag)) {
        applyCascBuildCache(res, req, buildKey, etag);
        return writeNotModified(res, etag);
      }
      applyCascBuildCache(res, req, buildKey, etag);
      return res.json(result);
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });

  router.get('/browse/model-skins', async (req, res) => {
    try {
      const fileDataID = Number(req.query.fileDataID);
      if (!Number.isFinite(fileDataID) || fileDataID <= 0) {
        return res.status(400).json({ error: 'fileDataID is required' });
      }
      if (!modelFiles && !allFiles) {
        await fetchBrowseIndex();
      }
      const fileName = (modelFiles ?? allFiles ?? []).find((f) => f.fileDataID === fileDataID)?.fileName;
      if (!fileName) {
        return res.status(404).json({ error: 'Model not found' });
      }
      await wowDataClient.waitUntilReady();
      await wowDataClient.initModelCaches();
      const skins = await wowDataClient.getModelSkins(fileDataID);
      const options = await getModelSkinOptions(fileDataID, fileName, skins);
      return res.header('Cache-Control', 'no-store').json({ fileDataID, fileName, skins: options });
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });
}
