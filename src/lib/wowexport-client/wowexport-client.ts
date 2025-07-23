/* eslint-disable @typescript-eslint/no-explicit-any */
/*!
 * Wow.Export RCP Client Library
 * A complete client library for communicating with wow.export's Remote Control Protocol
 *
 * Usage:
 * const client = new WowExportClient();
 * await client.connect('localhost', 17751);
 * await client.loadCASC('C:/World of Warcraft');
 * await client.exportModel(12345, './output/model.obj');
 */
import chalk from 'chalk';
import { randomUUID } from 'crypto';
import { EventEmitter } from 'events';
import { Socket } from 'net';

import { args } from '../constants';
import { wowExportPath } from '../global-config';
import { waitUntil } from '../utils';

// Type definitions
export interface ServerInfo {
    version: string;
    flavour: string;
    build: string;
    rcp: number;
}

export interface CASCInfo {
    type: string;
    build: any;
    buildConfig: any;
    buildName: string;
    buildKey: string;
}

export interface CASCBuild {
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

export interface SearchResult {
    entries: FileEntry[];
}

export interface ExportInfo {
    exportID: number;
}

export interface ExportFile {
    type: string;
    fileDataID: number;
    file: string;
}

export interface ExportResult {
    fileDataID: number;
    files: ExportFile[];
}

export interface ConfigResponse {
    [key: string]: any;
}

export interface HookEvent {
    hookID: string;
    [key: string]: any;
}

export interface RCPResponse {
    id: string;
    [key: string]: any;
}

export type HookID = 'HOOK_BUSY_STATE' | 'HOOK_INSTALL_READY' | 'HOOK_EXPORT_COMPLETE';

const debug = false;

export class WowExportClient extends EventEmitter {
  private socket: Socket;

  private buffer: string = '';

  public status = {
    connected: false,
    configLoaded: false,
    cascLoaded: false,
  };

  public get isReady() {
    return this.status.connected && this.status.configLoaded && this.status.cascLoaded;
  }

  public async waitUntilReady() {
    return waitUntil(() => this.isReady);
  }

  constructor(host: string = '127.0.0.1', port: number = 17751) {
    super();
    this.setMaxListeners(100);

    this.socket = new Socket();

    void (async () => {
      for (let failedAttempts = 0; ;) {
        try {
          if (!this.status.connected) {
            await this.connect(host, port);
            console.log(chalk.green('✅ Connected to wow.export RCP'), chalk.gray(`at ${host}:${port}`));
            failedAttempts = 0;
          }
          if (!this.status.configLoaded) {
            const config = await this.getConfig();
            wowExportPath.value = config.exportDirectory.replace('\\', '/');
            this.status.configLoaded = true;
            console.log(chalk.green('✅ Retrieved wow.export asset dir:'), chalk.gray(wowExportPath.value));
            failedAttempts = 0;
          }
          if (!this.status.cascLoaded) {
            const info = await this.getCASCInfo();
            this.status.cascLoaded = true;
            console.log(chalk.green('✅ Retrieved wow.export CASC info:'), info.buildName);
            failedAttempts = 0;
          }
        } catch (err) {
          if (failedAttempts === 0) {
            if (!this.status.connected || !this.status.configLoaded) {
              console.error(chalk.yellow(`⏳ Cannot connecting to wow.export RCP server at ${host}:${port}, did you run it?`));
            } else if (!this.status.cascLoaded) {
              if (args.cascRemote) {
                const [region, product] = args.cascRemote.split(':');
                console.log({ region, product });
                const cascRemote = await this.loadCASCRemote(region);
                const buildIdx = cascRemote.findIndex((build: any) => build.Product === product);
                console.log('Selected build:', cascRemote[buildIdx]);
                const cascInfo = await this.loadCASCBuild(buildIdx);
                console.log({ cascInfo });
              } else {
                console.error(chalk.yellow('⏳ Cannot getting wow.export CASC info, did you select game installation?'));
              }
            }
          }
          failedAttempts++;
        }
        await new Promise((resolve) => { setTimeout(resolve, 3000); });
      }
    })();
    void this.registerHook('HOOK_EXPORT_COMPLETE');
  }

  /**
     * Connect to a wow.export RCP server
     * @param host - Server hostname (default: 'localhost')
     * @param port - Server port (default: 17751)
     * @returns Promise that resolves when connected
     */
  async connect(host: string = 'localhost', port: number = 17751): Promise<void> {
    // If already connected, disconnect first
    if (this.status.connected) {
      return Promise.resolve();
    }

    // Create a new socket if the current one is in use
    if (this.socket) {
      this.socket.destroy();
    }
    this.socket = new Socket();
    this.socket.on('data', (data: Buffer) => this.onData(data));
    this.socket.on('close', () => this.onConnectionClose());

    return new Promise((resolve, reject) => {
      debug && console.log(`Connecting to wow.export RCP at [${host}]:${port}`);

      this.socket.connect(port, host, () => {
        this.status.connected = true;
        debug && console.log('Connected to wow.export RCP server');
        this.socket.on('error', () => {
          this.status.connected = false;
          this.status.configLoaded = false;
          this.status.cascLoaded = false;
        });
        resolve();
      });
      this.socket.once('error', reject);
    });
  }

  /**
     * Disconnect from the server
     */
  disconnect(): void {
  }

  private waitForConnection(): Promise<void> {
    return new Promise((resolve) => {
      this.once('CONNECTED', resolve);
    });
  }

  async syncConfig(): Promise<void> {
    await Promise.all(Object.entries(desiredConfig).map(([key, value]) => this.setConfig(key, value)));
  }

  /**
     * Send a command to the server and wait for response
     * @param command - Command ID
     * @param data - Command data
     * @returns Promise that resolves with response data
     */
  async sendCommand(command: string, data: any = {}): Promise<RCPResponse> {
    if (!this.status.connected) {
      await this.waitForConnection();
    }

    const requestId = randomUUID();
    const payload = { id: command, ...data, requestId };

    let timeoutS = 30000;
    if (command === 'LOAD_CASC_BUILD') {
      timeoutS = 300000; // load casc build can take a long time
    }

    return new Promise((resolve, reject) => {
      // Set up response handler - listen for the specific response type
      const responseHandler = (response: RCPResponse) => {
        // Handle different response patterns based on command
        let shouldResolve = false;

        switch (command) {
          case 'LISTFILE_SEARCH':
            shouldResolve = response.id === 'LISTFILE_SEARCH_RESULT';
            break;
          case 'LISTFILE_QUERY_ID':
          case 'LISTFILE_QUERY_NAME':
            shouldResolve = response.id === 'LISTFILE_RESULT';
            break;
          case 'EXPORT_MODEL':
          case 'EXPORT_TEXTURE':
          case 'EXPORT_CHARACTER':
            shouldResolve = response.id === 'EXPORT_START' && response.requestId === requestId;
            break;
          case 'CONFIG_GET':
            shouldResolve = response.id === 'CONFIG_SINGLE' || response.id === 'CONFIG_FULL';
            break;
          case 'CONFIG_SET':
            shouldResolve = response.id === 'CONFIG_SET_DONE';
            break;
          case 'CONFIG_RESET':
            shouldResolve = response.id === 'CONFIG_SINGLE' || response.id === 'CONFIG_FULL';
            break;
          case 'LOAD_CASC_LOCAL':
          case 'LOAD_CASC_REMOTE':
            shouldResolve = response.id === 'CASC_INSTALL_BUILDS' || response.id === 'ERR_INVALID_INSTALL';
            break;
          case 'LOAD_CASC_BUILD':
            shouldResolve = response.id === 'CASC_INFO' || response.id === 'ERR_NO_CASC_SETUP' || response.id === 'ERR_INVALID_CASC_BUILD' || response.id === 'ERR_CASC_FAILED';
            break;
          case 'GET_CASC_INFO':
            shouldResolve = response.id === 'CASC_INFO' || response.id === 'CASC_UNAVAILABLE';
            break;
          case 'CLEAR_CACHE':
            shouldResolve = response.id === 'CACHE_CLEARED';
            break;
          case 'HOOK_REGISTER':
            shouldResolve = response.id === 'HOOK_REGISTERED';
            break;
          case 'HOOK_DEREGISTER':
            shouldResolve = response.id === 'HOOK_DEREGISTERED';
            break;
          case 'GET_CONSTANTS':
            shouldResolve = response.id === 'CONSTANTS';
            break;
          case 'GET_CDN_REGIONS':
            shouldResolve = response.id === 'CDN_REGIONS';
            break;
          case 'GET_MODEL_SKINS':
            shouldResolve = response.id === 'MODEL_SKINS' && response.fileDataID === data.fileDataID;
            break;
          default:
            // For unknown commands, resolve on any response
            shouldResolve = true;
        }

        if (shouldResolve) {
          this.removeListener('response', responseHandler);
          resolve(response);
        }
      };

      this.on('response', responseHandler);

      // Send the command
      this.write(payload);

      // Timeout after 30 seconds
      setTimeout(() => {
        this.removeListener('response', responseHandler);
        reject(new Error(`Command ${command} timed out`));
      }, timeoutS);
    });
  }

  /**
     * Write data to the socket using RCP protocol
     * @param data - Data to send
     */
  private write(data: any): void {
    const json = JSON.stringify(data);
    const message = `${json.length}\0${json}`;
    this.socket.write(message);
    if (debug && data.id !== 'CONFIG_SET') {
      debug && console.log(`→ ${data.id} (${json.length} bytes)`);
    }
  }

  /**
     * Handle incoming data from the server
     * @param data - Raw data from socket
     */
  private onData(data: Buffer): void {
    this.buffer += data.toString('utf8');
    this.processBuffer();
  }

  /**
     * Process the message buffer for complete messages
     */
  private processBuffer(): void {
    const delimiter = this.buffer.indexOf('\0');
    if (delimiter > 0) {
      // eslint-disable-next-line radix
      const size = parseInt(this.buffer.substring(0, delimiter));
      if (isNaN(size) || size <= 0) {
        throw new Error('Invalid stream segmentation');
      }

      const offset = delimiter + 1;
      const availableSize = this.buffer.length - offset;

      if (availableSize >= size) {
        // Extract complete message
        const messageData = this.buffer.substring(offset, offset + size);
        const json: RCPResponse = JSON.parse(messageData);

        if (debug && json.id !== 'CONFIG_SET_DONE') {
          debug && console.log(`← ${json.id} (${size} bytes)`);
          debug && console.log(JSON.stringify(json, null, 2));
        }
        // Emit the message with both the specific ID and generic 'response' event
        this.emit(json.id, json);
        this.emit('response', json);

        // Remove processed message from buffer
        this.buffer = this.buffer.substring(offset + size);

        // Process any remaining messages
        if (this.buffer.length > 0) {
          this.processBuffer();
        }
      }
    }
  }

  /**
     * Handle connection close
     */
  private onConnectionClose(): void {
    debug && console.log('Connection to wow.export RCP server closed');
    this.status.connected = false;
    this.status.configLoaded = false;
    this.status.cascLoaded = false;
    this.emit('disconnected');
  }

  // ===== HIGH-LEVEL API METHODS =====

  /**
     * Get server information
     * @returns Promise with server info
     */
  async getServerInfo(): Promise<ServerInfo> {
    return new Promise((resolve) => {
      this.once('CONNECTED', resolve);
    });
  }

  /**
     * Get configuration
     * @param key - Optional config key
     * @returns Promise with configuration data
     */
  async getConfig(key?: string): Promise<ConfigResponse> {
    const data = key ? { key } : {};
    const response = await this.sendCommand('CONFIG_GET', data);

    if (response.id === 'CONFIG_SINGLE') {
      return { [response.key]: response.value };
    } if (response.id === 'CONFIG_FULL') {
      return response.config;
    }

    throw new Error('Unexpected response to CONFIG_GET');
  }

  /**
     * Set configuration value
     * @param key - Config key
     * @param value - Config value
     * @returns Promise with updated config
     */
  async setConfig(key: string, value: any): Promise<ConfigResponse> {
    const response = await this.sendCommand('CONFIG_SET', { key, value });

    if (response.id === 'CONFIG_SET_DONE') {
      return { [response.key]: response.value };
    }

    throw new Error('Failed to set configuration');
  }

  /**
     * Reset configuration to defaults
     * @param key - Optional config key to reset
     * @returns Promise with reset configuration
     */
  async resetConfig(key?: string): Promise<ConfigResponse> {
    const data = key ? { key } : {};
    const response = await this.sendCommand('CONFIG_RESET', data);

    if (response.id === 'CONFIG_SINGLE') {
      return { [response.key]: response.value };
    } if (response.id === 'CONFIG_FULL') {
      return response.config;
    }

    throw new Error('Failed to reset configuration');
  }

  /**
     * Load CASC from local installation
     * @param installDirectory - WoW installation path
     * @returns Promise with CASC builds information
     */
  async loadCASCLocal(installDirectory: string): Promise<CASCBuild[]> {
    const response = await this.sendCommand('LOAD_CASC_LOCAL', { installDirectory });

    if (response.id === 'CASC_INSTALL_BUILDS') {
      return response.builds;
    } if (response.id === 'ERR_INVALID_INSTALL') {
      throw new Error('Invalid WoW installation directory');
    } else if (response.id === 'ERR_CASC_ACTIVE') {
      throw new Error('CASC is already active');
    }

    throw new Error('Failed to load CASC');
  }

  /**
     * Load CASC from remote CDN
     * @param regionTag - CDN region (e.g., 'eu', 'us')
     * @returns Promise with CASC builds information
     */
  async loadCASCRemote(regionTag: string): Promise<CASCBuild[]> {
    const response = await this.sendCommand('LOAD_CASC_REMOTE', { regionTag });

    if (response.id === 'CASC_INSTALL_BUILDS') {
      return response.builds;
    } if (response.id === 'ERR_INVALID_INSTALL') {
      throw new Error('Invalid CDN region');
    } else if (response.id === 'ERR_CASC_ACTIVE') {
      throw new Error('CASC is already active');
    }

    throw new Error('Failed to load CASC');
  }

  /**
     * Load specific CASC build
     * @param buildIndex - Build index from builds list
     * @returns Promise with CASC information
     */
  async loadCASCBuild(buildIndex: number): Promise<CASCInfo> {
    const response = await this.sendCommand('LOAD_CASC_BUILD', { buildIndex });

    if (response.id === 'CASC_INFO') {
      return response as unknown as CASCInfo;
    } if (response.id === 'ERR_NO_CASC_SETUP') {
      throw new Error('No CASC setup available');
    } else if (response.id === 'ERR_INVALID_CASC_BUILD') {
      throw new Error('Invalid build index');
    } else if (response.id === 'ERR_CASC_FAILED') {
      throw new Error('Failed to load CASC build');
    }

    throw new Error('Failed to load CASC build');
  }

  /**
     * Get CASC information
     * @returns Promise with CASC information
     */
  async getCASCInfo(): Promise<CASCInfo> {
    const response = await this.sendCommand('GET_CASC_INFO');

    if (response.id === 'CASC_INFO') {
      return response as unknown as CASCInfo;
    } if (response.id === 'CASC_UNAVAILABLE') {
      throw new Error('CASC not available');
    }

    throw new Error('Failed to get CASC info');
  }

  /**
     * Search for files in listfile
     * @param search - Search pattern
     * @param useRegex - Use regular expression
     * @returns Promise with search results
     */
  async searchFiles(search: string, useRegex: boolean = false): Promise<FileEntry[]> {
    const response = await this.sendCommand('LISTFILE_SEARCH', { search, useRegularExpression: useRegex });

    if (response.id === 'LISTFILE_SEARCH_RESULT') {
      return response.entries;
    } if (response.id === 'ERR_LISTFILE_NOT_LOADED') {
      throw new Error('Listfile not loaded');
    }

    throw new Error('Failed to search files');
  }

  /**
     * Get file by ID
     * @param fileDataID - File data ID
     * @returns Promise with file information
     */
  async getFileByID(fileDataID: number): Promise<FileEntry> {
    const response = await this.sendCommand('LISTFILE_QUERY_ID', { fileDataID });

    if (response.id === 'LISTFILE_RESULT') {
      return response as unknown as FileEntry;
    } if (response.id === 'ERR_LISTFILE_NOT_LOADED') {
      throw new Error('Listfile not loaded');
    }

    throw new Error('Failed to get file by ID');
  }

  /**
     * Get file by name
     * @param fileName - File name
     * @returns Promise with file information
     */
  async getFileByName(fileName: string): Promise<FileEntry> {
    const response = await this.sendCommand('LISTFILE_QUERY_NAME', { fileName });

    if (response.id === 'LISTFILE_RESULT') {
      return response as unknown as FileEntry;
    } if (response.id === 'ERR_LISTFILE_NOT_LOADED') {
      throw new Error('Listfile not loaded');
    }

    throw new Error('Failed to get file by name');
  }

  async getModelSkins(fileDataID: number): Promise<{
    id: string;
    label: string;
    displayID: number;
    textureIDs: number[];
    extraGeosets: any[];
  }[]> {
    const response = await this.sendCommand('GET_MODEL_SKINS', { fileDataID });
    if (response.id === 'MODEL_SKINS') {
      return response.skins;
    }
    throw new Error('Failed to get model skins');
  }

  /**
     * Export models
     * @param fileDataIDs - File data ID(s)
     * @returns Promise that resolves with export results when export completes
     */
  async exportModels(models: { fileDataID: number, skinName?: string }[]): Promise<ExportResult[]> {
    if (models.length === 0) {
      return [];
    }

    let exportID = -1;
    const isComplete = (eventData: any) => {
      if (exportID === -1) {
        return 'not ready';
      }
      return eventData.hookID === 'HOOK_EXPORT_COMPLETE' && eventData.exportID === exportID;
    };
    const hookEventPromise = this.waitForHookEvent(isComplete);

    debug && console.log('exportModels start', models);
    const response = await this.sendCommand('EXPORT_MODEL', { models });

    if (response.id === 'EXPORT_START') {
      const exportInfo = response as unknown as ExportInfo;
      debug && console.log(`Export model started with ID: ${exportInfo.exportID}`);
      exportID = exportInfo.exportID;

      // Emit a synthetic event to re-evaluate any events that arrived before exportID was known
      this.emit('HOOK_EVENT', { hookID: '__intern_flush__' });

      const result = await hookEventPromise;
      return result.succeeded;
    } if (response.id === 'ERR_NO_CASC') {
      throw new Error('No CASC loaded');
    }

    throw new Error('Failed to start model export');
  }

  /**
     * Export textures
     * @param fileDataIDs - File data ID(s)
     * @returns Promise that resolves with export results when export completes
     */
  async exportTextures(fileDataIDs: number[]): Promise<ExportFile[]> {
    if (fileDataIDs.length === 0) {
      return [];
    }

    let exportID = -1;
    const isComplete = (eventData: any) => {
      if (exportID === -1) {
        return 'not ready';
      }
      return eventData.hookID === 'HOOK_EXPORT_COMPLETE' && eventData.exportID === exportID;
    };
    const hookEventPromise = this.waitForHookEvent(isComplete);

    const response = await this.sendCommand('EXPORT_TEXTURE', { fileDataID: fileDataIDs });

    if (response.id === 'EXPORT_START') {
      const exportInfo = response as unknown as ExportInfo;
      debug && console.log(`Export textures started with ID: ${exportInfo.exportID}`);
      exportID = exportInfo.exportID;

      // Emit a synthetic event to re-evaluate any events that arrived before exportID was known
      this.emit('HOOK_EVENT', { hookID: '__intern_flush__' });
      return (await hookEventPromise).succeeded;
    } if (response.id === 'ERR_NO_CASC') {
      throw new Error('No CASC loaded');
    }

    throw new Error('Failed to start texture export');
  }

  /**
     * Export character
     * @param data - Character export data
     * @returns Promise with export files
     */
  async exportCharacter(data: {
    race: number;
    gender: number;
    customizations: { [optionId: string]: number };
    format: string;
    include_animations: boolean;
    include_base_clothing: boolean;
    excludeAnimationIds?: number[];
  }): Promise<{
    exportPath: string;
    fileName: string;
    fileManifest: ExportFile[];
  }> {
    let exportID = -1;
    const isComplete = (eventData: any) => {
      if (exportID === -1) {
        return 'not ready';
      }

      if (eventData.hookID === 'HOOK_EXPORT_COMPLETE') {
        debug && console.log('export character complete', eventData);
      }

      return eventData.hookID === 'HOOK_EXPORT_COMPLETE'
        && eventData.exportPath && eventData.fileName
        && eventData.fileManifest && eventData.exportID === exportID;
    };
    const hookEventPromise = this.waitForHookEvent(isComplete);
    const response = await this.sendCommand('EXPORT_CHARACTER', data);

    if (response.id === 'EXPORT_START') {
      const exportInfo = response as unknown as ExportInfo;
      debug && console.log(`Export character started with ID: ${exportInfo.exportID}`);
      exportID = exportInfo.exportID;

      // Emit a synthetic event to re-evaluate any events that arrived before exportID was known
      this.emit('HOOK_EVENT', { hookID: '__intern_flush__' });

      return hookEventPromise;
    } if (response.id === 'ERR_NO_CASC') {
      throw new Error('No CASC loaded');
    }

    throw new Error(`Failed to start character export${JSON.stringify(response, null, 2)}`);
  }

  private waitForHookEvent(matcher: (eventData: any) => boolean | 'not ready'): Promise<any> {
    return new Promise((resolve) => {
      const pending: unknown[] = [];
      const eventHandler = (eventData: any) => {
        debug && console.log(`Recevied hook event "${eventData.hookID}"`);
        const result = matcher(eventData);
        if (result === 'not ready') {
          pending.push(eventData);
          return;
        }
        if (result) {
          this.off('HOOK_EVENT', eventHandler);
          resolve(eventData);
        } else {
          pending.forEach((event) => {
            if (matcher(event)) {
              this.off('HOOK_EVENT', eventHandler);
              resolve(event);
            }
          });
          pending.length = 0;
        }
      };

      this.on('HOOK_EVENT', eventHandler);
    });
  }

  /**
     * Clear cache
     * @returns Promise that resolves when cache is cleared
     */
  async clearCache(): Promise<void> {
    const response = await this.sendCommand('CLEAR_CACHE');

    if (response.id === 'CACHE_CLEARED') {
      return;
    }

    throw new Error('Failed to clear cache');
  }

  /**
     * Register for events
     * @param hookID - Hook ID to register for
     * @returns Promise that resolves when registered
     */
  async registerHook(hookID: HookID): Promise<void> {
    const response = await this.sendCommand('HOOK_REGISTER', { hookID });

    if (response.id === 'HOOK_REGISTERED') {
      return;
    } if (response.id === 'ERR_UNKNOWN_HOOK') {
      throw new Error('Unknown hook ID');
    }

    throw new Error('Failed to register hook');
  }

  /**
     * Deregister from events
     * @param hookID - Hook ID to deregister from
     * @returns Promise that resolves when deregistered
     */
  async deregisterHook(hookID: HookID): Promise<void> {
    const response = await this.sendCommand('HOOK_DEREGISTER', { hookID });

    if (response.id === 'HOOK_DEREGISTERED') {
      return;
    }

    throw new Error('Failed to deregister hook');
  }

  /**
     * Get constants
     * @returns Promise with constants
     */
  async getConstants(): Promise<any> {
    const response = await this.sendCommand('GET_CONSTANTS');

    if (response.id === 'CONSTANTS') {
      return response.constants;
    }

    throw new Error('Failed to get constants');
  }

  /**
     * Get CDN regions
     * @returns Promise with CDN regions
     */
  async getCDNRegions(): Promise<any[]> {
    const response = await this.sendCommand('GET_CDN_REGIONS');

    if (response.id === 'CDN_REGIONS') {
      return response.regions;
    }

    throw new Error('Failed to get CDN regions');
  }

  /**
     * Restart the wow.export application
     * @returns Promise that resolves when restart is initiated
     */
  async restartApp(): Promise<void> {
    await this.sendCommand('RESTART_APP');
    // Note: This will disconnect the client
  }

  // ===== CONVENIENCE METHODS =====

  /**
     * Complete workflow: Load CASC and get ready for exports
     * @param installPath - WoW installation path
     * @param buildIndex - Build index to load
     * @returns Promise with CASC information
     */
  async initializeCASC(installPath: string, buildIndex: number = 0): Promise<CASCInfo> {
    debug && console.log('Loading CASC from local installation...');
    const builds = await this.loadCASCLocal(installPath);

    debug && console.log(`Found ${builds.length} builds`);
    if (builds.length === 0) {
      throw new Error('No builds found in installation');
    }

    debug && console.log(`Loading build ${buildIndex}: ${builds[buildIndex]?.Product || 'Unknown'}`);
    const cascInfo = await this.loadCASCBuild(buildIndex);

    debug && console.log(`CASC loaded: ${cascInfo.buildName} (${cascInfo.buildKey})`);
    return cascInfo;
  }
}

export const wowExportClient = new WowExportClient();

export const desiredConfig = {
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
  modelsExportUV2: false,
  modelsExportTextures: true,
  modelsExportAlpha: true,
  modelsExportAnimations: true,
};
