/**
 * Master listfile management, ported from wow.export (src/js/casc/listfile.js).
 * UI tab pre-filtering (textures/sounds/videos/text/models arrays) is omitted;
 * unknown-file providers are injected by the DB cache modules (Phase 4).
 */
import { promises as fsp } from 'fs';
import path from 'path';

import { write } from '@/lib/wow/log';

import { BufferWrapper } from '../../formats/buffer';
import { constants } from '../../formats/constants';
import { batchWork, downloadFile } from '../../formats/generics';
import { wowConfig } from '../../server/config';

const nameLookup = new Map<string, number>();
const idLookup = new Map<number, string>();

let loaded = false;

const preloadedIdLookup = new Map<number, string>();
const preloadedNameLookup = new Map<string, number>();
let isPreloaded = false;
let preloadPromise: Promise<boolean> | null = null;

/** Replace the extension of a file path (replaceExtension equivalent). */
function replaceExtension(file: string, ext = ''): string {
  return path.join(path.dirname(file), path.basename(file, path.extname(file)) + ext);
}

/** Internal implementation of preload logic. */
async function _doPreload(): Promise<boolean> {
  try {
    write('Preloading master listfile...');

    const url = String(wowConfig.listfileURL);
    if (typeof url !== 'string') throw new Error('Missing/malformed listfileURL in configuration!');

    // Ensure listfile cache directory exists
    await fsp.mkdir(constants.CACHE.DIR_LISTFILE, { recursive: true });

    const cacheFile = path.join(constants.CACHE.DIR_LISTFILE, constants.CACHE.LISTFILE_DATA);

    preloadedIdLookup.clear();
    preloadedNameLookup.clear();
    isPreloaded = false;

    let data: BufferWrapper;
    if (url.startsWith('http')) {
      // Listfile URL is http, check for cache/updates
      let requireDownload = false;
      let cached: BufferWrapper | null = null;
      let lastModified = 0;

      // Try to load existing cached data and get its modification time
      try {
        cached = await BufferWrapper.readFile(cacheFile);
        const stats = await fsp.stat(cacheFile);
        lastModified = stats.mtime.getTime();
      } catch (e) {
        // No cached file
      }

      if (lastModified > 0) {
        let ttl = Number(wowConfig.listfileCacheRefresh) || 0;
        ttl *= 24 * 60 * 60 * 1000; // Reduce from days to milliseconds.

        if (ttl === 0 || (Date.now() - lastModified) > ttl) {
          // Local cache file needs updating.
          write('Cached listfile is out-of-date (> %d).', ttl);
          requireDownload = true;
        } else if (cached === null) {
          // Ensure that the local cache file *actually* exists before relying on it.
          write('Listfile is missing despite file stats. User tamper?');
          requireDownload = true;
        } else {
          write('Listfile is cached locally.');
        }
      } else {
        // This listfile has never been cached.
        requireDownload = true;
        write('Listfile is not cached, downloading fresh.');
      }

      if (requireDownload) {
        try {
          let fallbackUrl = String(wowConfig.listfileFallbackURL);
          // Remove %s placeholder since we don't use buildConfig for master listfile
          fallbackUrl = fallbackUrl.replace('%s', '');

          data = await downloadFile([url, fallbackUrl]);

          // Store the downloaded data (file modification time will be set automatically)
          await fsp.writeFile(cacheFile, data.raw);
        } catch (e) {
          if (cached === null) {
            write('Failed to download listfile during preload, no cached version for fallback: %s', (e as Error).message);
            return false;
          }

          write('Failed to download listfile during preload, using cached version: %s', (e as Error).message);
          data = cached;
        }
      } else {
        data = cached!;
      }
    } else {
      // User has configured a local listfile location
      write('Preloading user-defined local listfile: %s', url);
      data = await BufferWrapper.readFile(url);
    }

    const lines = data.readLines();
    write('Processing %d listfile lines in chunks...', lines.length);

    await batchWork('listfile parsing', lines, (line) => {
      if (line.length === 0) return;

      const tokens = line.split(';');
      if (tokens.length !== 2) {
        write('Invalid listfile line (token count): %s', line);
        return;
      }

      const fileDataID = Number(tokens[0]);
      if (Number.isNaN(fileDataID)) {
        write('Invalid listfile line (non-numerical ID): %s', line);
        return;
      }

      const fileName = tokens[1].toLowerCase();
      preloadedIdLookup.set(fileDataID, fileName);
      preloadedNameLookup.set(fileName, fileDataID);
    }, 50000);

    if (preloadedIdLookup.size === 0) {
      write('No entries found in preloaded listfile');
      return false;
    }

    isPreloaded = true;
    write('Preloaded %d listfile entries', preloadedIdLookup.size);
    return true;
  } catch (e) {
    write('Error during listfile preload: %s', (e as Error).message);
    isPreloaded = false;
    return false;
  }
}

/**
 * Preload the master listfile.
 * Multiple calls to this function will return the same promise.
 */
export async function preload(): Promise<boolean> {
  if (preloadPromise) return preloadPromise;
  if (isPreloaded) return true;

  preloadPromise = _doPreload();
  return preloadPromise;
}

/**
 * Ensure listfile is preloaded and ready for use.
 * This should be called during the loading process before accessing listfile data.
 */
export async function prepareListfile(): Promise<boolean> {
  if (isPreloaded) return true;

  if (preloadPromise) {
    write('Waiting for listfile preload to complete...');
    return preloadPromise;
  }

  write('Starting listfile preload...');
  return preload();
}

/**
 * Apply preloaded listfile data filtered by rootEntries.
 * @returns Number of entries applied, or 0 if preload not available/failed.
 */
export function applyPreload(rootEntries: Map<number, unknown>): number {
  if (!isPreloaded) {
    write('No preloaded listfile available, falling back to normal loading');
    return 0;
  }

  try {
    write('Applying preloaded listfile data...');

    // Clear current data
    idLookup.clear();
    nameLookup.clear();

    // Apply preloaded entries filtered by rootEntries
    let appliedCount = 0;
    for (const [fileDataID, fileName] of preloadedIdLookup.entries()) {
      if (rootEntries.has(fileDataID)) {
        idLookup.set(fileDataID, fileName);
        nameLookup.set(fileName, fileDataID);
        appliedCount++;
      }
    }

    if (appliedCount === 0) {
      write('No preloaded entries matched rootEntries');
      return 0;
    }

    loaded = true;
    write('Applied %d preloaded listfile entries', appliedCount);

    // Active idLookup/nameLookup now hold the filtered subset; drop the full preload
    // (~2M entries) to reclaim RAM. A later CASC build switch re-parses from disk cache.
    preloadedIdLookup.clear();
    preloadedNameLookup.clear();
    isPreloaded = false;
    preloadPromise = null;

    return appliedCount;
  } catch (e) {
    write('Error applying preloaded listfile: %s', (e as Error).message);
    return 0;
  }
}

/** Providers of unknown fileDataIDs, registered by DB cache modules. */
type UnknownIdProvider = () => Iterable<number> | Promise<Iterable<number>>;
let unknownModelProvider: UnknownIdProvider | null = null;
let unknownTextureProvider: UnknownIdProvider | null = null;

export function setUnknownModelProvider(provider: UnknownIdProvider): void {
  unknownModelProvider = provider;
}

export function setUnknownTextureProvider(provider: UnknownIdProvider): void {
  unknownTextureProvider = provider;
}

/** Load unknown texture files from TextureFileData. */
export async function loadUnknownTextures(): Promise<number> {
  if (!unknownTextureProvider) return 0;
  const unkBlp = loadIDTable(await unknownTextureProvider(), '.blp');
  write('Added %d unknown BLP textures from TextureFileData to listfile', unkBlp);
  return unkBlp;
}

/** Load unknown model files from ModelFileData. */
export async function loadUnknownModels(): Promise<number> {
  if (!unknownModelProvider) return 0;
  const unkM2 = loadIDTable(await unknownModelProvider(), '.m2');
  write('Added %d unknown M2 models from ModelFileData to listfile', unkM2);
  return unkM2;
}

/**
 * Load unknown files from ModelFileData only.
 * TextureFileData unknown files are loaded separately.
 */
export async function loadUnknowns(): Promise<void> {
  await loadUnknownModels();
}

/** Load file IDs from a data table. */
export function loadIDTable(ids: Iterable<number>, ext: string): number {
  let loadCount = 0;

  for (const fileDataID of ids) {
    if (!idLookup.has(fileDataID)) {
      const fileName = `unknown/${fileDataID}${ext}`;
      idLookup.set(fileDataID, fileName);
      nameLookup.set(fileName, fileDataID);
      loadCount++;
    }
  }

  return loadCount;
}

export type ExtensionFilter = string | [string, RegExp];

/** Return an array of filenames ending with the given extension(s). */
export function getFilenamesByExtension(exts: ExtensionFilter | ExtensionFilter[]): string[] {
  // Box into an array for reduced code.
  let filters = exts;
  if (!Array.isArray(exts) || (exts.length === 2 && typeof exts[0] === 'string' && exts[1] instanceof RegExp)) {
    filters = [exts as ExtensionFilter];
  }

  const entries: number[] = [];

  for (const [fileDataID, filename] of idLookup.entries()) {
    for (const ext of filters as ExtensionFilter[]) {
      if (Array.isArray(ext)) {
        if (filename.endsWith(ext[0]) && !filename.match(ext[1])) {
          entries.push(fileDataID);
          continue;
        }
      } else if (filename.endsWith(ext)) {
        entries.push(fileDataID);
        continue;
      }
    }
  }

  return formatEntries(entries);
}

/** Sort and format listfile entries for file list display. */
export function formatEntries(entries: number[]): string[] {
  // If sorting by ID, perform the sort while the array is only IDs.
  if (wowConfig.listfileSortByID) entries.sort((a, b) => a - b);

  let formatted: string[];
  if (wowConfig.listfileShowFileDataIDs) formatted = entries.map((e) => `${getByIDOrUnknown(e)} [${e}]`);
  else formatted = entries.map((e) => getByIDOrUnknown(e));

  // If sorting by name, sort now that the filenames have been added.
  if (!wowConfig.listfileSortByID) formatted.sort();

  return formatted;
}

export function ingestIdentifiedFiles(entries: Map<number, string>): void {
  for (const [fileDataID, ext] of entries) {
    const fileName = `unknown/${fileDataID}${ext}`;
    idLookup.set(fileDataID, fileName);
    nameLookup.set(fileName, fileDataID);
  }
}

/** Returns a full listfile, sorted and formatted. */
export function getFullListfile(): string[] {
  return formatEntries([...idLookup.keys()]);
}

/** Get a filename from a given file data ID. */
export function getByID(id: number): string | undefined {
  return idLookup.get(id);
}

/** Get a filename from a given file data ID or format it as an unknown file. */
export function getByIDOrUnknown(id: number, ext = ''): string {
  return idLookup.get(id) ?? formatUnknownFile(id, ext);
}

/** Get a file data ID by a given file name. */
export function getByFilename(filename: string): number | undefined {
  let lookup = nameLookup.get(filename.toLowerCase().replace(/\\/g, '/'));

  // In the rare occasion we have a reference to an MDL/MDX file and it fails
  // to resolve (as expected), attempt to resolve the M2 of the same name.
  if (!lookup && (filename.endsWith('.mdl') || filename.endsWith('mdx'))) {
    lookup = nameLookup.get(replaceExtension(filename, '.m2').replace(/\\/g, '/'));
  }

  return lookup;
}

export interface ListfileEntry {
  fileDataID: number;
  fileName: string;
}

/** Returns an array of listfile entries filtered by the given search term. */
export function getFilteredEntries(search: string | RegExp): ListfileEntry[] {
  const results: ListfileEntry[] = [];
  const isRegExp = search instanceof RegExp;

  for (const [fileDataID, fileName] of idLookup.entries()) {
    if (search === '' || (isRegExp ? fileName.match(search) : fileName.includes(search as string))) {
      results.push({ fileDataID, fileName });
    }
  }

  return results;
}

/** Strips a prefixed file ID from a listfile entry. */
export function stripFileEntry(entry: string): string {
  if (typeof entry === 'string' && entry.includes(' [')) return entry.substring(0, entry.lastIndexOf(' ['));

  return entry;
}

/** Returns a file path for an unknown fileDataID. */
export function formatUnknownFile(fileDataID: number, ext = ''): string {
  return `unknown/${fileDataID}${ext}`;
}

/** Returns true if a listfile has been loaded. */
export function isLoaded(): boolean {
  return loaded;
}

export function getMemoryStats(): {
  idLookup: number;
  nameLookup: number;
  preloadedIdLookup: number;
  preloadedNameLookup: number;
  isPreloaded: boolean;
  loaded: boolean;
  } {
  return {
    idLookup: idLookup.size,
    nameLookup: nameLookup.size,
    preloadedIdLookup: preloadedIdLookup.size,
    preloadedNameLookup: preloadedNameLookup.size,
    isPreloaded,
    loaded,
  };
}

/** Adds an entry to the listfile. */
export function addEntry(fileDataID: number, fileName: string): void {
  idLookup.set(fileDataID, fileName);
  nameLookup.set(fileName, fileDataID);
}

/** Clear in-memory listfile state so a new CASC build can load cleanly. */
export function resetForCascUnload(): void {
  loaded = false;
  isPreloaded = false;
  preloadPromise = null;
  idLookup.clear();
  nameLookup.clear();
  preloadedIdLookup.clear();
  preloadedNameLookup.clear();
  browseModels = null;
  browseTextures = null;
}

const browseM2WmoRegex = /\.(m2|wmo)$/i;
const browseBadWmoRegex = /_([0-9]{3}|lod\d)\.wmo$/i;
const browseTextureRegex = /\.(blp|png|tga|dds)$/i;

let browseModels: ListfileEntry[] | null = null;
let browseTextures: ListfileEntry[] | null = null;

function buildBrowseFileIndex(): { models: ListfileEntry[]; textures: ListfileEntry[] } {
  const models: ListfileEntry[] = [];
  const textures: ListfileEntry[] = [];
  for (const [fileDataID, fileName] of idLookup.entries()) {
    if (browseBadWmoRegex.test(fileName)) continue;
    if (browseM2WmoRegex.test(fileName)) {
      models.push({ fileDataID, fileName });
    } else if (browseTextureRegex.test(fileName)) {
      textures.push({ fileDataID, fileName });
    }
  }
  models.sort((a, b) => a.fileName.localeCompare(b.fileName));
  textures.sort((a, b) => a.fileName.localeCompare(b.fileName));
  return { models, textures };
}

/** Returns cached browse model/texture indexes (mirrors Go CollectBrowseFileIndex). */
export function collectBrowseFileIndex(): { models: ListfileEntry[]; textures: ListfileEntry[] } {
  if (browseModels !== null && browseTextures !== null) {
    return { models: browseModels, textures: browseTextures };
  }
  const built = buildBrowseFileIndex();
  browseModels = built.models;
  browseTextures = built.textures;
  return built;
}

export default {
  loadUnknowns,
  loadUnknownTextures,
  loadUnknownModels,
  loadIDTable,
  preload,
  prepareListfile,
  applyPreload,
  getByID,
  getByFilename,
  getFullListfile,
  getFilenamesByExtension,
  getFilteredEntries,
  getByIDOrUnknown,
  stripFileEntry,
  formatEntries,
  formatUnknownFile,
  ingestIdentifiedFiles,
  isLoaded,
  addEntry,
  collectBrowseFileIndex,
  setUnknownModelProvider,
  setUnknownTextureProvider,
};
