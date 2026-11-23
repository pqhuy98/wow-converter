import { useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import ModelViewer from '@wow-converter/model-viewer';

type Request = { kind: 'read'; path: string } | { kind: 'copy'; text: string };
type Reply = { id: number; value?: ArrayBuffer; error?: string };

declare function acquireVsCodeApi(): { postMessage(message: Request & { id: number }): void };

const api = acquireVsCodeApi();
const pending = new Map<number, (reply: Reply) => void>();
let nextId = 0;

window.addEventListener('message', ({ data }: MessageEvent<Reply>) => {
  const reply = pending.get(data?.id);
  if (!reply) return;
  pending.delete(data.id);
  reply(data);
});

function send(request: Request): Promise<ArrayBuffer | undefined> {
  return new Promise((resolve, reject) => {
    const id = ++nextId;
    pending.set(id, ({ value, error }) => {
      if (error) reject(new Error(error));
      else resolve(value);
    });
    api.postMessage({ id, ...request });
  });
}

function Viewer({ model, link }: { model: string; link: string }) {
  const [error, setError] = useState('');
  const localFiles = useMemo(() => ({
    async read(path: string) {
      const data = await send({ kind: 'read', path });
      if (!data) throw new Error(`The viewer could not read ${path}.`);
      return data;
    },
    async copyText(text: string) {
      await send({ kind: 'copy', text });
    },
    link,
    onError: setError,
  }), [link]);

  return (
    <div className="relative h-screen w-screen bg-background">
      <ModelViewer modelPath={model} localFiles={localFiles} alwaysFullscreen />
      {error && (
        <div className="fixed top-16 left-2 right-2 z-[60] rounded-md bg-destructive p-3 text-destructive-foreground" role="alert">
          {error}
        </div>
      )}
    </div>
  );
}

const root = document.getElementById('root');
if (root?.dataset.model && root.dataset.link) {
  const theme = document.body.classList;
  const syncTheme = () => document.documentElement.classList.toggle(
    'dark',
    theme.contains('vscode-dark') || (theme.contains('vscode-high-contrast') && !theme.contains('vscode-high-contrast-light')),
  );
  new MutationObserver(syncTheme).observe(document.body, { attributes: true, attributeFilter: ['class'] });
  syncTheme();
  createRoot(root).render(<Viewer model={decodeURIComponent(root.dataset.model)} link={decodeURIComponent(root.dataset.link)} />);
}
