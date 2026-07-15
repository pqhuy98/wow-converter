import axios, {
  AxiosInstance,
} from 'axios';
import chalk from 'chalk';
import { writeFile } from 'fs/promises';
import {
  ensureDir, ensureDirSync, exists,
} from 'fs-extra';
import { Agent as HttpAgent } from 'http';
import { Agent as HttpsAgent } from 'https';
import path from 'path';

import { waitUntil } from '@/lib/utils';
import { clearConverterRuntimeCaches } from '@/lib/wow/clear-runtime-caches';
import {
  getDataServerHttpOrigin, getDataServerSocketPath,
} from '@/lib/wow-data-server/transport';
import { DATA_SERVER_TOKEN_HEADER } from '@/wow-data-server/auth';

interface CASCInfo {
  type: string;
  build: {
    Product: string;
    Version: string;
  };
  buildConfig: unknown;
  buildName: string;
  buildKey: string;
}

interface CASCBuild {
  Product: string;
  Region: string;
  BuildConfig: string;
  CDNConfig: string;
  KeyRing: string;
  BuildId: string;
  VersionsName: string;
}

export interface FileEntry {
  fileDataID: number;
  fileName: string;
}

interface SearchResult {
  entries: FileEntry[];
}

export interface ExportFile {
  type: string;
  fileDataID: number;
  file: string;
}

export interface ConfigResponse {
  [key: string]: unknown;
}

export interface ModelSkin {
  id: string;
  label: string;
  displayID: number;
  textures: number[];
  extraGeosets?: number[];
}

export interface ExportCharacterParams {
  race: number;
  gender: number;
  fileDataIdOverride?: number;
  customizations: { [optionId: string]: number };
  geosetIds: number[];
  hideGeosetIds: number[];
  format: string;
  include_animations: boolean;
  include_base_clothing: boolean;
  excludeAnimationIds?: number[];
}

export interface CharacterMetaRequest {
  race: number;
  gender: number;
  fileDataIdOverride?: number;
  customizations: { [optionId: string]: number };
}

export interface CharMetaChoiceMaterial {
  custMaterial: { ChrModelTextureTargetID: number; FileDataID: number };
  textureLayer: {
    TextureType: number;
    Layer: number;
    Flags: number;
    BlendMode: number;
    TextureSectionTypeBitMask: number;
    ChrModelTextureTargetID: number[];
  };
  material: { TextureType: number; Width: number; Height: number; Flags?: number };
  section: {
    SectionType?: number; X: number; Y: number; Width: number; Height: number; OverlapSectionMask?: number;
  } | null;
  filename: string | null;
}

export interface CharacterMetaResponse {
  fileDataID: number;
  fileName: string;
  textureLayoutID: number;
  choices: Record<number, { geosets: number[]; materials: CharMetaChoiceMaterial[] }>;
}

export interface MapListItem {
  id: number;
  name: string;
  dir: string;
  expansionID: number;
}

export interface ExportADTParams {
  mapID: number;
  mapDir: string;
  tileX: number;
  tileY: number;
  quality?: number;
  includeM2?: boolean;
  includeWMO?: boolean;
  includeWMOSets?: boolean;
  includeGameObjects?: boolean;
  includeLiquid?: boolean;
  includeFoliage?: boolean;
  includeHoles?: boolean;
  splitAlphaMaps?: boolean;
  splitLargeTerrainBakes?: boolean;
  gameObjects?: unknown[];
  progressKey?: string;
  tileIndex?: number;
  tileCount?: number;
  stepsPerTile?: number;
}

export interface ExportProgressResponse {
  id: 'EXPORT_PROGRESS';
  completedSteps: number;
  totalSteps: number;
  tileIndex: number;
  tileCount: number;
  stepsPerTile: number;
  currentTile?: { x: number; y: number };
  taskName?: string;
  taskValue?: number;
  taskMax?: number;
}

export interface ExportADTResult {
  exportID: number;
  mapID: number;
  mapDir: string;
  tileX: number;
  tileY: number;
  tileIndex: number;
  exportPath: string;
  exportType: string;
  mainFile: string | null;
}

export class WowDataClient {
  private readonly http: AxiosInstance;

  private assetDir = '';

  private remoteAssetDir = '';

  private readonly isRemote: boolean;

  private readonly cacheDir = path.resolve('.cache');

  private readonly socketPath?: string;

  public status = {
    connected: false,
    configLoaded: false,
    cascLoaded: false,
  };

  cascInfo: CASCInfo | null = null;

  constructor(baseURL = getDataServerHttpOrigin(), socketPath = getDataServerSocketPath()) {
    this.socketPath = socketPath;
    this.isRemote = !socketPath && !/^(http(s)?:\/\/)?(127\.0\.0\.1|localhost)/.test(baseURL);
    if (this.isRemote) {
      ensureDirSync(this.cacheDir);
    }

    const httpAgent = new HttpAgent({ keepAlive: true, maxSockets: 64, keepAliveMsecs: 15000 });
    const httpsAgent = new HttpsAgent({ keepAlive: true, maxSockets: 64, keepAliveMsecs: 15000 });

    this.http = axios.create({
      baseURL,
      timeout: 1200000,
      httpAgent,
      httpsAgent,
    });
    const debug = false;
    this.http.interceptors.request.use((config) => {
      if (this.socketPath) {
        config.socketPath = this.socketPath;
      }
      const token = process.env.WOW_DATA_SERVER_TOKEN;
      if (token) {
        config.headers.set(DATA_SERVER_TOKEN_HEADER, token);
      }
      if (config.url?.includes('/getCascInfo') || config.url?.includes('/searchFiles')) return config;
      debug && console.log('request', config.method, config.url, config.data);
      return config;
    });
    this.http.interceptors.response.use((response) => {
      if (response.config.url?.includes('/getCascInfo') || response.config.url?.includes('/searchFiles')) return response;
      debug && console.log('response', response.status, response.data);
      return response;
    });
    this.startHeartbeat();
  }

  public get isReady() {
    return this.status.connected && this.status.configLoaded && this.status.cascLoaded;
  }

  public async waitUntilReady() {
    if (!this.isReady) await waitUntil(() => this.isReady);
    await this.refreshCascInfo();
  }

  /** Re-fetch CASC info so buildKey matches the data server after /setup changes. */
  private async refreshCascInfo(): Promise<void> {
    try {
      const info = await this.getCASCInfo();
      this.applyCascInfo(info);
    } catch {
      this.clearRuntimeCaches();
    }
  }

  /** Drop converter-side WoW caches and CASC readiness state. */
  public clearRuntimeCaches(): void {
    clearConverterRuntimeCaches();
    this.cascInfo = null;
    this.status.cascLoaded = false;
  }

  private applyCascInfo(info: CASCInfo): void {
    const prevKey = this.cascInfo?.buildKey;
    if (prevKey && prevKey !== info.buildKey) {
      clearConverterRuntimeCaches();
      console.log(chalk.yellow('CASC build changed:'), prevKey, '->', info.buildKey);
    }
    this.cascInfo = info;
    this.status.cascLoaded = true;
  }

  public isClassic() {
    return this.cascInfo?.build.Product.includes('classic');
  }

  async getAssetDir() {
    if (this.assetDir) return this.assetDir;
    const config = await this.getConfig();
    this.remoteAssetDir = config.exportDirectory as string;
    this.assetDir = this.isRemote
      ? this.cacheDir
      : this.remoteAssetDir;
    return this.assetDir;
  }

  public async syncConfig(): Promise<void> {
    const config = await this.getConfig();
    await Promise.all(Object.entries(desiredConfig).map(([key, value]) => {
      if (config[key] !== value) {
        return this.setConfig(key, value);
      }
      return Promise.resolve();
    }));
  }

  public async getConfig(key?: string): Promise<ConfigResponse> {
    const params = key ? { key } : undefined;
    const json = await this.getJSON('/rest/getConfig', params);
    if (json.id === 'CONFIG_SINGLE') return { [json.key]: json.value };
    if (json.id === 'CONFIG_FULL') return json.config;
    throw new Error('Unexpected response to getConfig');
  }

  public async getMapList(): Promise<MapListItem[]> {
    const json = await this.getJSON('/rest/getMapList');
    if (json.id === 'MAP_LIST' && Array.isArray(json.maps)) return json.maps as MapListItem[];
    if (json.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    throw new Error('Failed to get map list');
  }

  public async setConfig(key: string, value: unknown): Promise<ConfigResponse> {
    const json = await this.postJSON('/rest/setConfig', { key, value });
    if (json.id === 'CONFIG_SET_DONE') return { [json.key]: json.value };
    throw new Error('Failed to set configuration');
  }

  public async loadCASCLocal(installDirectory: string): Promise<CASCBuild[]> {
    const json = await this.postJSON('/rest/loadCascLocal', { installDirectory });
    if (json.id === 'CASC_INSTALL_BUILDS') return json.builds;
    if (json.id === 'ERR_INVALID_INSTALL') {
      throw new Error(typeof json.message === 'string' ? json.message : 'Invalid WoW installation directory');
    }
    if (json.id === 'ERR_CASC_ACTIVE') throw new Error('CASC is already active');
    throw new Error('Failed to load CASC (local)');
  }

  public async loadCASCRemote(regionTag: string): Promise<CASCBuild[]> {
    const json = await this.postJSON('/rest/loadCascRemote', { regionTag });
    if (json.id === 'CASC_INSTALL_BUILDS') return json.builds;
    if (json.id === 'ERR_INVALID_INSTALL') {
      throw new Error(typeof json.message === 'string' ? json.message : 'Invalid CDN region');
    }
    if (json.id === 'ERR_CASC_ACTIVE') throw new Error('CASC is already active');
    throw new Error('Failed to load CASC (remote)');
  }

  public async loadCASCBuild(buildIndex: number): Promise<CASCInfo> {
    const json = await this.postJSON('/rest/loadCascBuild', { buildIndex });
    if (json.id === 'CASC_INFO') return json as unknown as CASCInfo;
    if (json.id === 'ERR_NO_CASC_SETUP') throw new Error('No CASC setup available');
    if (json.id === 'ERR_INVALID_CASC_BUILD') throw new Error('Invalid build index');
    if (json.id === 'ERR_CASC_FAILED') throw new Error('Failed to load CASC build');
    throw new Error('Failed to load CASC build');
  }

  public async getCASCInfo(): Promise<CASCInfo> {
    const res = await this.safeGetJSON('/rest/getCascInfo');
    if (res.ok && res.json.id === 'CASC_INFO') return res.json as unknown as CASCInfo;
    if (res.json?.id === 'CASC_UNAVAILABLE') throw new Error('CASC not available');
    throw new Error('Failed to get CASC info');
  }

  private searchFileBlocked = false;

  public async searchFiles(search: string, useRegex: boolean = false): Promise<FileEntry[]> {
    if (this.searchFileBlocked) await waitUntil(() => !this.searchFileBlocked);
    this.searchFileBlocked = true;
    try {
      const json = await this.getJSON('/rest/searchFiles', { search, useRegularExpression: useRegex ? '1' : '0' });
      if (json.id === 'LISTFILE_SEARCH_RESULT') return (json as SearchResult).entries;
      if (json.id === 'ERR_LISTFILE_NOT_LOADED') throw new Error('Listfile not loaded');
      throw new Error('Failed to search files');
    } finally {
      this.searchFileBlocked = false;
    }
  }

  public async getFileByID(fileDataID: number): Promise<FileEntry> {
    const json = await this.getJSON('/rest/getFileById', { fileDataID: String(fileDataID) });
    if (json.id === 'LISTFILE_RESULT') return json as unknown as FileEntry;
    if (json.id === 'ERR_LISTFILE_NOT_LOADED') throw new Error('Listfile not loaded');
    throw new Error('Failed to get file by ID');
  }

  public async getFileByName(fileName: string): Promise<FileEntry> {
    const json = await this.getJSON('/rest/getFileByName', { fileName });
    if (json.id === 'LISTFILE_RESULT') return json as unknown as FileEntry;
    if (json.id === 'ERR_LISTFILE_NOT_LOADED') throw new Error('Listfile not loaded');
    throw new Error('Failed to get file by name');
  }

  public async getModelSkins(fileDataID: number): Promise<ModelSkin[]> {
    const json = await this.getJSON('/rest/getModelSkins', { fileDataID: String(fileDataID) });
    if (json.id === 'MODEL_SKINS') return json.skins as ModelSkin[];
    throw new Error('Failed to get model skins');
  }

  /** Pre-load creature/item DB2 skin caches (call before bulk creature export). */
  public async initModelCaches(): Promise<void> {
    const json = await this.getJSON('/rest/initModelCaches');
    if (json.id === 'MODEL_CACHES_READY') return;
    throw new Error('Failed to initialize model caches');
  }

  /** Filtered browse model/texture index (avoids full listfile copy). */
  public async collectBrowseFileIndex(): Promise<{ models: FileEntry[]; textures: FileEntry[] }> {
    const json = await this.getJSON('/rest/collectBrowseFileIndex');
    if (json.id === 'BROWSE_FILE_INDEX') {
      return { models: json.models as FileEntry[], textures: json.textures as FileEntry[] };
    }
    if (json.id === 'ERR_LISTFILE_NOT_LOADED') throw new Error('Listfile not loaded');
    throw new Error('Failed to collect browse file index');
  }

  /** Filtered minimap/ADT tile listfile entries (avoids full listfile copy). */
  public async collectMapTileFileIndex(): Promise<FileEntry[]> {
    const json = await this.getJSON('/rest/collectMapTileFileIndex');
    if (json.id === 'MAP_TILE_FILE_INDEX') return json.entries as FileEntry[];
    if (json.id === 'ERR_LISTFILE_NOT_LOADED') throw new Error('Listfile not loaded');
    throw new Error('Failed to collect map tile file index');
  }

  /** Creature display metadata from DB2 tables (model, textures, geosets). */
  public async resolveNpcDisplayMeta(displayId: number): Promise<{
    found: boolean;
    model?: number;
    textures?: Record<string, number>;
    geosets?: { geosetIndex: number; geosetValue: number }[];
  }> {
    const json = await this.getJSON('/rest/resolveNpcDisplay', { displayId: String(displayId) });
    if (json.id === 'NPC_DISPLAY_META') {
      return {
        found: json.found === true,
        model: json.model as number | undefined,
        textures: json.textures as Record<string, number> | undefined,
        geosets: json.geosets as { geosetIndex: number; geosetValue: number }[] | undefined,
      };
    }
    if (json.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    throw new Error('Failed to resolve NPC display metadata');
  }

  /** Download a raw (BLTE-decoded) CASC file by fileDataID. */
  public async downloadCascFile(fileDataID: number): Promise<Buffer> {
    const res = await this.http.request<ArrayBuffer | Buffer>({
      method: 'GET',
      url: '/rest/cascFile',
      params: { fileDataID: String(fileDataID) },
      responseType: 'arraybuffer',
      validateStatus: () => true,
    });
    if (res.status !== 200 || !(res.data instanceof ArrayBuffer || Buffer.isBuffer(res.data))) {
      let id = '';
      try { id = JSON.parse(Buffer.from(res.data as ArrayBuffer).toString('utf-8')).id; } catch { /* not json */ }
      if (res.status === 404 || id === 'ERR_NOT_FOUND') {
        throw new Error(`CASC file not found: ${fileDataID}`);
      }
      throw new Error(`Failed to download CASC file ${fileDataID} (${res.status}${id ? ` ${id}` : ''})`);
    }
    const buf = Buffer.isBuffer(res.data) ? res.data : Buffer.from(res.data);
    if (buf.length === 0) throw new Error(`CASC file is empty: ${fileDataID}`);
    return buf;
  }

  /** Character metadata (model fileDataID, geosets, bake layers) for in-process conversion. */
  public async getCharMeta(params: CharacterMetaRequest): Promise<CharacterMetaResponse> {
    const { status, data: json } = await this.postJSONAllowError('/rest/charMeta', params);
    if (status === 200 && json.id === 'CHAR_META') return json as unknown as CharacterMetaResponse;
    if (status === 409 || json?.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    if (status === 400) throw new Error('Invalid parameters for character metadata');
    if (status >= 500) throw new Error(`Server error during character metadata lookup: ${json?.message ?? 'unknown'}`);
    throw new Error('Unexpected response for character metadata');
  }

  public async exportADT(params: ExportADTParams): Promise<ExportADTResult> {
    const { status, data: json } = await this.postJSONAllowError('/rest/exportADT', params);
    if (status === 200 && json.id === 'EXPORT_RESULT') {
      await this.getAssetDir();
      const result: ExportADTResult = {
        exportID: json.exportID,
        mapID: json.mapID,
        mapDir: json.mapDir,
        tileX: json.tileX,
        tileY: json.tileY,
        tileIndex: json.tileIndex,
        exportPath: path.join(this.assetDir, path.relative(this.remoteAssetDir, json.exportPath)),
        exportType: json.exportType,
        mainFile: json.mainFile,
      };
      return result;
    }
    if (status === 409 || json?.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    if (status === 400) throw new Error(`Invalid parameters for ADT export: ${json?.message ?? 'unknown'}`);
    if (status >= 500) throw new Error(`Server error during ADT export: ${json?.message ?? 'unknown'}`);
    throw new Error('Unexpected response for ADT export');
  }

  public async getExportProgress(progressKey: string): Promise<ExportProgressResponse | undefined> {
    const { ok, json } = await this.safeGetJSON('/rest/exportProgress', { key: progressKey });
    if (ok && json?.id === 'EXPORT_PROGRESS') {
      return json as ExportProgressResponse;
    }
    return undefined;
  }

  public async finalizeExportProgress(progressKey: string): Promise<ExportProgressResponse | undefined> {
    const { status, data: json } = await this.postJSONAllowError('/rest/finalizeExportProgress', { key: progressKey });
    if (status === 200 && json?.id === 'EXPORT_PROGRESS') {
      return json as ExportProgressResponse;
    }
    return undefined;
  }

  public async resetConnection(): Promise<void> {
    await this.bootstrap();
  }

  private bootPromise: Promise<void> | null = null;

  private logWarnedBootstrap = false;

  private async bootstrap(): Promise<void> {
    if (this.bootPromise) return this.bootPromise;
    this.bootPromise = (async () => {
      try {
        await this.getConfig();
        this.status.connected = true;
        this.status.configLoaded = true;

        try {
          const info = await this.getCASCInfo();
          this.applyCascInfo(info);
          console.log(chalk.green('✅ WoW data ready:'), info.build.Product, info.buildName);
          this.logWarnedCASC = false;
        } catch {
          if (!this.logWarnedCASC) {
            const port = process.env.PORT ? parseInt(process.env.PORT, 10) : 3001;
            const setupUrl = `http://127.0.0.1:${port}/setup`;
            console.log(chalk.gray('WoW data not loaded yet, open'), chalk.blue(setupUrl));
            this.logWarnedCASC = true;
          }
        }
      } catch {
        if (!this.logWarnedBootstrap) {
          console.error(chalk.yellow(`⏳ Cannot connect to wow-data-server at ${this.socketPath}. Is it running?`));
          this.logWarnedBootstrap = true;
        }
        this.status.connected = false;
        this.status.configLoaded = false;
        this.status.cascLoaded = false;
      }
    })().finally(() => {
      this.bootPromise = null;
    });
    return this.bootPromise;
  }

  private async pollCascReady(): Promise<void> {
    if (this.status.cascLoaded) return;
    try {
      const info = await this.getCASCInfo();
      this.applyCascInfo(info);
      console.log(chalk.green('✅ WoW data ready:'), info.build.Product, info.buildName);
      this.logWarnedCASC = false;
    } catch {
      // Still waiting for CASC to be configured.
    }
  }

  private startHeartbeat(): void {
    const tick = () => {
      try {
        if (!this.status.connected) {
          if (!this.bootPromise) void this.bootstrap();
          return;
        }
        if (!this.status.cascLoaded) {
          void this.pollCascReady();
          return;
        }
        void this.safeGetJSON('/rest/getCascInfo').then((res) => {
          if (!(res.ok && res.json?.id === 'CASC_INFO')) {
            this.clearRuntimeCaches();
            return;
          }
          this.applyCascInfo(res.json as unknown as CASCInfo);
        });
      } catch {
        this.status.connected = false;
      }
    };
    setInterval(tick, 500);
    tick();
  }

  private logWarnedCASC = false;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private async getJSON(path: string, params?: Record<string, unknown>): Promise<any> {
    const res = await this.http.request({ method: 'GET', url: path, params });
    return res.data;
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private async postJSON(path: string, body?: unknown): Promise<any> {
    const res = await this.http.request({ method: 'POST', url: path, data: body ?? {} });
    return res.data;
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private async postJSONAllowError(path: string, body?: unknown): Promise<{ status: number; data: any }> {
    const res = await this.http.request({
      method: 'POST',
      url: path,
      data: body ?? {},
      validateStatus: () => true,
    });
    return { status: res.status, data: res.data };
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  private async safeGetJSON(path: string, params?: Record<string, unknown>): Promise<{ ok: boolean; json: any }> {
    try {
      const res = await this.http.request({
        method: 'GET',
        url: path,
        params,
        validateStatus: () => true,
      });
      return { ok: res.status >= 200 && res.status < 300, json: res.data };
    } catch {
      return { ok: false, json: null };
    }
  }

  private normalizeRelative(p: string) {
    return p.replace(/\\/g, '/').replace(/^\/+/, '');
  }

  private async prefetchFiles(files: ExportFile[], allowCache: (file: string) => boolean = () => true): Promise<void> {
    if (!this.isRemote || files.length === 0) return;
    await this.getAssetDir();
    await Promise.all(files.map(async (file) => this.fetchFile(
      path.relative(this.remoteAssetDir, file.file),
      allowCache,
    )));
  }

  private async fetchFile(relativePath: string, allowCache: (file: string) => boolean): Promise<string> {
    const rel = this.normalizeRelative(relativePath);

    if (!this.isRemote) return path.resolve(this.assetDir, rel);

    const dest = path.resolve(this.cacheDir, rel);
    if (allowCache(rel) && await exists(dest)) return dest;

    const dir = path.dirname(dest);
    await ensureDir(dir);

    console.log('Fetch file from remote wow-data-server', relativePath);
    const res = await this.http.request<ArrayBuffer | Buffer>({
      method: 'GET',
      url: '/rest/download',
      params: {
        path: rel,
      },
      responseType: 'arraybuffer',
      validateStatus: () => true,
    });
    if (res.status !== 200 || !(res.data instanceof ArrayBuffer || Buffer.isBuffer(res.data))) {
      throw new Error(`Failed to download remote file: ${rel} (${res.status})`);
    }
    const buf = Buffer.isBuffer(res.data) ? res.data : Buffer.from(res.data);

    await writeFile(dest, buf);
    return dest;
  }
}

let _wowDataClient: WowDataClient | undefined;

export function getWowDataClient(): WowDataClient {
  if (!_wowDataClient) {
    _wowDataClient = new WowDataClient();
  }
  return _wowDataClient;
}

/** Lazy singleton so bundled transport env is applied before the client is created. */
export const wowDataClient: WowDataClient = new Proxy({} as WowDataClient, {
  get(_target, prop) {
    const client = getWowDataClient();
    const value = Reflect.get(client, prop, client) as unknown;
    return typeof value === 'function' ? (value as (...args: unknown[]) => unknown).bind(client) : value;
  },
});

const desiredConfig = {
  copyMode: 'FULL',
  listfileShowFileDataIDs: true,
  enableM2Skins: true,
  enableSharedTextures: true,
  enableSharedChildren: true,
  enableAbsoluteMTLPaths: false,
  enableAbsoluteCSVPaths: false,
  removePathSpaces: true,
  removePathSpacesCopy: true,
  exportTextureFormat: 'PNG',
  exportModelFormat: 'OBJ',
  exportM2Bones: true,
  exportM2Meta: true,
  exportWMOMeta: true,
  modelsExportSkin: true,
  modelsExportSkel: true,
  modelsExportBone: true,
  modelsExportAnim: true,
  modelsExportUV2: true,
  modelsExportTextures: true,
  modelsExportAlpha: true,
  modelsExportAnimations: true,
  modelsExportCollision: true,
};
