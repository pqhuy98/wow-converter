/**
 * .bone file loader, ported from wow.export (src/js/3D/loaders/BONELoader.js).
 * See: https://wowdev.wiki/BONE
 */
import type { BufferWrapper } from '../buffer';

const CHUNK_BIDA = 0x41444942;
const CHUNK_BOMT = 0x544D4F42;

export class BONELoader {
  data: BufferWrapper;

  isLoaded = false;

  boneIDs?: number[];

  boneOffsetMatrices?: number[][][];

  constructor(data: BufferWrapper) {
    this.data = data;
  }

  /** Load the bone file. */
  load(): void {
    // Prevent multiple loading of the same file.
    if (this.isLoaded === true) return;

    this.data.readUInt32LE(); // Version?

    while (this.data.remainingBytes > 0) {
      const chunkID = this.data.readUInt32LE();
      const chunkSize = this.data.readUInt32LE();
      const nextChunkPos = this.data.offset + chunkSize;

      switch (chunkID) {
        case CHUNK_BIDA: this.parse_chunk_bida(chunkSize); break; // Bone ID
        case CHUNK_BOMT: this.parse_chunk_bomt(chunkSize); break; // Bone offset matrices
        default: break;
      }

      // Ensure that we start at the next chunk exactly.
      this.data.seek(nextChunkPos);
    }

    this.isLoaded = true;
  }

  private parse_chunk_bida(chunkSize: number): void {
    this.boneIDs = this.data.readUInt16LE(chunkSize / 2);
  }

  private parse_chunk_bomt(chunkSize: number): void {
    const amount = (chunkSize / 16) / 4;
    this.boneOffsetMatrices = new Array(amount);
    for (let i = 0; i < amount; i++) {
      this.boneOffsetMatrices[i] = new Array(4);
      for (let j = 0; j < 4; j++) this.boneOffsetMatrices[i][j] = this.data.readFloatLE(4);
    }
  }
}

export default BONELoader;
