import {
  beforeEach, describe, expect, test,
} from 'bun:test';

import {
  clearTextureSources,
  hasTextureSource,
  registerTextureSource,
  releaseGeneratedPngSources,
  releaseTextureSourcePaths,
} from '@/lib/converter/common/texture-source';

describe('texture source lifecycle', () => {
  beforeEach(() => {
    clearTextureSources();
  });

  test('releases PNG and BLP sources owned by a completed export', () => {
    registerTextureSource('textures/skin.png', { kind: 'png', png: Buffer.from('png') });
    registerTextureSource('textures/cloak.png', { kind: 'blp', fileDataID: 123 });

    expect(releaseTextureSourcePaths(['textures/skin.png', 'textures/cloak.png'])).toBe(2);
    expect(hasTextureSource('textures/skin.png')).toBe(false);
    expect(hasTextureSource('textures/cloak.png')).toBe(false);
  });

  test('keeps the PNG-only release behavior available', () => {
    registerTextureSource('textures/skin.png', { kind: 'png', png: Buffer.from('png') });
    registerTextureSource('textures/cloak.png', { kind: 'blp', fileDataID: 123 });

    expect(releaseGeneratedPngSources(['textures/skin.png', 'textures/cloak.png'])).toBe(1);
    expect(hasTextureSource('textures/skin.png')).toBe(false);
    expect(hasTextureSource('textures/cloak.png')).toBe(true);
  });

  test('allows a later export to register a fresh source at the same path', () => {
    const path = 'textures/shared.png';
    registerTextureSource(path, { kind: 'blp', fileDataID: 123 });
    releaseTextureSourcePaths([path]);
    registerTextureSource(path, { kind: 'blp', fileDataID: 456 });

    expect(hasTextureSource(path)).toBe(true);
    expect(releaseTextureSourcePaths([path])).toBe(1);
  });
});
