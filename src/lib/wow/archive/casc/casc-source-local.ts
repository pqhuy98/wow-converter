/**
 * Local installation CASC source, ported from wow.export (src/js/casc/casc-source-local.js).
 */
import { promises as fsp } from 'fs';
import path from 'path';
import util from 'util';

import { timeEnd, timeLog, write } from '@/lib/wow/log';
import { createProgress, runtimeState } from '@/lib/wow/server/runtime';

import { BufferWrapper } from '../../formats/buffer';
import { constants } from '../../formats/constants';
import { fileExists, readFile } from '../../formats/generics';
import { BLTEReader } from './blte-reader';
import { BuildCache } from './build-cache';
import type { CascKey } from './casc-key';
import { cascKeyFromHex, cascKeyToHex } from './casc-key';
import { CASC } from './casc-source';
import { CASCRemote } from './casc-source-remote';
import { CDNConfigEntries, parseCDNConfig } from './cdn-config';
import { cdnResolver } from './cdn-resolver';
import * as listfile from './listfile';
import { parseVersionConfig } from './version-config';

interface LocalIndexEntry {
  index: number;
  offset: number;
  size: number;
}

export class CASCLocal extends CASC {
  dir: string;

  dataDir: string;

  storageDir: string;

  localIndexes = new Map<string, LocalIndexEntry>();

  cache!: BuildCache;

  remote?: CASCRemote;

  /**
   * Create a new CASC source using a local installation.
   * @param dir Installation path.
   */
  constructor(dir: string) {
    super(false);

    this.dir = dir;
    this.dataDir = path.join(dir, constants.BUILD.DATA_DIR);
    this.storageDir = path.join(this.dataDir, 'data');
  }

  /** Initialize local CASC source. */
  async init(): Promise<void> {
    write('Initializing local CASC installation: %s', this.dir);

    const buildInfo = path.join(this.dir, constants.BUILD.MANIFEST);
    const config = parseVersionConfig(await fsp.readFile(buildInfo, 'utf8'));

    // Filter known products.
    this.builds = config.filter((entry) => constants.PRODUCTS.some((e) => e.product === entry.Product));
  }

  /** Obtain a file by its fileDataID. */
  async getFile(
    fileDataID: number,
    partialDecryption = false,
    suppressLog = false,
    supportFallback = true,
    forceFallback = false,
    contentKey: CascKey | null = null,
  ): Promise<BLTEReader> {
    if (!suppressLog) write('Loading local CASC file %d (%s)', fileDataID, listfile.getByID(fileDataID));

    const encodingKey = contentKey !== null ? this.getEncodingKeyForContentKey(contentKey) : this.getEncodingKey(fileDataID);
    const encodingKeyHex = cascKeyToHex(encodingKey);
    const data = supportFallback
      ? await this.getDataFileWithRemoteFallback(encodingKeyHex, forceFallback)
      : await this.getDataFile(encodingKeyHex);
    return new BLTEReader(data, encodingKeyHex, partialDecryption);
  }

  /**
   * Returns a list of available products in the installation.
   * Format example: "PTR: World of Warcraft 8.3.0.32272"
   */
  getProductList(): string[] {
    const products: string[] = [];
    for (const entry of this.builds) {
      const product = constants.PRODUCTS.find((e) => e.product === entry.Product)!;
      products.push(util.format('%s (%s) %s', product.title, entry.Branch.toUpperCase(), entry.Version));
    }

    return products;
  }

  /** Load the CASC interface with the given build. */
  async load(buildIndex: number): Promise<void> {
    this.resetForLoad();
    this.localIndexes.clear();

    this.build = this.builds[buildIndex];
    write('Loading local CASC build: %o', this.build);

    this.cache = new BuildCache(this.build.BuildKey);
    await this.cache.init();

    this.progress = createProgress(8);
    await this.loadConfigs();
    await this.loadIndexes();
    await this.loadEncoding();
    await this.loadRoot();

    await this.prepareListfile();
    await this.loadListfile(this.build.BuildKey);

    await this.initializeComponents();
    this.isLoaded = true;
  }

  /** Load the BuildConfig from the installation directory. */
  async loadConfigs(): Promise<void> {
    // Load and parse configs from disk with CDN fallback.
    await this.progress.step('Fetching build configurations');

    if (await fileExists('fakebuildconfig')) {
      this.buildConfig = parseCDNConfig(await fsp.readFile('fakebuildconfig', 'utf8'));
      write('WARNING: Using fake build config. No support given for weird stuff happening.');

      // Reconstruct version from the fake config's build name.
      // This is used for e.g. DBD version selection so needs to be correct.
      const splitName = this.buildConfig.buildName.split('patch');
      const buildNumber = splitName[0].replace('WOW-', '');
      const splitPatch = splitName[1].split('_');

      this.build.Version = `${splitPatch[0]}.${buildNumber}`;
    } else {
      this.buildConfig = await this.getConfigFileWithRemoteFallback(this.build.BuildKey);
    }

    this.cdnConfig = await this.getConfigFileWithRemoteFallback(this.build.CDNKey);
  }

  /** Get config from disk with CDN fallback. */
  async getConfigFileWithRemoteFallback(key: string): Promise<CDNConfigEntries> {
    const configPath = this.formatConfigPath(key);
    if (!await fileExists(configPath)) {
      write('Local config file %s does not exist, falling back to CDN...', key);
      if (!this.remote) await this.initializeRemoteCASC();

      const cdnHosts = await cdnResolver.getRankedHosts(runtimeState.selectedCDNRegionTag, this.remote!.serverConfig);
      return this.remote!.getCDNConfig(key, cdnHosts);
    }
    return parseCDNConfig(await fsp.readFile(configPath, 'utf8'));
  }

  /** Load and parse storage indexes from the local installation. */
  async loadIndexes(): Promise<void> {
    timeLog();
    await this.progress.step('Loading indexes');

    let indexCount = 0;

    const entries = await fsp.readdir(this.storageDir, { withFileTypes: true });
    for (const entry of entries) {
      if (entry.isFile() && entry.name.endsWith('.idx')) {
        await this.parseIndex(path.join(this.storageDir, entry.name));
        indexCount++;
      }
    }

    timeEnd('Loaded %d entries from %d journal indexes', this.localIndexes.size, indexCount);
  }

  /** Parse a local installation journal index for entries. */
  async parseIndex(file: string): Promise<void> {
    const entries = this.localIndexes;
    const index = await BufferWrapper.readFile(file);

    const headerHashSize = index.readInt32LE();
    index.move(4); // headerHash uint32
    index.move(headerHashSize); // headerHash byte[headerHashSize]

    index.seek((8 + headerHashSize + 0x0F) & 0xFFFFFFF0); // Next 0x10 boundary.

    const dataLength = index.readInt32LE();
    index.move(4);

    const nBlocks = dataLength / 18;
    for (let i = 0; i < nBlocks; i++) {
      const key = index.readHexString(9);
      if (entries.has(key)) {
        index.move(1 + 4 + 4); // idxHigh + idxLow + size
        continue;
      }

      const idxHigh = index.readUInt8();
      const idxLow = index.readInt32BE();

      entries.set(key, {
        index: (idxHigh << 2) | ((idxLow & 0xC0000000) >>> 30),
        offset: idxLow & 0x3FFFFFFF,
        size: index.readInt32LE(),
      });
    }
  }

  /** Load and parse encoding from the local installation. */
  async loadEncoding(): Promise<void> {
    // Parse encoding file.
    timeLog();
    const encKeys = this.buildConfig.encoding.split(' ');

    await this.progress.step('Loading encoding table');
    const encRaw = await this.getDataFileWithRemoteFallback(encKeys[1]);
    this.parseEncodingFile(encRaw, encKeys[1]);
    timeEnd('Parsed encoding table (%d entries)', this.encodingKeys.size);
  }

  /** Load and parse root table from local installation. */
  async loadRoot(): Promise<void> {
    // Get root key from encoding table.
    const rootKey = this.encodingKeys.get(cascKeyFromHex(this.buildConfig.root));
    if (rootKey === undefined) throw new Error('No encoding entry found for root key');

    // Parse root file.
    timeLog();
    await this.progress.step('Loading root file');
    const rootKeyHex = cascKeyToHex(rootKey);
    const root = await this.getDataFileWithRemoteFallback(rootKeyHex);
    const rootEntryCount = this.parseRootFile(root, rootKeyHex);
    timeEnd('Parsed root file (%d entries, %d types)', rootEntryCount, this.rootTypes.length);
  }

  /**
   * Initialize a remote CASC instance to download missing
   * files needed during local initialization.
   */
  async initializeRemoteCASC(): Promise<void> {
    const remote = new CASCRemote(runtimeState.selectedCDNRegionTag);
    await remote.init();

    const buildIndex = remote.builds.findIndex((build) => build.Product === this.build.Product);
    await remote.preload(buildIndex, this.cache);

    this.remote = remote;
  }

  /**
   * Obtain a data file from the local archives.
   * If not stored locally, file will be downloaded from a CDN.
   */
  async getDataFileWithRemoteFallback(key: string, forceFallback = false): Promise<BufferWrapper> {
    try {
      // If forceFallback is true, we have corrupt local data.
      if (forceFallback) throw new Error('Local data is corrupted, forceFallback set.');

      // Attempt 1: Extract from local archives.
      const local = await this.getDataFile(key);

      if (!BLTEReader.check(local)) throw new Error('Local data file is not a valid BLTE');
      validateBLTEData(local, key);

      return local;
    } catch (e) {
      const localError = (e as Error).message;

      // Attempt 2: Load from cache from previous fallback.
      write('Local data file %s could not be used (%s), falling back to cache...', key, localError);
      const cached = await this.cache.getFile(key, constants.CACHE.DIR_DATA);
      if (cached !== null) {
        try {
          validateBLTEData(cached, key);
          return cached;
        } catch (cacheError) {
          write('Cached data file %s is invalid (%s), falling back to CDN...', key, (cacheError as Error).message);
        }
      }

      // Attempt 3: Download from CDN.
      write('Local data file %s not cached or cache invalid, falling back to CDN...', key);
      if (!this.remote) await this.initializeRemoteCASC();

      const remote = this.remote!;
      const archive = remote.archives.get(key);
      let data: BufferWrapper;
      if (archive !== undefined) {
        // Archive exists for key, attempt partial remote download.
        write('Local data file %s has archive, attempt partial download...', key);
        data = await remote.getDataFilePartial(remote.formatCDNKey(archive.key), archive.offset, archive.size);
      } else {
        // No archive for this file, attempt direct download.
        write('Local data file %s has no archive, attempting direct download...', key);
        data = await remote.getDataFile(remote.formatCDNKey(key));
      }

      validateBLTEData(data, key);
      void this.cache.storeFile(key, data, constants.CACHE.DIR_DATA);
      return data;
    }
  }

  /** Obtain a data file from the local archives. */
  async getDataFile(key: string): Promise<BufferWrapper> {
    const entry = this.localIndexes.get(key.substring(0, 18));
    if (!entry) throw new Error(`Requested file does not exist in local data: ${key}`);

    const data = await readFile(this.formatDataPath(entry.index), entry.offset + 0x1E, entry.size - 0x1E);

    let isZeroed = true;
    for (let i = 0, n = data.remainingBytes; i < n; i++) {
      if (data.readUInt8() !== 0x0) {
        isZeroed = false;
        break;
      }
    }

    if (isZeroed) throw new Error(`Requested data file is empty or missing: ${key}`);

    data.seek(0);
    return data;
  }

  /**
   * Format a local path to a data archive.
   * 67 -> <install>/Data/data/data.067
   */
  formatDataPath(id: number): string {
    return path.join(this.dataDir, 'data', `data.${id.toString().padStart(3, '0')}`);
  }

  /**
   * Format a local path to an archive index from the key.
   * 0b45bd2721fd6c86dac2176cbdb7fc5b -> <install>/Data/indices/0b45bd2721fd6c86dac2176cbdb7fc5b.index
   */
  formatIndexPath(key: string): string {
    return path.join(this.dataDir, 'indices', `${key}.index`);
  }

  /**
   * Format a local path to a config file from the key.
   * 0af716e8eca5aeff0a3965d37e934ffa -> <install>/Data/config/0a/f7/0af716e8eca5aeff0a3965d37e934ffa
   */
  formatConfigPath(key: string): string {
    return path.join(this.dataDir, 'config', this.formatCDNKey(key));
  }

  /**
   * Format a CDN key for use in local file reading.
   * Path separators used by this method are platform specific.
   * 49299eae4e3a195953764bb4adb3c91f -> 49\29\49299eae4e3a195953764bb4adb3c91f
   */
  formatCDNKey(key: string): string {
    return path.join(key.substring(0, 2), key.substring(2, 4), key);
  }

  /** Get the current build ID. */
  getBuildName(): string {
    return this.build.Version;
  }

  /** Returns the build configuration key. */
  getBuildKey(): string {
    return this.build.BuildKey;
  }
}

function validateBLTEData(data: BufferWrapper, key: string): void {
  new BLTEReader(data, key);
  data.seek(0);
}

export default CASCLocal;
