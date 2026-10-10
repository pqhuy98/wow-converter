/* eslint-disable max-classes-per-file */
/** Capture a PNG of an exported MDX from the local viewer. Uses the system browser. */
import { type ChildProcess, spawn, spawnSync } from 'child_process';
import { existsSync, mkdtempSync, rmSync } from 'fs';
import { tmpdir } from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

const READY_MS = 45_000;

/** Installer-recorded Edge path, including a non-default directory. Empty when the key is missing. */
function edgeFromRegistry(): string {
  if (process.platform !== 'win32') return '';
  const keys = [
    'HKCU\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\msedge.exe',
    'HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\App Paths\\msedge.exe',
  ];
  for (const key of keys) {
    let stdout = '';
    try {
      const result = spawnSync('reg', ['query', key, '/ve'], { encoding: 'utf8', windowsHide: true });
      stdout = result.stdout ?? '';
    } catch {
      return '';
    }
    const exe = stdout.match(/REG_SZ\s+(.+)/)?.[1]?.trim() ?? '';
    if (exe !== '' && existsSync(exe)) return exe;
  }
  return '';
}

function findBrowser(): string {
  const edge = edgeFromRegistry();
  if (edge !== '') {
    console.info(`browser: Edge from registry (${edge})`);
    return edge;
  }
  console.info('browser: Edge not in registry');
  const local = process.env.LOCALAPPDATA ?? '';
  const pf = process.env.ProgramFiles ?? 'C:\\Program Files';
  const pf86 = process.env['ProgramFiles(x86)'] ?? 'C:\\Program Files (x86)';
  const absolute = process.platform === 'darwin'
    ? [
      '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
      '/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge',
      '/Applications/Chromium.app/Contents/MacOS/Chromium',
    ]
    : [
      path.join(pf, 'Google/Chrome/Application/chrome.exe'),
      path.join(local, 'Google/Chrome/Application/chrome.exe'),
      path.join(pf, 'Microsoft/Edge/Application/msedge.exe'),
      path.join(pf86, 'Microsoft/Edge/Application/msedge.exe'),
    ];
  for (const candidate of absolute) {
    if (existsSync(candidate)) {
      console.info(`browser: ${candidate}`);
      return candidate;
    }
  }
  const names = ['google-chrome', 'google-chrome-stable', 'chromium', 'chromium-browser', 'microsoft-edge'];
  for (const name of names) {
    const found = Bun.which(name);
    if (found) {
      console.info(`browser: ${found}`);
      return found;
    }
  }
  throw new Error('Chrome, Edge, or Chromium is required');
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

type CdpReply = (msg: Record<string, unknown>) => void;

class Cdp {
  private next = 0;

  private readonly pending = new Map<number, CdpReply>();

  constructor(private readonly ws: WebSocket) {
    ws.addEventListener('message', (ev: MessageEvent) => {
      const data: unknown = ev.data;
      const text = typeof data === 'string' ? data : '';
      const raw: unknown = JSON.parse(text);
      if (!isRecord(raw) || typeof raw.id !== 'number') return;
      const resolve = this.pending.get(raw.id);
      if (!resolve) return;
      this.pending.delete(raw.id);
      resolve(raw);
    });
  }

  send(method: string, params?: Record<string, unknown>): Promise<Record<string, unknown>> {
    const id = ++this.next;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error(`cdp timeout ${method}`)), 20_000);
      this.pending.set(id, (msg) => {
        clearTimeout(timer);
        if (isRecord(msg.error)) reject(new Error(`${method}: ${JSON.stringify(msg.error)}`));
        else resolve(isRecord(msg.result) ? msg.result : {});
      });
      this.ws.send(JSON.stringify({ id, method, params }));
    });
  }
}

function openSocket(url: string): Promise<WebSocket> {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(url);
    ws.addEventListener('open', () => resolve(ws));
    ws.addEventListener('error', () => reject(new Error(`websocket failed ${url}`)));
  });
}

async function devtoolsPort(proc: ChildProcess): Promise<number> {
  return new Promise((resolve, reject) => {
    let buf = '';
    const timer = setTimeout(() => reject(new Error(`browser did not open a devtools port\n${buf}`)), 15_000);
    const onData = (chunk: Buffer) => {
      buf += chunk.toString();
      const match = buf.match(/DevTools listening on ws:\/\/(?:127\.0\.0\.1|\[::1\]):(\d+)\//);
      if (!match?.[1]) return;
      clearTimeout(timer);
      resolve(Number(match[1]));
    };
    proc.stderr?.on('data', onData);
    proc.stdout?.on('data', onData);
    proc.on('exit', (code) => {
      clearTimeout(timer);
      reject(new Error(`browser exited ${code ?? 0} before devtools was ready`));
    });
    proc.once('error', (err) => {
      clearTimeout(timer);
      reject(err);
    });
  });
}

async function pageSocket(port: number): Promise<string> {
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    const list: unknown = await fetch(`http://127.0.0.1:${port}/json/list`).then((res) => res.json());
    if (Array.isArray(list)) {
      for (const entry of list) {
        if (isRecord(entry) && entry.type === 'page' && typeof entry.webSocketDebuggerUrl === 'string') {
          return entry.webSocketDebuggerUrl;
        }
      }
    }
    await Bun.sleep(50);
  }
  throw new Error('no browser page');
}

async function waitUntilReady(cdp: Cdp, readyMs: number = READY_MS): Promise<string> {
  const deadline = Date.now() + readyMs;
  while (Date.now() < deadline) {
    const result = await cdp.send('Runtime.evaluate', {
      expression: '({ ready: document.documentElement?.dataset.viewerReady || \'\', error: document.documentElement?.dataset.viewerError || \'\', seq: document.documentElement?.dataset.viewerSequence || \'\' })',
      returnByValue: true,
    });
    const remote = isRecord(result.result) ? result.result : undefined;
    const value = remote && isRecord(remote.value) ? remote.value : undefined;
    const error = typeof value?.error === 'string' ? value.error : '';
    if (error !== '') throw new Error(error);
    if (value?.ready === '1') return typeof value.seq === 'string' ? value.seq : '';
    await Bun.sleep(50);
  }
  throw new Error('viewer did not become ready');
}

const browsers = new Map<ChildProcess, string>();
let browserCleanup = false;

function trackBrowser(proc: ChildProcess, tempDir: string): void {
  browsers.set(proc, tempDir);
  if (browserCleanup) return;
  browserCleanup = true;
  process.once('SIGINT', () => process.exit(130));
  process.once('SIGTERM', () => process.exit(143));
  process.on('exit', () => {
    for (const [browser] of browsers) {
      killBrowser(browser);
      try {
        removeBrowserTemp(browser);
      } catch (err: unknown) {
        console.error('Could not remove screenshot browser temp directory:', err);
      }
    }
  });
}

function removeBrowserTemp(proc: ChildProcess): void {
  const tempDir = browsers.get(proc);
  if (!tempDir) return;
  // Bun can ignore rmSync's retry options; exit hooks must wait synchronously for Windows locks.
  const wait = new Int32Array(new SharedArrayBuffer(4));
  for (let attempt = 0; ; attempt++) {
    try {
      rmSync(tempDir, { recursive: true, force: true });
      break;
    } catch (err: unknown) {
      if (attempt === 20) throw err;
      Atomics.wait(wait, 0, 0, 100);
    }
  }
  browsers.delete(proc);
}

function killBrowser(proc: ChildProcess): void {
  if (proc.exitCode != null || proc.signalCode != null || proc.pid == null) return;
  if (process.platform === 'win32') {
    spawnSync('taskkill', ['/pid', String(proc.pid), '/T', '/F'], { stdio: 'ignore', windowsHide: true });
  } else {
    proc.kill();
  }
}

async function waitBrowserExit(proc: ChildProcess, graceful = false): Promise<void> {
  if (graceful) {
    for (let i = 0; i < 15; i++) {
      if (proc.exitCode != null || proc.signalCode != null) return;
      await Bun.sleep(200);
    }
  }
  killBrowser(proc);
  for (let i = 0; i < 25; i++) {
    if (proc.exitCode != null || proc.signalCode != null || proc.pid == null) return;
    await Bun.sleep(200);
  }
  throw new Error('Screenshot browser did not exit; its temp directory is still in use');
}

export interface ShotCaptureRequest {
  readonly base: string;
  readonly model: string;
  readonly seq: string;
  readonly width: number;
  readonly height: number;
  readonly views: readonly string[];
  readonly readyMs?: number;
  readonly cameras?: Readonly<Record<string, unknown>>;
}

/** One headless browser, reused across models. */
export class ShotBrowser {
  private constructor(
    private readonly proc: ChildProcess,
    private readonly cdp: Cdp,
    private readonly page: WebSocket,
  ) {}

  static async open(width: number, height: number): Promise<ShotBrowser> {
    const browser = findBrowser();
    const tempDir = mkdtempSync(path.join(tmpdir(), 'wow-shot-'));
    const profile = path.join(tempDir, 'profile');
    const proc = spawn(browser, [
      '--headless=new',
      '--remote-debugging-port=0',
      `--user-data-dir=${profile}`,
      '--no-first-run',
      '--no-default-browser-check',
      '--disable-extensions',
      '--disable-background-networking',
      '--disable-component-update',
      '--disable-sync',
      '--hide-scrollbars',
      `--window-size=${width},${height}`,
      '--force-device-scale-factor=1',
      '--enable-webgl',
      '--ignore-gpu-blocklist',
      'about:blank',
    ], {
      stdio: ['ignore', 'pipe', 'pipe'],
      windowsHide: true,
      env: { ...process.env, TEMP: tempDir, TMP: tempDir, TMPDIR: tempDir },
    });
    trackBrowser(proc, tempDir);
    try {
      const port = await devtoolsPort(proc);
      const page = await openSocket(await pageSocket(port));
      const cdp = new Cdp(page);
      await cdp.send('Page.enable');
      // Exported files are rewritten at the same URLs between captures and product switches.
      await cdp.send('Network.enable');
      await cdp.send('Network.setCacheDisabled', { cacheDisabled: true });
      return new ShotBrowser(proc, cdp, page);
    } catch (err: unknown) {
      await waitBrowserExit(proc);
      removeBrowserTemp(proc);
      throw err;
    }
  }

  async capture(opts: ShotCaptureRequest): Promise<{ sequence: string; views: Map<string, Buffer> }> {
    try {
      const viewerUrl = new URL('/viewer', opts.base);
      viewerUrl.searchParams.set('model', opts.model);
      viewerUrl.searchParams.set('source', 'export');
      viewerUrl.searchParams.set('shot', '1');
      viewerUrl.searchParams.set('seq', opts.seq);
      await this.cdp.send('Emulation.setDeviceMetricsOverride', {
        width: opts.width,
        height: opts.height,
        deviceScaleFactor: 1,
        mobile: false,
      });
      await this.cdp.send('Page.navigate', { url: viewerUrl.toString() });
      const sequence = await waitUntilReady(this.cdp, opts.readyMs ?? READY_MS);
      await this.cdp.send('Runtime.evaluate', {
        // Next's development issue indicator can appear after a model finishes loading.
        expression: "document.head.insertAdjacentHTML('beforeend', '<style>nextjs-portal { display: none !important; }</style>')",
      });
      const views = new Map<string, Buffer>();
      for (const view of opts.views) {
        const camera = opts.cameras?.[view];
        const switched = await this.cdp.send('Runtime.evaluate', {
          expression: `window.__shotView && window.__shotView(${JSON.stringify(view)}, ${camera !== undefined ? JSON.stringify(camera) : 'null'})`,
          returnByValue: true,
        });
        const remote = isRecord(switched.result) ? switched.result : undefined;
        if (remote?.value !== view) throw new Error(`viewer rejected view ${view}`);
        await this.cdp.send('Runtime.evaluate', {
          expression: 'new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve(true))))',
          awaitPromise: true,
        });
        const shot = await this.cdp.send('Page.captureScreenshot', { format: 'png' });
        if (typeof shot.data !== 'string') throw new Error('screenshot had no data');
        views.set(view, Buffer.from(shot.data, 'base64'));
      }
      return { sequence, views };
    } finally {
      await this.cdp.send('Page.navigate', { url: 'about:blank' }).catch(() => {
        // The page is already gone.
      });
    }
  }

  async close(): Promise<void> {
    if (!browsers.has(this.proc)) return;
    if (this.page.readyState === WebSocket.OPEN) {
      this.page.send(JSON.stringify({ id: 0, method: 'Browser.close' }));
    }
    this.page.close();
    await waitBrowserExit(this.proc, true);
    removeBrowserTemp(this.proc);
  }
}

// Snapshot tests spawn shot-wowhead; this skill CLI uses the Go server code.
if (import.meta.main) {
  const result = spawnSync('go', ['run', './cmd/shot-converter', ...process.argv.slice(2)], {
    cwd: fileURLToPath(new URL('../../../golang/', import.meta.url)),
    stdio: 'inherit',
    windowsHide: true,
  });
  if (result.error) console.error(result.error.message);
  process.exit(result.status ?? 1);
}
