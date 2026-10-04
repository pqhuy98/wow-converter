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
  const classic = actor && actor.b && actor.b.aq;
  const model = actor && (classic ? actor.b : actor.a);
  // Classic and retail ship independently minified viewer builds.
  const fields = classic
    ? { rows: 'f', name: 'i', variant: 'l', length: 'f', state: 'Q', clip: 'c', time: 'b', advance: 'X' }
    : { rows: 'B', name: 'f', variant: 'a', length: 'l', state: 'C', clip: 'e', time: 'c', advance: 'aX' };
  const data = model && (classic ? model.aq : model.bf);
  const animations = data && data[fields.rows];
  const state = model && model[fields.state];
  const clock = state && state.d;
  if (!model || !animations || !clock) return Object.assign(base, { phase: 'loading' });

  if (!window.__whInit) {
    renderer.clearBackground();
    renderer.bgTexture = null;
    renderer.options.background = '';
    renderer.clearColor[0] = 0.15;
    renderer.clearColor[1] = 0.15;
    renderer.clearColor[2] = 0.15;
    renderer.fov = 45;
    renderer.onResize(1440, 900, 1440 / 900);
    const clock0 = clock;
    window.__whTimeUnit = clock0 && !classic && typeof clock0[fields.time] === 'number' && clock0[fields.time] > 0 && clock0[fields.time] <= 1 ? 'frac' : 'ms';
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
        window.__whPlaced = { view, dist, target: [target[0], target[1], target[2]], scale: window.__whScale || 1, panX: window.__whPanX || 0, panY: window.__whPanY || 0 };
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
    actor.setAnimNoSubAnim && actor.setAnimNoSubAnim(true);
    const origAnim = model[fields.advance].bind(model);
    model[fields.advance] = function(animationState, _dt) {
      const clock = state && state.d;
      if (clock && typeof window.__whTime === 'number') clock[fields.time] = window.__whTime;
      if (state && typeof window.__whTime === 'number') {
        if (classic) { state.e = 1; state.c = 0; } else { state.a = 1; state.b = 0; }
      }
      const result = origAnim(animationState, 0);
      if (clock && typeof window.__whTime === 'number') clock[fields.time] = window.__whTime;
      return result;
    };
    window.__whInit = true;
    return Object.assign(base, { phase: 'init' });
  }

  const want = String(window.__whWantSeq || 'Stand');
  const rows = animations.filter((row) => row && typeof row[fields.name] === 'string' && row[fields.variant] === (window.__whWantVariant || 0));
  const picked = pickAnim(rows, want);
  if (!picked) {
    return Object.assign(base, {
      phase: 'bad-anim',
      names: rows.map((row) => row[fields.name]).slice(0, 24).join(', '),
    });
  }
  window.__whAnim = picked[fields.name];
  const playing = clock[fields.clip] && clock[fields.clip][fields.name];
  const len = typeof picked[fields.length] === 'number' ? picked[fields.length] : 0;
  const index = animations.indexOf(picked);
  if (classic && data.U(index)) {
    if (!window.__whLoadingAnimation) {
      window.__whLoadingAnimation = true;
      data.i(index).finally(() => { window.__whLoadingAnimation = false; });
    }
    return Object.assign(base, { phase: 'loading-animation' });
  }
  if (playing !== picked[fields.name]) actor.setAnimation(picked[fields.name], true);
  clock[fields.clip] = picked;
  if (classic) clock.d = index;
  window.__whTime = window.__whTimeUnit === 'frac' ? 0.5 : len / 2;
  clock[fields.time] = window.__whTime;
  if (classic) { state.e = 1; state.c = 0; } else { state.a = 1; state.b = 0; }
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
  const saved = window.__whSaved && window.__whSaved[view];
  if (!saved && window.__whFitView !== view) {
    window.__whFitView = view;
    window.__whScale = 1;
    window.__whPanX = 0;
    window.__whPanY = 0;
  }

  base.dist = Math.round((renderer.distance || 0) * 10) / 10;
  base.fitN = saved ? 100 : (window.__whRefined && window.__whRefined[view] ? 1 : 0);
  base.span = Math.round((window.__whFill || 0) * 100) / 100;
  base.mid = window.__whTime || 0;
  base.unit = window.__whTimeUnit || '';
  base.len = len;

  if (window.__whMeasuring) return Object.assign(base, { phase: 'measuring', anim: picked[fields.name] });
  if (window.__whShot) {
    const url = window.__whShot;
    const placement = window.__whShotPlacement;
    window.__whShot = '';
    window.__whMeasuring = true;
    const img = new Image();
    img.onload = () => {
      if (!placement || placement.view !== (window.__whWantView || 'front')) {
        window.__whMeasuring = false;
        return;
      }
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
        for (let y = 0; y < fh; y += 1) {
          for (let x = 0; x < fw; x += 1) {
            const p = (y * fw + x) * 4;
            const r = pixels[p];
            const g = pixels[p + 1];
            const b = pixels[p + 2];
            if (Math.abs(r - 38) <= 2 && Math.abs(g - 38) <= 2 && Math.abs(b - 38) <= 2) continue;
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
          window.__whBox = Math.round(window.__whH * 100) / 100;
          window.__whBoxW = Math.round(window.__whW * 100) / 100;
        }
      }
      window.__whColored = colored;
      window.__whSeq = (window.__whSeq || 0) + 1;
      const locked = window.__whSaved && window.__whSaved[placement.view];
      const aim = window.__whAim;
      const gotH = window.__whH || 0;
      const gotW = window.__whW || 0;
      const useH = aim ? aim.h >= aim.w : gotH >= gotW;
      const got = useH ? gotH : gotW;
      if (colored < 10 || got <= 0) {
        window.__whMeasuring = false;
        return;
      }
      if (!locked && aim && !(window.__whRefined && window.__whRefined[placement.view])
        && placement.view !== 'top' && placement.view !== 'bottom') {
        const ratio = got / (useH ? aim.h : aim.w);
        window.__whScale = Math.max(0.25, Math.min(3, (placement.scale || 1) * ratio));
        window.__whPanX = ((placement.panX || 0) + window.__whCx - 0.5) / ratio - (aim.cx - 0.5);
        window.__whPanY = ((placement.panY || 0) + window.__whCy - 0.5) / ratio - (aim.cy - 0.5);
        window.__whRefined = window.__whRefined || {};
        window.__whRefined[placement.view] = true;
        window.__whMeasuring = false;
        return;
      }
      window.__whSaved = window.__whSaved || {};
      if (!locked) window.__whSaved[placement.view] = { dist: placement.dist, target: placement.target };
      window.__whPng = paint ? full.toDataURL('image/png') : url;
      window.__whMeasuring = false;
    };
    img.src = url;
    return Object.assign(base, { phase: 'measuring', anim: picked[fields.name] });
  }
  if (!window.__whPending) {
    window.__whPending = true;
    renderer.screenshotCallback = () => {
      window.__whShot = renderer.screenshotDataURL;
      window.__whShotPlacement = window.__whPlaced;
      window.__whPending = false;
    };
    renderer.makeDataURL = ['image/png'];
  }
  const live = clock[fields.clip] && clock[fields.clip][fields.name];
  return Object.assign(base, { phase: 'pending', anim: live || picked[fields.name] });

  function pickAnim(rows, name) {
    const want = name.toLowerCase();
    const exact = rows.find((row) => row[fields.name].toLowerCase() === want);
    if (exact) return exact;
    return rows.find((row) => row[fields.name].toLowerCase().startsWith(want)) || null;
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
