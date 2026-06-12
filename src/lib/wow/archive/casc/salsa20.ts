/**
 * Faithful TypeScript port of wow.export's Salsa20 (src/js/casc/salsa20.js).
 * Based off original works by Dmitry Chestnykh.
 */
import { BufferWrapper } from '../../formats/buffer';

const SIGMA_32 = [0x61707865, 0x3320646E, 0x79622D32, 0x6B206574];
const SIGMA_16 = [0x61707865, 0x3120646E, 0x79622D36, 0x6B206574];

export class Salsa20 {
  private rounds: number;

  private sigma: number[];

  private keyWords: number[] = [];

  private nonceWords: [number, number] = [0, 0];

  private counter: [number, number] = [0, 0];

  private block: number[] = [];

  private blockUsed = 64;

  /**
   * @param nonce 8 byte nonce.
   * @param key 16 or 32 byte key as a hex string (32 or 64 hex chars).
   * @param rounds Defaults to 20.
   */
  constructor(nonce: number[], key: string, rounds = 20) {
    if (nonce.length !== 8) throw new Error(`Unexpected nonce length. 8 bytes expected, got ${nonce.length}`);
    if (key.length !== 32 && key.length !== 64) throw new Error(`Unexpected key length. 16 or 32 bytes expected, got ${key.length}`);

    this.rounds = rounds;
    this.sigma = key.length === 32 ? SIGMA_16 : SIGMA_32;

    this.setKey([...Buffer.from(key, 'hex')]);
    this.setNonce(Array.from(nonce));
  }

  setKey(key: number[]): void {
    // Expand 16-byte (4-word) key into a 32-byte (8-word) key.
    if (key.length === 16) {
      for (let i = 0; i < 16; i++) key[16 + i] = key[i];
    }

    for (let i = 0, j = 0; i < 8; i++, j += 4) {
      this.keyWords[i] = (key[j] & 0xFF) | ((key[j + 1] & 0xFF) << 8) | ((key[j + 2] & 0xFF) << 16) | ((key[j + 3] & 0xFF) << 24);
    }

    this._reset();
  }

  setNonce(nonce: number[]): void {
    this.nonceWords[0] = (nonce[0] & 0xFF) | ((nonce[1] & 0xFF) << 8) | ((nonce[2] & 0xFF) << 16) | ((nonce[3] & 0xFF) << 24);
    this.nonceWords[1] = (nonce[4] & 0xFF) | ((nonce[5] & 0xFF) << 8) | ((nonce[6] & 0xFF) << 16) | ((nonce[7] & 0xFF) << 24);

    this._reset();
  }

  getBytes(byteCount: number): BufferWrapper {
    const out = BufferWrapper.alloc(byteCount);
    for (let i = 0; i < byteCount; i++) {
      if (this.blockUsed === 64) {
        this._generateBlock();
        this._increment();
        this.blockUsed = 0;
      }

      out.writeUInt8(this.block[this.blockUsed]);
      this.blockUsed++;
    }

    out.seek(0);
    return out;
  }

  process(buf: BufferWrapper): BufferWrapper {
    const out = BufferWrapper.alloc(buf.byteLength);
    const bytes = this.getBytes(buf.byteLength);

    buf.seek(0);
    for (let i = 0, n = buf.byteLength; i < n; i++) {
      out.writeUInt8(bytes.readUInt8() ^ buf.readUInt8());
    }

    out.seek(0);
    return out;
  }

  private _reset(): void {
    this.counter[0] = 0;
    this.counter[1] = 0;
    this.blockUsed = 64;
  }

  private _increment(): void {
    this.counter[0] = (this.counter[0] + 1) & 0xFFFFFFFF;
    if (this.counter[0] === 0) this.counter[1] = (this.counter[1] + 1) & 0xFFFFFFFF;
  }

  private _generateBlock(): void {
    const j0 = this.sigma[0];
    const j1 = this.keyWords[0];
    const j2 = this.keyWords[1];
    const j3 = this.keyWords[2];
    const j4 = this.keyWords[3];
    const j5 = this.sigma[1];
    const j6 = this.nonceWords[0];
    const j7 = this.nonceWords[1];
    const j8 = this.counter[0];
    const j9 = this.counter[1];
    const j10 = this.sigma[2];
    const j11 = this.keyWords[4];
    const j12 = this.keyWords[5];
    const j13 = this.keyWords[6];
    const j14 = this.keyWords[7];
    const j15 = this.sigma[3];

    let x0 = j0; let x1 = j1; let x2 = j2; let x3 = j3; let x4 = j4; let x5 = j5; let x6 = j6; let x7 = j7;
    let x8 = j8; let x9 = j9; let x10 = j10; let x11 = j11; let x12 = j12; let x13 = j13; let x14 = j14; let x15 = j15;

    let u: number;
    for (let i = 0, n = this.rounds; i < n; i += 2) {
      u = x0 + x12;
      x4 ^= (u << 7) | (u >>> (32 - 7));
      u = x4 + x0;
      x8 ^= (u << 9) | (u >>> (32 - 9));
      u = x8 + x4;
      x12 ^= (u << 13) | (u >>> (32 - 13));
      u = x12 + x8;
      x0 ^= (u << 18) | (u >>> (32 - 18));

      u = x5 + x1;
      x9 ^= (u << 7) | (u >>> (32 - 7));
      u = x9 + x5;
      x13 ^= (u << 9) | (u >>> (32 - 9));
      u = x13 + x9;
      x1 ^= (u << 13) | (u >>> (32 - 13));
      u = x1 + x13;
      x5 ^= (u << 18) | (u >>> (32 - 18));

      u = x10 + x6;
      x14 ^= (u << 7) | (u >>> (32 - 7));
      u = x14 + x10;
      x2 ^= (u << 9) | (u >>> (32 - 9));
      u = x2 + x14;
      x6 ^= (u << 13) | (u >>> (32 - 13));
      u = x6 + x2;
      x10 ^= (u << 18) | (u >>> (32 - 18));

      u = x15 + x11;
      x3 ^= (u << 7) | (u >>> (32 - 7));
      u = x3 + x15;
      x7 ^= (u << 9) | (u >>> (32 - 9));
      u = x7 + x3;
      x11 ^= (u << 13) | (u >>> (32 - 13));
      u = x11 + x7;
      x15 ^= (u << 18) | (u >>> (32 - 18));

      u = x0 + x3;
      x1 ^= (u << 7) | (u >>> (32 - 7));
      u = x1 + x0;
      x2 ^= (u << 9) | (u >>> (32 - 9));
      u = x2 + x1;
      x3 ^= (u << 13) | (u >>> (32 - 13));
      u = x3 + x2;
      x0 ^= (u << 18) | (u >>> (32 - 18));

      u = x5 + x4;
      x6 ^= (u << 7) | (u >>> (32 - 7));
      u = x6 + x5;
      x7 ^= (u << 9) | (u >>> (32 - 9));
      u = x7 + x6;
      x4 ^= (u << 13) | (u >>> (32 - 13));
      u = x4 + x7;
      x5 ^= (u << 18) | (u >>> (32 - 18));

      u = x10 + x9;
      x11 ^= (u << 7) | (u >>> (32 - 7));
      u = x11 + x10;
      x8 ^= (u << 9) | (u >>> (32 - 9));
      u = x8 + x11;
      x9 ^= (u << 13) | (u >>> (32 - 13));
      u = x9 + x8;
      x10 ^= (u << 18) | (u >>> (32 - 18));

      u = x15 + x14;
      x12 ^= (u << 7) | (u >>> (32 - 7));
      u = x12 + x15;
      x13 ^= (u << 9) | (u >>> (32 - 9));
      u = x13 + x12;
      x14 ^= (u << 13) | (u >>> (32 - 13));
      u = x14 + x13;
      x15 ^= (u << 18) | (u >>> (32 - 18));
    }

    x0 += j0;
    x1 += j1;
    x2 += j2;
    x3 += j3;
    x4 += j4;
    x5 += j5;
    x6 += j6;
    x7 += j7;
    x8 += j8;
    x9 += j9;
    x10 += j10;
    x11 += j11;
    x12 += j12;
    x13 += j13;
    x14 += j14;
    x15 += j15;

    const words = [x0, x1, x2, x3, x4, x5, x6, x7, x8, x9, x10, x11, x12, x13, x14, x15];
    for (let w = 0; w < 16; w++) {
      const x = words[w];
      this.block[w * 4 + 0] = (x >>> 0) & 0xFF;
      this.block[w * 4 + 1] = (x >>> 8) & 0xFF;
      this.block[w * 4 + 2] = (x >>> 16) & 0xFF;
      this.block[w * 4 + 3] = (x >>> 24) & 0xFF;
    }
  }
}
