// Interactive host for the same embedded renderer used by source screenshots.
if (query.get('interactive') === '1') startViewer().catch(showViewerError);

function showViewerError(error) {
  const status = document.getElementById('viewer-status');
  if (status) status.textContent = String(error);
  document.documentElement.dataset.viewerError = String(error);
}

async function startViewer() {
  document.title = 'WoW Viewer';
  const style = document.createElement('style');
  style.textContent = '#viewer-toolbar{position:absolute;top:12px;left:12px;display:flex;gap:8px;align-items:center;font:14px system-ui;color:white}#viewer-toolbar button,#viewer-toolbar select{background:#333;color:white;border:1px solid #666;border-radius:5px;padding:8px}#viewer-status{position:absolute;top:60px;left:12px;color:white;font:14px system-ui;max-width:80%}#viewer-help{position:absolute;bottom:12px;left:12px;color:#ddd;font:13px system-ui;pointer-events:none}canvas{touch-action:none}';
  document.head.append(style);
  const status = document.createElement('div'); status.id = 'viewer-status'; status.textContent = 'Loading WoW model…'; document.body.append(status);
  const started = performance.now();
  while (!runtimeReady) {
    if (failure) throw new Error(failure);
    if (performance.now() - started > 60000) throw new Error('WoW renderer did not finish loading');
    await new Promise(resolve => setTimeout(resolve, 50));
  }
  disableNativeControls();
  await settleAssets();
  if (failedAssets.length) throw new Error('Failed source assets: ' + failedAssets.join(', '));
  if (selected) Module._setAnimationId(selected.animationID);
  const toolbar = document.createElement('div'); toolbar.id = 'viewer-toolbar';
  const label = document.createElement('span'); label.textContent = info.name; toolbar.append(label);
  const animations = document.createElement('select'); animations.setAttribute('aria-label', 'Animation');
  // Native selection is by animation ID, with variants chosen by the renderer.
  const animationIDs = new Set();
  for (const row of info.metadata.sequences) {
    if (animationIDs.has(row.animationID)) continue;
    animationIDs.add(row.animationID);
    const option = document.createElement('option'); option.value = String(row.index); option.textContent = row.wowName; option.selected = row.animationID === selected?.animationID; animations.append(option);
  }
  animations.disabled = !info.metadata.sequences.length;
  animations.onchange = () => { selected = info.metadata.sequences.find(row => row.index === Number(animations.value)); if (selected) Module._setAnimationId(selected.animationID); };
  let playing = true;
  const play = document.createElement('button'); play.textContent = 'Pause'; play.onclick = () => { playing = !playing; play.textContent = playing ? 'Pause' : 'Play'; };
  const reset = document.createElement('button'); reset.textContent = 'Reset camera';
  if (info.metadata.sequences.length) toolbar.append(animations);
  toolbar.append(play, reset); document.body.append(toolbar);
  if (info.skins?.length) {
    const skins = document.createElement('select'); skins.setAttribute('aria-label', 'Skin');
    for (const skin of info.skins) {
      const option = document.createElement('option'); option.value = skin.id; option.textContent = skin.label; option.selected = skin.id === info.skinId; skins.append(option);
    }
    skins.onchange = () => {
      const skin = info.skins.find(row => row.id === skins.value);
      if (!skin) return;
      setTextureReplacements(skin.textureReplacements);
      updateCamera();
      info.skinId = skin.id;
      const url = new URL(window.location.href); url.searchParams.set('skin', skin.id); window.history.replaceState(null, '', url);
    };
    toolbar.insertBefore(skins, animations);
  }
  const help = document.createElement('div'); help.id = 'viewer-help'; help.textContent = 'Left: rotate · Right: move on ground · Middle or left+right: pan · Wheel: zoom · Touch: rotate / pinch'; document.body.append(help);
  const bounds = info.viewerBounds;
  const radius = Math.max(0, ...bounds.Min.map((v, i) => bounds.Max[i] - v)) / 2;
  const orbit = { horizontalAngle: 0, verticalAngle: Math.PI / 6, distance: 1, target: [0, 0, 0] };
  let pendingCamera;
  const clampDistance = () => { orbit.distance = Math.max(1 / 56, Math.min(2000000 / 56, orbit.distance)); };
  const updateCamera = () => {
    const { distance: d, horizontalAngle: h, verticalAngle: v, target } = orbit;
    pendingCamera = { target: [...target], eye: [target[0] + d * Math.cos(v) * Math.cos(h), target[1] + d * Math.cos(v) * Math.sin(h), target[2] + d * Math.sin(v)], up: [0, 0, 1] };
  };
  reset.onclick = () => { orbit.horizontalAngle = 0; orbit.verticalAngle = Math.PI / 6; orbit.distance = Math.max(200 / 56, Math.min(1000 / 56, radius * 5)); orbit.target = [(bounds.Min[0] + bounds.Max[0]) / 2, (bounds.Min[1] + bounds.Max[1]) / 2, 0]; updateCamera(); };
  reset.click();
  // Native post-load framing can arrive after the first camera application.
  await settleAssets();
  reset.click();
  const rotate = (dx, dy) => { orbit.horizontalAngle -= dx * Math.PI / 360; orbit.verticalAngle = Math.max(-Math.PI / 2 + 0.01, Math.min(Math.PI / 2 - 0.01, orbit.verticalAngle + dy * Math.PI / 360)); };
  const normalize = vector => { const length = Math.hypot(...vector); return length ? vector.map(v => v / length) : [0, 0, 0]; };
  const pan = (dx, dy, ground) => {
    const h = orbit.horizontalAngle, v = orbit.verticalAngle;
    const right = normalize([-Math.sin(h), Math.cos(h), 0]);
    const up = normalize([-Math.sin(v) * Math.cos(h), -Math.sin(v) * Math.sin(h), ground ? 0 : Math.cos(v)]);
    const sw = -dx / canvas.width * orbit.distance, sh = dy / canvas.height * orbit.distance;
    orbit.target = orbit.target.map((value, i) => value + right[i] * sw + up[i] * sh);
  };
  let buttons = 0, lastX = 0, lastY = 0, pinchDistance = 0;
  canvas.oncontextmenu = event => event.preventDefault();
  canvas.addEventListener('mousedown', event => { event.preventDefault(); buttons = event.buttons; lastX = event.clientX; lastY = event.clientY; });
  window.addEventListener('mouseup', event => { buttons = event.buttons; });
  window.addEventListener('blur', () => { buttons = 0; });
  window.addEventListener('mousemove', event => {
    if (!buttons) return;
    const dx = event.clientX - lastX, dy = event.clientY - lastY; lastX = event.clientX; lastY = event.clientY;
    if ((buttons & 4) || (buttons & 3) === 3) pan(dx, dy, false);
    else if (buttons & 1) rotate(dx, dy);
    else if (buttons & 2) pan(dx, dy, true);
    updateCamera();
  });
  canvas.addEventListener('wheel', event => { event.preventDefault(); orbit.distance *= 1 + (event.deltaY > 0 ? 0.1 : -0.1); clampDistance(); updateCamera(); }, { passive: false });
  const touchStart = event => { event.preventDefault(); lastX = event.touches[0]?.clientX ?? 0; lastY = event.touches[0]?.clientY ?? 0; pinchDistance = event.touches.length === 2 ? Math.hypot(event.touches[0].clientX - event.touches[1].clientX, event.touches[0].clientY - event.touches[1].clientY) : 0; };
  canvas.addEventListener('touchstart', touchStart, { passive: false });
  canvas.addEventListener('touchmove', event => {
    event.preventDefault();
    if (event.touches.length === 1) { rotate(event.touches[0].clientX - lastX, event.touches[0].clientY - lastY); lastX = event.touches[0].clientX; lastY = event.touches[0].clientY; }
    else if (event.touches.length === 2) { const distance = Math.hypot(event.touches[0].clientX - event.touches[1].clientX, event.touches[0].clientY - event.touches[1].clientY); if (pinchDistance > 0) orbit.distance *= 2 - distance / pinchDistance; pinchDistance = distance; clampDistance(); }
    updateCamera();
  }, { passive: false });
  canvas.addEventListener('touchend', touchStart, { passive: false });
  window.addEventListener('resize', () => { canvas.width = Math.max(1, innerWidth); canvas.height = Math.max(1, innerHeight); Module._setSceneSize(canvas.width, canvas.height); updateCamera(); });
  let previous = performance.now();
  const frame = now => {
    if (runtimeAborted) { showViewerError(failure); return; }
    // A rejected browser graphics promise need not abort the native renderer.
    try {
      let dt = playing ? Math.min((now - previous) / 1000, 0.1) : 0;
      if (pendingCamera) {
        setCamera(pendingCamera, false);
        pendingCamera = undefined;
        // Native camera tick uses milliseconds. Settle its spring before drawing,
        // including paused playback; 0.2 ms has negligible animation advancement.
        dt = Math.max(dt, 0.0002);
      }
      Module._gameloop(dt);
    }
    catch (error) { showViewerError(error); return; }
    previous = now; requestAnimationFrame(frame);
  };
  // Prime the native render pipeline with the final default camera before
  // revealing the canvas. Loading/autofit frames must never become visible.
  // A newly textured model can complete native scene setup on positive-time
  // updates, after zero-time asset polling has already gone idle.
  for (let i = 0; i < 5; i++) Module._gameloop(0.0002);
  await settleAssets();
  setCamera(pendingCamera, false);
  pendingCamera = undefined;
  Module._gameloop(0.0002);
  for (let i = 0; i < 5; i++) Module._gameloop(0);
  canvas.style.visibility = 'visible';
  status.textContent = ''; document.documentElement.dataset.viewerReady = 'true'; requestAnimationFrame(frame);
}

function disableNativeControls() {
  // The vendored GLFW host installs capture-phase input handlers independently
  // of hammerJsAssignControl. They must not compete with our orbit controller.
  for (const [event, callback] of [
    ['mousemove', 'onMousemove'], ['mousedown', 'onMouseButtonDown'],
    ['mouseup', 'onMouseButtonUp'], ['wheel', 'onMouseWheel'],
    ['mousewheel', 'onMouseWheel'], ['mouseenter', 'onMouseenter'],
    ['mouseleave', 'onMouseleave'], ['touchmove', 'onMousemove'],
    ['touchstart', 'onMouseButtonDown'], ['touchcancel', 'onMouseButtonUp'],
    ['touchend', 'onMouseButtonUp'], ['click', 'onClickRequestPointerLock'],
  ]) canvas.removeEventListener(event, GLFW[callback], true);
  for (const [event, callback] of [['keydown', 'onKeydown'], ['keypress', 'onKeyPress'], ['keyup', 'onKeyup']]) {
    window.removeEventListener(event, GLFW[callback], true);
  }
  // GLFW_CURSOR / GLFW_CURSOR_NORMAL also releases any existing pointer lock.
  if (GLFW.active) GLFW.setInputMode(GLFW.active.id, 208897, 212993);
}
