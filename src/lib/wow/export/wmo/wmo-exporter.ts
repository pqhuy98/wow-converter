/**
 * WMO export helpers for ADT map export (interior doodad placement CSV only).
 */
import path from 'path';

import { write } from '@/lib/wow/log';

import * as listfile from '../../archive/casc/listfile';
import { BufferWrapper } from '../../formats/buffer';
import { WMOLoader } from '../../formats/wmo/wmo-loader';
import { wowConfig } from '../../server/config';
import type { ADTExportOptions } from '../adt/map-export-utils';
import type { ExportProgress } from '../export-progress';
import { modelReferencePath, placementCsvPath } from '../model-reference-path';
import { CSVWriter } from '../writers/csv-writer';
import {
  getExportPath, replaceFile, win32ToPosix,
} from '../writers/export-helper';
import { outputFileExists } from '../writers/output-sink';

export interface WMODoodadSetMaskEntry {
  checked: boolean;
}

export class WMOExporter {
  wmo: WMOLoader;

  doodadSetMask?: WMODoodadSetMaskEntry[];

  constructor(data: BufferWrapper, fileID: string | number) {
    this.wmo = new WMOLoader(data, fileID);
  }

  /** Set the mask used for doodad set control. */
  setDoodadSetMask(mask: WMODoodadSetMaskEntry[] | undefined): void {
    this.doodadSetMask = mask;
  }

  /** Write WMO interior doodad placement CSV (converter resolves M2 from FileDataID). */
  async exportDoodadPlacementCsv(
    out: string,
    config: ADTExportOptions | typeof wowConfig,
    progress?: ExportProgress,
  ): Promise<void> {
    const wmo = this.wmo;
    wmo.load();
    const doodadSetMask = this.doodadSetMask;

    const csvPath = placementCsvPath(out);
    if (!config.overwriteFiles && await outputFileExists(csvPath)) {
      write('Skipping model placement export %s (file exists, overwrite disabled)', csvPath);
      return;
    }

    const useAbsolute = config.enableAbsoluteCSVPaths;
    const usePosix = config.pathFormat === 'posix';
    const outDir = path.dirname(out);
    const csv = new CSVWriter(csvPath);
    csv.addField('ModelFile', 'PositionX', 'PositionY', 'PositionZ', 'RotationW', 'RotationX', 'RotationY', 'RotationZ', 'ScaleFactor', 'DoodadSet', 'FileDataID');

    const wmoLabel = path.basename(out, path.extname(out));
    const doodadSets = wmo.doodadSets ?? [];
    for (let i = 0, n = doodadSets.length; i < n; i++) {
      if (!doodadSetMask?.[i]?.checked) continue;

      const set = doodadSets[i];
      const count = set.doodadCount;
      write('Writing interior doodad placements for set %s (%d entries)...', set.name, count);
      progress?.setLabel(`${wmoLabel}, ${set.name}`, 0, count);

      for (let j = 0; j < count; j++) {
        if (progress && j > 0 && j % 50 === 0) {
          progress.setLabel(`${wmoLabel}, ${set.name}`, j, count);
        }
        const doodad = wmo.doodads![set.firstInstanceIndex + j];
        let fileDataID = 0;

        if (wmo.fileDataIDs) {
          fileDataID = wmo.fileDataIDs[doodad.offset];
        } else {
          const fileName = wmo.doodadNames![doodad.offset];
          fileDataID = listfile.getByFilename(fileName) || 0;
        }

        if (fileDataID <= 0) continue;

        try {
          const fileName = modelReferencePath(fileDataID, 'm2');
          const m2Path = config.enableSharedChildren ? getExportPath(fileName) : replaceFile(out, fileName);

          let modelPath = path.relative(outDir, m2Path);
          if (useAbsolute === true) modelPath = path.resolve(outDir, modelPath);
          if (usePosix) modelPath = win32ToPosix(modelPath);

          csv.addRow({
            ModelFile: modelPath,
            PositionX: doodad.position[0],
            PositionY: doodad.position[1],
            PositionZ: doodad.position[2],
            RotationW: doodad.rotation[3],
            RotationX: doodad.rotation[0],
            RotationY: doodad.rotation[1],
            RotationZ: doodad.rotation[2],
            ScaleFactor: doodad.scale,
            DoodadSet: set.name,
            FileDataID: fileDataID,
          });
        } catch (e) {
          write('Failed to load doodad %d for %s: %s', fileDataID, set.name, (e as Error).message);
        }
      }
    }

    await csv.write();
  }

  /** Clear the WMO exporting cache (no-op; kept for ADT batch memory release). */
  static clearCache(): void {
    // interior doodads are resolved from CASC at convert time
  }
}

export default WMOExporter;
