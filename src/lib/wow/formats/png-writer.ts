/**
 * Pure-JS PNG encoder with adaptive filtering, ported byte-faithfully from
 * wow.export (src/js/png-writer.js). Output must remain byte-identical to
 * wow.export's PNG artifacts.
 */
import { BufferWrapper } from './buffer';

type FilterFn = (data: Buffer, dataOfs: number, byteWidth: number, raw: Buffer, rawOfs: number, bytesPerPixel: number) => void;
type FilterSumFn = (data: Buffer, dataOfs: number, byteWidth: number, bytesPerPixel: number) => number;

const paeth = (left: number, up: number, upLeft: number): number => {
  const p = left + up - upLeft;
  const paethLeft = Math.abs(p - left);
  const paethUp = Math.abs(p - up);
  const paethUpLeft = Math.abs(p - upLeft);

  if (paethLeft <= paethUp && paethLeft <= paethUpLeft) return left;

  if (paethUp <= paethUpLeft) return up;

  return upLeft;
};

const FILTERS: Record<number, FilterFn> = {
  // None
  0: (data, dataOfs, byteWidth, raw, rawOfs) => {
    for (let x = 0; x < byteWidth; x++) raw[rawOfs + x] = data[dataOfs + x];
  },

  // Sub
  1: (data, dataOfs, byteWidth, raw, rawOfs, bytesPerPixel) => {
    for (let x = 0; x < byteWidth; x++) {
      const left = x >= bytesPerPixel ? data[dataOfs + x - bytesPerPixel] : 0;
      raw[rawOfs + x] = data[dataOfs + x] - left;
    }
  },

  // Up
  2: (data, dataOfs, byteWidth, raw, rawOfs) => {
    for (let x = 0; x < byteWidth; x++) {
      const up = dataOfs > 0 ? data[dataOfs + x - byteWidth] : 0;
      raw[rawOfs + x] = data[dataOfs + x] - up;
    }
  },

  // Average
  3: (data, dataOfs, byteWidth, raw, rawOfs, bytesPerPixel) => {
    for (let x = 0; x < byteWidth; x++) {
      const left = x >= bytesPerPixel ? data[dataOfs + x - bytesPerPixel] : 0;
      const up = dataOfs > 0 ? data[dataOfs + x - byteWidth] : 0;
      raw[rawOfs + x] = data[dataOfs + x] - ((left + up) >> 1);
    }
  },

  // Paeth
  4: (data, dataOfs, byteWidth, raw, rawOfs, bytesPerPixel) => {
    for (let x = 0; x < byteWidth; x++) {
      const left = x >= bytesPerPixel ? data[dataOfs + x - bytesPerPixel] : 0;
      const up = dataOfs > 0 ? data[dataOfs + x - byteWidth] : 0;
      const upLeft = dataOfs > 0 && x >= bytesPerPixel ? data[dataOfs + x - (byteWidth + bytesPerPixel)] : 0;
      raw[rawOfs + x] = data[dataOfs + x] - paeth(left, up, upLeft);
    }
  },
};

const FILTER_SUMS: Record<number, FilterSumFn> = {
  // None
  0: (data, dataOfs, byteWidth) => {
    let sum = 0;
    for (let i = dataOfs, len = dataOfs + byteWidth; i < len; i++) sum += Math.abs(data[i]);

    return sum;
  },

  // Sub
  1: (data, dataOfs, byteWidth, bytesPerPixel) => {
    let sum = 0;
    for (let x = 0; x < byteWidth; x++) {
      const left = x >= bytesPerPixel ? data[dataOfs + x - bytesPerPixel] : 0;
      sum += Math.abs(data[dataOfs + x] - left);
    }

    return sum;
  },

  // Up
  2: (data, dataOfs, byteWidth) => {
    let sum = 0;
    for (let x = dataOfs, len = dataOfs + byteWidth; x < len; x++) {
      const up = dataOfs > 0 ? data[x - byteWidth] : 0;
      sum += Math.abs(data[x] - up);
    }

    return sum;
  },

  // Average
  3: (data, dataOfs, byteWidth, bytesPerPixel) => {
    let sum = 0;
    for (let x = 0; x < byteWidth; x++) {
      const left = x >= bytesPerPixel ? data[dataOfs + x - bytesPerPixel] : 0;
      const up = dataOfs > 0 ? data[dataOfs + x - byteWidth] : 0;
      sum += Math.abs(data[dataOfs + x] - ((left + up) >> 1));
    }

    return sum;
  },

  // Paeth
  4: (data, dataOfs, byteWidth, bytesPerPixel) => {
    let sum = 0;
    for (let x = 0; x < byteWidth; x++) {
      const left = x >= bytesPerPixel ? data[dataOfs + x - bytesPerPixel] : 0;
      const up = dataOfs > 0 ? data[dataOfs + x - byteWidth] : 0;
      const upLeft = dataOfs > 0 && x >= bytesPerPixel ? data[dataOfs + x - (byteWidth + bytesPerPixel)] : 0;
      sum += Math.abs(data[dataOfs + x] - paeth(left, up, upLeft));
    }

    return sum;
  },
};

/** Apply adaptive filtering to image data. */
const filter = (data: Buffer, width: number, height: number, bytesPerPixel: number): Buffer => {
  const byteWidth = width * bytesPerPixel;
  let dataOfs = 0;

  let rawOfs = 0;
  const raw = Buffer.alloc((byteWidth + 1) * height);

  let selectedFilter = 0;
  for (let y = 0; y < height; y++) {
    let min = Infinity;

    for (let i = 0, len = Object.keys(FILTERS).length; i < len; i++) {
      const sum = FILTER_SUMS[i](data, dataOfs, byteWidth, bytesPerPixel);
      if (sum < min) {
        selectedFilter = i;
        min = sum;
      }
    }

    raw[rawOfs] = selectedFilter;
    rawOfs++;

    FILTERS[selectedFilter](data, dataOfs, byteWidth, raw, rawOfs, bytesPerPixel);
    rawOfs += byteWidth;
    dataOfs += byteWidth;
  }
  return raw;
};

export class PNGWriter {
  width: number;

  height: number;

  bytesPerPixel = 4;

  bitDepth = 8;

  colorType = 6; // RGBA

  data: Buffer;

  constructor(width: number, height: number) {
    this.width = width;
    this.height = height;
    this.data = Buffer.alloc(width * height * 4);
  }

  /** Get the internal pixel data for this PNG. */
  getPixelData(): Buffer {
    return this.data;
  }

  getBuffer(): BufferWrapper {
    const filtered = new BufferWrapper(filter(this.data, this.width, this.height, this.bytesPerPixel));
    const deflated = filtered.deflate();
    const buf = BufferWrapper.alloc(8 + 25 + deflated.byteLength + 12 + 12, false);

    // 8-byte PNG signature.
    buf.writeUInt32LE(0x474E5089);
    buf.writeUInt32LE(0x0A1A0A0D);

    const ihdr = BufferWrapper.alloc(4 + 13, false);
    ihdr.writeUInt32LE(0x52444849); // IHDR
    ihdr.writeUInt32BE(this.width); // Image width
    ihdr.writeUInt32BE(this.height); // Image height
    ihdr.writeUInt8(this.bitDepth); // Bit-depth
    ihdr.writeUInt8(this.colorType); // Colour type
    ihdr.writeUInt8(0); // Compression (0)
    ihdr.writeUInt8(0); // Filter (0)
    ihdr.writeUInt8(0); // Interlace (0)
    ihdr.seek(0);

    buf.writeUInt32BE(13);
    buf.writeBuffer(ihdr);
    buf.writeInt32BE(ihdr.getCRC32());

    const idat = BufferWrapper.alloc(4 + deflated.byteLength, false);
    idat.writeUInt32LE(0x54414449); // IDAT
    idat.writeBuffer(deflated);

    idat.seek(0);

    buf.writeUInt32BE(deflated.byteLength);
    buf.writeBuffer(idat);
    buf.writeInt32BE(idat.getCRC32());

    buf.writeUInt32BE(0);
    buf.writeUInt32LE(0x444E4549); // IEND
    buf.writeUInt32LE(0x826042AE); // CRC IEND

    return buf;
  }

  /** Write this PNG to a file. */
  async write(file: string): Promise<void> {
    return this.getBuffer().writeToFile(file);
  }
}

export default PNGWriter;
