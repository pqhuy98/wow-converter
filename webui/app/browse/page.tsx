import BrowseModelPage from '@/components/browse-model';

export const dynamic = 'force-static';

type FileEntry = { fileDataID: number; fileName: string };
let allFiles: FileEntry[] | null = null;

export default async function Page() {
  if (!allFiles) {
    const res = await fetch('http://localhost:3001/api/browse?q=model');
    allFiles = await res.json();
  }
  if (!allFiles?.length) {
    throw new Error('No m2 list files found');
  }
  return <BrowseModelPage allFiles={allFiles} />;
}
