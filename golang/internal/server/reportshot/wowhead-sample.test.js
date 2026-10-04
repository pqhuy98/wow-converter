import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';
import { runInNewContext } from 'node:vm';

const script = readFileSync(new URL('./wowhead-sample.js', import.meta.url), 'utf8');

for (const classic of [false, true]) {
  test(`${classic ? 'classic' : 'retail'} selects the exact variant and freezes its midpoint`, () => {
    const rows = classic
      ? [{ i: 'Stand', l: 0, f: 1000 }, { i: 'Stand', l: 1, f: 4000 }]
      : [{ f: 'Stand', a: 0, l: 1000 }, { f: 'Stand', a: 1, l: 4000 }];
    const group = classic ? { d: { c: rows[0], b: 300, d: 0 }, e: 0, c: 1 } : { d: { e: rows[0], c: 300 }, a: 0, b: 1 };
    const time = classic ? 'b' : 'c';
    const advance = classic ? 'X' : 'aX';
    const model = classic ? { aq: { f: rows, U: () => false }, Q: group } : { bf: { B: rows }, C: group };
    model[advance] = (state, dt) => { state.d[time] += dt; };
    const actor = {
      [classic ? 'b' : 'a']: model,
      getBounds: () => [[-1, 4, 0], [3, 8, 3]],
      setAnimation: () => {},
      setAnimNoSubAnim: () => {},
    };
    const renderer = {
      actors: [actor], canvas: {}, clearBackground: () => {}, onResize: () => {}, updateCamera: () => {},
      options: {}, clearColor: [0, 0, 0], zoom: { current: 0, target: 0 },
      target: [0, 0, 0], eye: [0, 0, 0], viewMatrix: new Array(16).fill(0),
      fov: 45, azimuth: 0, zenith: Math.PI / 2, distance: 0,
    };
    const context = { window: { __whViewer: { renderer }, __whWantSeq: 'Stand', __whWantVariant: 1 } };
    assert.equal(runInNewContext(script, context).phase, 'init');
    assert.equal(runInNewContext(script, context).phase, 'pending');
    assert.equal(group.d[classic ? 'c' : 'e'], rows[1]);
    assert.equal(group.d[time], 2000);
    model[advance](group, 500);
    assert.equal(group.d[time], 2000);
    if (classic) assert.equal(group.d.d, 1);
    Object.assign(context.window, {
      __whAim: { cx: 0.45, cy: 0.55, w: 0.3, h: 0.8 },
      __whH: 0.4, __whW: 0.2, __whCx: 0.6, __whCy: 0.4,
    });
    runInNewContext(script, context);
    assert.equal(context.window.__whScale, 1);
    context.window.__whWantView = 'front';
    renderer.updateCamera();
    assert.equal(JSON.stringify(context.window.__whPlaced.target), '[1,6,1.5]');
    context.window.__whWantView = 'top';
    renderer.updateCamera();
    assert.equal(JSON.stringify(context.window.__whPlaced.target), '[1,6,1.5]');
    context.window.__whWantView = 'bottom';
    renderer.updateCamera();
    assert.equal(JSON.stringify(context.window.__whPlaced.target), '[1,6,1.5]');
  });
}

for (const refined of [false, true]) {
  test(`visible small side shot ${refined ? 'finishes after refinement' : 'can refine its camera'}`, () => {
    const context = smallShotContext(false, refined);
    runInNewContext(script, context);
    assert.ok(context.window.__whColored > 10);
    assert.ok(context.window.__whW < 0.2 && context.window.__whH < 0.2);
    assert.equal(context.window.__whMeasuring, false);
    if (refined) {
      assert.equal(context.window.__whPng, 'captured-small-model');
      assert.equal(context.window.__whSaved.left.dist, 40);
    } else {
      assert.equal(context.window.__whRefined.left, true);
      assert.ok(context.window.__whScale > 0);
      assert.equal(context.window.__whPng, undefined);
    }
  });
}

test('small-shot acceptance still rejects an empty canvas', () => {
  const context = smallShotContext(true, true);
  runInNewContext(script, context);
  assert.equal(context.window.__whColored, 0);
  assert.equal(context.window.__whPng, undefined);
  assert.equal(context.window.__whSaved, undefined);
  assert.equal(context.window.__whMeasuring, false);
});

function smallShotContext(blank, refined) {
  const row = { f: 'Stand', a: 0, l: 1000 };
  const model = { bf: { B: [row] }, C: { d: { e: row, c: 500 }, a: 1, b: 0 } };
  const renderer = { actors: [{ a: model }], canvas: {}, zoom: { current: 0, target: 0 } };
  const pixels = new Uint8ClampedArray(100 * 100 * 4);
  for (let y = 0; y < 100; y++) {
    for (let x = 0; x < 100; x++) {
      const offset = (y * 100 + x) * 4;
      const foreground = !blank && x >= 40 && x < 59 && y >= 40 && y < 58;
      pixels.set(foreground ? [200, 80, 40, 255] : [38, 38, 38, 255], offset);
    }
  }
  const paint = {
    fillRect() {}, drawImage() {}, getImageData: () => ({ data: pixels }),
  };
  return {
    window: {
      __whViewer: { renderer }, __whInit: true, __whWantSeq: 'Stand', __whWantView: 'left',
      __whShot: 'rendered-frame', __whShotPlacement: { view: 'left', dist: 40, target: [0, 0, 1], scale: 2.5 },
      __whAim: { cx: 0.5, cy: 0.5, w: 0.18, h: 0.17 },
      __whRefined: refined ? { left: true } : {},
    },
    document: {
      createElement: () => ({ getContext: () => paint, toDataURL: () => 'captured-small-model' }),
    },
    Image: class {
      width = 100;
      height = 100;
      set src(_value) { this.onload(); }
    },
  };
}
