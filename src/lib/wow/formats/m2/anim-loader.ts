/**
 * .anim file loader, ported from wow.export (src/js/3D/loaders/ANIMLoader.js).
 * See: https://wowdev.wiki/M2#.anim_files
 */
import type { BufferWrapper } from '../buffer';

const CHUNK_AFM2 = 0x324D4641;
const CHUNK_AFSA = 0x41534641;
const CHUNK_AFSB = 0x42534641;

export class ANIMLoader {
  data: BufferWrapper;

  isLoaded = false;

  animData?: number[];

  skeletonAttachmentData?: number[];

  skeletonBoneData?: number[];

  constructor(data: BufferWrapper) {
    this.data = data;
  }

  /** Load the animation file. */
  load(isChunked = true): void {
    // Prevent multiple loading of the same file.
    if (this.isLoaded === true) return;

    if (!isChunked) {
      this.animData = this.data.readUInt8(this.data.remainingBytes);
      this.isLoaded = true;
      return;
    }

    while (this.data.remainingBytes > 0) {
      const chunkID = this.data.readUInt32LE();
      const chunkSize = this.data.readUInt32LE();
      const nextChunkPos = this.data.offset + chunkSize;

      switch (chunkID) {
        case CHUNK_AFM2: this.parse_chunk_afm2(chunkSize); break; // AFM2 old animation data or ??? if AFSA/AFSB are present
        case CHUNK_AFSA: this.parse_chunk_afsa(chunkSize); break; // Skeleton Attachment animation data
        case CHUNK_AFSB: this.parse_chunk_afsb(chunkSize); break; // Skeleton Bone animation data
        default: break;
      }

      // Ensure that we start at the next chunk exactly.
      this.data.seek(nextChunkPos);
    }

    this.isLoaded = true;
  }

  private parse_chunk_afm2(chunkSize: number): void {
    this.animData = this.data.readUInt8(chunkSize);
  }

  private parse_chunk_afsa(chunkSize: number): void {
    this.skeletonAttachmentData = this.data.readUInt8(chunkSize);
  }

  private parse_chunk_afsb(chunkSize: number): void {
    this.skeletonBoneData = this.data.readUInt8(chunkSize);
  }
}

export default ANIMLoader;
