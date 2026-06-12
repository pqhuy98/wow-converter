/**
 * Base CASC source, ported from wow.export (src/js/casc/casc-source.js).
 * UI couplings (Vue watches, listfile tab filters) are replaced by wowConfig
 * and the runtime load-func registry.
 */
import { write } from '@/lib/wow/log';
import { createProgress, Progress, runLoadFuncs } from '@/lib/wow/server/runtime';

import { BufferWrapper } from '../../formats/buffer';
import { wowConfig } from '../../server/config';
import { BLTEReader } from './blte-reader';
import { BuildCache } from './build-cache';
import {
  asCascKey, CascKey, cascKeyFromHex, cascKeyToHex,
} from './casc-key';
import { CDNConfigEntries } from './cdn-config';
import { ContentFlags } from './content-flags';
import { InstallManifest } from './install-manifest';
import * as listfile from './listfile';
import { LocaleFlags } from './locale-flags';
import { VersionConfigEntry } from './version-config';

const ENC_MAGIC = 0x4E45;
const ROOT_MAGIC = 0x4D465354;

export interface RootType {
  contentFlags: number;
  localeFlags: number;
}

/** Map of rootTypeIdx -> content key (compact binary). */
export type RootEntry = Map<number, CascKey>;

export abstract class CASC {
  encodingSizes = new Map<CascKey, number>();

  encodingKeys = new Map<CascKey, CascKey>();

  rootTypes: RootType[] = [];

  rootEntries = new Map<number, RootEntry>();

  isRemote: boolean;

  isLoaded = false;

  locale: number;

  progress: Progress = createProgress();

  buildConfig!: CDNConfigEntries;

  cdnConfig!: CDNConfigEntries;

  builds: VersionConfigEntry[] = [];

  build!: VersionConfigEntry;

  abstract cache: BuildCache;

  constructor(isRemote = false) {
    this.isRemote = isRemote;

    const locale = wowConfig.cascLocale;
    if (!Number.isNaN(locale)) {
      this.locale = locale;
    } else {
      write('Invalid locale set in configuration, defaulting to enUS');
      this.locale = LocaleFlags.enUS;
    }
  }

  abstract getFile(
    fileDataID: number,
    partialDecrypt?: boolean,
    suppressLog?: boolean,
    supportFallback?: boolean,
    forceFallback?: boolean,
    contentKey?: CascKey | null,
  ): Promise<BLTEReader>;

  abstract getDataFile(key: string): Promise<BufferWrapper>;

  abstract formatCDNKey(key: string): string;

  abstract getBuildName(): string;

  abstract getBuildKey(): string;

  abstract getProductList(): string[];

  abstract load(buildIndex: number): Promise<void>;

  /** Clear parsed index state before (re)loading a build. */
  resetForLoad(): void {
    this.encodingSizes.clear();
    this.encodingKeys.clear();
    this.rootTypes.length = 0;
    this.rootEntries.clear();
    this.isLoaded = false;
  }

  /** Provides an array of fileDataIDs that match the current locale. */
  getValidRootEntries(): number[] {
    const entries: number[] = [];

    for (const [fileDataID, entry] of this.rootEntries.entries()) {
      let include = false;

      for (const rootTypeIdx of entry.keys()) {
        const rootType = this.rootTypes[rootTypeIdx];
        if ((rootType.localeFlags & this.locale) && ((rootType.contentFlags & ContentFlags.LowViolence) === 0)) {
          include = true;
          break;
        }
      }

      if (include) entries.push(fileDataID);
    }

    return entries;
  }

  /** Retrieves the install manifest for this CASC instance. */
  async getInstallManifest(): Promise<InstallManifest> {
    const installKeys = this.buildConfig.install.split(' ');
    const installKey = installKeys.length === 1
      ? this.encodingKeys.get(cascKeyFromHex(installKeys[0]))!
      : cascKeyFromHex(installKeys[1]);

    const installKeyHex = cascKeyToHex(installKey);
    const raw = this.isRemote
      ? await this.getDataFile(this.formatCDNKey(installKeyHex))
      : await (this as unknown as { getDataFileWithRemoteFallback(key: string): Promise<BufferWrapper> }).getDataFileWithRemoteFallback(installKeyHex);
    const manifest = new BLTEReader(raw, installKeyHex);

    return new InstallManifest(manifest);
  }

  /** Check if a file exists by its fileDataID. */
  fileExists(fileDataID: number): boolean {
    const root = this.rootEntries.get(fileDataID);
    if (root === undefined) return false;

    for (const [rootTypeIdx] of root.entries()) {
      const rootType = this.rootTypes[rootTypeIdx];
      if ((rootType.localeFlags & this.locale) && ((rootType.contentFlags & ContentFlags.LowViolence) === 0)) return true;
    }

    return false;
  }

  /**
   * Obtain the encoding key for a file by its fileDataID.
   * (Underlying implementation of getFile; CASCLocal and CASCRemote implement readers.)
   */
  protected getEncodingKey(fileDataID: number): CascKey {
    const root = this.rootEntries.get(fileDataID);
    if (root === undefined) throw new Error(`fileDataID does not exist in root: ${fileDataID}`);

    let contentKey: CascKey | null = null;
    for (const [rootTypeIdx, key] of root.entries()) {
      const rootType = this.rootTypes[rootTypeIdx];

      // Select the first root entry that has a matching locale and no LowViolence flag set.
      if ((rootType.localeFlags & this.locale) && ((rootType.contentFlags & ContentFlags.LowViolence) === 0)) {
        contentKey = key;
        break;
      }
    }

    if (contentKey === null) throw new Error(`No root entry found for locale: ${this.locale}`);

    return this.getEncodingKeyForContentKey(contentKey);
  }

  getEncodingKeyForContentKey(contentKey: CascKey): CascKey {
    const key = asCascKey(contentKey);
    const encodingKey = this.encodingKeys.get(key);
    if (encodingKey === undefined) throw new Error(`No encoding entry found: ${cascKeyToHex(key)}`);

    return encodingKey;
  }

  /**
   * Obtain a file by a filename.
   * fileName must exist in the loaded listfile.
   */
  async getFileByName(
    fileName: string,
    partialDecrypt = false,
    suppressLog = false,
    supportFallback = true,
    forceFallback = false,
  ): Promise<BLTEReader> {
    let fileDataID: number | undefined;

    // If filename is "unknown/<fdid>", skip listfile lookup
    if (fileName.startsWith('unknown/') && !fileName.includes('.')) fileDataID = parseInt(fileName.split('/')[1], 10);
    else fileDataID = listfile.getByFilename(fileName);

    if (fileDataID === undefined) throw new Error(`File not mapping in listfile: ${fileName}`);

    return this.getFile(fileDataID, partialDecrypt, suppressLog, supportFallback, forceFallback);
  }

  /**
   * Prepare listfile data before loading.
   * Ensures preloading is complete to avoid race conditions.
   */
  async prepareListfile(): Promise<void> {
    await this.progress.step('Preparing listfiles...');
    await listfile.prepareListfile();
  }

  /** Load the listfile for selected build. */
  async loadListfile(_buildKey?: string): Promise<void> {
    await this.progress.step('Loading listfiles');
    listfile.applyPreload(this.rootEntries);
  }

  /**
   * Initialize external components as part of the CASC load process
   * (DB caches and other registered load functions).
   */
  async initializeComponents(): Promise<void> {
    await this.progress.step('Initializing components');
    await runLoadFuncs();
  }

  /** Parse entries from a root file. Returns the entry count. */
  parseRootFile(data: BufferWrapper, hash: string): number {
    const root = new BLTEReader(data, hash);

    const magic = root.readUInt32LE();
    const rootTypes = this.rootTypes;
    const rootEntries = this.rootEntries;

    if (magic === ROOT_MAGIC) { // 8.2
      let headerSize = root.readUInt32LE();
      let version = root.readUInt32LE();

      if (headerSize !== 0x18) {
        version = 0; // This will break with future header size increases.
      } else if (version !== 1 && version !== 2) {
        throw new Error(`Unknown root version: ${version}`);
      }

      let totalFileCount: number;
      let namedFileCount: number;

      if (version === 0) {
        totalFileCount = headerSize;
        namedFileCount = version;
        headerSize = 12;
      } else {
        totalFileCount = root.readUInt32LE();
        namedFileCount = root.readUInt32LE();
      }

      root.seek(headerSize);

      const allowNamelessFiles = totalFileCount !== namedFileCount;

      while (root.remainingBytes > 0) {
        const numRecords = root.readUInt32LE();

        let contentFlags = 0;
        let localeFlags = 0;

        if (version === 0 || version === 1) {
          contentFlags = root.readUInt32LE();
          localeFlags = root.readUInt32LE();
        } else if (version === 2) {
          localeFlags = root.readUInt32LE();
          const cflags1 = root.readUInt32LE();
          const cflags2 = root.readUInt32LE();
          const cflags3 = root.readUInt8();
          contentFlags = cflags1 | cflags2 | (cflags3 << 17);
        }

        const fileDataIDs = new Array<number>(numRecords);

        let fileDataID = 0;
        for (let i = 0; i < numRecords; i++) {
          const nextID = fileDataID + root.readInt32LE();
          fileDataIDs[i] = nextID;
          fileDataID = nextID + 1;
        }

        // Parse MD5 content keys for entries.
        for (let i = 0; i < numRecords; i++) {
          const fdid = fileDataIDs[i];
          let entry = rootEntries.get(fdid);

          if (!entry) {
            entry = new Map();
            rootEntries.set(fdid, entry);
          }

          entry.set(rootTypes.length, root.readBinaryKey(16));
        }

        // Skip lookup hashes for entries.
        if (!(allowNamelessFiles && (contentFlags & ContentFlags.NoNameHash))) root.move(8 * numRecords);

        // Push the rootType after parsing the block so that
        // rootTypes.length can be used for the type index above.
        rootTypes.push({ contentFlags, localeFlags });
      }
    } else { // Classic
      root.seek(0);
      while (root.remainingBytes > 0) {
        const numRecords = root.readUInt32LE();

        const contentFlags = root.readUInt32LE();
        const localeFlags = root.readUInt32LE();

        const fileDataIDs = new Array<number>(numRecords);

        let fileDataID = 0;
        for (let i = 0; i < numRecords; i++) {
          const nextID = fileDataID + root.readInt32LE();
          fileDataIDs[i] = nextID;
          fileDataID = nextID + 1;
        }

        // Parse MD5 content keys for entries.
        for (let i = 0; i < numRecords; i++) {
          const key = root.readBinaryKey(16);
          root.move(8); // hash

          const fdid = fileDataIDs[i];
          let entry = rootEntries.get(fdid);

          if (!entry) {
            entry = new Map();
            rootEntries.set(fdid, entry);
          }

          entry.set(rootTypes.length, key);
        }

        // Push the rootType after parsing the block so that
        // rootTypes.length can be used for the type index above.
        rootTypes.push({ contentFlags, localeFlags });
      }
    }

    return rootEntries.size;
  }

  /** Parse entries from an encoding file. */
  parseEncodingFile(data: BufferWrapper, hash: string): void {
    const encodingSizes = this.encodingSizes;
    const encodingKeys = this.encodingKeys;

    const encoding = new BLTEReader(data, hash);

    const magic = encoding.readUInt16LE();
    if (magic !== ENC_MAGIC) throw new Error(`Invalid encoding magic: ${magic}`);

    encoding.move(1); // version
    const hashSizeCKey = encoding.readUInt8();
    const hashSizeEKey = encoding.readUInt8();
    const cKeyPageSize = encoding.readInt16BE() * 1024;
    encoding.move(2); // eKeyPageSize
    const cKeyPageCount = encoding.readInt32BE();
    encoding.move(4 + 1); // eKeyPageCount + unk11
    const specBlockSize = encoding.readInt32BE();

    encoding.move(specBlockSize + (cKeyPageCount * (hashSizeCKey + 16)));

    const pagesStart = encoding.offset;
    for (let i = 0; i < cKeyPageCount; i++) {
      const pageStart = pagesStart + (cKeyPageSize * i);
      encoding.seek(pageStart);

      while (encoding.offset < (pageStart + pagesStart)) {
        const keysCount = encoding.readUInt8();
        if (keysCount === 0) break;

        const size = encoding.readInt40BE();
        const cKey = encoding.readBinaryKey(hashSizeCKey);

        encodingSizes.set(cKey, size);
        encodingKeys.set(cKey, encoding.readBinaryKey(hashSizeEKey));

        encoding.move(hashSizeEKey * (keysCount - 1));
      }
    }
  }

  /**
   * Run any necessary clean-up once a CASC instance is no longer needed.
   */
  cleanup(): void {
    // No config watches in the headless port.
  }
}

export default CASC;
