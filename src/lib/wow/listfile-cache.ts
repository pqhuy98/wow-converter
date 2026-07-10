import { waitUntil } from '@/lib/utils';
import { FileEntry, wowDataClient } from '@/lib/wow-data-client/wow-data-client';

let listFiles: FileEntry[] | null = null;
let pending = false;

const clearHooks: Array<() => void> = [];

/** Register extra listfile-derived caches (browse, maps) cleared on CASC unload. */
export function registerListfileClearHook(fn: () => void): void {
  clearHooks.push(fn);
}

/** Drop cached listfile data after CASC unload or build change. */
export function resetListfileCache(): void {
  listFiles = null;
  pending = false;
  for (const fn of clearHooks) fn();
}

/** Cached full listfile search (millions of rows — fetch at most once per process). */
export async function getListFiles(): Promise<FileEntry[]> {
  if (listFiles) return listFiles;
  if (pending) {
    await waitUntil(() => listFiles !== null);
    return listFiles!;
  }
  pending = true;
  await wowDataClient.waitUntilReady();
  listFiles = await wowDataClient.searchFiles('');
  return listFiles;
}
