export interface DownloadRequestBody {
  files: string[];
  source?: 'export' | 'browse';
}

function pickZipFilenameFromFiles(files: string[]): string {
  const model = files.find((f) => /\.(mdx|mdl)$/i.test(f));
  if (!model) return 'assets.zip';
  const lastSlash = Math.max(model.lastIndexOf('/'), model.lastIndexOf('\\'));
  let base = lastSlash >= 0 ? model.slice(lastSlash + 1) : model;
  base = base.replace(/__([0-9a-fA-F]{32})(?=\.(?:mdx|mdl)$)/, '');
  return `${base.replace(/\.(mdx|mdl)$/i, '')}.zip`;
}

export async function downloadAssetsZip(request: DownloadRequestBody): Promise<void> {
  const { files } = request;
  if (!files || files.length === 0) {
    // eslint-disable-next-line no-alert
    alert('Nothing to download – exported files list is empty');
    return;
  }

  const resp = await fetch('/api/download', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(request),
  });
  if (!resp.ok) {
    let message = `Download failed with ${resp.status}`;
    try {
      const data = await resp.json();
      if (data && typeof data.error === 'string') message += `: ${data.error}`;
    } catch { /* noop */ }
    throw new Error(message);
  }

  const blob = await resp.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = pickZipFilenameFromFiles(files);
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
