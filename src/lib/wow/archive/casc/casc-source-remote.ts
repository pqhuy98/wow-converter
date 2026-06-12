/**
 * Remote (CDN) CASC source, ported from wow.export (src/js/casc/casc-source-remote.js).
 */
import util from 'util';

import { timeEnd, timeLog, write } from '@/lib/wow/log';
import { createProgress } from '@/lib/wow/server/runtime';

import { BufferWrapper } from '../../formats/buffer';
import { constants } from '../../formats/constants';
import {
  downloadFile, filesize, get, queue,
} from '../../formats/generics';
import { BLTEReader } from './blte-reader';
import { BuildCache } from './build-cache';
import type { CascKey } from './casc-key';
import { cascKeyFromHex, cascKeyToHex } from './casc-key';
import { CASC } from './casc-source';
import { CDNConfigEntries, parseCDNConfig } from './cdn-config';
import { cdnResolver } from './cdn-resolver';
import * as listfile from './listfile';
import { parseVersionConfig, VersionConfigEntry } from './version-config';

const EMPTY_HASH = '00000000000000000000000000000000';

interface ArchiveEntry {
  key: string;
  size: number;
  offset: number;
}

export class CASCRemote extends CASC {
  archives = new Map<string, ArchiveEntry>();

  region: string;

  host!: string;

  cache!: BuildCache;

  serverConfig!: VersionConfigEntry;

  /**
   * Create a new CASC source using a Blizzard CDN.
   * @param region Region tag (eu, us, etc).
   */
  constructor(region: string) {
    super(true);

    this.region = region;
  }

  /** Initialize remote CASC source. */
  async init(): Promise<void> {
    write('Initializing remote CASC source (%s)', this.region);
    this.host = util.format(constants.PATCH.HOST, this.region);
    this.builds = [];

    // Collect version configs for all products.
    const promises = constants.PRODUCTS.map((p) => this.getVersionConfig(p.product));
    const results = await Promise.allSettled(promises);

    // Iterate through successful requests and extract product config for our region.
    for (const result of results) {
      if (result.status === 'fulfilled') {
        const entry = result.value.find((e) => e.Region === this.region);
        if (entry) this.builds.push(entry);
      }
    }

    write('%o', this.builds);
  }

  /** Download the remote version config for a specific product. */
  async getVersionConfig(product: string): Promise<VersionConfigEntry[]> {
    const config = await this.getConfig(product, constants.PATCH.VERSION_CONFIG);
    config.forEach((entry) => { entry.Product = product; });
    return config;
  }

  /** Download and parse a version config file. */
  async getConfig(product: string, file: string): Promise<VersionConfigEntry[]> {
    const url = this.host + product + file;
    const res = await get(url);

    if (!res.ok) throw new Error(util.format('HTTP %d from remote CASC endpoint: %s', res.status, url));

    return parseVersionConfig(await res.text());
  }

  /**
   * Download and parse a CDN config file.
   * Attempts multiple CDN hosts in order of ping speed if one fails.
   * @param key
   * @param cdnHosts Optional array of CDN hosts to try (in priority order)
   */
  async getCDNConfig(key: string, cdnHosts: string[] | null = null): Promise<CDNConfigEntries> {
    // If no hosts provided, use the current host
    const hostsToTry = cdnHosts || [this.host];

    let lastError: Error | null = null;
    for (const host of hostsToTry) {
      try {
        const url = `${host}config/${this.formatCDNKey(key)}`;
        write('Attempting to retrieve CDN config from: %s', url);
        const res = await get(url);

        if (!res.ok) throw new Error(util.format('HTTP %d from CDN config endpoint', res.status));

        const configText = await res.text();
        const config = parseCDNConfig(configText);

        if (host !== this.host) {
          write('Successfully retrieved config from fallback host: %s', host);
          this.host = host;
        }

        return config;
      } catch (error) {
        write('Failed to retrieve CDN config from %s: %s', host, (error as Error).message);
        lastError = error as Error;

        cdnResolver.markHostFailed(host);
        continue;
      }
    }

    throw new Error(util.format('Unable to retrieve CDN config file %s from any CDN host. Last error: %s', key, lastError?.message || 'Unknown error'));
  }

  /** Obtain a file by its fileDataID. */
  async getFile(
    fileDataID: number,
    partialDecrypt = false,
    suppressLog = false,
    _supportFallback = true,
    _forceFallback = false,
    contentKey: CascKey | null = null,
  ): Promise<BLTEReader> {
    if (!suppressLog) write('Loading remote CASC file %d (%s)', fileDataID, listfile.getByID(fileDataID));

    const encodingKey = contentKey !== null ? this.getEncodingKeyForContentKey(contentKey) : this.getEncodingKey(fileDataID);
    const encodingKeyHex = cascKeyToHex(encodingKey);
    let data = await this.cache.getFile(encodingKeyHex, constants.CACHE.DIR_DATA);

    if (data === null) {
      const archive = this.archives.get(encodingKeyHex);
      if (archive !== undefined) {
        data = await this.getDataFilePartial(this.formatCDNKey(archive.key), archive.offset, archive.size);

        if (!suppressLog) write('Downloading CASC file %d from archive %s', fileDataID, archive.key);
      } else {
        data = await this.getDataFile(this.formatCDNKey(encodingKeyHex));

        if (!suppressLog) write('Downloading unarchived CASC file %d', fileDataID);

        if (data === null) throw new Error(`No remote unarchived/archive indexed for encoding key: ${encodingKeyHex}`);
      }

      void this.cache.storeFile(encodingKeyHex, data, constants.CACHE.DIR_DATA);
    } else if (!suppressLog) {
      write('Loaded CASC file %d from cache', fileDataID);
    }

    return new BLTEReader(data, encodingKeyHex, partialDecrypt);
  }

  /**
   * Returns a list of available products on the remote CDN.
   * Format example: "PTR: World of Warcraft 8.3.0.32272"
   */
  getProductList(): string[] {
    const products: string[] = [];
    for (const entry of this.builds) {
      // This check exists because some regions (e.g. China) may not have all products.
      if (entry === undefined) continue;

      const product = constants.PRODUCTS.find((e) => e.product === entry.Product)!;
      products.push(util.format('%s %s', product.title, entry.VersionsName));
    }

    return products;
  }

  /**
   * Preload requirements for reading remote files without initializing the
   * entire instance. Used by local CASC install for CDN fallback.
   */
  async preload(buildIndex: number, cache: BuildCache | null = null): Promise<void> {
    this.build = this.builds[buildIndex];
    write('Preloading remote CASC build: %o', this.build);

    if (cache) {
      this.cache = cache;
    } else {
      this.cache = new BuildCache(this.build.BuildConfig);
      await this.cache.init();
    }

    await this.loadServerConfig();
    await this.resolveCDNHost();
    await this.loadConfigs();
    await this.loadArchives();
  }

  /** Load the CASC interface with the given build. */
  async load(buildIndex: number): Promise<void> {
    this.resetForLoad();
    this.progress = createProgress(12);
    await this.preload(buildIndex);

    await this.loadEncoding();
    await this.loadRoot();

    await this.prepareListfile();
    await this.loadListfile(this.build.BuildConfig);

    await this.initializeComponents();
    this.isLoaded = true;
  }

  /** Download and parse the encoding file. */
  async loadEncoding(): Promise<void> {
    const encKeys = this.buildConfig.encoding.split(' ');
    const encKey = encKeys[1];

    timeLog();

    await this.progress.step('Loading encoding table');
    let encRaw = await this.cache.getFile(constants.CACHE.BUILD_ENCODING);
    if (encRaw === null) {
      // Encoding file not cached, download it.
      write('Encoding for build %s not cached, downloading.', this.cache.key);
      encRaw = await this.getDataFile(this.formatCDNKey(encKey));

      // Store back into cache (no need to block).
      void this.cache.storeFile(constants.CACHE.BUILD_ENCODING, encRaw);
    } else {
      write('Encoding for build %s cached locally.', this.cache.key);
    }

    timeEnd('Loaded encoding table (%s)', filesize(encRaw.byteLength));

    // Parse encoding file.
    timeLog();
    await this.progress.step('Parsing encoding table');
    this.parseEncodingFile(encRaw, encKey);
    timeEnd('Parsed encoding table (%d entries)', this.encodingKeys.size);
  }

  /** Download and parse the root file. */
  async loadRoot(): Promise<void> {
    // Get root key from encoding table.
    const rootKey = this.encodingKeys.get(cascKeyFromHex(this.buildConfig.root));
    if (rootKey === undefined) throw new Error('No encoding entry found for root key');

    timeLog();
    await this.progress.step('Loading root table');

    const rootKeyHex = cascKeyToHex(rootKey);
    let root = await this.cache.getFile(constants.CACHE.BUILD_ROOT);
    if (root === null) {
      // Root file not cached, download.
      write('Root file for build %s not cached, downloading.', this.cache.key);

      root = await this.getDataFile(this.formatCDNKey(rootKeyHex));
      void this.cache.storeFile(constants.CACHE.BUILD_ROOT, root);
    }

    timeEnd('Loaded root file (%s)', filesize(root.byteLength));

    // Parse root file.
    timeLog();
    await this.progress.step('Parsing root file');
    const rootEntryCount = this.parseRootFile(root, rootKeyHex);
    timeEnd('Parsed root file (%d entries, %d types)', rootEntryCount, this.rootTypes.length);
  }

  /** Download and parse archive files. */
  async loadArchives(): Promise<void> {
    // Download archive indexes.
    const archiveKeys = this.cdnConfig.archives.split(' ');
    const archiveCount = archiveKeys.length;

    timeLog();

    if (this.progress) await this.progress.step('Loading archives');

    await queue(archiveKeys, async (key: string) => this.parseArchiveIndex(key), 50);

    // Quick and dirty way to get the total archive size using config.
    const archiveTotalSize = this.cdnConfig.archivesIndexSize.split(' ').reduce((x, e) => Number(x) + Number(e), 0);
    timeEnd('Loaded %d archives (%d entries, %s)', archiveCount, this.archives.size, filesize(archiveTotalSize));
  }

  /**
   * Download the CDN configuration and store the entry for our selected region.
   */
  async loadServerConfig(): Promise<void> {
    if (this.progress) await this.progress.step('Fetching CDN configuration');

    // Download CDN server list.
    const serverConfigs = await this.getConfig(this.build.Product, constants.PATCH.SERVER_CONFIG);
    write('%o', serverConfigs);

    // Locate the CDN entry for our selected region.
    const serverConfig = serverConfigs.find((e) => e.Name === this.region);
    if (!serverConfig) throw new Error(`CDN config does not contain entry for region ${this.region}`);
    this.serverConfig = serverConfig;
  }

  /**
   * Load and parse the contents of an archive index.
   * Will use global cache and download if missing.
   */
  async parseArchiveIndex(key: string): Promise<void> {
    const fileName = `${key}.index`;

    let data = await this.cache.getFile(fileName, constants.CACHE.DIR_INDEXES);
    if (data === null) {
      const cdnKey = `${this.formatCDNKey(key)}.index`;
      data = await this.getDataFile(cdnKey);
      void this.cache.storeFile(fileName, data, constants.CACHE.DIR_INDEXES);
    }

    // Skip to the end of the archive to find the count.
    data.seek(-12);
    const count = data.readInt32LE();

    if (count * 24 > data.byteLength) throw new Error(`Unable to parse archive, unexpected size: ${data.byteLength}`);

    data.seek(0); // Reset position.

    for (let i = 0; i < count; i++) {
      let hash = data.readHexString(16);

      // Skip zero hashes.
      if (hash === EMPTY_HASH) hash = data.readHexString(16);

      this.archives.set(hash, { key, size: data.readInt32BE(), offset: data.readInt32BE() });
    }
  }

  /** Download a data file from the CDN. */
  async getDataFile(file: string): Promise<BufferWrapper> {
    return downloadFile(`${this.host}data/${file}`);
  }

  /** Download a partial chunk of a data file from the CDN. */
  async getDataFilePartial(file: string, ofs: number, len: number): Promise<BufferWrapper> {
    return downloadFile(`${this.host}data/${file}`, undefined, ofs, len);
  }

  /** Download the CDNConfig and BuildConfig. */
  async loadConfigs(): Promise<void> {
    if (this.progress) await this.progress.step('Fetching build configurations');

    const cdnHosts = await cdnResolver.getRankedHosts(this.region, this.serverConfig);

    this.cdnConfig = await this.getCDNConfig(this.build.CDNConfig, cdnHosts);
    this.buildConfig = await this.getCDNConfig(this.build.BuildConfig, cdnHosts);
  }

  /** Resolve the fastest CDN host for this region and server configuration. */
  async resolveCDNHost(): Promise<void> {
    if (this.progress) await this.progress.step('Locating fastest CDN server');

    this.host = await cdnResolver.getBestHost(this.region, this.serverConfig);
  }

  /**
   * Format a CDN key for use in CDN requests.
   * 49299eae4e3a195953764bb4adb3c91f -> 49/29/49299eae4e3a195953764bb4adb3c91f
   */
  formatCDNKey(key: string): string {
    return `${key.substring(0, 2)}/${key.substring(2, 4)}/${key}`;
  }

  /** Get the current build ID. */
  getBuildName(): string {
    return this.build.VersionsName;
  }

  /** Returns the build configuration key. */
  getBuildKey(): string {
    return this.build.BuildConfig;
  }
}

export default CASCRemote;
