/**
 * WMO loader, ported from wow.export (src/js/3D/loaders/WMOLoader.js).
 */
import { ReadStringBlock } from '@/lib/wow/formats/adt/loader-generics';
import { getCasc } from '@/lib/wow/server/runtime';

import * as listfile from '../../archive/casc/listfile';
import { BufferWrapper } from '../buffer';

export interface WMOFog {
  flags: number;
  position: number[];
  radiusSmall: number;
  radiusLarge: number;
  fog: { end: number; startScalar: number; color: number };
  underwaterFog: { end: number; startScalar: number; color: number };
}

export interface WMOMaterial {
  flags: number;
  shader: number;
  blendMode: number;
  texture1: number;
  color1: number;
  color1b: number;
  texture2: number;
  color2: number;
  groupType: number;
  texture3: number;
  color3: number;
  flags3: number;
  runtimeData: number[];
}

export interface WMOGroupInfo {
  flags: number;
  boundingBox1: number[];
  boundingBox2: number[];
  nameIndex: number;
}

export interface WMODoodadSet {
  name: string;
  firstInstanceIndex: number;
  doodadCount: number;
  unused: number;
}

export interface WMODoodad {
  offset: number;
  flags: number;
  position: number[];
  rotation: number[];
  scale: number;
  color: number[];
}

export interface WMORenderBatch {
  possibleBox1: number[];
  possibleBox2: number[];
  firstFace: number;
  numFaces: number;
  firstVertex: number;
  lastVertex: number;
  flags: number;
  materialID: number;
}

export interface WMOLiquid {
  vertX: number;
  vertY: number;
  tileX: number;
  tileY: number;
  vertices: { data: number; height: number }[];
  tiles: number[];
  corner: number[];
  materialID: number;
}

export class WMOLoader {
  data?: BufferWrapper;

  loaded = false;

  renderingOnly: boolean;

  fileDataID?: number;

  fileName?: string;

  version?: number;

  // MOHD
  materialCount?: number;

  groupCount?: number;

  portalCount?: number;

  lightCount?: number;

  modelCount?: number;

  doodadCount?: number;

  setCount?: number;

  ambientColor?: number;

  areaTableID?: number;

  boundingBox1?: number[];

  boundingBox2?: number[];

  flags?: number;

  lodCount?: number;

  groups?: WMOLoader[];

  textureNames?: Record<number, string>;

  fogs?: WMOFog[];

  materials?: WMOMaterial[];

  portalVertices?: number[][];

  portalInfo?: { startVertex: number; count: number; plane: number[] }[];

  mopr?: { portalIndex: number; groupIndex: number; side: number }[];

  groupNames?: Record<number, string>;

  groupInfo?: WMOGroupInfo[];

  doodadSets?: WMODoodadSet[];

  fileDataIDs?: number[];

  doodadNames?: Record<number, string>;

  doodads?: WMODoodad[];

  groupIDs?: number[];

  // Group data
  liquid?: WMOLiquid;

  vertexColours?: number[][];

  nameOfs?: number;

  descOfs?: number;

  ofsPortals?: number;

  numPortals?: number;

  numBatchesA?: number;

  numBatchesB?: number;

  numBatchesC?: number;

  liquidType?: number;

  groupID?: number;

  indices?: number[];

  vertices?: number[];

  uvs?: number[][];

  normals?: number[];

  renderBatches?: WMORenderBatch[];

  materialInfo?: { flags: number; materialID: number }[];

  constructor(data: BufferWrapper, fileID?: number | string, renderingOnly = false) {
    this.data = data;
    this.renderingOnly = renderingOnly;

    if (fileID !== undefined) {
      if (typeof fileID === 'string') {
        this.fileDataID = listfile.getByFilename(fileID);
        this.fileName = fileID;
      } else {
        this.fileDataID = fileID;
        this.fileName = listfile.getByID(fileID);
      }
    }
  }

  /** Load the WMO object. */
  load(): void {
    // Prevent duplicate loading.
    if (this.loaded) return;

    const data = this.data!;
    while (data.remainingBytes > 0) {
      const chunkID = data.readUInt32LE();
      const chunkSize = data.readUInt32LE();
      const nextChunkPos = data.offset + chunkSize;

      const handler = WMOChunkHandlers[chunkID];
      if (handler && (!this.renderingOnly || !WMOOptionalChunks.includes(chunkID))) handler.call(this, data, chunkSize);

      // Ensure that we start at the next chunk exactly.
      data.seek(nextChunkPos);
    }

    // Mark this instance as loaded.
    this.loaded = true;
    this.data = undefined;
  }

  /** Get a group from this WMO. */
  async getGroup(index: number): Promise<WMOLoader> {
    if (!this.groups) throw new Error('Attempted to obtain group from a root WMO.');

    const casc = getCasc();

    let group = this.groups[index];
    if (group) return group;

    let data: BufferWrapper;
    if (this.groupIDs) data = await casc.getFile(this.groupIDs[index]);
    else data = await casc.getFileByName(this.fileName!.replace('.wmo', `_${index.toString().padStart(3, '0')}.wmo`));

    group = new WMOLoader(data, undefined, this.renderingOnly);
    this.groups[index] = group;
    group.load();

    return group;
  }
}

/** Optional chunks that are not required for rendering. */
const WMOOptionalChunks: number[] = [
  0x4D4C4951, // MLIQ (Liquid)
  0x4D464F47, // MFOG (Fog)
  0x4D4F5056, // MOPV (Portal Vertices)
  0x4D4F5052, // MOPR (Map Object Portal References)
  0x4D4F5054, // MOPT (Portal Triangles)
  0x4D4F4356, // MOCV (Vertex Colors)
  0x4D44414C, // MDAL (Ambient Color)
];

const WMOChunkHandlers: Record<number, (this: WMOLoader, data: BufferWrapper, chunkSize: number) => void> = {
  // MVER (Version) [WMO Root, WMO Group]
  0x4D564552(data) {
    this.version = data.readUInt32LE();
    if (this.version !== 17) throw new Error(`Unsupported WMO version: ${this.version}`);
  },

  // MOHD (Header) [WMO Root]
  0x4D4F4844(data) {
    this.materialCount = data.readUInt32LE();
    this.groupCount = data.readUInt32LE();
    this.portalCount = data.readUInt32LE();
    this.lightCount = data.readUInt32LE();
    this.modelCount = data.readUInt32LE();
    this.doodadCount = data.readUInt32LE();
    this.setCount = data.readUInt32LE();
    this.ambientColor = data.readUInt32LE();
    this.areaTableID = data.readUInt32LE();
    this.boundingBox1 = data.readFloatLE(3);
    this.boundingBox2 = data.readFloatLE(3);
    this.flags = data.readUInt16LE();
    this.lodCount = data.readUInt16LE();

    this.groups = new Array(this.groupCount);
  },

  // MOTX (Textures) [Classic, WMO Root]
  0x4D4F5458(data, chunkSize) {
    this.textureNames = ReadStringBlock(data, chunkSize);
  },

  // MFOG (Fog) [WMO Root]
  0x4D464F47(data, chunkSize) {
    const count = chunkSize / 48;
    const fogs = new Array<WMOFog>(count);
    this.fogs = fogs;

    for (let i = 0; i < count; i++) {
      fogs[i] = {
        flags: data.readUInt32LE(),
        position: data.readFloatLE(3),
        radiusSmall: data.readFloatLE(),
        radiusLarge: data.readFloatLE(),
        fog: {
          end: data.readFloatLE(),
          startScalar: data.readFloatLE(),
          color: data.readUInt32LE(),
        },
        underwaterFog: {
          end: data.readFloatLE(),
          startScalar: data.readFloatLE(),
          color: data.readUInt32LE(),
        },
      };
    }
  },

  // MOMT (Materials) [WMO Root]
  0x4D4F4D54(data, chunkSize) {
    const count = chunkSize / 64;
    const materials = new Array<WMOMaterial>(count);
    this.materials = materials;

    for (let i = 0; i < count; i++) {
      materials[i] = {
        flags: data.readUInt32LE(),
        shader: data.readUInt32LE(),
        blendMode: data.readUInt32LE(),
        texture1: data.readUInt32LE(),
        color1: data.readUInt32LE(),
        color1b: data.readUInt32LE(),
        texture2: data.readUInt32LE(),
        color2: data.readUInt32LE(),
        groupType: data.readUInt32LE(),
        texture3: data.readUInt32LE(),
        color3: data.readUInt32LE(),
        flags3: data.readUInt32LE(),
        runtimeData: data.readUInt32LE(4),
      };
    }
  },

  // MOPV (Portal Vertices) [WMO Root]
  0x4D4F5056(data, chunkSize) {
    const vertexCount = chunkSize / (3 * 4);
    this.portalVertices = new Array(vertexCount);
    for (let i = 0; i < vertexCount; i++) this.portalVertices[i] = data.readFloatLE(3);
  },

  // MOPT (Portal Triangles) [WMO Root]
  0x4D4F5054(data) {
    this.portalInfo = new Array(this.portalCount);
    for (let i = 0; i < this.portalCount!; i++) {
      this.portalInfo[i] = {
        startVertex: data.readUInt16LE(),
        count: data.readUInt16LE(),
        plane: data.readFloatLE(4),
      };
    }
  },

  // MOPR (Map Object Portal References) [WMO Root]
  0x4D4F5052(data, chunkSize) {
    const entryCount = chunkSize / 8;
    this.mopr = new Array(entryCount);

    for (let i = 0; i < entryCount; i++) {
      this.mopr[i] = {
        portalIndex: data.readUInt16LE(),
        groupIndex: data.readUInt16LE(),
        side: data.readInt16LE(),
      };

      data.move(4); // Filler
    }
  },

  // MOGN (Group Names) [WMO Root]
  0x4D4F474E(data, chunkSize) {
    this.groupNames = ReadStringBlock(data, chunkSize);
  },

  // MOGI (Group Info) [WMO Root]
  0x4D4F4749(data, chunkSize) {
    const count = chunkSize / 32;
    const groupInfo = new Array<WMOGroupInfo>(count);
    this.groupInfo = groupInfo;

    for (let i = 0; i < count; i++) {
      groupInfo[i] = {
        flags: data.readUInt32LE(),
        boundingBox1: data.readFloatLE(3),
        boundingBox2: data.readFloatLE(3),
        nameIndex: data.readInt32LE(),
      };
    }
  },

  // MODS (Doodad Sets) [WMO Root]
  0x4D4F4453(data, chunkSize) {
    const count = chunkSize / 32;
    const doodadSets = new Array<WMODoodadSet>(count);
    this.doodadSets = doodadSets;

    for (let i = 0; i < count; i++) {
      doodadSets[i] = {
        name: data.readString(20).replace(/\0/g, ''),
        firstInstanceIndex: data.readUInt32LE(),
        doodadCount: data.readUInt32LE(),
        unused: data.readUInt32LE(),
      };
    }
  },

  // MODI (Doodad IDs) [WMO Root]
  0x4D4F4449(data, chunkSize) {
    this.fileDataIDs = data.readUInt32LE(chunkSize / 4);
  },

  // MODN (Doodad Names) [WMO Root]
  0x4D4F444E(data, chunkSize) {
    this.doodadNames = ReadStringBlock(data, chunkSize);

    // Doodads are still referenced as MDX in Classic doodad names, replace them with m2.
    for (const [ofs, file] of Object.entries(this.doodadNames)) this.doodadNames[Number(ofs)] = file.toLowerCase().replace('.mdx', '.m2');
  },

  // MODD (Doodad Definitions) [WMO Root]
  0x4D4F4444(data, chunkSize) {
    const count = chunkSize / 40;
    const doodads = new Array<WMODoodad>(count);
    this.doodads = doodads;

    for (let i = 0; i < count; i++) {
      doodads[i] = {
        offset: data.readUInt24LE(),
        flags: data.readUInt8(),
        position: data.readFloatLE(3),
        rotation: data.readFloatLE(4),
        scale: data.readFloatLE(),
        color: data.readUInt8(4),
      };
    }
  },

  // GFID (Group file Data IDs) [WMO Root]
  0x47464944(data, chunkSize) {
    this.groupIDs = data.readUInt32LE(chunkSize / 4);
  },

  // MLIQ (Liquid Data) [WMO Group]
  0x4D4C4951(data) {
    // See https://wowdev.wiki/WMO#MLIQ_chunk for using this raw data.
    const liquidVertsX = data.readUInt32LE();
    const liquidVertsY = data.readUInt32LE();

    const liquidTilesX = data.readUInt32LE();
    const liquidTilesY = data.readUInt32LE();

    const liquidCorner = data.readFloatLE(3);
    const liquidMaterialID = data.readUInt16LE();

    const vertCount = liquidVertsX * liquidVertsY;
    const liquidVertices = new Array<{ data: number; height: number }>(vertCount);

    for (let i = 0; i < vertCount; i++) {
      // For water (SMOWVert): uint8 flow1, flow2, flow1Pct, filler.
      // For magma (SMOMVert): int16 s, t.
      liquidVertices[i] = {
        data: data.readUInt32LE(),
        height: data.readFloatLE(),
      };
    }

    const tileCount = liquidTilesX * liquidTilesY;
    const liquidTiles = new Array<number>(tileCount);

    for (let i = 0; i < tileCount; i++) liquidTiles[i] = data.readUInt8();

    this.liquid = {
      vertX: liquidVertsX,
      vertY: liquidVertsY,
      tileX: liquidTilesX,
      tileY: liquidTilesY,
      vertices: liquidVertices,
      tiles: liquidTiles,
      corner: liquidCorner,
      materialID: liquidMaterialID,
    };
  },

  // MOCV (Vertex Colouring) [WMO Group]
  0x4D4F4356(data, chunkSize) {
    if (!this.vertexColours) this.vertexColours = [];

    this.vertexColours.push(data.readUInt32LE(chunkSize / 4));
  },

  // MDAL (Ambient Color) [WMO Group]
  0x4D44414C(data) {
    this.ambientColor = data.readUInt32LE();
  },

  // MOGP (Group Header) [WMO Group]
  0x4D4F4750(data, chunkSize) {
    const endOfs = data.offset + chunkSize;

    this.nameOfs = data.readUInt32LE();
    this.descOfs = data.readUInt32LE();

    this.flags = data.readUInt32LE();
    this.boundingBox1 = data.readFloatLE(3);
    this.boundingBox2 = data.readFloatLE(3);

    this.ofsPortals = data.readUInt16LE();
    this.numPortals = data.readUInt16LE();

    this.numBatchesA = data.readUInt16LE();
    this.numBatchesB = data.readUInt16LE();
    this.numBatchesC = data.readUInt32LE();

    data.move(4); // Unused.

    this.liquidType = data.readUInt32LE();
    this.groupID = data.readUInt32LE();

    data.move(8); // Unknown.

    // Read sub-chunks.
    while (data.offset < endOfs) {
      const chunkID = data.readUInt32LE();
      const subChunkSize = data.readUInt32LE();
      const nextChunkPos = data.offset + subChunkSize;

      const handler = WMOChunkHandlers[chunkID];
      if (handler) handler.call(this, data, subChunkSize);

      // Ensure that we start at the next chunk exactly.
      data.seek(nextChunkPos);
    }
  },

  // MOVI (indices) [WMO Group]
  0x4D4F5649(data, chunkSize) {
    this.indices = data.readUInt16LE(chunkSize / 2);
  },

  // MOVT (vertices) [WMO Group]
  0x4D4F5654(data, chunkSize) {
    const count = chunkSize / 4;
    const vertices = new Array<number>(count);
    this.vertices = vertices;

    for (let i = 0; i < count; i += 3) {
      vertices[i] = data.readFloatLE();
      vertices[i + 2] = data.readFloatLE() * -1;
      vertices[i + 1] = data.readFloatLE();
    }
  },

  // MOTV (UVs) [WMO Group]
  0x4D4F5456(data, chunkSize) {
    if (!this.uvs) this.uvs = [];

    const count = chunkSize / 4;
    const uvs = new Array<number>(count);
    for (let i = 0; i < count; i += 2) {
      uvs[i] = data.readFloatLE();
      uvs[i + 1] = (data.readFloatLE() - 1) * -1;
    }

    this.uvs.push(uvs);
  },

  // MONR (Normals) [WMO Group]
  0x4D4F4E52(data, chunkSize) {
    const count = chunkSize / 4;
    const normals = new Array<number>(count);
    this.normals = normals;

    for (let i = 0; i < count; i += 3) {
      normals[i] = data.readFloatLE();
      normals[i + 2] = data.readFloatLE() * -1;
      normals[i + 1] = data.readFloatLE();
    }
  },

  // MOBA (Render Batches) [WMO Group]
  0x4D4F4241(data, chunkSize) {
    const count = chunkSize / 24;
    const batches = new Array<WMORenderBatch>(count);
    this.renderBatches = batches;

    for (let i = 0; i < count; i++) {
      batches[i] = {
        possibleBox1: data.readUInt16LE(3),
        possibleBox2: data.readUInt16LE(3),
        firstFace: data.readUInt32LE(),
        numFaces: data.readUInt16LE(),
        firstVertex: data.readUInt16LE(),
        lastVertex: data.readUInt16LE(),
        flags: data.readUInt8(),
        materialID: data.readUInt8(),
      };
    }
  },

  // MOPY (Material Info) [WMO Group]
  0x4D4F5059(data, chunkSize) {
    const count = chunkSize / 2;
    const materialInfo = new Array<{ flags: number; materialID: number }>(count);
    this.materialInfo = materialInfo;

    for (let i = 0; i < count; i++) materialInfo[i] = { flags: data.readUInt8(), materialID: data.readUInt8() };
  },
};

export default WMOLoader;
