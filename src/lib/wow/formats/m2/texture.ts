/**
 * M2 texture reference, ported from wow.export (src/js/3D/Texture.js).
 */
import { getCasc } from '@/lib/wow/server/runtime';

import type { BLTEReader } from '../../archive/casc/blte-reader';
import * as listfile from '../../archive/casc/listfile';

export class Texture {
  flags: number;

  /**
   * Usually a numeric fileDataID; exporters patch in string keys
   * ('data-<type>' for canvas textures, or creature variant texture paths),
   * matching wow.export's loose typing.
   */
  fileDataID: number | string;

  data?: BLTEReader;

  constructor(flags: number, fileDataID?: number) {
    this.flags = flags;
    this.fileDataID = fileDataID || 0;
  }

  /** Set the texture file using a file name. */
  setFileName(fileName: string): void {
    this.fileDataID = listfile.getByFilename(fileName) || 0;
  }

  /**
   * Obtain the texture file for this texture, instance cached.
   * Returns null if fileDataID is not set.
   */
  async getTextureFile(): Promise<BLTEReader | null> {
    if (typeof this.fileDataID === 'number' && this.fileDataID > 0) {
      if (!this.data) this.data = await getCasc().getFile(this.fileDataID);

      return this.data;
    }

    return null;
  }
}

export default Texture;
