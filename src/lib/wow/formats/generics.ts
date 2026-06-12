/**
 * Network/file helpers ported from wow.export (src/js/generics.js).
 * Only the headless surface is ported (no DOM/UI helpers).
 */
import fs, { promises as fsp } from 'fs';
import http from 'http';
import https from 'https';
import path from 'path';
import zlib from 'zlib';

import { constants } from '@/lib/wow/formats/constants';
import { write } from '@/lib/wow/log';

import { BufferWrapper } from './buffer';

/** Prefer IPv4 + HTTP/1.1 ALPN for Blizzard TACT/CDN; avoids flaky HTTP/2 + Range on some edges. */
const blizzardHttpsAgent = new https.Agent({ keepAlive: true, family: 4, maxSockets: 64 });
const defaultHttpsAgent = new https.Agent({ keepAlive: true, maxSockets: 64 });
const defaultHttpAgent = new http.Agent({ keepAlive: true, maxSockets: 64 });

/**
 * Async wrapper for fetch() with fallback URL support.
 */
export async function get(url: string | string[]): Promise<Response> {
  const fetchOptions: RequestInit = {
    headers: { 'User-Agent': constants.USER_AGENT },
    redirect: 'follow',
  };

  let index = 1;
  let res: Response | null = null;

  const urlStack = Array.isArray(url) ? [...url] : [url];

  while ((res === null || !res.ok) && urlStack.length > 0) {
    const currentUrl = urlStack.shift()!;
    res = await fetch(currentUrl, fetchOptions);
    write(`get -> [${index++}][${res.status}] ${currentUrl}`);
  }

  return res!;
}

/**
 * Dispatch an async handler for an array of items with a limit to how
 * many can be resolving at once.
 */
export async function queue<T>(items: T[], handler: (item: T) => Promise<unknown>, limit: number): Promise<void> {
  return new Promise((resolve, reject) => {
    let free = limit;
    let complete = -1;
    let index = 0;
    const check = () => {
      complete++;
      free++;

      while (free > 0 && index < items.length) {
        handler(items[index]).then(check).catch(reject);
        index++; free--;
      }

      if (complete === items.length) resolve();
    };

    check();
  });
}

/**
 * Process large arrays in chunks, yielding to the event loop between batches
 * (headless equivalent of wow.export's batchWork redraw yielding).
 */
export async function batchWork<T>(
  name: string,
  items: T[],
  handler: (item: T, index: number) => void,
  batchSize = 1000,
): Promise<void> {
  const totalItems = items.length;
  for (let start = 0; start < totalItems; start += batchSize) {
    const end = Math.min(start + batchSize, totalItems);
    for (let i = start; i < end; i++) handler(items[i], i);

    if (end < totalItems) await new Promise<void>((resolve) => { setImmediate(resolve); });
  }
  write('batchWork "%s" processed %d items', name, totalItems);
}

/** Format a number (bytes) to a displayable file size. */
const doOnceCache = new Map<string, { status: 'pending' | 'complete'; result?: unknown }>();

/** Wrap an async function so it only ever runs once; concurrent callers await the first run. */
export function doOnce<T>(key: string, func: () => Promise<T>): () => Promise<T> {
  return async () => {
    if (!doOnceCache.has(key)) {
      doOnceCache.set(key, { status: 'pending' });
      const result = await func();
      doOnceCache.set(key, { result, status: 'complete' });
      return result;
    }
    while (doOnceCache.get(key)!.status !== 'complete') {
      await new Promise((resolve) => { setTimeout(resolve, 100); });
    }
    return doOnceCache.get(key)!.result as T;
  };
}

/** Clear doOnce memoization so DB/character caches can rebuild after a CASC switch. */
export function resetDoOnceCache(): void {
  doOnceCache.clear();
}

export function filesize(bytes: number): string {
  let size = Number.isNaN(bytes) ? 0 : bytes;
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];

  let unitIndex = Math.floor(Math.log(size) / Math.log(1024));
  if (!Number.isFinite(unitIndex) || unitIndex < 0) unitIndex = 0;

  size = Number((size / 1024 ** unitIndex).toFixed(2));
  return `${size} ${units[unitIndex]}`;
}

/** Ping a URL and measure the response time. */
export async function ping(url: string): Promise<number> {
  const pingStart = Date.now();
  await get(url);
  return Date.now() - pingStart;
}

export async function getJSON(url: string): Promise<unknown> {
  const res = await get(url);
  if (!res.ok) throw new Error(`Unable to request JSON from end-point. HTTP ${res.status} ${res.statusText}`);
  return res.json();
}

/** Read a JSON file from disk, returning null on error. */
export async function readJSON(file: string, ignoreComments = false): Promise<unknown | null> {
  try {
    const raw = await fsp.readFile(file, 'utf8');
    if (ignoreComments) {
      return JSON.parse(raw.split(/\r?\n/).filter((e) => !e.startsWith('//')).join('\n'));
    }
    return JSON.parse(raw);
  } catch (e) {
    return null;
  }
}

function isBlizzardCdnHost(hostname: string): boolean {
  return /\.blizzard\.com$/i.test(hostname) || /\.battle\.net$/i.test(hostname);
}

/**
 * Single GET (follows redirects by delegating to requestData).
 * Blizzard hosts: IPv4 + ALPN http/1.1 + dedicated agent.
 */
function requestDataSingleHop(url: string, partialOfs: number, partialLen: number): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    let parsed: URL;
    try {
      parsed = new URL(url);
    } catch (e) {
      reject(new Error(`Invalid URL: ${url}`));
      return;
    }

    const isHttps = parsed.protocol === 'https:';
    const blizzard = isBlizzardCdnHost(parsed.hostname);
    const headers: Record<string, string> = {
      'User-Agent': constants.USER_AGENT,
    };

    if (partialOfs > -1 && partialLen > -1) headers.Range = `bytes=${partialOfs}-${partialOfs + partialLen - 1}`;

    const options: https.RequestOptions & { ALPNProtocols?: string[] } = {
      hostname: parsed.hostname,
      port: parsed.port || (isHttps ? 443 : 80),
      path: parsed.pathname + parsed.search,
      method: 'GET',
      headers,
      agent: isHttps
        ? (blizzard ? blizzardHttpsAgent : defaultHttpsAgent)
        : defaultHttpAgent,
    };

    if (blizzard) {
      options.family = 4;
      if (isHttps) options.ALPNProtocols = ['http/1.1'];
    }

    const protocol = isHttps ? https : http;
    const req = protocol.request(options, (res) => {
      if (res.statusCode === 301 || res.statusCode === 302) {
        const loc = res.headers.location;
        if (!loc) {
          reject(new Error('Redirect without Location header'));
          return;
        }
        write(`Got redirect to ${loc}`);
        res.resume();
        resolve(requestData(new URL(loc, url).href, partialOfs, partialLen));
        return;
      }

      if (!res.statusCode || res.statusCode < 200 || res.statusCode > 302) {
        res.resume();
        reject(new Error(`Status Code: ${res.statusCode}`));
        return;
      }

      const chunks: Buffer[] = [];
      res.on('data', (chunk: Buffer) => chunks.push(chunk));
      res.on('end', () => resolve(Buffer.concat(chunks)));
      res.on('error', reject);
    });

    req.on('error', reject);
    req.setTimeout(600000, () => {
      req.destroy();
      reject(new Error('Request timeout'));
    });
    req.end();
  });
}

function isRetryableDownloadError(err: unknown): boolean {
  const code = (err as NodeJS.ErrnoException)?.code;
  const msg = (err as Error)?.message ?? '';
  if (code === 'ECONNRESET' || code === 'ECONNREFUSED' || code === 'ETIMEDOUT' || code === 'EPIPE') return true;
  if (/socket hang up|timeout/i.test(msg)) return true;
  return false;
}

async function requestData(url: string, partialOfs: number, partialLen: number): Promise<Buffer> {
  const maxAttempts = 4;
  let lastErr: unknown;
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      return await requestDataSingleHop(url, partialOfs, partialLen);
    } catch (err) {
      lastErr = err;
      if (!isRetryableDownloadError(err) || attempt === maxAttempts) throw err;
      const delay = Math.min(400 * attempt * attempt, 8000);
      write(`requestData retry ${attempt}/${maxAttempts} after ${(err as Error).message} (wait ${delay}ms)`);
      await new Promise<void>((r) => { setTimeout(r, delay); });
    }
  }
  throw lastErr;
}

/**
 * Download a file (optionally to a local file).
 * GZIP deflation will be used if headers are set.
 * Data is always returned even if `out` is provided.
 */
export async function downloadFile(
  url: string | string[],
  out?: string,
  partialOfs = -1,
  partialLen = -1,
  deflate = false,
): Promise<BufferWrapper> {
  const urlStack = Array.isArray(url) ? url : [url];

  for (const currentUrl of urlStack) {
    try {
      write(`downloadFile -> ${currentUrl}`);

      let data = await requestData(currentUrl, partialOfs, partialLen);

      if (deflate) {
        data = await new Promise<Buffer>((resolve, reject) => {
          zlib.inflate(data, (err, result) => (err ? reject(err) : resolve(result)));
        });
      }

      const wrapped = new BufferWrapper(data);

      if (out) {
        await createDirectory(path.dirname(out));
        await wrapped.writeToFile(out);
      }

      return wrapped;
    } catch (error) {
      write(`Failed to download from ${currentUrl}: ${(error as Error).message}`);
    }
  }

  throw new Error('All download attempts failed.');
}

/** Create all directories in a given path if they do not exist. */
export async function createDirectory(dir: string): Promise<void> {
  await fsp.mkdir(dir, { recursive: true });
}

/** Wrapper for asynchronously checking if a file exists. */
export async function fileExists(file: string): Promise<boolean> {
  try {
    await fsp.access(file);
    return true;
  } catch (e) {
    return false;
  }
}

/** Read a portion of a file. */
export async function readFile(file: string, offset: number, length: number): Promise<BufferWrapper> {
  const fd = await fsp.open(file);
  const buf = BufferWrapper.alloc(length);

  await fd.read(buf.raw, 0, length, offset);
  await fd.close();

  return buf;
}

/** Recursively delete a directory and everything inside of it. Returns total size deleted. */
export async function deleteDirectory(dir: string): Promise<number> {
  let deleteSize = 0;
  try {
    const entries = await fsp.readdir(dir);
    for (const entry of entries) {
      const entryPath = path.join(dir, entry);
      const entryStat = await fsp.stat(entryPath);

      if (entryStat.isDirectory()) {
        deleteSize += await deleteDirectory(entryPath);
      } else {
        await fsp.unlink(entryPath);
        deleteSize += entryStat.size;
      }
    }

    await fsp.rmdir(dir);
  } catch (e) {
    // Something failed to delete.
  }

  return deleteSize;
}

/** Calculate the hash of a file. */
export async function getFileHash(file: string, method: string, encoding: 'hex' | 'base64'): Promise<string> {
  const { createHash } = await import('crypto');
  return new Promise((resolve) => {
    const fd = fs.createReadStream(file);
    const hash = createHash(method);

    fd.on('data', (chunk) => hash.update(chunk));
    fd.on('end', () => resolve(hash.digest(encoding)));
  });
}
