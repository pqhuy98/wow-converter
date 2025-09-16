import axios, {
  AxiosInstance,
} from 'axios';
import chalk from 'chalk';
import fs, { existsSync } from 'fs';
import { emptyDirSync, ensureDirSync } from 'fs-extra';
import { Agent as HttpAgent } from 'http';
import { Agent as HttpsAgent } from 'https';
import path from 'path';

import { waitUntil } from '../utils';

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

interface ExportResult {
  fileDataID: number;
  files: ExportFile[];
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
  customizations: { [optionId: string]: number };
  geosetIds: number[];
  hideGeosetIds: number[];
  format: string;
  include_animations: boolean;
  include_base_clothing: boolean;
  excludeAnimationIds?: number[];
}

export type ExportCharacterResult = {
  exportPath: string;
  fileName: string;
  fileManifest: ExportFile[];
}

export class WowExportRestClient {
  private readonly http: AxiosInstance;

  private assetDir = '';

  private remoteAssetDir = '';

  private readonly isRemote: boolean;

  private readonly cacheDir = path.resolve('.cache');

  public status = {
    connected: false,
    configLoaded: false,
    cascLoaded: false,
  };

  cascInfo: CASCInfo | null = null;

  constructor(private baseURL = 'http://127.0.0.1:17752') {
    this.isRemote = !/^(http(s)?:\/\/)?(127\.0\.0\.1|localhost)/.test(baseURL);
    if (this.isRemote) {
      ensureDirSync(this.cacheDir);
    }

    // Keep TCP connections alive to avoid handshake overhead and reduce resets.
    const httpAgent = new HttpAgent({ keepAlive: true, maxSockets: 64, keepAliveMsecs: 15000 });
    const httpsAgent = new HttpsAgent({ keepAlive: true, maxSockets: 64, keepAliveMsecs: 15000 });

    this.http = axios.create({
      baseURL,
      timeout: 300000,
      httpAgent,
      httpsAgent,
    });
    const debug = false;
    this.http.interceptors.request.use((config) => {
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
    if (this.isReady) return;
    await waitUntil(() => this.isReady);
  }

  public isClassic() {
    return this.cascInfo?.build.Product.includes('classic');
  }

  async getAssetDir() {
    if (this.assetDir) return this.assetDir;
    this.remoteAssetDir = (await this.getConfig()).exportDirectory as string;
    this.assetDir = this.isRemote
      ? this.cacheDir
      : this.remoteAssetDir;
    return this.assetDir;
  }

  // ===== HIGH-LEVEL API METHODS (public) =====

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

  public async setConfig(key: string, value: unknown): Promise<ConfigResponse> {
    const json = await this.postJSON('/rest/setConfig', { key, value });
    if (json.id === 'CONFIG_SET_DONE') return { [json.key]: json.value };
    throw new Error('Failed to set configuration');
  }

  public async loadCASCLocal(installDirectory: string): Promise<CASCBuild[]> {
    const json = await this.postJSON('/rest/loadCascLocal', { installDirectory });
    if (json.id === 'CASC_INSTALL_BUILDS') return json.builds;
    if (json.id === 'ERR_INVALID_INSTALL') throw new Error('Invalid WoW installation directory');
    if (json.id === 'ERR_CASC_ACTIVE') throw new Error('CASC is already active');
    throw new Error('Failed to load CASC (local)');
  }

  public async loadCASCRemote(regionTag: string): Promise<CASCBuild[]> {
    const json = await this.postJSON('/rest/loadCascRemote', { regionTag });
    if (json.id === 'CASC_INSTALL_BUILDS') return json.builds;
    if (json.id === 'ERR_INVALID_INSTALL') throw new Error('Invalid CDN region');
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

  public async exportModels(models: { fileDataID: number; skinName?: string }[]): Promise<ExportResult[]> {
    if (models.length === 0) return [];
    const { status, data: json } = await this.postJSONAllowError('/rest/exportModels', { models });
    if (status === 200 && json.id === 'EXPORT_RESULT') {
      const results = json.succeeded as ExportResult[];
      await Promise.all(results.map(async (result: ExportResult) => {
        await this.prefetchFiles(result.files, (file) => file.endsWith('.png'));
        result.files.forEach((_, i) => {
          result.files[i].file = path.join(this.assetDir, path.relative(this.remoteAssetDir, result.files[i].file));
        });
      }));
      return results;
    }
    if (status === 409 || json?.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    if (status === 400) throw new Error('Invalid parameters for model export');
    if (status === 422) throw new Error('Model export failed for all files');
    if (status >= 500) throw new Error(`Server error during model export: ${json?.message ?? 'unknown'}`);
    throw new Error('Unexpected response for model export');
  }

  public async exportTextures(fileDataIDs: number[]): Promise<ExportFile[]> {
    if (fileDataIDs.length === 0) return [];
    const { status, data: json } = await this.postJSONAllowError('/rest/exportTextures', { fileDataID: fileDataIDs });
    if (status === 200 && json.id === 'EXPORT_RESULT') {
      const results = json.succeeded as ExportFile[];
      await this.prefetchFiles(results);
      results.forEach((_, i) => {
        results[i].file = path.join(this.assetDir, path.relative(this.remoteAssetDir, results[i].file));
      });
      return results;
    }
    if (status === 409 || json?.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    if (status === 400) throw new Error('Invalid parameters for texture export');
    if (status === 422) throw new Error('Texture export failed for all files');
    if (status >= 500) throw new Error(`Server error during texture export: ${json?.message ?? 'unknown'}`);
    throw new Error('Unexpected response for texture export');
  }

  public async exportCharacter(data: ExportCharacterParams): Promise<ExportCharacterResult> {
    const { status, data: json } = await this.postJSONAllowError('/rest/exportCharacter', data);
    if (status === 200 && json.id === 'EXPORT_RESULT') {
      const result = json as ExportCharacterResult;
      await this.prefetchFiles(result.fileManifest, (file) => file.endsWith('.png'));
      result.exportPath = path.join(this.assetDir, path.relative(this.remoteAssetDir, result.exportPath));
      result.fileManifest.forEach((_, i) => {
        result.fileManifest[i].file = path.join(this.assetDir, path.relative(this.remoteAssetDir, result.fileManifest[i].file));
      });
      return result;
    }
    if (status === 409 || json?.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    if (status === 400) throw new Error('Invalid parameters for character export');
    if (status >= 500) throw new Error(`Server error during character export: ${json?.message ?? 'unknown'}`);
    throw new Error('Unexpected response for character export');
  }

  public async resetConnection(): Promise<void> {
    await this.bootstrap();
  }

  public clearCacheFiles() {
    if (this.isRemote) {
      emptyDirSync(this.cacheDir);
    }
  }

  // ===== HELPERS (private) =====

  private bootPromise: Promise<void> | null = null;

  private logWarnedBootstrap = false;

  private async bootstrap(): Promise<void> {
    if (this.bootPromise) return this.bootPromise;
    this.bootPromise = (async () => {
      try {
        // Connected if REST server responds to any request.
        await this.getConfig();
        this.status.connected = true;
        this.status.configLoaded = true;

        try {
          const info = await this.getCASCInfo();
          this.cascInfo = info;
          this.status.cascLoaded = true;
          console.log(chalk.green('✅ Retrieved wow.export WoW installation:'), info.build.Product, info.buildName);
        } catch (err) {
          // Attempt automatic CASC loading using environment hints
          await this.tryAutoLoadCASC();
        }

        if (this.isReady) {
          console.log(chalk.green('✅ Connected to wow.export:'), chalk.gray(this.baseURL));
          this.logWarnedBootstrap = false;
        }
      } catch (e) {
        if (!this.logWarnedBootstrap) {
          console.error(chalk.yellow(`⏳ Cannot connect to wow.export at ${this.baseURL}. Is it running?`));
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

  private startHeartbeat(): void {
    const tick = () => {
      try {
        if (!this.isReady) {
          if (!this.bootPromise) {
            void this.bootstrap();
          }
        } else {
          void this.safeGetJSON('/rest/getCascInfo').then((res) => {
            if (!(res.ok && res.json?.id === 'CASC_INFO')) {
              this.status.connected = false;
              this.status.cascLoaded = false;
            }
          });
        }
      } catch (e) {
        this.status.connected = false;
      }
    };
    setInterval(tick, 500);
    tick();
  }

  private logWarnedCASC = false;

  private async tryAutoLoadCASC(): Promise<void> {
    if (this.status.cascLoaded) return;

    const localPath = process.env.CASC_LOCAL_WOW;
    const localProduct = process.env.CASC_LOCAL_PRODUCT;
    const remoteRegion = process.env.CASC_REMOTE_REGION;
    const remoteProduct = process.env.CASC_REMOTE_PRODUCT;

    if (localPath) {
      try {
        console.log(chalk.gray(`Attempting to load local CASC from "${localPath}", product: ${localProduct}`));
        const builds = await this.loadCASCLocal(localPath);
        const buildIdx = Math.max(0, builds.findIndex((b) => b.Product === localProduct));
        const info = await this.loadCASCBuild(buildIdx);
        this.cascInfo = info;
        this.status.cascLoaded = true;
        console.log(chalk.green('✅ Loaded local CASC:'), info.build.Product, info.buildName);
        this.logWarnedCASC = false;
        return;
      } catch (e) {
        if (!this.logWarnedCASC) {
          console.error(chalk.yellow(`⚠️ Failed to load local CASC: ${e}`));
          this.logWarnedCASC = true;
        }
      }
    }

    if (!this.status.cascLoaded && remoteRegion && remoteProduct) {
      try {
        console.log(chalk.gray(`Attempting to load remote CASC, region: ${remoteRegion}, product: ${remoteProduct}`));
        const builds = await this.loadCASCRemote(remoteRegion);
        const buildIdx = Math.max(0, builds.findIndex((b) => b.Product === remoteProduct));
        const info = await this.loadCASCBuild(buildIdx);
        this.cascInfo = info;
        this.status.cascLoaded = true;
        console.log(chalk.green('✅ Loaded remote CASC:'), info.build.Product, info.buildName);
        this.logWarnedCASC = false;
        return;
      } catch (e) {
        if (!this.logWarnedCASC) {
          console.error(chalk.yellow(`⚠️ Failed to load remote CASC: ${e}`));
          this.logWarnedCASC = true;
        }
      }
    }

    if (!this.status.cascLoaded && !this.logWarnedCASC) {
      console.error(chalk.yellow('⏳ Please choose your WoW installation in wow.export.'));
      this.logWarnedCASC = true;
    }
  }

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
    } catch (e) {
      return { ok: false, json: null };
    }
  }

  // ===== FILE TRANSFER (remote mode) =====

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
    if (!this.isRemote) return path.resolve(relativePath);

    const rel = this.normalizeRelative(relativePath);
    const dest = path.resolve(this.cacheDir, rel);
    if (allowCache(rel) && existsSync(dest)) return dest;

    const dir = path.dirname(dest);
    ensureDirSync(dir);

    console.log('Fetch file from remote wow.export', relativePath, this.isRemote);
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

    fs.writeFileSync(dest, buf);
    return dest;
  }
}

export const wowExportClient = new WowExportRestClient(
  process.env.WOW_EXPORT_BASE_URL || 'http://127.0.0.1:17752',
);

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
};

// backoff helper removed; no request-level retries used.
