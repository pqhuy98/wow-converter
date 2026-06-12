/**
 * Loader generics, ported from wow.export (src/js/3D/loaders/LoaderGenerics.js).
 */
import { BufferWrapper } from '../buffer';

/** Process a null-terminated string block. */
export function ReadStringBlock(data: BufferWrapper, chunkSize: number): Record<number, string> {
  const chunk = data.readBuffer(chunkSize, false) as Buffer;
  const entries: Record<number, string> = {};

  let readOfs = 0;
  for (let i = 0; i < chunkSize; i++) {
    if (chunk[i] === 0x0) {
      // Skip padding bytes.
      if (readOfs === i) {
        readOfs += 1;
        continue;
      }

      entries[readOfs] = chunk.toString('utf8', readOfs, i).replace(/\0/g, '');
      readOfs = i + 1;
    }
  }

  return entries;
}

export default { ReadStringBlock };
