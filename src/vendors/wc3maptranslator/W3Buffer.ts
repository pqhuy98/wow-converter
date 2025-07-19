/* eslint-disable no-underscore-dangle */
// eslint-disable-next-line @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-var-requires
import { roundTo } from 'round-to';

export class W3Buffer {
  private _offset = 0;

  private readonly _buffer: Buffer;

  constructor(buffer: Buffer) {
    this._buffer = buffer;
  }

  public readInt(): number {
    const int: number = this._buffer.readInt32LE(this._offset);
    this._offset += 4;
    return int;
  }

  public readShort(): number {
    const int: number = this._buffer.readInt16LE(this._offset);
    this._offset += 2;
    return int;
  }

  public readFloat(): number {
    const float: number = this._buffer.readFloatLE(this._offset);
    this._offset += 4;
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return, @typescript-eslint/no-unsafe-call
    return roundTo(float, 3);
  }

  public readString(): string {
    const string: number[] = [];

    while (this._buffer[this._offset] !== 0x00) {
      string.push(this._buffer[this._offset]);
      this._offset += 1;
    }
    this._offset += 1; // consume the \0 end-of-string delimiter

    // eslint-disable-next-line @typescript-eslint/no-unsafe-argument
    return Buffer.from(string).toString();
  }

  public readChars(len = 1): string {
    const string: number[] = [];
    const numCharsToRead = len;

    for (let i = 0; i < numCharsToRead; i++) {
      string.push(this._buffer[this._offset]);
      this._offset += 1;
    }

    // if (ch === 0x0) return '0' //Curse spell has a "Crs" field, whose 4th byte is probably a 0x0, and not a "0", causing the editor to just ignore this change when converting back...
    return string.map((ch) => String.fromCharCode(ch)).join('');
  }

  public readByte(): number {
    // TODO what kind of binary? Do we use a BigInt or a node provided type from Buffer?
    const byte = this._buffer[this._offset];
    this._offset += 1;
    return byte;
  }

  public isExhausted(): boolean {
    return this._offset === this._buffer.length;
  }
}
