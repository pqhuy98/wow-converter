import crypto from 'crypto';
import express from 'express';
import fsExtra from 'fs-extra';
import _ from 'lodash';
import path from 'path';

import { FileEntry, MapListItem, wowExportClient } from '@/lib/wowexport-client/wowexport-client';

import { isDev } from '../config';
import { getListFiles } from './shared';

type TileInfo = { x: number; y: number; hasTexture: boolean };
type MapWithTiles = MapListItem & { tiles: TileInfo[] };

const tileBlpRegex = /^world\/minimaps\/([^/]+)\/map(\d{1,2})_(\d{1,2})\.blp$/i;
const tileAdtRegex = /^world\/maps\/([^/]+)\/\1_(\d{2})_(\d{2})\.adt$/i;

let mapsWithTiles: MapWithTiles[] = [];
const mapsByDir = new Map<string, MapWithTiles>(); // dir(lowercased) -> map with tiles
const fileNameToEntry = new Map<string, FileEntry>(); // normalized lowercased path -> entry

async function buildMapsIndex(): Promise<void> {
  await wowExportClient.waitUntilReady();

  let baseMaps: MapListItem[] = [];
  try {
    baseMaps = await wowExportClient.getMapList();
  } catch {
    baseMaps = [];
  }
  const files = await getListFiles();

  const adtByDir = new Map<string, Set<string>>(); // key: "x,y"
  const texByDir = new Map<string, Set<string>>();

  for (const fileEntry of files) {
    const fileName = fileEntry.fileName.replace(/\\/g, '/').toLowerCase();

    // Track minimap textures
    const matchesBlp = tileBlpRegex.exec(fileName);
    if (matchesBlp) {
      const dir = matchesBlp[1].toLowerCase();
      const x = parseInt(matchesBlp[2], 10);
      const y = parseInt(matchesBlp[3], 10);
      if (x >= 0 && x < 64 && y >= 0 && y < 64) {
        let set = texByDir.get(dir);
        if (!set) { set = new Set<string>(); texByDir.set(dir, set); }
        set.add(`${x},${y}`);
        fileNameToEntry.set(fileName, fileEntry);
      }
      continue;
    }

    // Track ADT presence
    const matchesAdt = tileAdtRegex.exec(fileName);
    if (matchesAdt) {
      const dir = matchesAdt[1].toLowerCase();
      const x = parseInt(matchesAdt[2], 10);
      const y = parseInt(matchesAdt[3], 10);
      if (x >= 0 && x < 64 && y >= 0 && y < 64) {
        let set = adtByDir.get(dir);
        if (!set) { set = new Set<string>(); adtByDir.set(dir, set); }
        set.add(`${x},${y}`);
      }
    }
  }

  mapsWithTiles = [];
  mapsByDir.clear();

  for (const m of baseMaps) {
    const dir = m.dir.toLowerCase();
    const tilesMap = new Map<string, TileInfo>();

    const adtSet = adtByDir.get(dir);
    if (adtSet) {
      for (const key of adtSet) {
        const [xs, ys] = key.split(',');
        const x = parseInt(xs, 10);
        const y = parseInt(ys, 10);
        tilesMap.set(key, { x, y, hasTexture: false });
      }
    }

    const texSet = texByDir.get(dir);
    if (texSet) {
      for (const key of texSet) {
        const [xs, ys] = key.split(',');
        const x = parseInt(xs, 10);
        const y = parseInt(ys, 10);
        const prev = tilesMap.get(key);
        if (prev) {
          prev.hasTexture = true;
        } else {
          tilesMap.set(key, { x, y, hasTexture: true });
        }
      }
    }

    const tiles = Array.from(tilesMap.values());
    const withTiles: MapWithTiles = { ...m, tiles };
    mapsWithTiles.push(withTiles);
    mapsByDir.set(dir, withTiles);
  }
  console.log(`Total maps: ${mapsWithTiles.length}, total map tiles: ${_.sumBy(mapsWithTiles, 'tiles.length')}`);
}

export function ControllerMaps(router: express.Router) {
  void buildMapsIndex();

  // GET /api/maps -> list maps
  router.get('/maps', (_req, res) => {
    try {
      if (!isDev) res.setHeader('Cache-Control', 'public, max-age=3600');
      return res.json(mapsWithTiles.map((m) => ({
        id: m.id, name: m.name, dir: m.dir, expansionID: m.expansionID,
      })));
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });

  // GET /api/maps/:map/wdt-mask -> tiles list with hasTexture flags
  router.get('/maps/:map/wdt-mask', (req, res) => {
    try {
      const key = String(req.params.map).toLowerCase();
      const entry = mapsByDir.get(key);
      const tiles = entry?.tiles ?? [];
      if (!isDev) res.setHeader('Cache-Control', 'public, max-age=3600');
      return res.json({ map: req.params.map, size: 64, tiles });
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });

  // GET /api/maps/:map/minimap/:x/:y -> PNG bytes
  // We export the BLP as PNG using wow.export's exportTextures.
  router.get('/maps/:map/minimap/:x/:y', async (req, res) => {
    try {
      const { map } = req.params;
      const x = parseInt(String(req.params.x), 10);
      const y = parseInt(String(req.params.y), 10);

      if (!(x >= 0 && x < 64 && y >= 0 && y < 64)) {
        return res.status(400).json({ error: 'x and y must be within 0..63' });
      }

      await wowExportClient.waitUntilReady();

      const buildKey = wowExportClient.cascInfo?.buildKey || '';
      const etagSeed = `${buildKey}|${map}|${x}|${y}`;
      const etag = crypto.createHash('md5').update(etagSeed).digest('hex');

      if (!isDev && req.headers['if-none-match'] === etag) {
        return res.status(304).end();
      }

      // If PNG already exists in the wow.export asset directory, serve it directly.
      const assetDir = await wowExportClient.getAssetDir();
      const preexistingPng = path.join(assetDir, 'world', 'minimaps', map, `map${x}_${y}.png`);
      if (fsExtra.existsSync(preexistingPng)) {
        res.setHeader('Content-Type', 'image/png');
        if (!isDev) res.setHeader('Cache-Control', 'public, max-age=86400');
        res.setHeader('ETag', etag);
        return res.sendFile(preexistingPng);
      }

      // Resolve the BLP using the prebuilt hash table
      const file = fileNameToEntry.get(`world/minimaps/${map}/map${x}_${y}.blp`);
      if (!file?.fileDataID) {
        return res.status(404).json({ error: 'Minimap tile not found' });
      }
      const textures = await wowExportClient.exportTextures([file.fileDataID]);
      const pngPath = textures.find((t) => /\.png$/i.test(t.file))?.file;
      if (!pngPath) {
        return res.status(500).json({ error: 'Failed to export minimap texture' });
      }

      res.setHeader('Content-Type', 'image/png');
      res.setHeader('Cache-Control', 'public, max-age=86400');
      res.setHeader('ETag', etag);
      return res.sendFile(pngPath);
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });
}
