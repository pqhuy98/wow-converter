import path from 'path';

import * as listfile from '../archive/casc/listfile';
import { replaceExtension } from './writers/export-helper';

/** Listfile-style model path for CSV placement (no OBJ intermediates). */
export function modelReferencePath(
  fileDataID: number,
  kind: 'm2' | 'wmo',
  wmoSet?: number,
): string {
  let fileName = listfile.getByID(fileDataID);
  if (fileName === undefined) {
    return listfile.formatUnknownFile(fileDataID, kind === 'm2' ? '.m2' : '.wmo');
  }
  if (kind === 'wmo' && wmoSet !== undefined) {
    return replaceExtension(fileName, `_set${wmoSet}.wmo`);
  }
  if (kind === 'm2' && !fileName.toLowerCase().endsWith('.m2')) {
    return replaceExtension(fileName, '.m2');
  }
  return fileName;
}

/** Strip export extensions from a CSV ModelFile value. */
export function stripModelReferenceExt(modelFile: string): string {
  return modelFile.replace(/\.(obj|m2|wmo)$/i, '');
}

/** Placement CSV path for a model reference (virtual .obj/.wmo/.m2 path). */
export function placementCsvPath(modelPath: string): string {
  return modelPath.replace(/\.(obj|m2|wmo)$/i, '_ModelPlacementInformation.csv');
}

/** Resolve on-disk path for a shared or tile-local model reference. */
export function resolveModelStoragePath(
  fileName: string,
  tileDir: string,
  enableSharedChildren: boolean,
  getExportPathFn: (file: string) => string,
): string {
  if (enableSharedChildren) return getExportPathFn(fileName);
  return path.join(tileDir, path.basename(fileName));
}
