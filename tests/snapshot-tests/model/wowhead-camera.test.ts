import { expect, test } from 'bun:test';

import { wantsWowheadShot } from './catalog';
import { readCameras, scaleCameras, wowheadPairHash } from './wowhead-camera';

const camera = {
  eye: [0, -5, 1],
  target: [0, 0, 1],
  up: [0, 0, 1],
  modelMatrix: [1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1],
  height: 2,
  modelScale: 56,
};
const source = { wowName: 'Stand', wowVariant: 3 };

test('requires a complete finite camera map from the current units protocol', () => {
  expect(readCameras({ front: camera }, ['front', 'back'])).toBeUndefined();
  expect(readCameras({ front: { ...camera, modelScale: undefined } }, ['front'])).toBeUndefined();
  expect(readCameras({ front: { ...camera, eye: [0, NaN, 1] } }, ['front'])).toBeUndefined();
  expect(readCameras({ front: camera }, ['front'])).toEqual({ front: camera });
});

test('pairs the image, URL, source animation variant, and cameras', () => {
  const cameras = { front: camera };
  const png = new Uint8Array([1, 2, 3]);
  const hash = wowheadPairHash('https://www.wowhead.com/npc=1', source, cameras, png);
  expect(wowheadPairHash('https://www.wowhead.com/npc=2', source, cameras, png)).not.toBe(hash);
  expect(wowheadPairHash('https://www.wowhead.com/npc=1', { ...source, wowVariant: 0 }, cameras, png)).not.toBe(hash);
  expect(wowheadPairHash('https://www.wowhead.com/npc=1', source, { front: { ...camera, eye: [0, -6, 1] } }, png)).not.toBe(hash);
  expect(wowheadPairHash('https://www.wowhead.com/npc=1', source, cameras, new Uint8Array([1, 2, 4]))).not.toBe(hash);
});

test('uses the fresh export scale without mutating the cached source camera', () => {
  expect(scaleCameras({ front: camera }, 84).front.modelScale).toBe(84);
  expect(camera.modelScale).toBe(56);
  expect(() => scaleCameras({ front: camera }, 0)).toThrow();
});

test('only pairs plain Wowhead cases without external weapons or mounts', () => {
  const model = {
    base: 'https://www.wowhead.com/npc=1', weaponR: '', weaponL: '', size: '',
  };
  expect(wantsWowheadShot('retail', model)).toBe(true);
  expect(wantsWowheadShot('classic', model)).toBe(true);
  expect(wantsWowheadShot('mount', model)).toBe(false);
  expect(wantsWowheadShot('retail', { ...model, mount: 'https://www.wowhead.com/npc=2' })).toBe(false);
  expect(wantsWowheadShot('retail', { ...model, weaponL: 'local::weapon.m2' })).toBe(false);
  expect(wantsWowheadShot('retail', { ...model, base: 'local::creature.m2' })).toBe(false);
});
