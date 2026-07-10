import crypto from 'crypto';
import express from 'express';
import fsExtra from 'fs-extra';
import path from 'path';

import { getCreaturesInTile } from '@/lib/azerothcore-client/creatures';
import { exportTexture } from '@/lib/converter/character/utils';
import { getListFiles, registerListfileClearHook } from '@/lib/wow/listfile-cache';
import { FileEntry, MapListItem, wowDataClient } from '@/lib/wow-data-client/wow-data-client';
import { assertDesktopOnly, desktopOnlyStatus } from '@/server/shared-hosting';
import {
  applyCascBuildCache, etagFromParts, matchNotModified, minimapPngPath, writeNotModified,
} from '@/server/utils/casc-cache';

import { registerMapGenerateRoutes } from './maps-generate';

type TileInfo = { x: number; y: number; hasTexture: boolean };
type MapWithTiles = MapListItem & { tiles: TileInfo[] };

const tileBlpRegex = /^world\/minimaps\/([^/]+)\/map(\d{1,2})_(\d{1,2})\.blp$/i;
const tileAdtRegex = /^world\/maps\/([^/]+)\/\1_(\d{2})_(\d{2})\.adt$/i;

let mapsWithTiles: MapWithTiles[] | null = null;
const mapsByDir = new Map<string, MapWithTiles>(); // dir(lowercased) -> map with tiles
const fileNameToEntry = new Map<string, FileEntry>(); // normalized lowercased path -> entry

registerListfileClearHook(() => {
  mapsWithTiles = null;
  mapsByDir.clear();
  fileNameToEntry.clear();
});

async function ensureMapsIndex(): Promise<void> {
  if (mapsWithTiles !== null) return;
  await buildMapsIndex();
}

async function buildMapsIndex(): Promise<void> {
  await wowDataClient.waitUntilReady();

  let baseMaps: MapListItem[] = [];
  try {
    baseMaps = await wowDataClient.getMapList();
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
  // console.log(`Total maps: ${mapsWithTiles.length}, total map tiles: ${_.sumBy(mapsWithTiles, 'tiles.length')}`);
}

export function ControllerMaps(router: express.Router) {
  void buildMapsIndex();

  registerMapGenerateRoutes(router, (key) => {
    const entry = mapsByDir.get(key);
    return entry ? { id: entry.id, dir: entry.dir } : undefined;
  });

  // GET /api/maps -> list maps
  router.get('/maps', async (req, res) => {
    try {
      await ensureMapsIndex();
      const list = mapsWithTiles ?? [];
      const buildKey = wowDataClient.cascInfo?.buildKey ?? '';
      const etag = etagFromParts('maps', buildKey, String(list.length));
      if (matchNotModified(req, etag)) {
        applyCascBuildCache(res, req, buildKey, etag);
        return writeNotModified(res, etag);
      }
      applyCascBuildCache(res, req, buildKey, etag);
      return res.json(list.map((m) => ({
        id: m.id, name: m.name, dir: m.dir, expansionID: m.expansionID,
      })));
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });

  // GET /api/maps/:map/wdt-mask -> tiles list with hasTexture flags
  router.get('/maps/:map/wdt-mask', async (req, res) => {
    try {
      await ensureMapsIndex();
      const key = String(req.params.map).toLowerCase();
      const entry = mapsByDir.get(key);
      const tiles = entry?.tiles ?? [];
      const buildKey = wowDataClient.cascInfo?.buildKey ?? '';
      const etag = etagFromParts('wdt-mask', buildKey, key, String(tiles.length));
      if (matchNotModified(req, etag)) {
        applyCascBuildCache(res, req, buildKey, etag);
        return writeNotModified(res, etag);
      }
      applyCascBuildCache(res, req, buildKey, etag);
      return res.json({ map: req.params.map, size: 64, tiles });
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });

  // POST /api/maps/:map/creatures-check -> count creatures in selected tiles only
  router.post('/maps/:map/creatures-check', async (req, res) => {
    try {
      assertDesktopOnly();
      const key = String(req.params.map).toLowerCase();
      const entry = mapsByDir.get(key);
      if (!entry) {
        return res.status(404).json({ error: 'Unknown map' });
      }

      const tiles = req.body?.tiles;
      if (!Array.isArray(tiles) || tiles.length === 0) {
        return res.status(400).json({ error: 'Invalid request body' });
      }

      const seen = new Set<string>();
      const checkedTiles: { x: number; y: number }[] = [];
      let creatureCount = 0;
      for (const tile of tiles) {
        const x = Number(tile?.x);
        const y = Number(tile?.y);
        if (!Number.isInteger(x) || !Number.isInteger(y) || x < 0 || x >= 64 || y < 0 || y >= 64) {
          return res.status(400).json({ error: 'Tile coordinates must be within 0..63' });
        }
        const tileKey = `${x},${y}`;
        if (seen.has(tileKey)) continue;
        seen.add(tileKey);
        checkedTiles.push({ x, y });
        const creatures = await getCreaturesInTile(entry.id, [x, y]);
        creatureCount += creatures.length;
      }

      if (checkedTiles.length === 0) {
        return res.status(400).json({ error: 'No valid tiles provided' });
      }

      return res.json({
        hasCreatures: creatureCount > 0,
        creatureCount,
        checkedTiles,
      });
    } catch (e) {
      const err = e as Error;
      const status = desktopOnlyStatus(err);
      if (status === 403) {
        return res.status(403).json({ error: err.message });
      }
      const message = err.message;
      if (message.includes('azerothcore database not found') || message.includes('ENOENT')) {
        return res.json({ hasCreatures: false, creatureCount: 0, checkedTiles: [] });
      }
      return res.status(500).json({ error: message });
    }
  });

  // GET /api/maps/:map/minimap/:x/:y -> PNG bytes
  // The minimap BLP is decoded to PNG via the pipeline-aware exportTexture helper.
  router.get('/maps/:map/minimap/:x/:y', async (req, res) => {
    try {
      const { map } = req.params;
      const x = parseInt(String(req.params.x), 10);
      const y = parseInt(String(req.params.y), 10);

      if (!(x >= 0 && x < 64 && y >= 0 && y < 64)) {
        return res.status(400).json({ error: 'x and y must be within 0..63' });
      }

      await wowDataClient.waitUntilReady();

      // Normalize directory and coordinates to match Blizzard naming
      const mapDir = String(map).toLowerCase();
      const xs = x.toString().padStart(2, '0');
      const ys = y.toString().padStart(2, '0');

      const buildKey = wowDataClient.cascInfo?.buildKey || '';
      const etagSeed = `${buildKey}|${map}|${x}|${y}`;
      const quotedETag = `"${crypto.createHash('md5').update(etagSeed).digest('hex')}"`;

      if (matchNotModified(req, quotedETag)) {
        applyCascBuildCache(res, req, buildKey, quotedETag, true);
        return writeNotModified(res, quotedETag);
      }

      const sendPng = async (pngPath: string) => {
        res.setHeader('Content-Type', 'image/png');
        applyCascBuildCache(res, req, buildKey, quotedETag, true);
        return res.send(await fsExtra.readFile(path.resolve(pngPath)));
      };

      // If PNG already exists in the export asset directory, serve it directly.
      const assetDir = await wowDataClient.getAssetDir();
      const preexistingPng = minimapPngPath(assetDir, buildKey, mapDir, xs, ys);
      if (fsExtra.existsSync(preexistingPng)) {
        return sendPng(preexistingPng);
      }

      // Resolve the BLP using the prebuilt hash table
      const blpPath = `world/minimaps/${mapDir}/map${xs}_${ys}.blp`;
      const file = fileNameToEntry.get(blpPath);
      if (!file?.fileDataID) {
        return res.status(404).json({ error: 'Minimap tile not found' });
      }
      const relPng = await exportTexture(file.fileDataID);
      const pngPath = path.join(assetDir, relPng);
      return sendPng(pngPath);
    } catch (e) {
      return res.status(500).json({ error: e instanceof Error ? e.message : String(e) });
    }
  });
}
