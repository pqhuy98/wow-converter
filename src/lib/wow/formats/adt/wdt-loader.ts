/**
 * WDT loader, ported from wow.export (src/js/3D/loaders/WDTLoader.js).
 */
import { BufferWrapper } from '../buffer';
import { constants } from '../constants';

const MAP_SIZE = constants.GAME.MAP_SIZE;
const MAP_SIZE_SQ = constants.GAME.MAP_SIZE_SQ;

export interface WDTEntry {
  rootADT: number;
  obj0ADT: number;
  obj1ADT: number;
  tex0ADT: number;
  lodADT: number;
  mapTexture: number;
  mapTextureN: number;
  minimapTexture: number;
}

export interface WorldModelPlacement {
  id: number;
  uid: number;
  position: number[];
  rotation: number[];
  upperExtents: number[];
  lowerExtents: number[];
  flags: number;
  doodadSetIndex: number;
  nameSet: number;
  padding: number;
}

export class WDTLoader {
  data: BufferWrapper;

  flags?: number;

  lgtFileDataID?: number;

  occFileDataID?: number;

  fogsFileDataID?: number;

  mpvFileDataID?: number;

  texFileDataID?: number;

  wdlFileDataID?: number;

  pd4FileDataID?: number;

  tiles?: number[];

  entries?: WDTEntry[];

  worldModel?: string;

  worldModelPlacement?: WorldModelPlacement;

  constructor(data: BufferWrapper) {
    this.data = data;
  }

  /** Load the WDT file, parsing it. */
  load(): void {
    while (this.data.remainingBytes > 0) {
      const chunkID = this.data.readUInt32LE();
      const chunkSize = this.data.readUInt32LE();
      const nextChunkPos = this.data.offset + chunkSize;

      const handler = WDTChunkHandlers[chunkID];
      if (handler) handler.call(this, this.data, chunkSize);

      // Ensure that we start at the next chunk exactly.
      this.data.seek(nextChunkPos);
    }
  }
}

const WDTChunkHandlers: Record<number, (this: WDTLoader, data: BufferWrapper, chunkSize: number) => void> = {
  // MPHD (Flags)
  0x4D504844(data) {
    this.flags = data.readUInt32LE();
    this.lgtFileDataID = data.readUInt32LE();
    this.occFileDataID = data.readUInt32LE();
    this.fogsFileDataID = data.readUInt32LE();
    this.mpvFileDataID = data.readUInt32LE();
    this.texFileDataID = data.readUInt32LE();
    this.wdlFileDataID = data.readUInt32LE();
    this.pd4FileDataID = data.readUInt32LE();
  },

  // MAIN (Tiles)
  0x4D41494E(data) {
    const tiles = new Array<number>(MAP_SIZE_SQ);
    this.tiles = tiles;
    for (let x = 0; x < MAP_SIZE; x++) {
      for (let y = 0; y < MAP_SIZE; y++) {
        tiles[(y * MAP_SIZE) + x] = data.readUInt32LE();
        data.move(4);
      }
    }
  },

  // MAID (File IDs)
  0x4D414944(data) {
    const entries = new Array<WDTEntry>(MAP_SIZE_SQ);
    this.entries = entries;

    for (let x = 0; x < MAP_SIZE; x++) {
      for (let y = 0; y < MAP_SIZE; y++) {
        entries[(y * MAP_SIZE) + x] = {
          rootADT: data.readUInt32LE(),
          obj0ADT: data.readUInt32LE(),
          obj1ADT: data.readUInt32LE(),
          tex0ADT: data.readUInt32LE(),
          lodADT: data.readUInt32LE(),
          mapTexture: data.readUInt32LE(),
          mapTextureN: data.readUInt32LE(),
          minimapTexture: data.readUInt32LE(),
        };
      }
    }
  },

  // MWMO (World WMO)
  0x4D574D4F(data, chunkSize) {
    this.worldModel = data.readString(chunkSize).replace('\0', '');
  },

  // MODF (World WMO Placement)
  0x4D4F4446(data) {
    this.worldModelPlacement = {
      id: data.readUInt32LE(),
      uid: data.readUInt32LE(),
      position: data.readFloatLE(3),
      rotation: data.readFloatLE(3),
      upperExtents: data.readFloatLE(3),
      lowerExtents: data.readFloatLE(3),
      flags: data.readUInt16LE(),
      doodadSetIndex: data.readUInt16LE(),
      nameSet: data.readUInt16LE(),
      padding: data.readUInt16LE(),
    };
  },
};

export default WDTLoader;
