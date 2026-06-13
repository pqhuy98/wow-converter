/**
 * Minimal CASC adapter for the converter process.
 *
 * All WoW parsers in src/lib/wow (M2Loader, SKELLoader, Skin, ANIMLoader,
 * BLPImage consumers, CharMaterialRenderer) read files through
 * getCasc().getFile(fileDataID). Registering this adapter into runtimeState
 * lets those parsers run inside the converter, backed by the raw-file layer
 * (local cache first, data-server REST fallback).
 *
 * Only getFile is supported; anything that needs listfile/DB2/encoding data
 * must stay on the data server.
 */
import type { CASC } from '@/lib/wow/archive/casc/casc-source';
import { BufferWrapper } from '@/lib/wow/formats/buffer';
import { runtimeState } from '@/lib/wow/server/runtime';

import { getRawWowFile } from './raw-client';

class RemoteCasc {
  isRemote = true;

  isLoaded = true;

  async getFile(fileDataID: number): Promise<BufferWrapper> {
    return new BufferWrapper(await getRawWowFile(fileDataID));
  }

  getFileByName(): never {
    throw new Error('RemoteCasc does not support name-based file access');
  }

  getDataFile(): never {
    throw new Error('RemoteCasc does not support raw data-file access');
  }
}

/**
 * Register the remote CASC adapter as the active CASC source for this process.
 * No-op when a real CASC source is already loaded (e.g. inside wow-data-server).
 */
export function ensureConverterCasc(): void {
  if (runtimeState.casc) return;
  runtimeState.casc = new RemoteCasc() as unknown as CASC;
}

/** Drop the converter's remote CASC adapter so the next export re-registers cleanly. */
export function resetConverterCasc(): void {
  if (runtimeState.casc?.isRemote) runtimeState.casc = null;
}
