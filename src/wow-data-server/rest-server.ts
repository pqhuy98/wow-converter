/**
 * Native WoW data server: hosts src/lib/wow in a standalone process and
 * serves raw CASC files plus metadata (listfile, skins, character meta) for
 * the converter-side direct pipeline, so wow-converter can hot-reload while
 * CASC/listfile/DB2 data stays warm. ADT terrain tiles are still exported
 * server-side (OBJ/MTL/CSV + textures) — there is no direct ADT pipeline.
 */
import fs, { promises as fsp } from 'fs';
import http from 'http';
import path from 'path';
import url from 'url';

import { CASC } from '@/lib/wow/archive/casc/casc-source';
import { CASCLocal } from '@/lib/wow/archive/casc/casc-source-local';
import { CASCRemote } from '@/lib/wow/archive/casc/casc-source-remote';
import * as listfile from '@/lib/wow/archive/casc/listfile';
import { readRawCachedFile, writeRawCachedFile } from '@/lib/wow/archive/client/raw-cache';
import { releaseAdtExportBatchMemory, releaseAdtExportTileMemory } from '@/lib/wow/export/adt/adt-export-memory';
import { ADTExporter } from '@/lib/wow/export/adt/adt-exporter';
import { buildADTExportOptions, collectGameObjects, getTileBounds } from '@/lib/wow/export/adt/map-export-utils';
import {
  createBatchExportProgress,
  finalizeExportProgress,
  getExportProgressSnapshot,
} from '@/lib/wow/export/export-progress';
import { getAllSkinsForModel } from '@/lib/wow/export/m2/model-export-service';
import { getExportPath } from '@/lib/wow/export/writers/export-helper';
import { safeRegexPattern } from '@/lib/wow/formats/regex-safe';
import { normalizeInstallDirectory } from '@/lib/wow/normalize-install-directory';
import { wowConfig, type WowReaderConfig } from '@/lib/wow/server/config';
import { collectMemoryDiagnostics, formatMemoryDiagnostics } from '@/lib/wow/server/memory-diagnostics';
import { runtimeState } from '@/lib/wow/server/runtime';
import { isSettableConfigKey } from '@/lib/wow/server/settable-config';
import { registerWowDataServerClearHook } from '@/lib/wow/wow-data-server-hooks';
import { prepareSocketPath } from '@/lib/wow-data-server/transport';

import {
  CharacterMetaParams, getCharacterMeta,
} from '../lib/wow/character/headless-character';
import { ensureModelCachesInitialized } from '../lib/wow/db/caches/init-cache';
import { DB2Row, WDCReader } from '../lib/wow/db/wdc-reader';
import { write } from '../lib/wow/log';
import { authorizeDataServerRequest } from './auth';
import { autoLoadCascFromEnv } from './auto-load-env';
import {
  awaitCascLoad, isCascLoaded, isCascLoading, loadCascBuildSingleFlight,
} from './casc-load';
import { softRestartRuntime } from './soft-restart';

type JSONValue = unknown;

export class WowDataServer {
  server: http.Server | null = null;

  port = Number(process.env.WOW_DATA_SERVER_PORT || 17753);

  private _exportId = 1;

  private _pendingCASC: CASC | null = null;

  // Response cache for export endpoints (10s TTL).
  private _responseCache = new Map<string, { ts: number; status: number; obj: JSONValue }>();

  private _responseCacheTTL = 10 * 1000;

  /** Cap burst unique export bodies cached within the TTL window. */
  private _responseCacheMaxEntries = 128;

  private _responseCacheTimer: ReturnType<typeof setInterval> | null = null;

  get isRunning(): boolean {
    return this.server !== null;
  }

  // ---------------- routing ----------------

  async handleGet(req: http.IncomingMessage, pathname: string, query: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    switch (pathname) {
      case '/rest/getCascInfo':
        return this.getCascInfo(res);
      case '/rest/getConfig':
        return this.getConfig(query, res);
      case '/rest/searchFiles':
        return this.searchFiles(query, res);
      case '/rest/collectBrowseFileIndex':
        return this.collectBrowseFileIndex(res);
      case '/rest/getFileById':
        return this.getFileById(query, res);
      case '/rest/getFileByName':
        return this.getFileByName(query, res);
      case '/rest/getModelSkins':
        return this.getModelSkins(query, res);
      case '/rest/initModelCaches':
        return this.initModelCaches(res);
      case '/rest/cascFile':
        return this.cascFile(query, res);
      case '/rest/download':
        return this.download(query, res);
      case '/rest/debugMemory':
        if (!authorizeDataServerRequest(req)) {
          return this.sendJSON(res, 403, { id: 'ERR_FORBIDDEN', message: 'missing or invalid data server token' });
        }
        return this.debugMemory(res);
      case '/rest/getMapList':
        return this.getMapList(res);
      case '/rest/exportProgress':
        return this.exportProgress(query, res);
      default:
        return this.sendJSON(res, 404, { id: 'ERR_NOT_FOUND' });
    }
  }

  async handlePost(req: http.IncomingMessage, pathname: string, body: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    if (!authorizeDataServerRequest(req)) {
      return this.sendJSON(res, 403, { id: 'ERR_FORBIDDEN', message: 'missing or invalid data server token' });
    }
    switch (pathname) {
      case '/rest/loadCascLocal':
        return this.loadCascLocal(body, res);
      case '/rest/loadCascRemote':
        return this.loadCascRemote(body, res);
      case '/rest/loadCascBuild':
        return this.loadCascBuild(body, res);
      case '/rest/unloadCasc':
        return this.handleUnloadCasc(res);
      case '/rest/softRestart':
        return this.handleSoftRestart(body, res);
      case '/rest/setConfig':
        return this.setConfig(body, res);
      case '/rest/charMeta':
        return this.charMeta(body, res);
      case '/rest/exportADT':
        return this.exportADT(body, res);
      case '/rest/finalizeExportProgress':
        return this.finalizeExportProgress(body, res);
      default:
        return this.sendJSON(res, 404, { id: 'ERR_NOT_FOUND' });
    }
  }

  // ---------------- handlers ----------------

  getCascInfo(res: http.ServerResponse): void {
    const casc = runtimeState.casc;
    if (!casc || !casc.isLoaded) {
      this.sendJSON(res, 503, { id: 'CASC_UNAVAILABLE' });
      return;
    }

    this.sendJSON(res, 200, {
      id: 'CASC_INFO',
      type: casc.constructor.name,
      build: casc.build,
      buildConfig: casc.buildConfig,
      buildName: casc.getBuildName(),
      buildKey: casc.getBuildKey(),
    });
  }

  /**
   * Serve a raw (BLTE-decoded) CASC file by fileDataID.
   * Always cached on disk under .cache/wow/data/<buildKey>/<fileDataID> so
   * subsequent requests (and a same-host converter) read straight from disk.
   */
  async cascFile(query: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    const casc = runtimeState.casc;
    if (!casc || !casc.isLoaded) {
      this.sendJSON(res, 409, { id: 'ERR_NO_CASC' });
      return;
    }

    const fileDataID = Number(query.fileDataID);
    if (!Number.isFinite(fileDataID) || fileDataID <= 0) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', required: { fileDataID: 'number' } });
      return;
    }

    try {
      const buildKey = casc.getBuildKey();
      let buf = await readRawCachedFile(buildKey, fileDataID);
      if (!buf) {
        const data = await casc.getFile(fileDataID);
        data.processAllBlocks();
        buf = data.raw;
        await writeRawCachedFile(buildKey, fileDataID, buf);
      }
      res.statusCode = 200;
      res.setHeader('Content-Type', 'application/octet-stream');
      res.setHeader('Content-Length', buf.length);
      res.end(buf);
    } catch (e) {
      const message = (e as Error).message;
      if (message.includes('does not exist in root')) {
        this.sendJSON(res, 404, { id: 'ERR_NOT_FOUND', fileDataID });
        return;
      }
      if (message.includes('No root entry found for locale')) {
        this.sendJSON(res, 404, { id: 'ERR_NOT_FOUND', fileDataID, message });
        return;
      }
      write('cascFile error for %d: %s', fileDataID, message);
      this.sendJSON(res, 500, { id: 'ERR_INTERNAL', message });
    }
  }

  /**
   * Securely download a file under the configured export directory.
   */
  async download(query: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    const exportDir = wowConfig.exportDirectory;
    if (typeof exportDir !== 'string' || exportDir.length === 0) {
      this.sendJSON(res, 503, { id: 'ERR_EXPORT_DIR_UNAVAILABLE' });
      return;
    }

    const requested = String(query.path || '');
    if (!requested || requested.includes('\0')) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', required: { path: 'string (relative)' } });
      return;
    }

    const base = path.resolve(exportDir);
    const abs = path.resolve(base, requested);
    if (!abs.startsWith(base + path.sep) && abs !== base) {
      this.sendJSON(res, 403, { id: 'ERR_FORBIDDEN' });
      return;
    }

    const ext = path.extname(abs).toLowerCase();
    const allowedExts = ['.png', '.json', '.obj', '.mtl', '.csv'];
    if (!allowedExts.includes(ext)) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_FILE_TYPE', ext, allowedExts });
      return;
    }

    let resolved: string;
    try {
      resolved = await fsp.realpath(abs);
    } catch {
      this.sendJSON(res, 404, { id: 'ERR_NOT_FOUND' });
      return;
    }
    if (!resolved.startsWith(base + path.sep) && resolved !== base) {
      this.sendJSON(res, 403, { id: 'ERR_FORBIDDEN' });
      return;
    }

    const stat = await fsp.lstat(resolved);
    if (!stat.isFile() || stat.isSymbolicLink()) {
      this.sendJSON(res, 404, { id: 'ERR_NOT_FOUND' });
      return;
    }

    const contentType = WowDataServer.contentTypeForExt(ext);

    res.statusCode = 200;
    res.setHeader('Content-Type', contentType);
    const stream = fs.createReadStream(resolved);
    stream.on('error', () => this.sendJSON(res, 500, { id: 'ERR_INTERNAL', message: 'Failed to read file' }));
    stream.pipe(res);
  }

  debugMemory(res: http.ServerResponse): void {
    const diag = collectMemoryDiagnostics();
    this.sendJSON(res, 200, {
      id: 'DEBUG_MEMORY',
      summary: formatMemoryDiagnostics(diag),
      responseCacheEntries: this._responseCache.size,
      ...diag,
    });
  }

  private static contentTypeForExt(ext: string): string {
    if (ext === '.png') return 'image/png';
    if (ext === '.json') return 'application/json; charset=utf-8';
    if (ext === '.obj' || ext === '.mtl' || ext === '.csv') return 'text/plain; charset=utf-8';
    return 'application/octet-stream';
  }

  getConfig(query: Record<string, unknown>, res: http.ServerResponse): void {
    if (typeof query.key === 'string') {
      this.sendJSON(res, 200, { id: 'CONFIG_SINGLE', key: query.key, value: (wowConfig as unknown as Record<string, unknown>)[query.key] });
      return;
    }
    this.sendJSON(res, 200, { id: 'CONFIG_FULL', config: wowConfig });
  }

  async getMapList(res: http.ServerResponse): Promise<void> {
    const casc = runtimeState.casc;
    if (!casc || !casc.isLoaded) {
      this.sendJSON(res, 409, { id: 'ERR_NO_CASC' });
      return;
    }

    try {
      const table = new WDCReader('DBFilesClient/Map.db2');
      await table.parse();
      const maps: { id: number; name: unknown; dir: unknown; expansionID: unknown }[] = [];
      for (const [id, entry] of table.getAllRows()) {
        const dir = entry.Directory as string;
        const wdtPath = path.posix.join('world/maps', dir, `${dir}.wdt`);
        if (listfile.getByFilename(wdtPath)) {
          maps.push({
            id, name: entry.MapName_lang, dir, expansionID: entry.ExpansionID,
          });
        }
      }
      this.sendJSON(res, 200, { id: 'MAP_LIST', maps });
    } catch (e) {
      this.sendJSON(res, 500, { id: 'ERR_INTERNAL', message: (e as Error).message });
    }
  }

  setConfig(body: Record<string, unknown>, res: http.ServerResponse): void {
    if (!body || typeof body.key !== 'string') {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', required: { key: 'string', value: 'any' } });
      return;
    }
    if (!isSettableConfigKey(body.key)) {
      this.sendJSON(res, 400, { id: 'ERR_FORBIDDEN_CONFIG_KEY', message: 'config key is not writable over HTTP' });
      return;
    }

    (wowConfig as WowReaderConfig)[body.key] = body.value as WowReaderConfig[typeof body.key];
    this.sendJSON(res, 200, { id: 'CONFIG_SET_DONE', key: body.key, value: wowConfig[body.key] });
  }

  searchFiles(query: Record<string, unknown>, res: http.ServerResponse): void {
    if (!listfile.isLoaded()) {
      this.sendJSON(res, 409, { id: 'ERR_LISTFILE_NOT_LOADED' });
      return;
    }

    const search = String(query.search || '');
    const useRegularExpression = String(query.useRegularExpression || '0') === '1';
    const safePattern = useRegularExpression ? safeRegexPattern(search) : search;
    const filter = useRegularExpression
      ? (safePattern ? new RegExp(safePattern, 'i') : /$^/)
      : search;
    this.sendJSON(res, 200, { id: 'LISTFILE_SEARCH_RESULT', entries: listfile.getFilteredEntries(filter) });
  }

  collectBrowseFileIndex(res: http.ServerResponse): void {
    if (!listfile.isLoaded()) {
      this.sendJSON(res, 409, { id: 'ERR_LISTFILE_NOT_LOADED' });
      return;
    }
    const { models, textures } = listfile.collectBrowseFileIndex();
    this.sendJSON(res, 200, { id: 'BROWSE_FILE_INDEX', models, textures });
  }

  getFileById(query: Record<string, unknown>, res: http.ServerResponse): void {
    if (!listfile.isLoaded()) {
      this.sendJSON(res, 409, { id: 'ERR_LISTFILE_NOT_LOADED' });
      return;
    }
    const fileDataID = Number(query.fileDataID);
    if (!Number.isFinite(fileDataID)) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', required: { fileDataID: 'number' } });
      return;
    }
    const fileName = listfile.getByID(fileDataID) ?? '';
    this.sendJSON(res, 200, { id: 'LISTFILE_RESULT', fileDataID, fileName });
  }

  getFileByName(query: Record<string, unknown>, res: http.ServerResponse): void {
    if (!listfile.isLoaded()) {
      this.sendJSON(res, 409, { id: 'ERR_LISTFILE_NOT_LOADED' });
      return;
    }
    const fileName = String(query.fileName || '');
    if (!fileName) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', required: { fileName: 'string' } });
      return;
    }
    const fileDataID = listfile.getByFilename(fileName) ?? 0;
    this.sendJSON(res, 200, { id: 'LISTFILE_RESULT', fileDataID, fileName });
  }

  async getModelSkins(query: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    const fileDataID = Number(query.fileDataID);
    if (!Number.isFinite(fileDataID)) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', required: { fileDataID: 'number' } });
      return;
    }

    try {
      await ensureModelCachesInitialized();
      const skins = getAllSkinsForModel(fileDataID);
      this.sendJSON(res, 200, { id: 'MODEL_SKINS', fileDataID, skins });
    } catch (e) {
      this.sendJSON(res, 500, { id: 'ERR_INTERNAL', message: (e as Error).message });
    }
  }

  async initModelCaches(res: http.ServerResponse): Promise<void> {
    try {
      await ensureModelCachesInitialized();
      this.sendJSON(res, 200, { id: 'MODEL_CACHES_READY' });
    } catch (e) {
      this.sendJSON(res, 500, { id: 'ERR_INTERNAL', message: (e as Error).message });
    }
  }

  async loadCascLocal(body: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    if (isCascLoaded()) {
      this.sendJSON(res, 409, { id: 'ERR_CASC_ACTIVE' });
      return;
    }
    if (isCascLoading()) {
      await awaitCascLoad();
      this.sendJSON(res, 409, { id: 'ERR_CASC_ACTIVE' });
      return;
    }
    if (!body || typeof body.installDirectory !== 'string') {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', required: { installDirectory: 'string' } });
      return;
    }

    const installDirectory = normalizeInstallDirectory(body.installDirectory);
    if (!installDirectory) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_INSTALL', message: 'installDirectory is required' });
      return;
    }

    if (this._pendingCASC instanceof CASCLocal
      && normalizeInstallDirectory(this._pendingCASC.dir) === installDirectory) {
      this.sendJSON(res, 200, { id: 'CASC_INSTALL_BUILDS', builds: this._pendingCASC.builds });
      return;
    }

    try {
      write('REST loadCascLocal requested: %s', installDirectory);
      const casc = new CASCLocal(installDirectory);
      await casc.init();
      this._pendingCASC = casc;
      this.sendJSON(res, 200, { id: 'CASC_INSTALL_BUILDS', builds: casc.builds });
    } catch (e) {
      write('loadCascLocal failed: %s', (e as Error).message);
      this.sendJSON(res, 400, { id: 'ERR_INVALID_INSTALL', message: (e as Error).message });
    }
  }

  async loadCascRemote(body: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    if (isCascLoaded()) {
      this.sendJSON(res, 409, { id: 'ERR_CASC_ACTIVE' });
      return;
    }
    if (isCascLoading()) {
      await awaitCascLoad();
      this.sendJSON(res, 409, { id: 'ERR_CASC_ACTIVE' });
      return;
    }
    if (!body || typeof body.regionTag !== 'string') {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', required: { regionTag: 'string' } });
      return;
    }

    try {
      write('REST loadCascRemote requested: %s', body.regionTag);
      const casc = new CASCRemote(body.regionTag);
      await casc.init();
      this._pendingCASC = casc;
      this.sendJSON(res, 200, { id: 'CASC_INSTALL_BUILDS', builds: casc.builds });
    } catch (e) {
      write('loadCascRemote failed: %s', (e as Error).message);
      this.sendJSON(res, 400, { id: 'ERR_INVALID_INSTALL' });
    }
  }

  async loadCascBuild(body: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    if (isCascLoaded()) {
      this.getCascInfo(res);
      return;
    }
    if (isCascLoading()) {
      try {
        await awaitCascLoad();
        this._pendingCASC = null;
        this.getCascInfo(res);
      } catch (e) {
        write('Failed while joining in-flight CASC load: %s', (e as Error).message);
        this.sendJSON(res, 500, { id: 'ERR_CASC_FAILED' });
      }
      return;
    }

    const casc = this._pendingCASC;
    if (!casc) {
      this.sendJSON(res, 409, { id: 'ERR_NO_CASC_SETUP' });
      return;
    }
    if (!body || typeof body.buildIndex !== 'number') {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', required: { buildIndex: 'number' } });
      return;
    }
    if (body.buildIndex < 0 || body.buildIndex >= casc.builds.length) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_CASC_BUILD' });
      return;
    }

    try {
      await loadCascBuildSingleFlight(casc, body.buildIndex);
      this._pendingCASC = null;
      this.getCascInfo(res);
    } catch (e) {
      write('Failed to load CASC (native server): %s', (e as Error).stack);
      this.sendJSON(res, 500, { id: 'ERR_CASC_FAILED' });
    }
  }

  async handleUnloadCasc(res: http.ServerResponse): Promise<void> {
    try {
      await softRestartRuntime();
      this._pendingCASC = null;
      this.sendJSON(res, 200, { id: 'CASC_UNLOADED' });
    } catch (e) {
      this.sendJSON(res, 409, { id: 'ERR_CASC_LOADING', message: (e as Error).message });
    }
  }

  async handleSoftRestart(body: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    try {
      await softRestartRuntime();
      this._pendingCASC = null;

      const reloadEnv = body?.reloadEnv === true;
      if (reloadEnv) {
        const result = await autoLoadCascFromEnv();
        this.sendJSON(res, 200, {
          id: 'SOFT_RESTART_DONE',
          cascLoaded: result.loaded,
          buildName: result.buildName,
          error: result.error,
        });
        return;
      }

      this.sendJSON(res, 200, { id: 'SOFT_RESTART_DONE', cascLoaded: false });
    } catch (e) {
      this.sendJSON(res, 409, { id: 'ERR_CASC_LOADING', message: (e as Error).message });
    }
  }

  /** Character metadata for the converter-side direct pipeline (no export). */
  async charMeta(body: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    const casc = runtimeState.casc;
    const buildKey = casc?.getBuildKey() ?? '';
    const cacheKey = this._makeCacheKey(`/rest/charMeta|${buildKey}`, body);
    const cached = this._getCachedResponse(cacheKey);
    if (cached) { this.sendJSON(res, cached.status, cached.obj); return; }
    if (!casc) {
      this._sendAndCache(res, cacheKey, 409, { id: 'ERR_NO_CASC' });
      return;
    }

    if (body?.race === undefined || body?.gender === undefined || body?.customizations === undefined) {
      this._sendAndCache(res, cacheKey, 400, {
        id: 'ERR_INVALID_PARAMETERS',
        required: { race: 'number', gender: 'number', customizations: 'object' },
      });
      return;
    }

    try {
      const meta = await getCharacterMeta(body as unknown as CharacterMetaParams);
      this._sendAndCache(res, cacheKey, 200, { id: 'CHAR_META', ...meta });
    } catch (e) {
      this._sendAndCache(res, cacheKey, 500, { id: 'ERR_INTERNAL', message: (e as Error).message });
    }
  }

  async exportADT(body: Record<string, unknown>, res: http.ServerResponse): Promise<void> {
    const progressKey = typeof body?.progressKey === 'string' ? body.progressKey : undefined;
    const casc = runtimeState.casc;
    const buildKey = casc?.getBuildKey() ?? '';
    const cacheBody = progressKey ? { ...body, progressKey: undefined } : body;
    const cacheKey = this._makeCacheKey(`/rest/exportADT|${buildKey}`, cacheBody);
    const cached = progressKey ? undefined : this._getCachedResponse(cacheKey);
    if (cached) { this.sendJSON(res, cached.status, cached.obj); return; }
    if (!casc) {
      this._sendAndCache(res, cacheKey, 409, { id: 'ERR_NO_CASC' });
      return;
    }

    // Validate required parameters.
    const mapID = body?.mapID;
    const mapDir = body?.mapDir;
    const tileX = body?.tileX;
    const tileY = body?.tileY;

    if (typeof mapID !== 'number' || typeof mapDir !== 'string' || typeof tileX !== 'number' || typeof tileY !== 'number') {
      this._sendAndCache(res, cacheKey, 400, {
        id: 'ERR_INVALID_PARAMETERS',
        required: {
          mapID: 'number',
          mapDir: 'string',
          tileX: 'number (0-63)',
          tileY: 'number (0-63)',
          quality: 'number (optional, -1=alpha, 0=no tex, 1-512=minimap, 513+=baked, default 4096)',
          includeM2: 'boolean (optional, default true)',
          includeWMO: 'boolean (optional, default true)',
          includeWMOSets: 'boolean (optional, default true)',
          includeGameObjects: 'boolean (optional, default false)',
          includeLiquid: 'boolean (optional, default true)',
          includeFoliage: 'boolean (optional, default true)',
          includeHoles: 'boolean (optional, default true)',
          splitAlphaMaps: 'boolean (optional, default false)',
          splitLargeTerrainBakes: 'boolean (optional, default false)',
          gameObjects: 'array (optional, additional game objects to export)',
        },
      });
      return;
    }

    if (tileX < 0 || tileX > 63 || tileY < 0 || tileY > 63) {
      this._sendAndCache(res, cacheKey, 400, { id: 'ERR_INVALID_TILE_COORDS', message: 'Tile coordinates must be 0-63' });
      return;
    }

    const exportID = this.nextExportID();
    const tileIndex = tileX * 64 + tileY;

    // Build request-specific options without mutating global config.
    const requestOptions = buildADTExportOptions(wowConfig, {
      mapsIncludeM2: body.includeM2,
      mapsIncludeWMO: body.includeWMO,
      mapsIncludeWMOSets: body.includeWMOSets,
      mapsIncludeGameObjects: body.includeGameObjects,
      mapsIncludeLiquid: body.includeLiquid,
      mapsIncludeFoliage: body.includeFoliage,
      mapsIncludeHoles: body.includeHoles,
      splitAlphaMaps: body.splitAlphaMaps,
      splitLargeTerrainBakes: body.splitLargeTerrainBakes,
    });

    try {
      const quality = body.quality !== undefined ? Number(body.quality) : 4096;
      // Match UI export layout: <exportDir>/maps/<mapDir>
      const baseDir = getExportPath(path.join('maps', mapDir));

      const exporter = new ADTExporter(mapID, mapDir, tileIndex);

      // Optional game objects set.
      let gameObjects = Array.isArray(body.gameObjects) ? new Set(body.gameObjects as DB2Row[]) : undefined;
      if (!gameObjects && requestOptions.mapsIncludeGameObjects === true) {
        const {
          startX, startY, endX, endY,
        } = getTileBounds(tileX, tileY);
        gameObjects = await collectGameObjects(mapID, (obj) => {
          const [posX, posY] = obj.Pos as number[];
          return posX > startX && posX < endX && posY > startY && posY < endY;
        });
      }

      const progress = progressKey
        && typeof body.tileIndex === 'number'
        && typeof body.tileCount === 'number'
        && typeof body.stepsPerTile === 'number'
        ? createBatchExportProgress({
          key: progressKey,
          tileIndex: body.tileIndex,
          tileCount: body.tileCount,
          stepsPerTile: body.stepsPerTile,
          currentTile: { x: tileX, y: tileY },
        })
        : undefined;

      let result;
      try {
        result = await exporter.export(baseDir, quality, gameObjects, requestOptions, progress);
      } finally {
        progress?.syncTileComplete();
        releaseAdtExportTileMemory();
      }

      // Keep WDT cache across REST exports for perf; only cleared on build change.

      const responseObj = {
        id: 'EXPORT_RESULT',
        type: 'ADT',
        exportID,
        mapID,
        mapDir,
        tileX,
        tileY,
        tileIndex,
        exportPath: baseDir,
        exportType: 'ADT_OBJ',
        mainFile: result.path ? path.relative(wowConfig.exportDirectory, result.path) : null,
      };
      if (progressKey) {
        this.sendJSON(res, 200, responseObj);
      } else {
        this._sendAndCache(res, cacheKey, 200, responseObj);
      }
    } catch (e) {
      write('ADT export error: %s', (e as Error).message);
      const errObj = { id: 'ERR_INTERNAL', message: (e as Error).message, stack: (e as Error).stack };
      if (progressKey) {
        this.sendJSON(res, 500, errObj);
      } else {
        this._sendAndCache(res, cacheKey, 500, errObj);
      }
    }
  }

  exportProgress(query: Record<string, unknown>, res: http.ServerResponse): void {
    const key = typeof query.key === 'string' ? query.key : '';
    if (!key) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', message: 'key is required' });
      return;
    }
    const snapshot = getExportProgressSnapshot(key);
    if (!snapshot) {
      this.sendJSON(res, 404, { id: 'ERR_NOT_FOUND' });
      return;
    }
    this.sendJSON(res, 200, { id: 'EXPORT_PROGRESS', ...snapshot });
  }

  finalizeExportProgress(body: Record<string, unknown>, res: http.ServerResponse): void {
    const key = typeof body?.key === 'string' ? body.key : '';
    if (!key) {
      this.sendJSON(res, 400, { id: 'ERR_INVALID_PARAMETERS', message: 'key is required' });
      return;
    }
    finalizeExportProgress(key);
    releaseAdtExportBatchMemory();
    const snapshot = getExportProgressSnapshot(key);
    if (!snapshot) {
      this.sendJSON(res, 404, { id: 'ERR_NOT_FOUND' });
      return;
    }
    this.sendJSON(res, 200, { id: 'EXPORT_PROGRESS', ...snapshot });
  }

  // --------------- internals ---------------

  load(options?: { socketPath?: string; port?: number }): void {
    if (this.isRunning) throw new Error('WoW data server is already running');

    registerWowDataServerClearHook(() => this._responseCache.clear());

    this.server = http.createServer((req, res) => {
      void (async () => {
        try {
          const { pathname, query } = url.parse(req.url || '', true);
          if (req.method === 'GET') {
            await this.handleGet(req, pathname || '', query as Record<string, unknown>, res);
            return;
          }
          if (req.method === 'POST') {
            const body = await this.readJSONBody(req);
            await this.handlePost(req, pathname || '', body, res);
            return;
          }
          this.sendJSON(res, 404, { id: 'ERR_NOT_FOUND' });
        } catch (e) {
          try { write('REST error: %s', (e as Error).message); } catch { /* ignore */ }
          this.sendJSON(res, 500, { id: 'ERR_INTERNAL', message: (e as Error).message });
        }
      })();
    });

    const socketPath = options?.socketPath;
    const port = options?.port ?? this.port;

    if (socketPath) {
      prepareSocketPath(socketPath);
      this.server.listen(socketPath, () => {
        write('wow-data-server listening on unix socket %s', socketPath);
      });
    } else {
      const host = process.env.WOW_DATA_SERVER_HOST ?? '127.0.0.1';
      this.server.listen(port, host, () => {
        write('wow-data-server listening for REST requests on %s:%d', host, port);
      });
    }

    this._responseCacheTimer = setInterval(() => this._pruneResponseCache(), this._responseCacheTTL);
    this._responseCacheTimer.unref?.();
  }

  stop(): void {
    if (!this.isRunning) return;

    if (this._responseCacheTimer) {
      clearInterval(this._responseCacheTimer);
      this._responseCacheTimer = null;
    }
    this._responseCache.clear();

    // Drop keep-alive connections so the port is released promptly on shutdown.
    this.server!.closeAllConnections?.();
    this.server!.close(() => {
      write('wow-data-server stopped');
    });
    this.server = null;
  }

  // ---------------- cache helpers ----------------

  private _makeCacheKey(endpoint: string, body: unknown): string {
    const stableStringify = (value: unknown): string => {
      const seen = new WeakSet<object>();
      const stringify = (val: unknown): unknown => {
        if (val === null || typeof val !== 'object') return val;
        if (seen.has(val)) return undefined;
        seen.add(val);
        if (Array.isArray(val)) return val.map(stringify);
        const out: Record<string, unknown> = {};
        for (const key of Object.keys(val).sort()) out[key] = stringify((val as Record<string, unknown>)[key]);
        return out;
      };
      return JSON.stringify(stringify(value));
    };
    return `${endpoint}:${stableStringify(body || {})}`;
  }

  private _pruneResponseCache(now = Date.now()): void {
    for (const [key, entry] of this._responseCache) {
      if (now - entry.ts > this._responseCacheTTL) {
        this._responseCache.delete(key);
      }
    }
  }

  private _evictOldestResponseCacheEntries(count: number): void {
    if (count <= 0) return;
    const oldest = [...this._responseCache.entries()]
      .sort((a, b) => a[1].ts - b[1].ts)
      .slice(0, count);
    for (const [key] of oldest) {
      this._responseCache.delete(key);
    }
  }

  private _getCachedResponse(key: string): { ts: number; status: number; obj: JSONValue } | null {
    const now = Date.now();
    const entry = this._responseCache.get(key);
    if (!entry) return null;
    if (now - entry.ts > this._responseCacheTTL) {
      this._responseCache.delete(key);
      return null;
    }
    return entry;
  }

  private _sendAndCache(res: http.ServerResponse, key: string, status: number, obj: JSONValue): void {
    this._responseCache.set(key, { ts: Date.now(), status, obj });
    if (this._responseCache.size > this._responseCacheMaxEntries) {
      this._pruneResponseCache();
      if (this._responseCache.size > this._responseCacheMaxEntries) {
        this._evictOldestResponseCacheEntries(this._responseCache.size - this._responseCacheMaxEntries);
      }
    }
    this.sendJSON(res, status, obj);
  }

  sendJSON(res: http.ServerResponse, statusCode: number, obj: JSONValue): void {
    res.statusCode = statusCode;
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify(obj));
  }

  private async readJSONBody(req: http.IncomingMessage): Promise<Record<string, unknown>> {
    return new Promise((resolve, reject) => {
      let data = '';
      req.on('data', (chunk) => { data += chunk; });
      req.on('end', () => {
        if (!data) { resolve({}); return; }
        try { resolve(JSON.parse(data)); } catch { reject(new Error('ERR_INVALID_JSON')); }
      });
      req.on('error', reject);
    });
  }

  nextExportID(): number {
    const id = this._exportId++;
    if (this._exportId > 0x7FFFFFFF) this._exportId = 1;
    return id;
  }
}

export default WowDataServer;
