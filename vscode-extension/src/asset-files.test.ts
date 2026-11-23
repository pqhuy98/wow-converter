import { expect, test } from 'bun:test';
import { mkdir, mkdtemp, rm, symlink, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createAssetReader } from './asset-files';

test('reads an export and resolves its asset paths inside the export root', async () => {
  const dir = await mkdtemp(join(tmpdir(), 'wow-model-viewer-'));
  try {
    const root = join(dir, 'exported-assets');
    const outside = join(dir, 'outside');
    const modelFolder = join(root, 'models');
    const textures = join(root, 'textures');
    const model = join(modelFolder, 'Model #1.MDX');
    await mkdir(modelFolder, { recursive: true });
    await mkdir(textures);
    await mkdir(outside);
    await mkdir(join(textures, 'directory.blp'));
    await writeFile(model, 'model');
    await writeFile(join(textures, 'skin.blp'), 'texture');
    await writeFile(join(outside, 'outside.blp'), 'outside');
    await symlink(outside, join(root, 'outside'), process.platform === 'win32' ? 'junction' : 'dir');

    const read = createAssetReader(model);
    const decoder = new TextDecoder();
    expect(decoder.decode(await read('Model #1.MDX'))).toBe('model');
    expect(decoder.decode(await read('textures\\skin.blp'))).toBe('texture');
    await expect(read('../outside/outside.blp')).rejects.toThrow('Invalid');
    await expect(read('C:\\outside.blp')).rejects.toThrow('Invalid');
    await expect(read('outside/outside.blp')).rejects.toThrow('Asset not found');
    await expect(read('textures/directory.blp')).rejects.toThrow('Asset not found');

    const encoder = new TextEncoder();
    const readRevision = createAssetReader(model, undefined, async (path) => {
      if (path === model) return encoder.encode('previous model');
      if (path === join(textures, 'skin.blp')) return encoder.encode('previous texture');
    });
    expect(decoder.decode(await readRevision('Model #1.MDX'))).toBe('previous model');
    expect(decoder.decode(await readRevision('textures/skin.blp'))).toBe('previous texture');

    const readEmptyRevision = createAssetReader(model, undefined, async () => undefined);
    await expect(readEmptyRevision('Model #1.MDX')).rejects.toThrow('not present in this Git revision');
    expect(decoder.decode(await readEmptyRevision('textures/skin.blp'))).toBe('texture');
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});
