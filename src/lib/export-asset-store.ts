/**
 * Read/write helpers for wow.export / wow-data-server artifacts on disk.
 */
import {
  mkdir, readFile as fsReadFile, stat as fsStat, writeFile as fsWriteFile,
} from 'fs/promises';
import { existsSync, mkdirSync, writeFileSync } from 'fs-extra';
import path from 'path';
import sharp from 'sharp';

function normalizeKey(filePath: string): string {
  return path.normalize(filePath);
}

/** Write a generated export artifact to disk (sync — hot paths during M2 conversion). */
export function putExportAsset(absPath: string, data: Buffer): void {
  if (data.length === 0) return;
  const key = normalizeKey(absPath);
  mkdirSync(path.dirname(key), { recursive: true });
  writeFileSync(key, data);
}

export async function writeExportAsset(absPath: string, data: Buffer | Uint8Array): Promise<void> {
  const key = normalizeKey(absPath);
  await mkdir(path.dirname(key), { recursive: true });
  await fsWriteFile(key, Buffer.from(data));
}

export function exportAssetExistsSync(absPath: string): boolean {
  return existsSync(normalizeKey(absPath));
}

export function exportAssetExists(absPath: string): Promise<boolean> {
  return Promise.resolve(exportAssetExistsSync(absPath));
}

export async function readExportAsset(absPath: string): Promise<Buffer> {
  return fsReadFile(normalizeKey(absPath));
}

export async function readExportAssetUtf8(absPath: string): Promise<string> {
  return (await readExportAsset(absPath)).toString('utf-8');
}

export async function exportAssetStat(absPath: string): Promise<{ size: number }> {
  const stat = await fsStat(normalizeKey(absPath));
  return { size: stat.size };
}

export function sharpFromExportAsset(absPath: string): sharp.Sharp {
  return sharp(normalizeKey(absPath));
}
