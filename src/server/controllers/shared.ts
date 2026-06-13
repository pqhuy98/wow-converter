import { waitUntil } from '@/lib/utils';
import { FileEntry, wowDataClient } from '@/lib/wow-data-client/wow-data-client';

let listFiles: FileEntry[] | null = null;
let pending = false;

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
