/** Save a PNG of the Wowhead model-viewer canvas. The toolbar and page stay out of the shot. */
import { type ChildProcess, spawn, spawnSync } from 'child_process';
import { existsSync, mkdirSync } from 'fs';
import { writeFile } from 'fs/promises';
import { tmpdir } from 'os';
import path from 'path';
import sharp from 'sharp';

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
    fitN: 0,
    span: 0,
    mid: 0,
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
    const clock0 = model.C && model.C.d;
    window.__whTimeUnit = clock0 && typeof clock0.c === 'number' && clock0.c > 0 && clock0.c <= 1 ? 'frac' : 'ms';
    const bind = actor.getBounds && actor.getBounds();
    const bindSpan = bind && bind[0] && bind[1]
      ? Math.abs(bind[1][0] - bind[0][0]) + Math.abs(bind[1][1] - bind[0][1]) + Math.abs(bind[1][2] - bind[0][2])
      : 0;
    if (bind && bindSpan > 0.01) {
      window.__whBind = [
        [bind[0][0], bind[0][1], bind[0][2]],
        [bind[1][0], bind[1][1], bind[1][2]],
      ];
    }
    const origDraw = renderer.updateCamera.bind(renderer);
    renderer.updateCamera = function() {
      const view = window.__whWantView || 'front';
      const box = window.__whBind;
      origDraw();
      if (box) {
        const target = [
          (box[0][0] + box[1][0]) / 2,
          (box[0][1] + box[1][1]) / 2,
          (box[0][2] + box[1][2]) / 2,
        ];
        renderer.zoom.current = 0;
        renderer.zoom.target = 0;
        const savedCam = window.__whSaved && window.__whSaved[view];
        let dist = savedCam
          ? savedCam.dist
          : fitDist(view, target, boxCorners(box[0], box[1]), renderer.azimuth, renderer.zenith, renderer.fov || 45) * (window.__whScale || 1);
        if (!savedCam) {
          const fov = ((renderer.fov || 45) > 3 ? (renderer.fov || 45) : 45) * Math.PI / 180;
          const tan = Math.tan(fov / 2);
          const eye0 = eyeFor(view, target, dist, renderer.azimuth, renderer.zenith);
          const up0 = view === 'top' || view === 'bottom' ? [0, 1, 0] : [0, 0, 1];
          let zx = eye0[0] - target[0];
          let zy = eye0[1] - target[1];
          let zz = eye0[2] - target[2];
          const zlen = Math.hypot(zx, zy, zz) || 1;
          zx /= zlen; zy /= zlen; zz /= zlen;
          let rx = up0[1] * zz - up0[2] * zy;
          let ry = up0[2] * zx - up0[0] * zz;
          let rz = up0[0] * zy - up0[1] * zx;
          const rlen = Math.hypot(rx, ry, rz) || 1;
          rx /= rlen; ry /= rlen; rz /= rlen;
          const ux = zy * rz - zz * ry;
          const uy = zz * rx - zx * rz;
          const uz = zx * ry - zy * rx;
          const shiftX = (window.__whPanX || 0) * 2 * dist * tan * (1440 / 900);
          const shiftY = -(window.__whPanY || 0) * 2 * dist * tan;
          target[0] += rx * shiftX + ux * shiftY;
          target[1] += ry * shiftX + uy * shiftY;
          target[2] += rz * shiftX + uz * shiftY;
        } else {
          target[0] = savedCam.target[0];
          target[1] = savedCam.target[1];
          target[2] = savedCam.target[2];
        }
        renderer.distance = dist;
        renderer.target[0] = target[0];
        renderer.target[1] = target[1];
        renderer.target[2] = target[2];
        window.__whPlaced = { view, dist, target: [target[0], target[1], target[2]] };
        const eye = eyeFor(view, target, dist, renderer.azimuth, renderer.zenith);
        const up = view === 'top' || view === 'bottom' ? [0, 1, 0] : [0, 0, 1];
        lookAt(renderer.viewMatrix, eye, target, up);
        renderer.eye[0] = eye[0];
        renderer.eye[1] = eye[1];
        renderer.eye[2] = eye[2];
      }
      // Converter SD light: one world-up light, then clamp(N·up + 0.7, 0, 1). Wowhead clamps ambient + primary * max(N·L, 0) to 1, so ambient 0.7 and primary 1 is that same curve. Normals are view-space, so the light direction is world +Z through the view matrix.
      const bag = renderer.gxDevice && renderer.gxDevice.i;
      const viewM = renderer.viewMatrix;
      if (bag && viewM && bag.uAmbientColor && bag.uPrimaryColor && bag.uSecondaryColor && bag.uLightDir1) {
        let ux = viewM[8];
        let uy = viewM[9];
        let uz = viewM[10];
        const ulen = Math.hypot(ux, uy, uz) || 1;
        ux /= ulen;
        uy /= ulen;
        uz /= ulen;
        bag.uAmbientColor[0] = 0.7;
        bag.uAmbientColor[1] = 0.7;
        bag.uAmbientColor[2] = 0.7;
        bag.uPrimaryColor[0] = 1;
        bag.uPrimaryColor[1] = 1;
        bag.uPrimaryColor[2] = 1;
        bag.uSecondaryColor[0] = 0;
        bag.uSecondaryColor[1] = 0;
        bag.uSecondaryColor[2] = 0;
        bag.uLightDir1[0] = ux;
        bag.uLightDir1[1] = uy;
        bag.uLightDir1[2] = uz;
        for (const key of Object.keys(bag)) {
          if (!/spec/i.test(key)) continue;
          const value = bag[key];
          if (value && typeof value.length === 'number' && value.length >= 3) {
            value[0] = 0;
            value[1] = 0;
            value[2] = 0;
          } else if (typeof value === 'number') {
            bag[key] = 0;
          }
        }
      }
    };
    const origAnim = model.aX.bind(model);
    model.aX = function(state, _dt) {
      const clock = model.C && model.C.d;
      if (clock && typeof window.__whTime === 'number') clock.c = window.__whTime;
      if (model.C && typeof window.__whTime === 'number') {
        model.C.a = 1;
        model.C.b = 0;
      }
      const result = origAnim(state, 0);
      if (clock && typeof window.__whTime === 'number') clock.c = window.__whTime;
      return result;
    };
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
  const len = typeof picked.l === 'number' ? picked.l : 0;
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
  renderer.zoom.current = 0;
  renderer.zoom.target = 0;
  if (window.__whFitView !== view) {
    window.__whFitView = view;
    if (!window.__whSaved || !window.__whSaved[view]) {
      window.__whScale = 1;
      window.__whPanX = 0;
      window.__whPanY = 0;
      window.__whFitN = 0;
      window.__whFill = 0;
      window.__whW = 0;
      window.__whH = 0;
      window.__whCx = undefined;
      window.__whCy = undefined;
      window.__whEdgeX = 0;
      window.__whEdgeY = 0;
      window.__whFitSeq = -1;
    }
  }
  const saved = window.__whSaved && window.__whSaved[view];
  if (saved) {
    if (playing !== picked.f) actor.setAnimation(picked.f, true);
    window.__whTime = window.__whTimeUnit === 'frac' || len <= 2 ? 0.5 : len / 2;
    if (model.C && model.C.d) {
      model.C.d.c = window.__whTime;
      model.C.a = 1;
      model.C.b = 0;
    }
  } else {
    const current = model.C && model.C.d && model.C.d.e;
    const curLen = current && typeof current.l === 'number' ? current.l : 0;
    window.__whTime = curLen > 2 ? curLen / 2 : 0;
    if (model.C && model.C.d && window.__whTime) model.C.d.c = window.__whTime;
    if (window.__whSeq !== window.__whFitSeq) {
      window.__whFitSeq = window.__whSeq;
      const aim0 = window.__whAim;
      const useH0 = !aim0 || aim0.h >= aim0.w;
      const clipped = useH0 ? window.__whEdgeY : window.__whEdgeX;
      if (clipped) {
        window.__whScale = Math.min(3, (window.__whScale || 1) * 1.12);
      } else {
        const cx = window.__whCx;
        const cy = window.__whCy;
        const aim = aim0;
        const useH = useH0;
        const got = useH ? (window.__whH || 0) : (window.__whW || 0);
        const goal = aim ? (useH ? aim.h : aim.w) : 0.92;
        const goalCx = aim ? aim.cx : 0.5;
        const goalCy = aim ? aim.cy : 0.5;
        if (got > 0.2 && goal > 0.2 && typeof cx === 'number' && typeof cy === 'number') {
          const ratio = got / goal;
          window.__whScale = Math.max(0.25, Math.min(3, (window.__whScale || 1) * Math.sqrt(ratio)));
          window.__whPanX = (window.__whPanX || 0) + (cx - goalCx) * 0.85;
          window.__whPanY = (window.__whPanY || 0) + (cy - goalCy) * 0.85;
          window.__whFitN = (window.__whFitN || 0) + 1;
          const close = Math.abs(got - goal) <= 0.006 && Math.abs(cx - goalCx) < 0.006 && Math.abs(cy - goalCy) < 0.006;
          if (close) {
            const placed = window.__whPlaced;
            if (placed && placed.view === view) {
              window.__whSaved = window.__whSaved || {};
              window.__whSaved[view] = { dist: placed.dist, target: placed.target };
            }
          }
        }
      }
    }
  }

  base.dist = Math.round((renderer.distance || 0) * 10) / 10;
  base.fitN = window.__whSaved && window.__whSaved[view] ? 100 : (window.__whFitN || 0);
  base.span = Math.round((window.__whFill || 0) * 100) / 100;
  base.mid = window.__whTime || 0;
  base.unit = window.__whTimeUnit || '';
  base.len = len;

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
      let colored = 0;
      if (paint) {
        paint.fillStyle = 'rgb(38,38,38)';
        paint.fillRect(0, 0, full.width, full.height);
        paint.drawImage(img, 0, 0);
        const fw = full.width;
        const fh = full.height;
        const pixels = paint.getImageData(0, 0, fw, fh).data;
        let meshMinX = fw;
        let meshMinY = fh;
        let meshMaxX = -1;
        let meshMaxY = -1;
        for (let y = 0; y < fh; y += 2) {
          for (let x = 0; x < fw; x += 2) {
            const p = (y * fw + x) * 4;
            const r = pixels[p];
            const g = pixels[p + 1];
            const b = pixels[p + 2];
            if (Math.abs(r - 38) <= 6 && Math.abs(g - 38) <= 6 && Math.abs(b - 38) <= 6) continue;
            if (x < meshMinX) meshMinX = x;
            if (y < meshMinY) meshMinY = y;
            if (x > meshMaxX) meshMaxX = x;
            if (y > meshMaxY) meshMaxY = y;
            const hi = r > g ? (r > b ? r : b) : (g > b ? g : b);
            const lo = r < g ? (r < b ? r : b) : (g < b ? g : b);
            if (hi - lo > 22 && hi > 40) colored++;
          }
        }
        if (meshMaxY >= 0) {
          window.__whW = (meshMaxX - meshMinX) / fw;
          window.__whH = (meshMaxY - meshMinY) / fh;
          window.__whFill = Math.max(window.__whW, window.__whH);
          window.__whCx = (meshMinX + meshMaxX) / 2 / fw;
          window.__whCy = (meshMinY + meshMaxY) / 2 / fh;
          window.__whEdgeX = meshMinX <= 2 || meshMaxX >= fw - 4 ? 1 : 0;
          window.__whEdgeY = meshMinY <= 2 || meshMaxY >= fh - 4 ? 1 : 0;
          window.__whBox = Math.round(window.__whH * 100) / 100;
          window.__whBoxW = Math.round(window.__whW * 100) / 100;
        }
      }
      window.__whColored = colored;
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
  const live = model.C && model.C.d && model.C.d.e && model.C.d.e.f;
  return Object.assign(base, { phase: 'pending', anim: live || picked.f });

  function pickAnim(rows, name) {
    const want = name.toLowerCase();
    const exact = rows.find((row) => row.f.toLowerCase() === want);
    if (exact) return exact;
    return rows.find((row) => row.f.toLowerCase().startsWith(want)) || null;
  }

  function boxCorners(min, max) {
    const corners = [];
    for (const x of [min[0], max[0]]) {
      for (const y of [min[1], max[1]]) {
        for (const z of [min[2], max[2]]) corners.push([x, y, z]);
      }
    }
    return corners;
  }

  function eyeFor(view, target, dist, azimuth, zenith) {
    if (view === 'top') return [target[0], target[1], target[2] + dist];
    if (view === 'bottom') return [target[0], target[1], target[2] - dist];
    return [
      -dist * Math.sin(zenith) * Math.cos(azimuth) + target[0],
      -dist * Math.sin(zenith) * Math.sin(azimuth) + target[1],
      -dist * Math.cos(zenith) + target[2],
    ];
  }

  function ndcMax(viewM, fovDeg, aspect, points) {
    const fov = fovDeg > 3 ? fovDeg * Math.PI / 180 : fovDeg;
    const f = 1 / Math.tan(fov / 2);
    let max = 0;
    for (let i = 0; i < points.length; i++) {
      const p = points[i];
      const x = viewM[0] * p[0] + viewM[4] * p[1] + viewM[8] * p[2] + viewM[12];
      const y = viewM[1] * p[0] + viewM[5] * p[1] + viewM[9] * p[2] + viewM[13];
      const z = viewM[2] * p[0] + viewM[6] * p[1] + viewM[10] * p[2] + viewM[14];
      const clipW = -z;
      if (clipW <= 1e-3) return Infinity;
      const ndcX = Math.abs((f / aspect) * x / clipW);
      const ndcY = Math.abs(f * y / clipW);
      if (ndcX > max) max = ndcX;
      if (ndcY > max) max = ndcY;
    }
    return max;
  }

  function fitDist(view, target, corners, azimuth, zenith, fovDeg) {
    const scratch = new Array(16);
    const aspect = 1440 / 900;
    const up = view === 'top' || view === 'bottom' ? [0, 1, 0] : [0, 0, 1];
    const at = (dist) => {
      lookAt(scratch, eyeFor(view, target, dist, azimuth, zenith), target, up);
      return ndcMax(scratch, fovDeg, aspect, corners);
    };
    let lo = 0.05;
    let hi = 4;
    let guard = 0;
    while (at(hi) > 0.92 && guard < 20) {
      hi *= 1.7;
      guard += 1;
    }
    for (let i = 0; i < 24; i++) {
      const mid = (lo + hi) / 2;
      if (at(mid) <= 0.92) hi = mid;
      else lo = mid;
    }
    return hi;
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
})()
`;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}

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
  const candidates = [
    path.join(pf, 'Google/Chrome/Application/chrome.exe'),
    path.join(local, 'Google/Chrome/Application/chrome.exe'),
    path.join(pf, 'Microsoft/Edge/Application/msedge.exe'),
    path.join(pf86, 'Microsoft/Edge/Application/msedge.exe'),
  ];
  for (const candidate of candidates) {
    if (existsSync(candidate)) {
      console.info(`browser: ${candidate}`);
      return candidate;
    }
  }
  for (const name of ['google-chrome', 'google-chrome-stable', 'chromium', 'microsoft-edge', 'chrome', 'msedge']) {
    const found = Bun.which(name);
    if (found) {
      console.info(`browser: ${found}`);
      return found;
    }
  }
  throw new Error('Chrome or Edge is required');
}

const VIEW_ORD: Record<string, string> = {
  front: '01',
  back: '02',
  left: '03',
  right: '04',
  top: '05',
  bottom: '06',
};

async function converterAim(
  outDir: string,
  slug: string,
  seq: string,
  view: string,
): Promise<{ cx: number; cy: number; w: number; h: number } | null> {
  const ord = VIEW_ORD[view];
  if (!ord) return null;
  const file = path.join(outDir, `${slug}-${seq}-${ord}-${view}-converter.png`);
  if (!existsSync(file)) return null;
  const { data, info } = await sharp(file).removeAlpha().raw().toBuffer({ resolveWithObject: true });
  const width = info.width;
  const height = info.height;
  let minX = width;
  let minY = height;
  let maxX = -1;
  let maxY = -1;
  for (let y = 0; y < height; y += 2) {
    for (let x = 0; x < width; x += 2) {
      const i = (y * width + x) * 3;
      const r = data[i] ?? 0;
      const g = data[i + 1] ?? 0;
      const b = data[i + 2] ?? 0;
      if (Math.abs(r - 38) <= 6 && Math.abs(g - 38) <= 6 && Math.abs(b - 38) <= 6) continue;
      if (x < minX) minX = x;
      if (y < minY) minY = y;
      if (x > maxX) maxX = x;
      if (y > maxY) maxY = y;
    }
  }
  if (maxY < 0) return null;
  return {
    cx: (minX + maxX) / 2 / width,
    cy: (minY + maxY) / 2 / height,
    w: (maxX - minX) / width,
    h: (maxY - minY) / height,
  };
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
    try {
      const list: unknown = await fetch(`http://127.0.0.1:${port}/json/list`).then((res) => res.json());
      if (Array.isArray(list)) {
        for (const entry of list) {
          if (isRecord(entry) && entry.type === 'page' && typeof entry.webSocketDebuggerUrl === 'string') {
            return entry.webSocketDebuggerUrl;
          }
        }
      }
    } catch {
      // Devtools is not accepting connections yet.
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

// One Chrome profile for every Wowhead shot. Leave it in place when the browser exits.
function shotProfile(name: string): string {
  const dir = path.join(tmpdir(), name);
  mkdirSync(dir, { recursive: true });
  return dir;
}

async function waitBrowserExit(proc: ChildProcess): Promise<void> {
  killBrowser(proc);
  for (let i = 0; i < 25; i++) {
    if (proc.exitCode != null) return;
    await Bun.sleep(200);
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
  const profile = shotProfile('wh-model-shot');
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
      const aim = await converterAim(outDir, slugFromUrl(pageUrl), args.seq, view);
      if (aim) console.log(`${view} aim cx ${aim.cx.toFixed(3)} cy ${aim.cy.toFixed(3)} w ${aim.w.toFixed(3)} h ${aim.h.toFixed(3)}`);
      const aimJs = aim ? JSON.stringify(aim) : 'null';
      await cdp.send('Runtime.evaluate', {
        expression: `window.__whAim = ${aimJs}; window.__whWantView = ${JSON.stringify(view)}; window.__whWantSeq = ${JSON.stringify(args.seq)}; window.__whSeq = 0;`,
        returnByValue: true,
      });
      const deadline = Date.now() + READY_MS;
      const history: { seq: number; colored: number; boxH: number; fitN: number }[] = [];
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
        const fitN = typeof value?.fitN === 'number' ? value.fitN : 0;
        const framed = typeof value?.span === 'number' ? value.span : 0;
        const mid = typeof value?.mid === 'number' ? value.mid : 0;
        const unit = typeof value?.unit === 'string' ? value.unit : '';
        const len = typeof value?.len === 'number' ? value.len : 0;
        last = `${view} ${phase} seq ${seq} box ${span} frame ${framed} time ${mid} ${unit} len ${len} dist ${dist} anim ${anim}`;
        const prev = history[history.length - 1];
        if (!prev || prev.seq !== seq) {
          history.push({ seq, colored, boxH: span, fitN });
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
          && tail.every((row) => row.fitN === fitN)
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
    await waitBrowserExit(proc);
  }
}

main().then(() => process.exit(0)).catch((err: unknown) => {
  console.error(err instanceof Error ? err.message : err);
  process.exit(1);
});
