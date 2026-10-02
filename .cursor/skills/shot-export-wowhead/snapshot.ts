/** Save a PNG of the Wowhead model-viewer canvas. The toolbar and page stay out of the shot. */
import { type ChildProcess, spawn, spawnSync } from 'child_process';
import { existsSync, mkdirSync, rmSync } from 'fs';
import { mkdtemp, writeFile } from 'fs/promises';
import { tmpdir } from 'os';
import path from 'path';

const WIDTH = 1440;
const HEIGHT = 900;
const READY_MS = 45_000;
const VIEWS = ['front', 'back', 'left', 'right', 'top', 'bottom'] as const;

const FORCE_BUFFER = `
(() => {
  let installed = false;
  Object.defineProperty(window, 'ZamModelViewer', {
    configurable: true,
    enumerable: true,
    get() { return undefined; },
    set(Ctor) {
      if (installed || typeof Ctor !== 'function') {
        Object.defineProperty(window, 'ZamModelViewer', { configurable: true, writable: true, value: Ctor });
        return;
      }
      installed = true;
      function Viewer() {
        const inst = new Ctor(...arguments);
        window.__whViewer = inst;
        return inst;
      }
      Viewer.prototype = Ctor.prototype;
      Object.setPrototypeOf(Viewer, Ctor);
      for (const key of Object.keys(Ctor)) Viewer[key] = Ctor[key];
      Object.defineProperty(window, 'ZamModelViewer', { configurable: true, writable: true, value: Viewer });
    },
  });
})();
`;

const SAMPLE = `
(() => {
  const viewer = window.__whViewer;
  const renderer = viewer && viewer.renderer;
  const base = {
    phase: 'wait',
    colored: window.__whColored || 0,
    boxH: window.__whBox || 0,
    boxW: window.__whBoxW || 0,
    seq: window.__whSeq || 0,
    view: window.__whWantView || '',
    anim: window.__whAnim || '',
    dist: 0,
  };
  if (!renderer || !renderer.canvas) return Object.assign(base, { phase: 'no-viewer' });
  const actor = renderer.actors && renderer.actors[0];
  const model = actor && actor.a;
  if (!model || !model.bf || !model.bf.B) return Object.assign(base, { phase: 'loading' });

  if (!window.__whInit) {
    renderer.clearBackground();
    renderer.bgTexture = null;
    renderer.options.background = '';
    renderer.clearColor[0] = 0.15;
    renderer.clearColor[1] = 0.15;
    renderer.clearColor[2] = 0.15;
    renderer.fov = 45;
    renderer.onResize(1440, 900, 1440 / 900);
    renderer.__whPan = {
      t: [renderer.translation[0], renderer.translation[1], renderer.translation[2]],
      m: [renderer.translationFromModel[0], renderer.translationFromModel[1], renderer.translationFromModel[2]],
      target: [renderer.target[0], renderer.target[1], renderer.target[2]],
    };
    const origDraw = renderer.updateCamera.bind(renderer);
    renderer.updateCamera = function() {
      const view = window.__whWantView;
      const saved = renderer.__whPan;
      if (view === 'top' || view === 'bottom') {
        const center = boundsCenter(actor);
        if (center) {
          renderer.target[0] = center[0];
          renderer.target[1] = center[1];
          renderer.target[2] = center[2];
        }
        renderer.translation[0] = 0;
        renderer.translation[1] = 0;
        renderer.translation[2] = 0;
        renderer.translationFromModel[0] = 0;
        renderer.translationFromModel[1] = 0;
        renderer.translationFromModel[2] = 0;
      } else if (saved) {
        renderer.target[0] = saved.target[0];
        renderer.target[1] = saved.target[1];
        renderer.target[2] = saved.target[2];
        renderer.translation[0] = saved.t[0];
        renderer.translation[1] = saved.t[1];
        renderer.translation[2] = saved.t[2];
        renderer.translationFromModel[0] = saved.m[0];
        renderer.translationFromModel[1] = saved.m[1];
        renderer.translationFromModel[2] = saved.m[2];
      }
      origDraw();
      if (view !== 'top' && view !== 'bottom') return;
      const dist = renderer.distance * Math.pow(renderer.zoom.rateStep + 1, -renderer.zoom.current);
      const sign = view === 'top' ? 1 : -1;
      const eye = [renderer.target[0], renderer.target[1], renderer.target[2] + sign * dist];
      lookAt(renderer.viewMatrix, eye, renderer.target, [0, 1, 0]);
      mul(renderer.viewMatrix, renderer.panningMatrix, renderer.viewMatrix);
      renderer.eye[0] = eye[0];
      renderer.eye[1] = eye[1];
      renderer.eye[2] = eye[2];
    };
    const origAnim = model.aX.bind(model);
    model.aX = function(state, _dt) { return origAnim(state, 0); };
    window.__whInit = true;
    return Object.assign(base, { phase: 'init' });
  }

  const want = String(window.__whWantSeq || 'Stand');
  const rows = model.bf.B.filter((row) => row && typeof row.f === 'string' && row.a === 0);
  const picked = pickAnim(rows, want);
  if (!picked) {
    return Object.assign(base, {
      phase: 'bad-anim',
      names: rows.map((row) => row.f).slice(0, 24).join(', '),
    });
  }
  window.__whAnim = picked.f;
  const playing = model.C && model.C.d && model.C.d.e && model.C.d.e.f;
  if (playing !== picked.f) actor.setAnimation(picked.f, false);
  if (model.C && model.C.d) model.C.d.c = picked.l / 2;

  const view = window.__whWantView || 'front';
  const orbit = {
    front: [Math.PI * 1.5, Math.PI / 2],
    back: [Math.PI / 2, Math.PI / 2],
    left: [0, Math.PI / 2],
    right: [Math.PI, Math.PI / 2],
  };
  const pair = orbit[view] || orbit.front;
  renderer.azimuth = pair[0];
  renderer.zenith = pair[1];
  renderer.zoom.current = 1;
  renderer.zoom.target = 1;
  const size = boundsSize(actor);
  if (size) renderer.distance = orbitDistance(view, size, renderer.fov, 1440 / 900);
  base.dist = Math.round(renderer.distance * 10) / 10;

  if (window.__whMeasuring) return Object.assign(base, { phase: 'measuring', anim: picked.f });
  if (window.__whShot) {
    const url = window.__whShot;
    window.__whShot = '';
    window.__whMeasuring = true;
    const img = new Image();
    img.onload = () => {
      const full = document.createElement('canvas');
      full.width = img.width;
      full.height = img.height;
      const paint = full.getContext('2d');
      if (paint) {
        paint.fillStyle = 'rgb(38,38,38)';
        paint.fillRect(0, 0, full.width, full.height);
        paint.drawImage(img, 0, 0);
      }
      const tw = 80;
      const th = 45;
      const tmp = document.createElement('canvas');
      tmp.width = tw;
      tmp.height = th;
      const ctx = tmp.getContext('2d');
      let colored = 0;
      let boxH = 0;
      let boxW = 0;
      if (ctx) {
        ctx.drawImage(full, 0, 0, tw, th);
        const data = ctx.getImageData(0, 0, tw, th).data;
        let minY = th;
        let maxY = -1;
        let minX = tw;
        let maxX = -1;
        for (let y = 0; y < th; y++) {
          for (let x = 0; x < tw; x++) {
            const p = (y * tw + x) * 4;
            const r = data[p];
            const g = data[p + 1];
            const b = data[p + 2];
            const max = r > g ? (r > b ? r : b) : (g > b ? g : b);
            const min = r < g ? (r < b ? r : b) : (g < b ? g : b);
            if (max - min > 22 && max > 40) {
              colored++;
              if (y < minY) minY = y;
              if (y > maxY) maxY = y;
              if (x < minX) minX = x;
              if (x > maxX) maxX = x;
            }
          }
        }
        if (maxY >= 0) boxH = (maxY - minY + 1) / th;
        if (maxX >= 0) boxW = (maxX - minX + 1) / tw;
      }
      window.__whColored = colored;
      window.__whBox = Math.round(boxH * 100) / 100;
      window.__whBoxW = Math.round(boxW * 100) / 100;
      window.__whPng = paint ? full.toDataURL('image/png') : url;
      window.__whMeasuring = false;
      window.__whSeq = (window.__whSeq || 0) + 1;
    };
    img.src = url;
    return Object.assign(base, { phase: 'measuring', anim: picked.f });
  }
  if (!window.__whPending) {
    window.__whPending = true;
    renderer.screenshotCallback = () => {
      window.__whShot = renderer.screenshotDataURL;
      window.__whPending = false;
    };
    renderer.makeDataURL = ['image/png'];
  }
  return Object.assign(base, { phase: 'pending', anim: picked.f });

  function pickAnim(rows, name) {
    const want = name.toLowerCase();
    const exact = rows.find((row) => row.f.toLowerCase() === want);
    if (exact) return exact;
    const pref = rows.filter((row) => row.f.toLowerCase().startsWith(want));
    if (pref.length === 1) return pref[0];
    for (const tail of ['unarmed', '1h', '2h']) {
      const hit = pref.find((row) => row.f.toLowerCase() === want + tail);
      if (hit) return hit;
    }
    return pref[0] || null;
  }

  function boundsCenter(unit) {
    const pair = unit.getBounds && unit.getBounds();
    if (!pair || !pair[0] || !pair[1]) return null;
    return [
      (pair[0][0] + pair[1][0]) / 2,
      (pair[0][1] + pair[1][1]) / 2,
      (pair[0][2] + pair[1][2]) / 2,
    ];
  }

  function boundsSize(unit) {
    const pair = unit.getBounds && unit.getBounds();
    if (!pair || !pair[0] || !pair[1]) return null;
    return {
      x: Math.abs(pair[1][0] - pair[0][0]),
      y: Math.abs(pair[1][1] - pair[0][1]),
      z: Math.abs(pair[1][2] - pair[0][2]),
    };
  }

  function orbitDistance(view, size, fovDeg, aspect) {
    const v = 2 * Math.tan((fovDeg * Math.PI / 180) / 2);
    let vertical = size.z;
    let horizontal = size.x;
    let depth = size.y;
    if (view === 'left' || view === 'right') {
      horizontal = size.y;
      depth = size.x;
    } else if (view === 'top' || view === 'bottom') {
      vertical = size.y;
      horizontal = size.x;
      depth = size.z;
    }
    const fit = Math.max(1.2 * vertical / v, 1.2 * horizontal / (v * aspect));
    return Math.max(fit, depth * 0.5 + 1.2, 2);
  }

  function lookAt(out, eye, center, up) {
    let zx = eye[0] - center[0];
    let zy = eye[1] - center[1];
    let zz = eye[2] - center[2];
    let len = Math.hypot(zx, zy, zz);
    if (len < 1e-6) return;
    zx /= len; zy /= len; zz /= len;
    let xx = up[1] * zz - up[2] * zy;
    let xy = up[2] * zx - up[0] * zz;
    let xz = up[0] * zy - up[1] * zx;
    len = Math.hypot(xx, xy, xz);
    if (len < 1e-6) { xx = 0; xy = 0; xz = 0; }
    else { xx /= len; xy /= len; xz /= len; }
    let yx = zy * xz - zz * xy;
    let yy = zz * xx - zx * xz;
    let yz = zx * xy - zy * xx;
    len = Math.hypot(yx, yy, yz);
    if (len < 1e-6) { yx = 0; yy = 0; yz = 0; }
    else { yx /= len; yy /= len; yz /= len; }
    out[0] = xx; out[1] = yx; out[2] = zx; out[3] = 0;
    out[4] = xy; out[5] = yy; out[6] = zy; out[7] = 0;
    out[8] = xz; out[9] = yz; out[10] = zz; out[11] = 0;
    out[12] = -(xx * eye[0] + xy * eye[1] + xz * eye[2]);
    out[13] = -(yx * eye[0] + yy * eye[1] + yz * eye[2]);
    out[14] = -(zx * eye[0] + zy * eye[1] + zz * eye[2]);
    out[15] = 1;
  }

  function mul(out, a, b) {
    const r = new Array(16);
    for (let col = 0; col < 4; col++) {
      for (let row = 0; row < 4; row++) {
        r[col * 4 + row] = a[row] * b[col * 4] + a[4 + row] * b[col * 4 + 1] + a[8 + row] * b[col * 4 + 2] + a[12 + row] * b[col * 4 + 3];
      }
    }
    for (let i = 0; i < 16; i++) out[i] = r[i];
  }
})()
`;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

function findBrowser(): string {
  const local = process.env.LOCALAPPDATA ?? '';
  const pf = process.env.ProgramFiles ?? 'C:\\Program Files';
  const pf86 = process.env['ProgramFiles(x86)'] ?? 'C:\\Program Files (x86)';
  const candidates = [
    path.join(pf, 'Google/Chrome/Application/chrome.exe'),
    path.join(local, 'Google/Chrome/Application/chrome.exe'),
    path.join(pf, 'Microsoft/Edge/Application/msedge.exe'),
    path.join(pf86, 'Microsoft/Edge/Application/msedge.exe'),
  ];
  for (const candidate of candidates) {
    if (existsSync(candidate)) return candidate;
  }
  throw new Error('Chrome or Edge is required');
}

function slugFromUrl(pageUrl: string): string {
  const match = pageUrl.match(/\/(?:npc|item|object|spell)=(\d+)(?:\/([^#?]+))?/i);
  if (match?.[2]) return match[2];
  if (match?.[1]) return match[1];
  return 'model';
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
      const timer = setTimeout(() => reject(new Error(`cdp timeout ${method}`)), 30_000);
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

function killBrowser(proc: ChildProcess): void {
  if (proc.exitCode != null || proc.pid == null) return;
  if (process.platform === 'win32') {
    spawnSync('taskkill', ['/pid', String(proc.pid), '/T', '/F'], { stdio: 'ignore' });
  } else {
    proc.kill();
  }
}

function remoteValue(result: Record<string, unknown>): Record<string, unknown> | undefined {
  const remote = isRecord(result.result) ? result.result : undefined;
  return remote && isRecord(remote.value) ? remote.value : undefined;
}

async function readPng(cdp: Cdp): Promise<string> {
  const grabbed = await cdp.send('Runtime.evaluate', {
    expression: 'window.__whPng || ""',
    returnByValue: true,
  });
  const url = isRecord(grabbed.result) ? grabbed.result.value : '';
  if (typeof url !== 'string' || !url.startsWith('data:image/png;base64,')) return '';
  return url.slice('data:image/png;base64,'.length);
}

interface ShotArgs {
  readonly url: string
  readonly seq: string
  readonly views: readonly string[]
  readonly out: string
}

function parseArgs(argv: readonly string[]): ShotArgs {
  let url = '';
  let seq = 'Stand';
  let view = '';
  let out = '';
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i] ?? '';
    if (arg === '--seq') seq = argv[++i] ?? seq;
    else if (arg === '--view') view = argv[++i] ?? view;
    else if (arg === '--out') out = argv[++i] ?? out;
    else if (!arg.startsWith('--') && url === '') url = arg;
    else if (!arg.startsWith('--') && out === '') out = arg;
  }
  if (url === '') url = 'https://www.wowhead.com/npc=37970/prince-valanar#modelviewer';
  if (!url.includes('#')) url += '#modelviewer';
  const views = view === '' ? [...VIEWS] : view.split(',').map((item) => item.trim()).filter((item) => item !== '');
  for (const name of views) {
    if (!VIEWS.includes(name as typeof VIEWS[number])) {
      throw new Error(`unknown view ${name}. Use ${VIEWS.join(', ')}`);
    }
  }
  return { url, seq, views, out };
}

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));
  const pageUrl = args.url;
  const outDir = args.out !== '' ? args.out : path.join(import.meta.dir, 'out');
  const browser = findBrowser();
  const profile = await mkdtemp(path.join(tmpdir(), 'wh-model-shot-'));
  const proc = spawn(browser, [
    '--headless=new',
    '--remote-debugging-port=0',
    `--user-data-dir=${profile}`,
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-extensions',
    '--hide-scrollbars',
    `--window-size=${WIDTH},${HEIGHT}`,
    '--force-device-scale-factor=1',
    '--enable-webgl',
    '--ignore-gpu-blocklist',
    'about:blank',
  ], { stdio: ['ignore', 'pipe', 'pipe'] });

  let page: WebSocket | undefined;
  try {
    const port = await devtoolsPort(proc);
    page = await openSocket(await pageSocket(port));
    const cdp = new Cdp(page);
    await cdp.send('Page.enable');
    await cdp.send('Page.addScriptToEvaluateOnNewDocument', { source: FORCE_BUFFER });
    await cdp.send('Emulation.setUserAgentOverride', {
      userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36',
      platform: 'Win32',
      userAgentMetadata: {
        brands: [
          { brand: 'Google Chrome', version: '131' },
          { brand: 'Chromium', version: '131' },
          { brand: 'Not_A Brand', version: '24' },
        ],
        fullVersion: '131.0.6778.205',
        platform: 'Windows',
        platformVersion: '15.0.0',
        architecture: 'x86',
        model: '',
        mobile: false,
      },
    });
    await cdp.send('Emulation.setDeviceMetricsOverride', {
      width: WIDTH,
      height: HEIGHT,
      deviceScaleFactor: 1,
      mobile: false,
    });
    await cdp.send('Page.navigate', { url: pageUrl });

    mkdirSync(outDir, { recursive: true });
    const started = Date.now();
    for (const view of args.views) {
      await cdp.send('Runtime.evaluate', {
        expression: `window.__whWantView = ${JSON.stringify(view)}; window.__whWantSeq = ${JSON.stringify(args.seq)}; window.__whSeq = 0;`,
        returnByValue: true,
      });
      const deadline = Date.now() + READY_MS;
      const history: { seq: number; colored: number; boxH: number }[] = [];
      let shot = '';
      let last = 'waiting';
      let anim = args.seq;
      while (Date.now() < deadline) {
        const blocked = await cdp.send('Runtime.evaluate', {
          expression: 'document.title',
          returnByValue: true,
        });
        const title = isRecord(blocked.result) && typeof blocked.result.value === 'string' ? blocked.result.value : '';
        if (title.includes('request could not be satisfied') || title.startsWith('403')) {
          throw new Error(`wowhead blocked the browser (${title})`);
        }
        const probed = await cdp.send('Runtime.evaluate', { expression: SAMPLE, returnByValue: true });
        const value = remoteValue(probed);
        if (value?.phase === 'bad-anim') {
          throw new Error(`no Wowhead animation ${args.seq}. Names include: ${typeof value.names === 'string' ? value.names : ''}`);
        }
        const seq = typeof value?.seq === 'number' ? value.seq : 0;
        const colored = typeof value?.colored === 'number' ? value.colored : 0;
        const boxH = typeof value?.boxH === 'number' ? value.boxH : 0;
        const boxW = typeof value?.boxW === 'number' ? value.boxW : 0;
        const phase = typeof value?.phase === 'string' ? value.phase : '';
        if (typeof value?.anim === 'string' && value.anim !== '') anim = value.anim;
        const span = Math.max(boxH, boxW);
        const dist = typeof value?.dist === 'number' ? value.dist : 0;
        last = `${view} ${phase} seq ${seq} box ${span} colored ${colored} dist ${dist} anim ${anim}`;
        const prev = history[history.length - 1];
        if (!prev || prev.seq !== seq) {
          history.push({ seq, colored, boxH: span });
          console.log(`${Date.now() - started}ms ${last}`);
        }
        const tail = history.slice(-3);
        const colors = tail.map((row) => row.colored);
        const boxes = tail.map((row) => row.boxH);
        const hi = Math.max(...colors);
        const lo = Math.min(...colors);
        const stable = tail.length === 3
          && tail[0].seq > 0
          && tail[2].seq === tail[0].seq + 2
          && Math.min(...boxes) > 0.35
          && lo > 8
          && hi - lo < Math.max(40, hi * 0.15);
        if (stable) {
          shot = await readPng(cdp);
          if (shot !== '') break;
        }
        await Bun.sleep(350);
      }
      if (shot === '') throw new Error(`model viewer did not settle (${last})`);
      const safeSeq = args.seq.replace(/[<>:"/\\|?*]/g, '_');
      const dest = path.join(outDir, `${slugFromUrl(pageUrl)}-${safeSeq}-${view}.png`);
      await writeFile(dest, Buffer.from(shot, 'base64'));
      if (anim !== args.seq) console.log(`animation ${args.seq} -> ${anim}`);
      console.log(path.resolve(dest));
    }
  } finally {
    page?.close();
    killBrowser(proc);
    try {
      rmSync(profile, { recursive: true, force: true });
    } catch {
      // The browser may still be releasing the profile directory.
    }
  }
}

main().then(() => process.exit(0)).catch((err: unknown) => {
  console.error(err instanceof Error ? err.message : err);
  process.exit(1);
});
