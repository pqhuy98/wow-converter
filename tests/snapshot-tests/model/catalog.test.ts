import { expect, test } from 'bun:test';

import { modelCase, type TextureBakingOptions } from './catalog';

const base = {
  base: 'local::models/character.m2',
  weaponR: '',
  weaponL: '',
  size: '',
};

test('preserves texture baking options without a mount', () => {
  const textureBaking: TextureBakingOptions = {
    enabled: true,
    animate: true,
    fps: 15,
    windowMS: 4000,
    resolutionScale: 0.5,
  };
  expect(modelCase({ ...base, textureBaking }, 'fixture')).toEqual({ ...base, textureBaking });
});

test('preserves texture baking options on mounted cases', () => {
  const textureBaking: TextureBakingOptions = { enabled: true, animate: false, resolutionScale: 1 };
  expect(modelCase({ ...base, mount: 'local::models/mount.m2', textureBaking }, 'fixture'))
    .toEqual({ ...base, mount: 'local::models/mount.m2', textureBaking });
});

test('validates texture baking options using the export API limits', () => {
  for (const textureBaking of [
    { enabled: 'yes' },
    { animate: 1 },
    { fps: 0 },
    { fps: 61 },
    { windowMS: 60001 },
    { resolutionScale: 0.75 },
  ]) {
    expect(() => modelCase({ ...base, textureBaking }, 'fixture')).toThrow();
  }
});
