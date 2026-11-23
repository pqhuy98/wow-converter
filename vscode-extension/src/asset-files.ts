import { readFile, realpath, stat } from 'node:fs/promises';
import { basename, dirname, extname, isAbsolute, join, relative, sep } from 'node:path';

const extensions = new Set(['.mdx', '.mdl', '.blp', '.dds', '.tga', '.png', '.jpg', '.jpeg', '.gif', '.bmp', '.webp']);
const converterFolders = new Set(['exported-assets', 'exported-assets-browse']);
const windowsDevice = /^(?:CON|PRN|AUX|NUL|COM[1-9¹²³]|LPT[1-9¹²³])(?: *\.|$)/i;

export function createAssetReader(
  modelFile: string,
  workspaceFolder?: string,
  readRevision?: (file: string) => Promise<Uint8Array | undefined>,
) {
  const modelFolder = dirname(modelFile);
  const modelName = basename(modelFile);
  const ancestors = [modelFolder];
  for (let folder = dirname(modelFolder); folder !== ancestors.at(-1); folder = dirname(folder)) {
    ancestors.push(folder);
  }

  const exportRoot = ancestors.find((folder) => converterFolders.has(basename(folder).toLowerCase()));
  const scope = exportRoot ?? (workspaceFolder && contains(workspaceFolder, modelFolder) ? workspaceFolder : modelFolder);
  const folders = ancestors.filter((folder) => contains(scope, folder));

  return async (asset: string): Promise<ArrayBuffer> => {
    if (asset === modelName) {
      if (readRevision) {
        const bytes = await readRevision(modelFile);
        if (!bytes) throw new Error('The model is not present in this Git revision.');
        return toArrayBuffer(bytes);
      }
      if (!(await stat(modelFile)).isFile()) throw new Error('The model path does not point to a file.');
      return toArrayBuffer(await readFile(modelFile));
    }

    const parts = asset.split(/[/\\]+/).filter((part) => part !== '.');
    const filename = parts.at(-1) ?? '';
    if (
      !extensions.has(extname(filename).toLowerCase())
      || parts.some((part) => !part || /[\0-\x1f<>:"|?*]|[. ]$/.test(part) || (process.platform === 'win32' && windowsDevice.test(part)))
    ) {
      throw new Error('Invalid model asset path.');
    }

    const candidates = folders.map((folder) => join(folder, ...parts));
    if (readRevision) {
      for (const candidate of candidates) {
        const bytes = await readRevision(candidate);
        if (bytes) return toArrayBuffer(bytes);
      }
    }

    const physicalScope = await realpath(scope);
    for (const candidate of candidates) {
      try {
        const file = await realpath(candidate);
        if (contains(physicalScope, file) && extensions.has(extname(file).toLowerCase()) && (await stat(file)).isFile()) {
          return toArrayBuffer(await readFile(file));
        }
      } catch (error) {
        if (!(error && typeof error === 'object' && 'code' in error
          && ['ENOENT', 'ENOTDIR'].includes(String(error.code)))) throw error;
      }
    }

    throw new Error(`Asset not found: ${asset}`);
  };
}

function contains(folder: string, file: string) {
  const path = relative(folder, file);
  return !isAbsolute(path) && path !== '..' && !path.startsWith(`..${sep}`);
}

function toArrayBuffer(data: Uint8Array): ArrayBuffer {
  return Uint8Array.from(data).buffer;
}
