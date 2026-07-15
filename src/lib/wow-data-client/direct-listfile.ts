import * as listfile from '@/lib/wow/archive/casc/listfile';
import type { FileEntry } from '@/lib/wow-data-client/wow-data-client';

/** Client that can build browse/map indexes without copying the full listfile. */
export interface DirectListfileClient {
  collectBrowseFileIndex(): Promise<{ models: FileEntry[]; textures: FileEntry[] }>;
  collectMapTileFileIndex(): Promise<FileEntry[]>;
}

export function isDirectListfileClient(client: unknown): client is DirectListfileClient {
  if (client == null || typeof client !== 'object') return false;
  const c = client as DirectListfileClient;
  return typeof c.collectBrowseFileIndex === 'function'
    && typeof c.collectMapTileFileIndex === 'function';
}

/** Same-process listfile scan when converter shares memory with wow-data-server (bundled app). */
export function tryInProcessListfileClient(): DirectListfileClient | null {
  if (!listfile.isLoaded()) return null;
  return {
    collectBrowseFileIndex: () => Promise.resolve(listfile.collectBrowseFileIndex()),
    collectMapTileFileIndex: () => Promise.resolve(listfile.collectMapTileFileIndex()),
  };
}
