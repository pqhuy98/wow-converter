/**
 * Faithful TypeScript port of wow.export's BufferWrapper (src/js/buffer.js).
 * A streamlined reader/writer around the Node Buffer class.
 *
 * DOM-specific helpers (canvas, audio, data URLs) are intentionally omitted;
 * this port only covers the headless read/write surface.
 */
import { createHash } from 'crypto';
import { promises as fsp } from 'fs';
import path from 'path';
import zlib from 'zlib';

import { crc32 } from './crc32';

type ReadFn = (this: Buffer, offset: number, byteLength: number) => number;
type ReadBigFn = (this: Buffer, offset: number) => bigint;
type WriteFn = (this: Buffer, value: number, offset: number, byteLength: number) => number;
type WriteBigFn = (this: Buffer, value: bigint, offset: number) => number;

const LE = {
  READ_INT: Buffer.prototype.readIntLE as ReadFn,
  READ_UINT: Buffer.prototype.readUIntLE as ReadFn,
  READ_FLOAT: Buffer.prototype.readFloatLE as unknown as ReadFn,
  READ_DOUBLE: Buffer.prototype.readDoubleLE as unknown as ReadFn,
  READ_BIG_INT: Buffer.prototype.readBigInt64LE as ReadBigFn,
  READ_BIG_UINT: Buffer.prototype.readBigUInt64LE as ReadBigFn,
  WRITE_INT: Buffer.prototype.writeIntLE as WriteFn,
  WRITE_UINT: Buffer.prototype.writeUIntLE as WriteFn,
  WRITE_FLOAT: Buffer.prototype.writeFloatLE as unknown as WriteFn,
  WRITE_BIG_INT: Buffer.prototype.writeBigInt64LE as WriteBigFn,
  WRITE_BIG_UINT: Buffer.prototype.writeBigUInt64LE as WriteBigFn,
};

const BE = {
  READ_INT: Buffer.prototype.readIntBE as ReadFn,
  READ_UINT: Buffer.prototype.readUIntBE as ReadFn,
  READ_FLOAT: Buffer.prototype.readFloatBE as unknown as ReadFn,
  READ_DOUBLE: Buffer.prototype.readDoubleBE as unknown as ReadFn,
  READ_BIG_INT: Buffer.prototype.readBigInt64BE as ReadBigFn,
  READ_BIG_UINT: Buffer.prototype.readBigUInt64BE as ReadBigFn,
  WRITE_INT: Buffer.prototype.writeIntBE as WriteFn,
  WRITE_UINT: Buffer.prototype.writeUIntBE as WriteFn,
  WRITE_FLOAT: Buffer.prototype.writeFloatBE as unknown as WriteFn,
  WRITE_BIG_INT: Buffer.prototype.writeBigInt64BE as WriteBigFn,
  WRITE_BIG_UINT: Buffer.prototype.writeBigUInt64BE as WriteBigFn,
};

export class BufferWrapper {
  protected _ofs = 0;

  protected _buf: Buffer;

  /** Alloc a buffer with the given length and return it wrapped. */
  static alloc(length: number, secure = false): BufferWrapper {
    return new BufferWrapper(secure ? Buffer.alloc(length) : Buffer.allocUnsafe(length));
  }

  static from(source: Buffer | Uint8Array | number[] | string): BufferWrapper {
    return new BufferWrapper(Buffer.from(source as Buffer));
  }

  static fromBase64(source: string): BufferWrapper {
    return new BufferWrapper(Buffer.from(source, 'base64'));
  }

  /** Concatenate an array of buffers into a single buffer. */
  static concat(buffers: BufferWrapper[]): BufferWrapper {
    return new BufferWrapper(Buffer.concat(buffers.map((buf) => buf.raw)));
  }

  /** Load a file from disk at the given path into a wrapped buffer. */
  static async readFile(file: string): Promise<BufferWrapper> {
    return new BufferWrapper(await fsp.readFile(file));
  }

  constructor(buf: Buffer) {
    this._buf = buf;
  }

  get byteLength(): number {
    return this._buf.byteLength;
  }

  get remainingBytes(): number {
    return this.byteLength - this._ofs;
  }

  get offset(): number {
    return this._ofs;
  }

  get raw(): Buffer {
    return this._buf;
  }

  get internalArrayBuffer(): ArrayBuffer {
    return this._buf.buffer as ArrayBuffer;
  }

  /** Set the absolute position. Negative values seek from the end. */
  seek(ofs: number): void {
    const pos = ofs < 0 ? this.byteLength + ofs : ofs;
    if (pos < 0 || pos > this.byteLength) {
      throw new Error(`seek() offset out of bounds ${ofs} -> ${pos} ! ${this.byteLength}`);
    }
    this._ofs = pos;
  }

  /** Shift the position relative to the current position. */
  move(ofs: number): void {
    const pos = this.offset + ofs;
    if (pos < 0 || pos > this.byteLength) {
      throw new Error(`move() offset out of bounds ${ofs} -> ${pos} ! ${this.byteLength}`);
    }
    this._ofs = pos;
  }

  readIntLE(byteLength: number): number;

  readIntLE(byteLength: number, count: number): number[];

  readIntLE(byteLength: number, count?: number): number | number[] {
    return this._readInt(count, LE.READ_INT, byteLength);
  }

  readUIntLE(byteLength: number): number;

  readUIntLE(byteLength: number, count: number): number[];

  readUIntLE(byteLength: number, count?: number): number | number[] {
    return this._readInt(count, LE.READ_UINT, byteLength);
  }

  readIntBE(byteLength: number): number;

  readIntBE(byteLength: number, count: number): number[];

  readIntBE(byteLength: number, count?: number): number | number[] {
    return this._readInt(count, BE.READ_INT, byteLength);
  }

  readUIntBE(byteLength: number): number;

  readUIntBE(byteLength: number, count: number): number[];

  readUIntBE(byteLength: number, count?: number): number | number[] {
    return this._readInt(count, BE.READ_UINT, byteLength);
  }

  readInt8(): number;

  readInt8(count: number): number[];

  readInt8(count?: number): number | number[] {
    return this._readInt(count, LE.READ_INT, 1);
  }

  readUInt8(): number;

  readUInt8(count: number): number[];

  readUInt8(count?: number): number | number[] {
    return this._readInt(count, LE.READ_UINT, 1);
  }

  readInt16LE(): number;

  readInt16LE(count: number): number[];

  readInt16LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_INT, 2);
  }

  readUInt16LE(): number;

  readUInt16LE(count: number): number[];

  readUInt16LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_UINT, 2);
  }

  readInt16BE(): number;

  readInt16BE(count: number): number[];

  readInt16BE(count?: number): number | number[] {
    return this._readInt(count, BE.READ_INT, 2);
  }

  readUInt16BE(): number;

  readUInt16BE(count: number): number[];

  readUInt16BE(count?: number): number | number[] {
    return this._readInt(count, BE.READ_UINT, 2);
  }

  readInt24LE(): number;

  readInt24LE(count: number): number[];

  readInt24LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_INT, 3);
  }

  readUInt24LE(): number;

  readUInt24LE(count: number): number[];

  readUInt24LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_UINT, 3);
  }

  readInt24BE(): number;

  readInt24BE(count: number): number[];

  readInt24BE(count?: number): number | number[] {
    return this._readInt(count, BE.READ_INT, 3);
  }

  readUInt24BE(): number;

  readUInt24BE(count: number): number[];

  readUInt24BE(count?: number): number | number[] {
    return this._readInt(count, BE.READ_UINT, 3);
  }

  readInt32LE(): number;

  readInt32LE(count: number): number[];

  readInt32LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_INT, 4);
  }

  readUInt32LE(): number;

  readUInt32LE(count: number): number[];

  readUInt32LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_UINT, 4);
  }

  readInt32BE(): number;

  readInt32BE(count: number): number[];

  readInt32BE(count?: number): number | number[] {
    return this._readInt(count, BE.READ_INT, 4);
  }

  readUInt32BE(): number;

  readUInt32BE(count: number): number[];

  readUInt32BE(count?: number): number | number[] {
    return this._readInt(count, BE.READ_UINT, 4);
  }

  readInt40LE(): number;

  readInt40LE(count: number): number[];

  readInt40LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_INT, 5);
  }

  readUInt40LE(): number;

  readUInt40LE(count: number): number[];

  readUInt40LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_UINT, 5);
  }

  readInt40BE(): number;

  readInt40BE(count: number): number[];

  readInt40BE(count?: number): number | number[] {
    return this._readInt(count, BE.READ_INT, 5);
  }

  readUInt40BE(): number;

  readUInt40BE(count: number): number[];

  readUInt40BE(count?: number): number | number[] {
    return this._readInt(count, BE.READ_UINT, 5);
  }

  readInt48LE(): number;

  readInt48LE(count: number): number[];

  readInt48LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_INT, 6);
  }

  readUInt48LE(): number;

  readUInt48LE(count: number): number[];

  readUInt48LE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_UINT, 6);
  }

  readInt64LE(): bigint;

  readInt64LE(count: number): bigint[];

  readInt64LE(count?: number): bigint | bigint[] {
    return this._readBigInt(count, LE.READ_BIG_INT);
  }

  readUInt64LE(): bigint;

  readUInt64LE(count: number): bigint[];

  readUInt64LE(count?: number): bigint | bigint[] {
    return this._readBigInt(count, LE.READ_BIG_UINT);
  }

  readInt64BE(): bigint;

  readInt64BE(count: number): bigint[];

  readInt64BE(count?: number): bigint | bigint[] {
    return this._readBigInt(count, BE.READ_BIG_INT);
  }

  readUInt64BE(): bigint;

  readUInt64BE(count: number): bigint[];

  readUInt64BE(count?: number): bigint | bigint[] {
    return this._readBigInt(count, BE.READ_BIG_UINT);
  }

  readFloatLE(): number;

  readFloatLE(count: number): number[];

  readFloatLE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_FLOAT, 4);
  }

  readFloatBE(): number;

  readFloatBE(count: number): number[];

  readFloatBE(count?: number): number | number[] {
    return this._readInt(count, BE.READ_FLOAT, 4);
  }

  readDoubleLE(): number;

  readDoubleLE(count: number): number[];

  readDoubleLE(count?: number): number | number[] {
    return this._readInt(count, LE.READ_DOUBLE, 8);
  }

  /** Read a portion of this buffer as a hex string. */
  readHexString(length: number): string {
    this._checkBounds(length);
    const hex = this._buf.toString('hex', this._ofs, this._ofs + length);
    this._ofs += length;
    return hex;
  }

  /** Read raw bytes as a compact CASC key (latin1, one char per byte). */
  readBinaryKey(length: number): string {
    this._checkBounds(length);
    const key = this._buf.toString('latin1', this._ofs, this._ofs + length);
    this._ofs += length;
    return key;
  }

  /** Read a wrapped sub-buffer from this buffer. */
  readBuffer(length?: number): BufferWrapper;

  readBuffer(length: number | undefined, wrap: true, inflate?: boolean): BufferWrapper;

  readBuffer(length: number | undefined, wrap: false, inflate?: boolean): Buffer;

  readBuffer(length: number = this.remainingBytes, wrap = true, inflate = false): BufferWrapper | Buffer {
    this._checkBounds(length);

    let buf = Buffer.allocUnsafe(length);
    this._buf.copy(buf, 0, this._ofs, this._ofs + length);
    this._ofs += length;

    if (inflate) buf = Buffer.from(zlib.inflateSync(buf));

    return wrap ? new BufferWrapper(buf) : buf;
  }

  readString(length: number = this.remainingBytes, encoding: BufferEncoding = 'utf8'): string {
    if (length === 0) return '';

    this._checkBounds(length);
    const str = this._buf.toString(encoding, this._ofs, this._ofs + length);
    this._ofs += length;
    return str;
  }

  readNullTerminatedString(encoding: BufferEncoding = 'utf8'): string {
    const startPos = this.offset;
    let length = 0;

    while (this.remainingBytes > 0) {
      if (this.readUInt8() === 0x0) break;
      length++;
    }

    this.seek(startPos);

    const str = this.readString(length, encoding);
    this.move(1); // Skip the null-terminator.
    return str;
  }

  /** Returns true if the buffer starts with any of the given string(s). */
  startsWith(input: string | string[], encoding: BufferEncoding = 'utf8'): boolean {
    this.seek(0);
    if (Array.isArray(input)) {
      for (const entry of input) {
        if (this.readString(entry.length, encoding) === entry) return true;
      }
      return false;
    }
    return this.readString(input.length, encoding) === input;
  }

  readJSON(length: number = this.remainingBytes, encoding: BufferEncoding = 'utf8'): unknown {
    return JSON.parse(this.readString(length, encoding));
  }

  /** Read the entire buffer split by lines (\r\n, \n, \r). Preserves current offset. */
  readLines(encoding: BufferEncoding = 'utf8'): string[] {
    const ofs = this._ofs;
    this.seek(0);

    const str = this.readString(this.remainingBytes, encoding);
    this.seek(ofs);

    return str.split(/\r\n|\n|\r/);
  }

  fill(value: number, length: number = this.remainingBytes): void {
    this._checkBounds(length);
    this._buf.fill(value, this._ofs, this._ofs + length);
    this._ofs += length;
  }

  writeInt8(value: number): void {
    this._writeInt(value, LE.WRITE_INT, 1);
  }

  writeUInt8(value: number): void {
    this._writeInt(value, LE.WRITE_UINT, 1);
  }

  writeInt16LE(value: number): void {
    this._writeInt(value, LE.WRITE_INT, 2);
  }

  writeUInt16LE(value: number): void {
    this._writeInt(value, LE.WRITE_UINT, 2);
  }

  writeInt32LE(value: number): void {
    this._writeInt(value, LE.WRITE_INT, 4);
  }

  writeUInt32LE(value: number): void {
    this._writeInt(value, LE.WRITE_UINT, 4);
  }

  writeInt32BE(value: number): void {
    this._writeInt(value, BE.WRITE_INT, 4);
  }

  writeUInt32BE(value: number): void {
    this._writeInt(value, BE.WRITE_UINT, 4);
  }

  writeBigInt64LE(value: bigint): void {
    this._checkBounds(8);
    LE.WRITE_BIG_INT.call(this._buf, value, this._ofs);
    this._ofs += 8;
  }

  writeBigUInt64LE(value: bigint): void {
    this._checkBounds(8);
    LE.WRITE_BIG_UINT.call(this._buf, value, this._ofs);
    this._ofs += 8;
  }

  writeFloatLE(value: number): void {
    this._writeInt(value, LE.WRITE_FLOAT, 4);
  }

  /** Write the contents of a buffer to this buffer. */
  writeBuffer(buf: Buffer | BufferWrapper, copyLength = 0): void {
    let startIndex = 0;
    let rawBuf: Buffer;
    let length = copyLength;

    if (buf instanceof BufferWrapper) {
      startIndex = buf.offset;
      if (length === 0) length = buf.remainingBytes;
      else buf._checkBounds(length);
      rawBuf = buf.raw;
    } else {
      if (length === 0) length = buf.byteLength;
      rawBuf = buf;
    }

    this._checkBounds(length);

    rawBuf.copy(this._buf, this._ofs, startIndex, startIndex + length);
    this._ofs += length;

    if (buf instanceof BufferWrapper) buf._ofs += length;
  }

  /** Write the contents of this buffer to a file. Directory path will be created if needed. */
  async writeToFile(file: string): Promise<void> {
    await fsp.mkdir(path.dirname(file), { recursive: true });
    await fsp.writeFile(file, this._buf);
  }

  indexOfChar(char: string, start: number = this.offset): number {
    if (char.length > 1) throw new Error('BufferWrapper.indexOfChar() given string, expected single character.');
    return this.indexOf(char.charCodeAt(0), start);
  }

  indexOf(byte: number, start: number = this.offset): number {
    const resetPos = this.offset;
    this.seek(start);

    while (this.remainingBytes > 0) {
      const mark = this.offset;
      if (this.readUInt8() === byte) {
        this.seek(resetPos);
        return mark;
      }
    }

    this.seek(resetPos);
    return -1;
  }

  toBase64(): string {
    return this._buf.toString('base64');
  }

  /** Replace the internal buffer with a different capacity. */
  setCapacity(capacity: number, secure = false): void {
    if (capacity === this.byteLength) return;

    const buf = secure ? Buffer.alloc(capacity) : Buffer.allocUnsafe(capacity);
    this._buf.copy(buf, 0, 0, Math.min(capacity, this.byteLength));
    this._buf = buf;
  }

  calculateHash(hash = 'md5', encoding: 'hex' | 'base64' = 'hex'): string {
    return createHash(hash).update(this._buf).digest(encoding);
  }

  isZeroed(): boolean {
    for (let i = 0, n = this.byteLength; i < n; i++) {
      if (this._buf[i] !== 0x0) return false;
    }
    return true;
  }

  deflate(): BufferWrapper {
    return new BufferWrapper(Buffer.from(zlib.deflateSync(this._buf)));
  }

  /** Get the CRC32 checksum for this buffer. */
  getCRC32(): number {
    return crc32(this.raw);
  }

  protected _checkBounds(length: number): void {
    if (this.remainingBytes < length) {
      throw new Error(`Buffer operation out-of-bounds: ${length} > ${this.remainingBytes}`);
    }
  }

  private _readInt(count: number | undefined, func: ReadFn, byteLength: number): number | number[] {
    if (count !== undefined) {
      this._checkBounds(byteLength * count);

      const values = new Array<number>(count);
      for (let i = 0; i < count; i++) {
        values[i] = func.call(this._buf, this._ofs, byteLength);
        this._ofs += byteLength;
      }
      return values;
    }

    this._checkBounds(byteLength);
    const value = func.call(this._buf, this._ofs, byteLength);
    this._ofs += byteLength;
    return value;
  }

  private _readBigInt(count: number | undefined, func: ReadBigFn): bigint | bigint[] {
    if (count !== undefined) {
      this._checkBounds(8 * count);

      const values = new Array<bigint>(count);
      for (let i = 0; i < count; i++) {
        values[i] = func.call(this._buf, this._ofs);
        this._ofs += 8;
      }
      return values;
    }

    this._checkBounds(8);
    const value = func.call(this._buf, this._ofs);
    this._ofs += 8;
    return value;
  }

  private _writeInt(value: number, func: WriteFn, byteLength: number): void {
    this._checkBounds(byteLength);
    func.call(this._buf, value, this._ofs, byteLength);
    this._ofs += byteLength;
  }
}
