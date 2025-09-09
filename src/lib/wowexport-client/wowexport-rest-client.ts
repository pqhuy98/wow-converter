/* eslint-disable @typescript-eslint/no-explicit-any */
import axios, { AxiosInstance } from 'axios';
import chalk from 'chalk';

import { waitUntil } from '../utils';
import {
  CASCBuild,
  CASCInfo,
  ConfigResponse,
  desiredConfig,
  ExportCharacterParams,
  ExportCharacterResult,
  ExportFile,
  ExportResult,
  FileEntry,
  ModelSkin,
  SearchResult,
} from './wowexport-client';

const debug = false;

export class WowExportRestClient {
  private readonly http: AxiosInstance;

  private assetDir = '';

  public status = {
    connected: false,
    configLoaded: false,
    cascLoaded: false,
    // REST does not use hooks; keep true to maintain isReady contract.
    exportHookRegistered: true,
  };

  cascInfo: CASCInfo | null = null;

  constructor(private host: string = '127.0.0.1', private port: number = 17752) {
    this.http = axios.create({ baseURL: `http://${host}:${port}`, timeout: 300000 });
    const debug = false;
    this.http.interceptors.request.use((config) => {
      debug && console.log('request', config.method, config.url, config.data);
      return config;
    });
    this.http.interceptors.response.use((response) => {
      debug && console.log('response', response.status, response.data);
      return response;
    });
    void this.bootstrap();
  }

  public get isReady() {
    return this.status.connected && this.status.configLoaded && this.status.cascLoaded && this.status.exportHookRegistered;
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
    const config = await this.getConfig();
    this.assetDir = (config as any).exportDirectory;
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

  public async setConfig(key: string, value: any): Promise<ConfigResponse> {
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
    const json = await this.postJSON('/rest/exportModels', { models });
    if (json.id === 'EXPORT_RESULT') return json.succeeded as ExportResult[];
    if (json.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    throw new Error('Failed to start model export');
  }

  public async exportTextures(fileDataIDs: number[]): Promise<ExportFile[]> {
    if (fileDataIDs.length === 0) return [];
    const json = await this.postJSON('/rest/exportTextures', { fileDataID: fileDataIDs });
    if (json.id === 'EXPORT_RESULT') return json.succeeded as ExportFile[];
    if (json.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    throw new Error('Failed to start texture export');
  }

  public async exportCharacter(data: ExportCharacterParams): Promise<ExportCharacterResult> {
    const json = await this.postJSON('/rest/exportCharacter', data);
    if (json.id === 'EXPORT_RESULT') return { exportPath: json.exportPath, fileName: json.fileName, fileManifest: json.fileManifest } as ExportCharacterResult;
    if (json.id === 'ERR_NO_CASC') throw new Error('No CASC loaded');
    throw new Error('Failed to start character export');
  }

  public async resetConnection(): Promise<void> {
    await this.bootstrap(true);
  }

  // ===== HELPERS (private) =====

  private async bootstrap(isReset: boolean = false): Promise<void> {
    try {
      // Connected if REST server responds to any request.
      await this.syncConfig();
      this.status.connected = true;
      this.status.configLoaded = true;
      debug && console.log('REST config retrieved');

      try {
        const info = await this.getCASCInfo();
        this.cascInfo = info;
        this.status.cascLoaded = true;
        console.log(chalk.green('✅ Retrieved wow.export CASC info (REST):'), info.build.Product, info.buildName);
      } catch (err) {
        // Attempt automatic CASC loading using environment hints
        await this.tryAutoLoadCASC();
      }

      if (this.isReady && !isReset) {
        console.log(chalk.green('✅ Connected to wow.export REST'), chalk.gray(`at ${this.host}:${this.port}`));
      }
    } catch (e) {
      console.error(chalk.yellow(`⏳ Cannot connect to wow.export REST at ${this.host}:${this.port}. Is it running?`));
    }
  }

  private async tryAutoLoadCASC(): Promise<void> {
    if (this.status.cascLoaded) return;

    const localPath = process.env.CASC_LOCAL_WOW;
    const localProduct = process.env.CASC_LOCAL_PRODUCT;
    const remoteRegion = process.env.CASC_REMOTE_REGION;
    const remoteProduct = process.env.CASC_REMOTE_PRODUCT;

    if (localPath) {
      try {
        console.log(chalk.gray(`Attempting to load local CASC (REST) from "${localPath}", product: ${localProduct}`));
        const builds = await this.loadCASCLocal(localPath);
        const buildIdx = Math.max(0, builds.findIndex((b) => b.Product === localProduct));
        const info = await this.loadCASCBuild(buildIdx);
        this.cascInfo = info;
        this.status.cascLoaded = true;
        console.log(chalk.green('✅ Loaded local CASC (REST):'), info.build.Product, info.buildName);
        return;
      } catch (e) {
        console.error(chalk.yellow(`⚠️ Failed to load local CASC (REST): ${e}`));
      }
    }

    if (!this.status.cascLoaded && remoteRegion && remoteProduct) {
      try {
        console.log(chalk.gray(`Attempting to load remote CASC (REST), region: ${remoteRegion}, product: ${remoteProduct}`));
        const builds = await this.loadCASCRemote(remoteRegion);
        const buildIdx = Math.max(0, builds.findIndex((b) => b.Product === remoteProduct));
        const info = await this.loadCASCBuild(buildIdx);
        this.cascInfo = info;
        this.status.cascLoaded = true;
        console.log(chalk.green('✅ Loaded remote CASC (REST):'), info.build.Product, info.buildName);
        return;
      } catch (e) {
        console.error(chalk.yellow(`⚠️ Failed to load remote CASC (REST): ${e}`));
      }
    }

    if (!this.status.cascLoaded) console.error(chalk.yellow('⏳ CASC not available (REST). Set CASC_LOCAL_WOW or CASC_REMOTE_REGION/CASC_REMOTE_PRODUCT'));
  }

  private async getJSON(path: string, params?: Record<string, any>): Promise<any> {
    const res = await this.http.get(path, { params });
    return res.data;
  }

  private async postJSON(path: string, body?: any): Promise<any> {
    const res = await this.http.post(path, body ?? {});
    return res.data;
  }

  private async safeGetJSON(path: string, params?: Record<string, any>): Promise<{ ok: boolean; json: any }> {
    try {
      const res = await this.http.get(path, { params, validateStatus: () => true });
      return { ok: res.status >= 200 && res.status < 300, json: res.data };
    } catch (e) {
      return { ok: false, json: null };
    }
  }
}
