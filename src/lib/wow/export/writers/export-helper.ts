/**
 * Path helpers for export artifacts, ported from wow.export
 * (src/js/casc/export-helper.js, static helpers only — the UI export
 * progress tracking is not needed headless).
 */
import path from 'path';

import { wowConfig } from '../../server/config';

/** Get an export path qualified by the configured export directory. */
export function getExportPath(file: string): string {
  // Remove whitespace due to MTL incompatibility for textures.
  const normalized = wowConfig.removePathSpaces ? file.replace(/\s/g, '') : file;

  return path.normalize(path.join(wowConfig.exportDirectory, normalized));
}

/** Takes the directory from fileA and combines it with the basename of fileB. */
export function replaceFile(fileA: string, fileB: string): string {
  return path.join(path.dirname(fileA), path.basename(fileB));
}

/** Replace an extension on a file path with another. */
export function replaceExtension(file: string, ext = ''): string {
  return path.join(path.dirname(file), path.basename(file, path.extname(file)) + ext);
}

/** Converts a win32 compatible path to a POSIX compatible path. */
export function win32ToPosix(str: string): string {
  return str.replaceAll('\\', '/');
}
