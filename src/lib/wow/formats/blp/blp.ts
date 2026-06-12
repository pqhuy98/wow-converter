/**
 * BLP texture decoder, ported from wow.export (src/js/casc/blp.js).
 * Canvas/WebP surfaces are omitted; PNG output goes through the byte-faithful
 * PNGWriter port.
 */
import { BufferWrapper } from '../buffer';
import { PNGWriter } from '../png-writer';

const DXT1 = 0x1;
const DXT3 = 0x2;
const DXT5 = 0x4;

const BLP_MAGIC = 0x32504c42;

type PixelSink = Buffer | Uint8Array | Uint8ClampedArray;

/** Unpack a colour value. */
const unpackColour = (block: ArrayLike<number>, index: number, ofs: number, colour: number[], colourOfs: number): number => {
  const value = block[index + ofs] | (block[index + 1 + ofs] << 8);

  const r = (value >> 11) & 0x1F;
  const g = (value >> 5) & 0x3F;
  const b = value & 0x1F;

  colour[colourOfs] = (r << 3) | (r >> 2);
  colour[colourOfs + 1] = (g << 2) | (g >> 4);
  colour[colourOfs + 2] = (b << 3) | (b >> 2);
  colour[colourOfs + 3] = 255;

  return value;
};

export class BLPImage {
  data: BufferWrapper;

  encoding: number;

  alphaDepth: number;

  alphaEncoding: number;

  containsMipmaps: number;

  width: number;

  height: number;

  mapOffsets: number[];

  mapSizes: number[];

  mapCount: number;

  palette: number[][] = [];

  private scale = 1;

  scaledWidth = 0;

  scaledHeight = 0;

  private scaledLength = 0;

  private rawData!: number[];

  constructor(data: BufferWrapper) {
    this.data = data;

    // Check magic value..
    if (this.data.readUInt32LE() !== BLP_MAGIC) throw new Error('Provided data is not a BLP file (invalid header magic).');

    // Check the BLP file type..
    const type = this.data.readUInt32LE();
    if (type !== 1) throw new Error(`Unsupported BLP type: ${type}`);

    // Read file flags..
    this.encoding = this.data.readUInt8();
    this.alphaDepth = this.data.readUInt8();
    this.alphaEncoding = this.data.readUInt8();
    this.containsMipmaps = this.data.readUInt8();

    // Read file dimensions..
    this.width = this.data.readUInt32LE();
    this.height = this.data.readUInt32LE();

    // Read mipmap data..
    this.mapOffsets = this.data.readUInt32LE(16);
    this.mapSizes = this.data.readUInt32LE(16);

    // Calculate available mipmaps..
    this.mapCount = 0;
    for (const ofs of this.mapOffsets) {
      if (ofs !== 0) this.mapCount++;
    }

    // Read colour palette..
    if (this.encoding === 1) {
      for (let i = 0; i < 256; i++) this.palette[i] = this.data.readUInt8(4);
    }
  }

  /** Retrieve this BLP as a PNG image. */
  toPNG(mask = 0b1111, mipmap = 0): BufferWrapper {
    this._prepare(mipmap);

    const png = new PNGWriter(this.scaledWidth, this.scaledHeight);
    const pixelData = png.getPixelData();

    switch (this.encoding) {
      case 1: this._getUncompressed(pixelData, mask); break;
      case 2: this._getCompressed(pixelData, mask); break;
      case 3: this._marshalBGRA(pixelData, mask); break;
      default: throw new Error(`Unsupported BLP encoding: ${this.encoding}`);
    }

    return png.getBuffer();
  }

  /** Save this BLP as PNG file. */
  async saveToPNG(file: string, mask = 0b1111, mipmap = 0): Promise<void> {
    return this.toPNG(mask, mipmap).writeToFile(file);
  }

  /** Prepare BLP for processing. */
  private _prepare(mipmap = 0): void {
    // Constrict the requested mipmap to a valid range..
    const level = Math.max(0, Math.min(mipmap || 0, this.mapCount - 1));

    // Calculate the scaled dimensions..
    this.scale = 2 ** level;
    this.scaledWidth = this.width / this.scale;
    this.scaledHeight = this.height / this.scale;
    this.scaledLength = this.scaledWidth * this.scaledHeight;

    // Extract the raw data we need..
    this.data.seek(this.mapOffsets[level]);
    this.rawData = this.data.readUInt8(this.mapSizes[level]);
  }

  /** Get the contents of this BLP as a BufferWrapper instance. */
  toBuffer(mipmap = 0, mask = 0b1111): BufferWrapper {
    this._prepare(mipmap);

    switch (this.encoding) {
      case 1: return this._getUncompressed(null, mask)!;
      case 2: return this._getCompressed(null, mask)!;
      case 3: return this._marshalBGRA(null, mask)!;
      default: throw new Error(`Unsupported BLP encoding: ${this.encoding}`);
    }
  }

  /** Get the contents of this raw BLP mipmap as a Buffer instance. */
  getRawMipmap(mipmap = 0): Buffer {
    this._prepare(mipmap);
    return Buffer.from(this.rawData);
  }

  /** Get the contents of this BLP as an RGBA UInt8 array. */
  toUInt8Array(mipmap = 0, mask = 0b1111): Uint8Array {
    this._prepare(mipmap);

    const arr = new Uint8Array(this.scaledWidth * this.scaledHeight * 4);
    switch (this.encoding) {
      case 1: this._getUncompressed(arr, mask); break;
      case 2: this._getCompressed(arr, mask); break;
      case 3: this._marshalBGRA(arr, mask); break;
      default: throw new Error(`Unsupported BLP encoding: ${this.encoding}`);
    }

    return arr;
  }

  /** Calculate the alpha using this file's alpha depth. */
  private _getAlpha(index: number): number {
    let byte: number;
    switch (this.alphaDepth) {
      case 1:
        byte = this.rawData[this.scaledLength + Math.floor(index / 8)];
        return (byte & (0x01 << (index % 8))) === 0 ? 0x00 : 0xFF;

      case 4:
        byte = this.rawData[this.scaledLength + (index / 2)];
        return index % 2 === 0 ? (byte & 0x0F) << 4 : byte & 0xF0;

      case 8:
        return this.rawData[this.scaledLength + index];

      default:
        return 0xFF;
    }
  }

  /** Extract compressed (DXT) data. */
  private _getCompressed(canvasData: PixelSink | null, mask = 0b1111): BufferWrapper | undefined {
    const flags = this.alphaDepth > 1 ? (this.alphaEncoding === 7 ? DXT5 : DXT3) : DXT1;
    const data = canvasData || Buffer.alloc(this.scaledWidth * this.scaledHeight * 4);

    let pos = 0;
    const blockBytes = (flags & DXT1) !== 0 ? 8 : 16;
    const target = new Array<number>(4 * 16);

    for (let y = 0, sh = this.scaledHeight; y < sh; y += 4) {
      for (let x = 0, sw = this.scaledWidth; x < sw; x += 4) {
        let blockPos = 0;

        if (this.rawData.length === pos) continue;

        let colourIndex = pos;
        if ((flags & (DXT3 | DXT5)) !== 0) colourIndex += 8;

        // Decompress colour..
        const isDXT1 = (flags & DXT1) !== 0;
        const colours: number[] = [];
        const a = unpackColour(this.rawData, colourIndex, 0, colours, 0);
        const b = unpackColour(this.rawData, colourIndex, 2, colours, 4);

        for (let i = 0; i < 3; i++) {
          const c = colours[i];
          const d = colours[i + 4];

          if (isDXT1 && a <= b) {
            colours[i + 8] = (c + d) / 2;
            colours[i + 12] = 0;
          } else {
            colours[i + 8] = (2 * c + d) / 3;
            colours[i + 12] = (c + 2 * d) / 3;
          }
        }

        colours[8 + 3] = 255;
        colours[12 + 3] = (isDXT1 && a <= b) ? 0 : 255;

        const index: number[] = [];
        for (let i = 0; i < 4; i++) {
          const packed = this.rawData[colourIndex + 4 + i];
          index[i * 4] = packed & 0x3;
          index[1 + i * 4] = (packed >> 2) & 0x3;
          index[2 + i * 4] = (packed >> 4) & 0x3;
          index[3 + i * 4] = (packed >> 6) & 0x3;
        }

        for (let i = 0; i < 16; i++) {
          const ofs = index[i] * 4;
          target[4 * i] = colours[ofs];
          target[4 * i + 1] = colours[ofs + 1];
          target[4 * i + 2] = colours[ofs + 2];
          target[4 * i + 3] = colours[ofs + 3];
        }

        if ((flags & DXT3) !== 0) {
          for (let i = 0; i < 8; i++) {
            const quant = this.rawData[pos + i];

            const low = (quant & 0x0F);
            const high = (quant & 0xF0);

            target[8 * i + 3] = (low | (low << 4));
            target[8 * i + 7] = (high | (high >> 4));
          }
        } else if ((flags & DXT5) !== 0) {
          const a0 = this.rawData[pos];
          const a1 = this.rawData[pos + 1];

          const alphaColours: number[] = [];
          alphaColours[0] = a0;
          alphaColours[1] = a1;

          if (a0 <= a1) {
            for (let i = 1; i < 5; i++) alphaColours[i + 1] = (((5 - i) * a0 + i * a1) / 5) | 0;

            alphaColours[6] = 0;
            alphaColours[7] = 255;
          } else {
            for (let i = 1; i < 7; i++) alphaColours[i + 1] = (((7 - i) * a0 + i * a1) / 7) | 0;
          }

          const indices: number[] = [];
          let alphaBlockPos = 2;
          let indicesPos = 0;

          for (let i = 0; i < 2; i++) {
            let value = 0;
            for (let j = 0; j < 3; j++) {
              const byte = this.rawData[pos + alphaBlockPos++];
              value |= (byte << (8 * j));
            }

            for (let j = 0; j < 8; j++) indices[indicesPos++] = (value >> (3 * j)) & 0x07;
          }

          for (let i = 0; i < 16; i++) target[4 * i + 3] = alphaColours[indices[i]];
        }

        for (let pY = 0; pY < 4; pY++) {
          for (let pX = 0; pX < 4; pX++) {
            const sX = x + pX;
            const sY = y + pY;

            if (sX < sw && sY < sh) {
              const pixel = 4 * (sw * sY + sX);
              data[pixel + 0] = (mask & 0b1) ? target[blockPos + 0] : 0;
              data[pixel + 1] = (mask & 0b10) ? target[blockPos + 1] : 0;
              data[pixel + 2] = (mask & 0b100) ? target[blockPos + 2] : 0;
              data[pixel + 3] = (mask & 0b1000) ? target[blockPos + 3] : 255;
            }
            blockPos += 4;
          }
        }
        pos += blockBytes;
      }
    }

    if (!canvasData) return new BufferWrapper(data as Buffer);
    return undefined;
  }

  /** Match the uncompressed data with the palette. */
  private _getUncompressed(canvasData: PixelSink | null, mask: number): BufferWrapper | undefined {
    if (canvasData) {
      for (let i = 0, n = this.scaledLength; i < n; i++) {
        const ofs = i * 4;
        const colour = this.palette[this.rawData[i]];

        canvasData[ofs] = (mask & 0b1) ? colour[2] : 0;
        canvasData[ofs + 1] = (mask & 0b10) ? colour[1] : 0;
        canvasData[ofs + 2] = (mask & 0b100) ? colour[0] : 0;
        canvasData[ofs + 3] = (mask & 0b1000) ? this._getAlpha(i) : 255;
      }
      return undefined;
    }

    const buf = BufferWrapper.alloc(this.scaledLength * 4);
    for (let i = 0, n = this.scaledLength; i < n; i++) {
      const colour = this.palette[this.rawData[i]];
      buf.writeUInt8((mask & 0b1) ? colour[2] : 0);
      buf.writeUInt8((mask & 0b10) ? colour[1] : 0);
      buf.writeUInt8((mask & 0b100) ? colour[0] : 0);
      buf.writeUInt8((mask & 0b1000) ? this._getAlpha(i) : 255);
    }
    buf.seek(0);
    return buf;
  }

  /** Marshal a BGRA array into an RGBA ordered buffer. */
  private _marshalBGRA(canvasData: PixelSink | null, mask: number): BufferWrapper | undefined {
    const data = this.rawData;

    if (canvasData) {
      for (let i = 0, n = data.length / 4; i < n; i++) {
        const ofs = i * 4;
        canvasData[ofs] = (mask & 0b1) ? data[ofs + 2] : 0;
        canvasData[ofs + 1] = (mask & 0b10) ? data[ofs + 1] : 0;
        canvasData[ofs + 2] = (mask & 0b100) ? data[ofs] : 0;
        canvasData[ofs + 3] = (mask & 0b1000) ? data[ofs + 3] : 255;
      }
      return undefined;
    }

    const buf = BufferWrapper.alloc(data.length);
    for (let i = 0, n = data.length / 4; i < n; i++) {
      const ofs = i * 4;
      buf.writeUInt8((mask & 0b1) ? data[ofs + 2] : 0);
      buf.writeUInt8((mask & 0b10) ? data[ofs + 1] : 0);
      buf.writeUInt8((mask & 0b100) ? data[ofs] : 0);
      buf.writeUInt8((mask & 0b1000) ? data[ofs + 3] : 255);
    }
    buf.seek(0);
    return buf;
  }
}

export default BLPImage;
