/**
 * M2 binary structure readers, ported from wow.export (src/js/3D/loaders/M2Generics.js).
 */
import type { BufferWrapper } from '../buffer';

export type M2TrackDataType = 'uint8' | 'uint16' | 'int16' | 'uint32' | 'float' | 'float3' | 'float4' | 'compquat';
export type M2ArrayDataType = 'uint8' | 'uint16' | 'int16' | 'uint32' | 'float' | 'float2' | 'float3';

/** A single track value: scalar or vector depending on the data type. */
export type M2TrackValue = number | number[];

export class M2Track<V = M2TrackValue> {
  globalSeq: number;

  interpolation: number;

  timestamps: number[][];

  values: V[][];

  constructor(globalSeq: number, interpolation: number, timestamps: number[][], values: V[][]) {
    this.globalSeq = globalSeq;
    this.interpolation = interpolation;
    this.timestamps = timestamps;
    this.values = values;
  }
}

export interface M2PartTrack<V = M2TrackValue> {
  timestamps: number[];
  values: V[];
}

export interface CAaBox {
  min: number[];
  max: number[];
}

/** A spline key: [value, inTan, outTan]. */
export type M2SplineKey = [number | number[], number | number[], number | number[]];

// M2 SplineKey value readers for tracks like camera.position/target/roll/FoV
function read_spline_key_array(data: BufferWrapper, valueType: 'float' | 'float3'): M2SplineKey {
  // Each key: value, inTan, outTan (same type).
  switch (valueType) {
    case 'float':
      return [
        data.readFloatLE(), // value
        data.readFloatLE(), // inTan
        data.readFloatLE(), // outTan
      ];
    case 'float3':
      return [
        data.readFloatLE(3),
        data.readFloatLE(3),
        data.readFloatLE(3),
      ];
    default:
      throw new Error(`Unsupported spline key valueType: ${String(valueType)}`);
  }
}

export function read_m2_spline_track(data: BufferWrapper, ofs: number, valueType: 'float' | 'float3'): M2Track<M2SplineKey> {
  const interpolation = data.readUInt16LE();
  const globalSeq = data.readUInt16LE();

  const timestamps = read_m2_array_array(data, ofs, 'uint32') as number[][];
  // Values are arrays of M2SplineKey<T>: for each key, value + inTan + outTan
  const arrCount = data.readUInt32LE();
  const arrOfs = data.readUInt32LE();

  const base = data.offset;
  data.seek(ofs + arrOfs);

  const values: M2SplineKey[][] = Array(arrCount);
  for (let i = 0; i < arrCount; i++) {
    const subArrCount = data.readUInt32LE();
    const subArrOfs = data.readUInt32LE();
    const subBase = data.offset;
    data.seek(ofs + subArrOfs);

    values[i] = Array(subArrCount);
    for (let j = 0; j < subArrCount; j++) values[i][j] = read_spline_key_array(data, valueType);

    data.seek(subBase);
  }

  data.seek(base);
  return new M2Track<M2SplineKey>(globalSeq, interpolation, timestamps, values);
}

/** Read a single value of the given track data type at the current position of `data`. */
function readTrackValue(data: BufferWrapper, dataType: M2TrackDataType): M2TrackValue {
  switch (dataType) {
    case 'uint32': return data.readUInt32LE();
    case 'uint16': return data.readUInt16LE();
    case 'int16': return data.readInt16LE();
    case 'float': return data.readFloatLE();
    case 'float3': return data.readFloatLE(3);
    case 'float4': return data.readFloatLE(4);
    case 'compquat': return data.readUInt16LE(4).map((e) => (e < 0 ? e + 32768 : e - 32767) / 32767);
    case 'uint8': return data.readUInt8();
    default:
      throw new Error(`Unknown data type: ${String(dataType)}`);
  }
}

/** Byte size of a single value of the given track data type (for .anim seeks). */
function trackValueSize(dataType: M2TrackDataType): number {
  switch (dataType) {
    case 'uint8': return 1;
    case 'uint16': return 2;
    case 'int16': return 2;
    case 'uint32': return 4;
    case 'float': return 4;
    case 'float3': return 12;
    case 'float4': return 16;
    case 'compquat': return 8;
    default:
      throw new Error(`Unhandled data type: ${String(dataType)}`);
  }
}

// See https://wowdev.wiki/M2#Standard_animation_block
export function read_m2_array_array(
  data: BufferWrapper,
  ofs: number,
  dataType: M2TrackDataType,
  useAnims = false,
  animFiles: Map<number, BufferWrapper> = new Map(),
): M2TrackValue[][] {
  const arrCount = data.readUInt32LE();
  const arrOfs = data.readUInt32LE();

  const base = data.offset;
  data.seek(ofs + arrOfs);

  const arr: M2TrackValue[][] = Array(arrCount);
  for (let i = 0; i < arrCount; i++) {
    const subArrCount = data.readUInt32LE();
    const subArrOfs = data.readUInt32LE();
    const subBase = data.offset;
    data.seek(ofs + subArrOfs);

    arr[i] = Array(subArrCount);
    for (let j = 0; j < subArrCount; j++) {
      if (useAnims && animFiles.has(i)) {
        const animFile = animFiles.get(i)!;
        animFile.seek(subArrOfs + (j * trackValueSize(dataType)));
        arr[i][j] = readTrackValue(animFile, dataType);
      } else {
        arr[i][j] = readTrackValue(data, dataType);
      }
    }

    data.seek(subBase);
  }

  data.seek(base);
  return arr;
}

// See https://wowdev.wiki/M2#Standard_animation_block
export function read_m2_track<V extends M2TrackValue = M2TrackValue>(
  data: BufferWrapper,
  ofs: number,
  dataType: M2TrackDataType,
  useAnims = false,
  animFiles: Map<number, BufferWrapper> = new Map(),
): M2Track<V> {
  const interpolation = data.readUInt16LE();
  const globalSeq = data.readUInt16LE();

  let timestamps: number[][];
  let values: M2TrackValue[][];

  if (useAnims) {
    timestamps = read_m2_array_array(data, ofs, 'uint32', useAnims, animFiles) as number[][];
    values = read_m2_array_array(data, ofs, dataType, useAnims, animFiles);
  } else {
    timestamps = read_m2_array_array(data, ofs, 'uint32') as number[][];
    values = read_m2_array_array(data, ofs, dataType);
  }

  return new M2Track(globalSeq, interpolation, timestamps, values as V[][]);
}

// See https://wowdev.wiki/Common_Types#CAaBox
export function read_caa_bb(data: BufferWrapper): CAaBox {
  return { min: data.readFloatLE(3), max: data.readFloatLE(3) };
}

/**
 * Read a 1D M2Array of a given data type at the current position.
 * Returns a flat array of values.
 */
export function read_m2_array(data: BufferWrapper, ofs: number, dataType: M2ArrayDataType): M2TrackValue[] {
  const arrCount = data.readUInt32LE();
  const arrOfs = data.readUInt32LE();

  const base = data.offset;
  const result: M2TrackValue[] = new Array(arrCount);

  if (arrCount > 0 && arrOfs > 0) {
    data.seek(ofs + arrOfs);

    for (let i = 0; i < arrCount; i++) {
      switch (dataType) {
        case 'uint32': result[i] = data.readUInt32LE(); break;
        case 'int16': result[i] = data.readInt16LE(); break;
        case 'uint16': result[i] = data.readUInt16LE(); break;
        case 'uint8': result[i] = data.readUInt8(); break;
        case 'float': result[i] = data.readFloatLE(); break;
        case 'float2': result[i] = data.readFloatLE(2); break;
        case 'float3': result[i] = data.readFloatLE(3); break;
        default:
          throw new Error(`Unknown data type for read_m2_array: ${String(dataType)}`);
      }
    }
  }

  data.seek(base);
  return result;
}

/**
 * Read an M2PartTrack: a single set of timestamps (uint16) and values (various types), normalized age-based.
 */
export function read_m2_part_track(data: BufferWrapper, ofs: number, valueType: M2ArrayDataType): M2PartTrack {
  // timestamps: M2Array<uint16>
  const timestamps = read_m2_array(data, ofs, 'uint16') as number[];
  // values: M2Array<valueType>
  const values = read_m2_array(data, ofs, valueType);
  return { timestamps, values };
}

export default {
  M2Track, read_m2_array_array, read_m2_track, read_caa_bb, read_m2_array, read_m2_part_track, read_m2_spline_track,
};
