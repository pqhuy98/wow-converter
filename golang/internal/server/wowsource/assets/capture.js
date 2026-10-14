/* The native renderer advances only when asked. Asset loading uses zero-time frames. */
const canvas = document.getElementById('canvas');
const query = new URLSearchParams(location.search);
if (query.get('interactive') === '1') canvas.style.visibility = 'hidden';
let pending = 0;
let activity = performance.now();
let runtimeReady = false;
let animationsReady = false;
let info;
let selected;
let failure = '';
let runtimeAborted = false;
const failedAssets = [];
window.addEventListener('error', (event) => { failure = event.message; });
window.addEventListener('unhandledrejection', (event) => { failure = String(event.reason); });

const open = XMLHttpRequest.prototype.open;
XMLHttpRequest.prototype.open = function(method, url, ...rest) {
  if (String(url).includes('/api/wow-source/raw')) {
    pending++;
    activity = performance.now();
    this.addEventListener('loadend', () => {
      pending--;
      activity = performance.now();
      if (this.status !== 200) failedAssets.push(String(url));
    }, { once: true });
  }
  return open.call(this, method, url, ...rest);
};
const getContext = HTMLCanvasElement.prototype.getContext;
HTMLCanvasElement.prototype.getContext = function(kind, options) {
  if (kind.startsWith('webgl') || kind === 'experimental-webgl') options = { ...options, preserveDrawingBuffer: true };
  return getContext.call(this, kind, options);
};

var Module = {
  canvas,
  locateFile: (name) => '/wow-source/' + name,
  printErr: (message) => console.error(message),
  onAbort: (message) => { runtimeAborted = true; failure = String(message); },
  animationArrayCallback: () => { animationsReady = true; },
  meshIdArrayCallback: () => {},
  onRuntimeInitialized: async function() {
    try {
      this.hammerJsAssignControl = () => {};
      const input = query.get('model') || '';
      const key = /^\d+$/.test(input) ? 'id' : 'name';
      const response = await fetch('/api/wow-source/info?' + key + '=' + encodeURIComponent(input) + (query.get('interactive') === '1' ? '&viewer=1&skin=' + encodeURIComponent(query.get('skin') || '') : ''));
      if (!response.ok) throw new Error(await response.text());
      info = await response.json();
      const sequence = query.get('seq') || 'Stand';
      const variant = Number(query.get('variant') || 0);
      selected = info.metadata.sequences.find((row) => row.wowName.toLowerCase() === sequence.toLowerCase() && row.wowVariant === variant);
      if (!selected && query.get('interactive') === '1') selected = info.metadata.sequences[0];
      if (!selected && info.kind !== 'wmo' && query.get('interactive') !== '1') throw new Error('Source animation not found: ' + sequence + ' variant ' + variant);
      const byName = allocateUTF8('/api/wow-source/raw?name=');
      const byID = allocateUTF8('/api/wow-source/raw?id=');
      this._createWebJsScene(canvas.width, canvas.height, byName, byID);
      canvas.width = query.get('interactive') === '1' ? window.innerWidth : 1440;
      canvas.height = query.get('interactive') === '1' ? window.innerHeight : 900;
      this._setSceneSize(canvas.width, canvas.height);
      this._free(byName); this._free(byID);
      this._setClearColor(38 / 255, 38 / 255, 38 / 255);
      this._setFarPlane(10000); this._setFarPlaneForCulling(10000);
      this._setSceneFileDataId(info.kind === 'wmo' ? 1 : 0, info.fileDataID, -1);
      const replacements = query.get('textures');
      if (replacements) setTextureReplacements(replacements.split(',').map(Number));
      else if (info.textureReplacements) setTextureReplacements(info.textureReplacements);
      runtimeReady = true;
    } catch (error) { failure = String(error); }
  },
};

function setTextureReplacements(replacements) {
  const ids = Int32Array.from(replacements);
  const ptr = Module._malloc(ids.byteLength);
  try { Module.HEAP32.set(ids, ptr / 4); Module._setTextures(ptr, ids.length); }
  finally { Module._free(ptr); }
}

window.__sourceStatus = () => {
  if (runtimeReady) Module._gameloop(0);
  return { ready: runtimeReady && pending === 0 && performance.now() - activity > 750,
    error: failure, failedAssets, pending, runtimeReady, animationsReady, info, selected };
};

window.__sourceShot = async (view, reference, time) => {
  if (failedAssets.length) throw new Error('Failed source assets: ' + failedAssets.join(', '));
  await settleAssets();
  if (selected) Module._setAnimationId(selected.animationID);
  const camera = reference ? sourceCamera(reference) : fitCamera(view, info.bounds);
  setCamera(camera);
  await settleAssets();
  // A particle-only model may not report an animation-list callback. Complete
  // native post-load framing before applying the requested camera and animation.
  if (selected) Module._setAnimationId(selected.animationID);
  setCamera(camera);
  if (failedAssets.length) throw new Error('Failed source assets: ' + failedAssets.join(', '));
  const elapsed = time < 0 ? (selected?.duration || 0) / 2000 : time;
  for (let t = 0; t < elapsed;) {
    const dt = Math.min(1 / 60, elapsed - t);
    Module._gameloop(dt);
    // Native upload/fence callbacks need a browser frame between simulation ticks.
    await new Promise((resolve) => setTimeout(resolve, 16));
    t += dt;
  }
  // The renderer pipelines updates/draws; drain without advancing the simulation.
  for (let i = 0; i < 5; i++) Module._gameloop(0);
  await settleAssets();
  if (failedAssets.length) throw new Error('Failed source assets: ' + failedAssets.join(', '));
  return canvas.toDataURL('image/png');
};

async function settleAssets() {
  const started = performance.now();
  do {
    Module._gameloop(0);
    await new Promise((resolve) => setTimeout(resolve, 16));
    if (performance.now() - started > 45000) throw new Error('Source assets did not finish loading');
  } while (pending > 0 || performance.now() - activity < 750 || performance.now() - started < 750);
}

function sourceCamera(reference) {
  const m = reference.modelMatrix;
  // Wowhead's model matrix is a rotation with uniform display scale. Invert it
  // here rather than changing either established screenshot camera.
  const s2 = m[0] ** 2 + m[1] ** 2 + m[2] ** 2;
  if (m.length !== 16 || !m.every(Number.isFinite) || s2 <= 0) throw new Error('Invalid reference model matrix');
  const transform = (p, point) => {
    const x = p[0] - (point ? m[12] : 0), y = p[1] - (point ? m[13] : 0), z = p[2] - (point ? m[14] : 0);
    return [(m[0]*x+m[1]*y+m[2]*z)/s2, (m[4]*x+m[5]*y+m[6]*z)/s2, (m[8]*x+m[9]*y+m[10]*z)/s2];
  };
  return { target: transform(reference.target, true), eye: transform(reference.eye, true), up: transform(reference.up, false) };
}

function fitCamera(view, bounds) {
  const target = bounds.Min.map((v, i) => (v + bounds.Max[i]) / 2);
  const extent = bounds.Min.map((v, i) => Math.abs(bounds.Max[i] - v));
  const direction = { front: [1,0,0], left: [0,1,0], back: [-1,0,0], right: [0,-1,0], top: [0,0,1], bottom: [0,0,-1] }[view];
  const horizontal = direction[0] ? extent[1] : extent[0];
  const vertical = direction[2] ? extent[1] : extent[2];
  const depth = direction[0] ? extent[0] : direction[1] ? extent[1] : extent[2];
  const distance = Math.max(1, vertical, horizontal / (1440/900)) / (2 * Math.tan(Math.PI/8) * 0.9) + depth/2;
  return { target, eye: target.map((v, i) => v + direction[i] * distance), up: direction[2] ? [0,1,0] : [0,0,1] };
}

function setCamera({ target, eye, up }, advance = true) {
  if (info.kind === 'wmo') {
    // The vendored WmoScene's MODF placement is Rz(180) plus the float32
    // ADT-world origin offset: 32*TILESIZE - (17066.6738,17064.6621).
    const place = p => [-p[0] - 0.0078125, -p[1] + 2.00390625, p[2]];
    target = place(target); eye = place(eye);
    up = [-up[0], -up[1], up[2]];
  }
  window.__sourceCamera = {target, eye, up};
  const delta = eye.map((v, i) => v - target[i]);
  const distance = Math.hypot(...delta);
  if (![...target,...eye,...up].every(Number.isFinite) || distance <= 0) throw new Error('Invalid source camera');
  const normal = delta.map((v) => v / distance);
  if (info.kind === 'wmo') {
    // WMOScene uses FirstPersonCamera, not M2Scene's centered planar camera.
    const yaw = Math.atan2(-normal[1], -normal[0]) * 180 / Math.PI;
    const pitch = Math.max(-89, Math.min(89, Math.asin(-normal[2]) * 180 / Math.PI));
    Module._setScenePos(...eye);
    // The bundled 2022 FirstPersonCamera resets its angles in setScenePos.
    Module._addHorizontalViewDir(-yaw);
    Module._addVerticalViewDir(-pitch);
    if (advance) Module._gameloop(0.0002);
    return;
  }
  const right0 = [up[1]*normal[2]-up[2]*normal[1], up[2]*normal[0]-up[0]*normal[2], up[0]*normal[1]-up[1]*normal[0]];
  if (Math.hypot(...right0) < 1e-8) throw new Error('Camera up is parallel to its direction');
  const right = right0.map((v) => v / Math.hypot(...right0));
  const vertical = [normal[1]*right[2]-normal[2]*right[1], normal[2]*right[0]-normal[0]*right[2], normal[0]*right[1]-normal[1]*right[0]];
  const dot = (a, b) => a.reduce((sum, v, i) => sum + v*b[i], 0);
  // Native M2Scene centers on sequence-zero bounds, or collision bounds without inline sequences.
  let box = info.orbitBounds;
  let span = box.Min.map((v, i) => box.Max[i] - v);
  if (dot(span, span) < 0.001) { box = info.bounds; span = box.Min.map((v, i) => box.Max[i] - v); }
  const center = dot(span, span) < 20000 ? box.Min.map((v, i) => (v + box.Max[i]) / 2) : [0,0,0];
  const relative = target.map((v, i) => v - center[i]);
  Module._setScenePos(1, 0, 0);
  const yaw = Math.atan2(-right[0], -right[1]) * 180 / Math.PI;
  Module._addHorizontalViewDir(yaw - 180);
  Module._addVerticalViewDir(Math.asin(normal[2]) * 180 / Math.PI);
  Module._zoomInFromMouseScroll(distance + dot(relative, normal) - 1);
  Module._addCameraViewOffset(-dot(relative, right), -dot(relative, vertical));
  // Tick requires positive time for the camera spring, before animation starts.
  if (advance) Module._gameloop(0.000001);
}
