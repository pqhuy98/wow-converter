/**
 * Read/write helpers for wow-data-server export artifacts on disk.
 */
import {
  access, mkdir, readFile as fsReadFile, stat as fsStat, writeFile as fsWriteFile,
} from 'fs/promises';
import path from 'path';
import sharp from 'sharp';

function normalizeKey(filePath: string): string {
  return path.normalize(filePath);
}

export async function writeExportAsset(absPath: string, data: Buffer | Uint8Array): Promise<void> {
  const key = normalizeKey(absPath);
  await mkdir(path.dirname(key), { recursive: true });
  await fsWriteFile(key, Buffer.from(data));
}

export async function exportAssetExists(absPath: string): Promise<boolean> {
  try {
    await access(normalizeKey(absPath));
    return true;
  } catch {
    return false;
  }
}

export async function readExportAsset(absPath: string): Promise<Buffer> {
  return fsReadFile(normalizeKey(absPath));
}

export async function readExportAssetUtf8(absPath: string): Promise<string> {
  return (await readExportAsset(absPath)).toString('utf-8');
}

function cascExportMarkerPath(absPath: string): string {
  return `${absPath}.casc`;
}

function formatCascExportMarker(buildKey: string, fileDataID: number): string {
  return `${buildKey}\t${fileDataID}`;
}

/** True when an on-disk export PNG belongs to the active CASC build + fileDataID. */
export async function isCascExportCurrent(
  absPath: string,
  buildKey: string,
  fileDataID: number,
): Promise<boolean> {
  if (!buildKey || !await exportAssetExists(absPath)) return false;
  try {
    const marker = await readExportAssetUtf8(cascExportMarkerPath(absPath));
    return marker === formatCascExportMarker(buildKey, fileDataID);
  } catch {
    return false;
  }
}

export async function writeCascExportMarker(
  absPath: string,
  buildKey: string,
  fileDataID: number,
): Promise<void> {
  if (!buildKey) return;
  await writeExportAsset(
    cascExportMarkerPath(absPath),
    Buffer.from(formatCascExportMarker(buildKey, fileDataID)),
  );
}

export async function exportAssetStat(absPath: string): Promise<{ size: number }> {
  const stat = await fsStat(normalizeKey(absPath));
  return { size: stat.size };
}

export function sharpFromExportAsset(absPath: string): sharp.Sharp {
  return sharp(normalizeKey(absPath));
}
