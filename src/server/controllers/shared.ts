import { waitUntil } from '@/lib/utils';
import { FileEntry, wowExportClient } from '@/lib/wowexport-client/wowexport-client';

let listFiles: FileEntry[] | null = null;
let pending = false;

export async function getListFiles(): Promise<FileEntry[]> {
  if (listFiles) return listFiles;
  if (pending) {
    await waitUntil(() => listFiles !== null);
    return listFiles!;
  }
  pending = true;
  await wowExportClient.waitUntilReady();
  listFiles = await wowExportClient.searchFiles('');
  return listFiles;
}
