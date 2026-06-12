/**
 * MTL material library writer, ported from wow.export
 * (src/js/3D/writers/MTLWriter.js).
 */
import path from 'path';

import { wowConfig } from '../../server/config';
import { outputFileExists, writeOutputFile } from './output-sink';

interface MTLMaterial {
  name: string;
  file: string;
}

export class MTLWriter {
  out: string;

  materials: MTLMaterial[] = [];

  constructor(out: string) {
    this.out = out;
  }

  /** Add a material to this material library. */
  addMaterial(name: string, file: string): void {
    this.materials.push({ name, file });
  }

  /** Returns true if this material library is empty. */
  get isEmpty(): boolean {
    return this.materials.length === 0;
  }

  /** Produce the MTL file content. */
  getContent(): string {
    const mtlDir = path.dirname(this.out);
    const useAbsolute = wowConfig.enableAbsoluteMTLPaths;

    const lines: string[] = [];
    for (const material of this.materials) {
      lines.push(`newmtl ${material.name}`);
      lines.push('illum 1');

      let materialFile = material.file;
      if (useAbsolute) materialFile = path.resolve(mtlDir, materialFile);

      lines.push(`map_Kd ${materialFile}`);
    }

    return `${lines.join('\n')}\n`;
  }

  /** Write the material library. */
  async write(overwrite = true): Promise<void> {
    // Don't bother writing an empty material library.
    if (this.isEmpty) return;

    // If overwriting is disabled, check file existence.
    if (!overwrite && await outputFileExists(this.out)) return;

    await writeOutputFile(this.out, this.getContent());
  }
}

export default MTLWriter;
