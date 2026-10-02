/* eslint-disable max-classes-per-file */
/** Capture a PNG of an exported MDX from the local viewer. Uses the system browser. */
import { type ChildProcess, spawn, spawnSync } from 'child_process';
import { existsSync, mkdirSync, rmSync } from 'fs';
import { mkdtemp, writeFile } from 'fs/promises';
import { tmpdir } from 'os';
import path from 'path';

const WIDTH = 1440;
const HEIGHT = 900;
const READY_MS = 45_000;
const VIEWS = ['front', 'back', 'left', 'right', 'top', 'bottom'] as const;

interface ShotOptions {
  model: string;
  seq: string;
  out: string;
  base: string;
  view: string;
}

function parseArgs(argv: readonly string[]): ShotOptions {
  let model = '';
  let seq = 'Stand';
  let out = '';
  let view = '';
  let base = process.env.WOW_CONVERTER_URL ?? 'http://127.0.0.1:3001';
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i] ?? '';
    if (arg === '--seq') seq = argv[++i] ?? seq;
    else if (arg === '--out') out = argv[++i] ?? out;
    else if (arg === '--view') view = argv[++i] ?? view;
    else if (arg === '--base') base = argv[++i] ?? base;
    else if (!arg.startsWith('--') && model === '') model = arg;
  }
  if (model === '') {
    throw new Error('usage: bun .cursor/skills/shot-export-wow-converter/shot-export.ts <model-path> [--seq Stand] [--view front] [--out dir] [--base http://127.0.0.1:3001]');
  }
  return {
    model, seq, out, base, view,
  };
}

function toAssetPath(input: string): string {
  const norm = input.replace(/\\/g, '/');
  const marker = '/exported-assets/';
  const at = norm.toLowerCase().indexOf(marker);
  if (at >= 0) return norm.slice(at + marker.length);
  return norm.replace(/^\.\//, '');
}

function findBrowser(): string {
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
    if (existsSync(candidate)) return candidate;
  }
  const names = ['google-chrome', 'google-chrome-stable', 'chromium', 'chromium-browser', 'microsoft-edge'];
  for (const name of names) {
    const found = Bun.which(name);
    if (found) return found;
  }
  throw new Error('Chrome, Edge, or Chromium is required');
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

class Cdp {
  private next = 0;

  private readonly pending = new Map<number, (msg: Record<string, unknown>) => void>();

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

function killBrowser(proc: ChildProcess): void {
  if (proc.exitCode != null || proc.pid == null) return;
  if (process.platform === 'win32') {
    spawnSync('taskkill', ['/pid', String(proc.pid), '/T', '/F'], { stdio: 'ignore' });
  } else {
    proc.kill();
  }
}

export interface ShotCaptureRequest {
  readonly base: string;
  readonly model: string;
  readonly seq: string;
  readonly width: number;
  readonly height: number;
  readonly views: readonly string[];
  readonly readyMs?: number;
}

/** One headless browser, reused across models. */
export class ShotBrowser {
  private constructor(
    private readonly proc: ChildProcess,
    private readonly profile: string,
    private readonly cdp: Cdp,
    private readonly page: WebSocket,
  ) {}

  static async open(width: number, height: number): Promise<ShotBrowser> {
    const browser = findBrowser();
    const profile = await mkdtemp(path.join(tmpdir(), 'wow-shot-'));
    const proc = spawn(browser, [
      '--headless=new',
      '--remote-debugging-port=0',
      `--user-data-dir=${profile}`,
      '--no-first-run',
      '--no-default-browser-check',
      '--disable-extensions',
      '--hide-scrollbars',
      `--window-size=${width},${height}`,
      '--force-device-scale-factor=1',
      '--enable-webgl',
      '--ignore-gpu-blocklist',
      'about:blank',
    ], { stdio: ['ignore', 'pipe', 'pipe'] });
    try {
      const port = await devtoolsPort(proc);
      const page = await openSocket(await pageSocket(port));
      const cdp = new Cdp(page);
      await cdp.send('Page.enable');
      return new ShotBrowser(proc, profile, cdp, page);
    } catch (err: unknown) {
      killBrowser(proc);
      try {
        rmSync(profile, { recursive: true, force: true });
      } catch {
        // The browser may still be releasing the profile directory.
      }
      throw err;
    }
  }

  async capture(opts: ShotCaptureRequest): Promise<{ sequence: string; views: Map<string, Buffer> }> {
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
    const views = new Map<string, Buffer>();
    for (const view of opts.views) {
      const switched = await this.cdp.send('Runtime.evaluate', {
        expression: `window.__shotView && window.__shotView(${JSON.stringify(view)})`,
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
  }

  async close(): Promise<void> {
    this.page.close();
    killBrowser(this.proc);
    try {
      rmSync(this.profile, { recursive: true, force: true });
    } catch {
      // The browser may still be releasing the profile directory.
    }
  }
}

async function main(): Promise<void> {
  const opts = parseArgs(process.argv.slice(2));
  const assetPath = toAssetPath(opts.model);
  const browser = await ShotBrowser.open(WIDTH, HEIGHT);
  try {
    const captured = await browser.capture({
      base: opts.base,
      model: assetPath,
      seq: opts.seq,
      width: WIDTH,
      height: HEIGHT,
      views: opts.view !== '' ? [opts.view] : [...VIEWS],
    });
    const baseName = path.basename(assetPath, path.extname(assetPath));
    const safeSeq = (captured.sequence !== '' ? captured.sequence : opts.seq).replace(/[<>:"/\\|?*]/g, '_');
    const dir = opts.out !== '' ? opts.out : path.join('tmp', 'shots');
    mkdirSync(dir, { recursive: true });
    for (const [view, png] of captured.views) {
      const dest = path.join(dir, `${baseName}-${safeSeq}-${view}.png`);
      await writeFile(dest, png);
      console.log(path.resolve(dest));
    }
  } finally {
    await browser.close();
  }
}

if (import.meta.main) {
  main().then(() => process.exit(0)).catch((err: unknown) => {
    console.error(err instanceof Error ? err.message : err);
    process.exit(1);
  });
}
