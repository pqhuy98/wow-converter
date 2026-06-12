/**
 * WDC2-WDC5 DB2 table reader, ported from wow.export (src/js/db/WDCReader.js).
 */
import assert from 'assert/strict';
import path from 'path';
import util from 'util';

import { replaceExtension } from '@/lib/wow/export/writers/export-helper';
import { write } from '@/lib/wow/log';
import { getCasc } from '@/lib/wow/server/runtime';

import { BufferWrapper } from '../formats/buffer';
import { constants } from '../formats/constants';
import { downloadFile } from '../formats/generics';
import { wowConfig } from '../server/config';
import { CompressionType } from './compression-type';
import { DBDEntry, DBDField, DBDParser } from './dbd-parser';
import { FieldType } from './field-type';

const TABLE_FORMATS: Record<number, { name: string; wdcVersion: number }> = {
  0x32434457: { name: 'WDC2', wdcVersion: 2 },
  0x434C5331: { name: 'CLS1', wdcVersion: 2 },
  0x33434457: { name: 'WDC3', wdcVersion: 3 },
  0x34434457: { name: 'WDC4', wdcVersion: 4 },
  0x35434457: { name: 'WDC5', wdcVersion: 5 },
};

/** A single field value within a DB2 row. */
export type DB2FieldValue = number | bigint | string | number[] | bigint[] | string[];

/** A DB2 row keyed by schema field names. */
export type DB2Row = Record<string, DB2FieldValue>;

type SchemaEntry = FieldType | [FieldType, number];

interface SectionHeader {
  tactKeyHash: bigint;
  fileOffset: number;
  recordCount: number;
  stringTableSize: number;
  copyTableSize?: number;
  offsetMapOffset?: number;
  offsetRecordsEnd?: number;
  idListSize: number;
  relationshipDataSize: number;
  offsetMapIDCount?: number;
  copyTableCount?: number;
}

interface FieldInfo {
  fieldOffsetBits: number;
  fieldSizeBits: number;
  additionalDataSize: number;
  fieldCompression: number;
  fieldCompressionPacking: number[];
}

interface Section {
  header: SectionHeader;
  isNormal: boolean;
  recordDataOfs: number;
  recordDataSize: number;
  stringBlockOfs: number;
  idList: number[];
  offsetMap?: { offset: number; size: number }[];
  relationshipMap?: Map<number, number>;
}

/** Returns the schema type for a DBD field. */
function convertDBDToSchemaType(entry: DBDField): FieldType {
  if (!entry.isInline && entry.isRelation) return FieldType.Relation;
  if (!entry.isInline && entry.isID) return FieldType.NonInlineID;

  // TODO: Handle string separate to locstring in the event we need it.
  if (entry.type === 'string' || entry.type === 'locstring') return FieldType.String;
  if (entry.type === 'float') return FieldType.Float;

  if (entry.type === 'int') {
    switch (entry.size) {
      case 8: return entry.isSigned ? FieldType.Int8 : FieldType.UInt8;
      case 16: return entry.isSigned ? FieldType.Int16 : FieldType.UInt16;
      case 32: return entry.isSigned ? FieldType.Int32 : FieldType.UInt32;
      case 64: return entry.isSigned ? FieldType.Int64 : FieldType.UInt64;
      default: throw new Error(`Unsupported DBD integer size ${entry.size}`);
    }
  }

  throw new Error(`Unrecognized DBD type ${String(entry.type)}`);
}

/**
 * Defines unified logic between WDC2 and WDC3+.
 */
export class WDCReader {
  fileName: string;

  rows = new Map<number, DB2Row>();

  copyTable = new Map<number, number>();

  stringTable = new Map<number, string>();

  schema = new Map<string, SchemaEntry>();

  isInflated = false;

  isLoaded = false;

  idField: string | null = null;

  idFieldIndex: number | null = null;

  relationshipLookup = new Map<number, number[]>();

  constructor(fileName: string) {
    this.fileName = fileName;
  }

  /** Returns the amount of rows available in the table. */
  get size(): number {
    return this.rows.size + this.copyTable.size;
  }

  /**
   * Get a row from this table.
   * Returns NULL if the row does not exist.
   */
  getRow(recordID: number): DB2Row | null {
    // The table needs to be loaded before we attempt to access a row.
    if (!this.isLoaded) throw new Error('Attempted to read a data table row before table was loaded.');

    const id = parseInt(String(recordID), 10);

    // Look this row up as a normal entry.
    const record = this.rows.get(id);
    if (record !== undefined) return record;

    // Check if the copy table contains a mapping entry.
    const copyID = this.copyTable.get(id);
    if (copyID !== undefined) {
      const copy = this.rows.get(copyID);
      if (copy !== undefined) {
        const tempCopy = { ...copy };
        tempCopy.ID = id;
        return tempCopy;
      }
    }

    // Row does not exist.
    return null;
  }

  /**
   * Returns all available rows in the table.
   * Calling this will permanently inflate internal copy data; use wisely.
   */
  getAllRows(): Map<number, DB2Row> {
    // The table needs to be loaded before we attempt to access the rows.
    if (!this.isLoaded) throw new Error('Attempted to read a data table rows before table was loaded.');

    const rows = this.rows;

    // Inflate all copy table data before returning.
    if (!this.isInflated) {
      for (const [destID, srcID] of this.copyTable) {
        const rowData = { ...rows.get(srcID) } as DB2Row;
        rowData.ID = destID;
        rows.set(destID, rowData);
      }

      this.isInflated = true;
    }

    return rows;
  }

  /**
   * Get rows by foreign key value (uses relationship maps).
   * Returns empty array if no rows found or table has no relationship data.
   */
  getRelationRows(foreignKeyValue: number): DB2Row[] {
    if (!this.isLoaded) throw new Error('Attempted to query relationship data before table was loaded.');

    const fkValue = parseInt(String(foreignKeyValue), 10);

    const recordIDs = this.relationshipLookup.get(fkValue);
    if (!recordIDs || recordIDs.length === 0) return [];

    const results: DB2Row[] = [];
    for (const recordID of recordIDs) {
      const row = this.rows.get(recordID);
      if (row !== undefined) results.push(row);
    }

    return results;
  }

  /** Load the schema for this table. */
  async loadSchema(layoutHash: string): Promise<void> {
    const casc = getCasc();
    const buildID = casc.getBuildName();

    const tableName = replaceExtension(path.basename(this.fileName));
    const dbdName = `${tableName}.dbd`;

    let structure: DBDEntry | null = null;
    write('Loading table definitions %s (%s %s)...', dbdName, buildID, layoutHash);

    // First check if a valid DBD exists in cache and contains a definition for this build.
    let rawDbd = await casc.cache.getFile(dbdName, constants.CACHE.DIR_DBD);
    if (rawDbd !== null) structure = new DBDParser(rawDbd).getStructure(buildID, layoutHash);

    // No cached definition, download updated DBD and check again.
    if (structure === null) {
      const dbd_url = util.format(wowConfig.dbdURL, tableName);
      const dbd_url_fallback = util.format(wowConfig.dbdFallbackURL, tableName);

      try {
        write(`No cached DBD, downloading new from ${dbd_url}`);
        rawDbd = await downloadFile([dbd_url, dbd_url_fallback]);

        // Persist the newly download DBD to disk for future loads.
        await casc.cache.storeFile(dbdName, rawDbd, constants.CACHE.DIR_DBD);

        // Parse the updated DBD and check for definition.
        structure = new DBDParser(rawDbd).getStructure(buildID, layoutHash);
      } catch (e) {
        write(e);
        throw new Error(`Unable to download DBD for ${tableName}`);
      }
    }

    if (structure === null) throw new Error(`No table definition available for ${tableName}`);

    this.buildSchemaFromDBDStructure(structure);
  }

  /** Builds a schema for this data table using the provided DBD structure. */
  buildSchemaFromDBDStructure(structure: DBDEntry): void {
    for (const field of structure.fields) {
      const fieldType = convertDBDToSchemaType(field);
      if (field.arrayLength > -1) this.schema.set(field.name, [fieldType, field.arrayLength]);
      else this.schema.set(field.name, fieldType);
    }
  }

  /** Gets index of ID field. */
  getIDIndex(): number | null {
    if (!this.isLoaded) throw new Error('Attempted to get ID index before table was loaded.');

    return this.idFieldIndex;
  }

  /** Parse the data table. */
  async parse(input?: BufferWrapper): Promise<void> {
    write('Loading DB file %s from CASC', this.fileName);

    const data = input ?? await getCasc().getFileByName(this.fileName, true, false, true);

    // wdc_magic
    const magic = data.readUInt32LE();
    const format = TABLE_FORMATS[magic];

    if (!format) throw new Error(`Unsupported DB2 type: ${magic}`);

    const wdcVersion = format.wdcVersion;
    write('Processing DB file %s as %s', this.fileName, format.name);

    // Skip over WDC5 specific information for now
    if (wdcVersion === 5) {
      data.readUInt32LE(); // Schema version?
      data.readUInt8(128); // Schema build string
    }

    // wdc_db2_header
    const recordCount = data.readUInt32LE();
    data.move(4); // fieldCount
    const recordSize = data.readUInt32LE();
    data.move(4); // stringTableSize
    data.move(4); // tableHash
    const layoutHash = data.readUInt8(4).reverse().map((e) => e.toString(16).padStart(2, '0')).join('')
      .toUpperCase();
    const minID = data.readUInt32LE();
    const maxID = data.readUInt32LE();
    data.move(4); // locale
    const flags = data.readUInt16LE();
    const idIndex = data.readUInt16LE();
    this.idFieldIndex = idIndex;
    const totalFieldCount = data.readUInt32LE();
    data.move(4); // bitpackedDataOffset
    data.move(4); // lookupColumnCount
    const fieldStorageInfoSize = data.readUInt32LE();
    const commonDataSize = data.readUInt32LE();
    const palletDataSize = data.readUInt32LE();
    const sectionCount = data.readUInt32LE();

    // Load the DBD and parse a schema from it.
    await this.loadSchema(layoutHash);

    // wdc_section_header section_headers[section_count]
    const sectionHeaders = new Array<SectionHeader>(sectionCount);
    for (let i = 0; i < sectionCount; i++) {
      sectionHeaders[i] = wdcVersion === 2 ? {
        tactKeyHash: data.readUInt64LE(),
        fileOffset: data.readUInt32LE(),
        recordCount: data.readUInt32LE(),
        stringTableSize: data.readUInt32LE(),
        copyTableSize: data.readUInt32LE(),
        offsetMapOffset: data.readUInt32LE(),
        idListSize: data.readUInt32LE(),
        relationshipDataSize: data.readUInt32LE(),
      } : {
        tactKeyHash: data.readUInt64LE(),
        fileOffset: data.readUInt32LE(),
        recordCount: data.readUInt32LE(),
        stringTableSize: data.readUInt32LE(),
        offsetRecordsEnd: data.readUInt32LE(),
        idListSize: data.readUInt32LE(),
        relationshipDataSize: data.readUInt32LE(),
        offsetMapIDCount: data.readUInt32LE(),
        copyTableCount: data.readUInt32LE(),
      };
    }

    // fields[header.total_field_count]
    const fields = new Array<{ size: number; position: number }>(totalFieldCount);
    for (let i = 0; i < totalFieldCount; i++) fields[i] = { size: data.readInt16LE(), position: data.readUInt16LE() };

    // field_info[header.field_storage_info_size / sizeof(field_storage_info)]
    const fieldInfo = new Array<FieldInfo>(fieldStorageInfoSize / (4 * 6));
    for (let i = 0, n = fieldInfo.length; i < n; i++) {
      fieldInfo[i] = {
        fieldOffsetBits: data.readUInt16LE(),
        fieldSizeBits: data.readUInt16LE(),
        additionalDataSize: data.readUInt32LE(),
        fieldCompression: data.readUInt32LE(),
        fieldCompressionPacking: data.readUInt32LE(3),
      };
    }

    // char pallet_data[header.pallet_data_size];
    let prevPos = data.offset;

    const palletData = new Array<number[] | undefined>(fieldInfo.length);
    for (let fieldIndex = 0, nFields = fieldInfo.length; fieldIndex < nFields; fieldIndex++) {
      const thisFieldInfo = fieldInfo[fieldIndex];
      if (thisFieldInfo.fieldCompression === CompressionType.BitpackedIndexed || thisFieldInfo.fieldCompression === CompressionType.BitpackedIndexedArray) {
        const pallet: number[] = palletData[fieldIndex] = [];
        for (let i = 0; i < thisFieldInfo.additionalDataSize / 4; i++) pallet[i] = data.readUInt32LE();
      }
    }

    // Ensure we've read the expected amount of pallet data.
    assert.strictEqual(data.offset, prevPos + palletDataSize, 'Read incorrect amount of pallet data');

    prevPos = data.offset;

    // char common_data[header.common_data_size];
    const commonData = new Array<Map<number, number> | undefined>(fieldInfo.length);
    for (let fieldIndex = 0, nFields = fieldInfo.length; fieldIndex < nFields; fieldIndex++) {
      const thisFieldInfo = fieldInfo[fieldIndex];
      if (thisFieldInfo.fieldCompression === CompressionType.CommonData) {
        const commonDataMap = commonData[fieldIndex] = new Map<number, number>();

        for (let i = 0; i < thisFieldInfo.additionalDataSize / 8; i++) commonDataMap.set(data.readUInt32LE(), data.readUInt32LE());
      }
    }

    // Ensure we've read the expected amount of common data.
    assert.strictEqual(data.offset, prevPos + commonDataSize, 'Read incorrect amount of common data');

    // New WDC4 chunk: TODO read
    if (wdcVersion > 3) {
      for (let sectionIndex = 0; sectionIndex < sectionCount - 1; sectionIndex++) {
        const entryCount = data.readUInt32LE();
        data.move(entryCount * 4);
      }
    }

    // data_sections[header.section_count];
    const sections = new Array<Section>(sectionCount);
    const copyTable = this.copyTable;
    const stringTable = this.stringTable;
    let previousStringTableSize = 0;
    for (let sectionIndex = 0; sectionIndex < sectionCount; sectionIndex++) {
      const header = sectionHeaders[sectionIndex];
      const isNormal = !(flags & 1);

      const recordDataOfs = data.offset;
      const recordsOfs = wdcVersion === 2 ? header.offsetMapOffset! : header.offsetRecordsEnd!;
      const recordDataSize = isNormal ? recordSize * header.recordCount : recordsOfs - header.fileOffset;
      const stringBlockOfs = recordDataOfs + recordDataSize;

      let offsetMap: { offset: number; size: number }[] | undefined;
      if (wdcVersion === 2 && !isNormal) {
        data.seek(header.offsetMapOffset!);
        // offset_map_entry offset_map[header.max_id - header.min_id + 1];
        const offsetMapCount = maxID - minID + 1;
        offsetMap = new Array(offsetMapCount);
        for (let i = 0, n = offsetMapCount; i < n; i++) offsetMap[minID + i] = { offset: data.readUInt32LE(), size: data.readUInt16LE() };
      }

      if ((wdcVersion > 2) && isNormal) {
        data.seek(stringBlockOfs);
        for (let i = 0; i < header.stringTableSize;) {
          const oldPos = data.offset;
          const stringResult = data.readString(data.indexOf(0x0) - data.offset, 'utf8');

          if (stringResult !== '') stringTable.set(i + previousStringTableSize, stringResult);

          if (data.offset === oldPos) data.seek(oldPos + 1);

          i += (data.offset - oldPos);
        }

        previousStringTableSize += header.stringTableSize;
      }

      data.seek(stringBlockOfs + header.stringTableSize);

      // uint32_t id_list[section_headers.id_list_size / 4];
      const idListSize = header.idListSize / 4;
      const idList = idListSize > 0 ? data.readUInt32LE(idListSize) : [];

      // copy_table_entry copy_table[section_headers.copy_table_count];
      const copyTableCount = wdcVersion === 2 ? (header.copyTableSize! / 8) : header.copyTableCount!;
      for (let i = 0; i < copyTableCount; i++) {
        const destinationRowID = data.readInt32LE();
        const sourceRowID = data.readInt32LE();
        if (destinationRowID !== sourceRowID) copyTable.set(destinationRowID, sourceRowID);
      }

      if (wdcVersion > 2) {
        // offset_map_entry offset_map[section_headers.offset_map_id_count];
        offsetMap = new Array(header.offsetMapIDCount!);
        for (let i = 0, n = header.offsetMapIDCount!; i < n; i++) offsetMap[i] = { offset: data.readUInt32LE(), size: data.readUInt16LE() };
      }

      prevPos = data.offset;

      // relationship_map
      let relationshipMap: Map<number, number> | undefined;

      if (header.relationshipDataSize > 0) {
        const relationshipEntryCount = data.readUInt32LE();
        data.move(8); // relationshipMinID (UInt32) and relationshipMaxID (UInt32)

        relationshipMap = new Map();
        for (let i = 0; i < relationshipEntryCount; i++) {
          const foreignID = data.readUInt32LE();
          const recordIndex = data.readUInt32LE();
          relationshipMap.set(recordIndex, foreignID);
        }

        // If a section is encrypted it is highly likely we don't read the correct amount of data here. Skip ahead if so.
        if (prevPos + header.relationshipDataSize !== data.offset) data.seek(prevPos + header.relationshipDataSize);
      }

      // uint32_t offset_map_id_list[section_headers.offset_map_id_count];
      // Duplicate of id_list for sections with offset records.
      // TODO: Read
      if (wdcVersion > 2) data.move(header.offsetMapIDCount! * 4);

      sections[sectionIndex] = {
        header, isNormal, recordDataOfs, recordDataSize, stringBlockOfs, idList, offsetMap, relationshipMap,
      };
    }

    const castBuffer = BufferWrapper.alloc(8, true);

    // Parse section records.
    for (let sectionIndex = 0; sectionIndex < sectionCount; sectionIndex++) {
      const section = sections[sectionIndex];
      const header = section.header;
      const offsetMap = section.offsetMap;
      const isNormal = section.isNormal;

      // Skip parsing entries from encrypted sections.
      if (header.tactKeyHash !== BigInt(0)) {
        let isZeroed = true;

        // Check if record data is all zeroes
        data.seek(section.recordDataOfs);
        for (let i = 0, n = section.recordDataSize; i < n; i++) {
          if (data.readUInt8() !== 0x0) {
            isZeroed = false;
            break;
          }
        }

        // Check if first integer after string block (from id list or copy table) is non-0
        if (isZeroed && (wdcVersion > 2) && isNormal && (header.idListSize > 0 || header.copyTableCount! > 0)) {
          data.seek(section.stringBlockOfs + header.stringTableSize);
          isZeroed = data.readUInt32LE() === 0;
        }

        // Check if first entry in offsetMap has size 0
        if (isZeroed && (wdcVersion > 2) && header.offsetMapIDCount! > 0) isZeroed = offsetMap![0].size === 0;

        if (isZeroed) {
          write(`Skipping all-zero encrypted section ${sectionIndex} in file ${this.fileName}`);
          continue;
        }
      }

      // Total recordDataSize of all forward sections
      let outsideDataSize = 0;

      for (let i = 0; i < sectionCount; i++) {
        if (i < sectionIndex) outsideDataSize += sections[i].recordDataSize;
      }

      const hasIDMap = section.idList.length > 0;
      const emptyIDMap = hasIDMap && section.idList.every((id) => id === 0);
      for (let i = 0, n = header.recordCount; i < n; i++) {
        let recordID: number | undefined;

        if (hasIDMap) recordID = section.idList[i];

        if (hasIDMap && emptyIDMap) recordID = i;

        const recordOfs = isNormal ? (i * recordSize) : offsetMap![wdcVersion === 2 ? recordID! : i].offset;

        const absoluteRecordOffs = recordOfs - (recordCount * recordSize);

        if (!isNormal) data.seek(recordOfs);
        else data.seek(section.recordDataOfs + recordOfs);

        const out: DB2Row = {};
        let fieldIndex = 0;
        for (const [prop, type] of this.schema.entries()) {
          if (type === FieldType.Relation) {
            if (section.relationshipMap!.has(i)) out[prop] = section.relationshipMap!.get(i)!;
            else out[prop] = 0;

            continue;
          }

          if (type === FieldType.NonInlineID) {
            if (hasIDMap) out[prop] = section.idList[i];

            continue;
          }

          const recordFieldInfo = fieldInfo[fieldIndex];

          let count: number | undefined;
          let fieldType = type as FieldType;
          if (Array.isArray(type)) [fieldType, count] = type;

          const fieldOffsetBytes = Math.floor(recordFieldInfo.fieldOffsetBits / 8);

          switch (recordFieldInfo.fieldCompression) {
            case CompressionType.None:
              switch (fieldType) {
                case FieldType.String:
                  if (isNormal) {
                    if (count! > 0) {
                      const arr = new Array<string>(count!);
                      out[prop] = arr;
                      for (let stringArrayIndex = 0; stringArrayIndex < count!; stringArrayIndex++) {
                        const dataPos = (recordFieldInfo.fieldOffsetBits + (stringArrayIndex * (recordFieldInfo.fieldSizeBits / count!))) >> 3;
                        const ofs = data.readUInt32LE();

                        const stringTableIndex = outsideDataSize + absoluteRecordOffs + dataPos + ofs;

                        if (ofs === 0 || stringTableIndex === 0) {
                          arr[stringArrayIndex] = '';
                        } else if (stringTable.has(stringTableIndex)) {
                          arr[stringArrayIndex] = stringTable.get(stringTableIndex)!;
                        } else {
                          throw new Error('Missing stringtable entry');
                        }
                      }
                    } else {
                      const dataPos = recordFieldInfo.fieldOffsetBits >> 3;
                      const ofs = data.readUInt32LE();

                      const stringTableIndex = outsideDataSize + absoluteRecordOffs + dataPos + ofs;

                      if (ofs === 0 || stringTableIndex === 0) {
                        out[prop] = '';
                      } else if (stringTable.has(stringTableIndex)) {
                        out[prop] = stringTable.get(stringTableIndex)!;
                      } else {
                        throw new Error('Missing stringtable entry');
                      }
                    }
                  } else if (count! > 0) {
                    const arr = new Array<string>(count!);
                    out[prop] = arr;
                    for (let stringArrayIndex = 0; stringArrayIndex < count!; stringArrayIndex++) {
                      arr[stringArrayIndex] = data.readString(data.indexOf(0x0) - data.offset, 'utf8');
                      data.readInt8(); // Read NUL character
                    }
                  } else {
                    out[prop] = data.readString(data.indexOf(0x0) - data.offset, 'utf8');
                    data.readInt8(); // Read NUL character
                  }
                  break;

                case FieldType.Int8: out[prop] = data.readInt8(count as number); break;
                case FieldType.UInt8: out[prop] = data.readUInt8(count as number); break;
                case FieldType.Int16: out[prop] = data.readInt16LE(count as number); break;
                case FieldType.UInt16: out[prop] = data.readUInt16LE(count as number); break;
                case FieldType.Int32: out[prop] = data.readInt32LE(count as number); break;
                case FieldType.UInt32: out[prop] = data.readUInt32LE(count as number); break;
                case FieldType.Int64: out[prop] = data.readInt64LE(count as number); break;
                case FieldType.UInt64: out[prop] = data.readUInt64LE(count as number); break;
                case FieldType.Float: out[prop] = data.readFloatLE(count as number); break;
                default: throw new Error(`Unsupported field type: ${fieldType}`);
              }
              break;

            case CompressionType.CommonData:
              if (commonData[fieldIndex]!.has(recordID!)) out[prop] = commonData[fieldIndex]!.get(recordID!)!;
              else out[prop] = recordFieldInfo.fieldCompressionPacking[0]; // Default value
              break;

            case CompressionType.Bitpacked:
            case CompressionType.BitpackedSigned:
            case CompressionType.BitpackedIndexed:
            case CompressionType.BitpackedIndexedArray: {
              // TODO: All bitpacked stuff requires testing on more DB2s before being able to call it done.
              data.seek(section.recordDataOfs + recordOfs + fieldOffsetBytes);

              let rawValue: bigint;
              if (data.remainingBytes >= 8) {
                rawValue = data.readUInt64LE();
              } else {
                castBuffer.seek(0);
                castBuffer.writeBuffer(data);

                castBuffer.seek(0);
                rawValue = castBuffer.readUInt64LE();
              }

              // Read bitpacked value, in the case BitpackedIndex(Array) this is an index into palletData.

              // Get the remaining amount of bits that remain (we read to the nearest byte)
              const bitOffset = BigInt(recordFieldInfo.fieldOffsetBits & 7);
              const bitSize = 1n << BigInt(recordFieldInfo.fieldSizeBits);
              const bitpackedValue = (rawValue >> bitOffset) & (bitSize - BigInt(1));

              if (recordFieldInfo.fieldCompression === CompressionType.BitpackedIndexedArray) {
                const arrCount = recordFieldInfo.fieldCompressionPacking[2];
                const arr = new Array<number>(arrCount);
                out[prop] = arr;
                for (let j = 0; j < arrCount; j++) arr[j] = palletData[fieldIndex]![Number(bitpackedValue) * arrCount + j];
              } else if (recordFieldInfo.fieldCompression === CompressionType.BitpackedIndexed) {
                const palletIndex = Number(bitpackedValue);
                if (palletIndex in palletData[fieldIndex]!) out[prop] = palletData[fieldIndex]![palletIndex];
                else throw new Error(`Encountered missing pallet data entry for key ${bitpackedValue}, field ${fieldIndex}`);
              } else {
                out[prop] = bitpackedValue;
              }

              if (recordFieldInfo.fieldCompression === CompressionType.BitpackedSigned) out[prop] = BigInt(BigInt.asIntN(recordFieldInfo.fieldSizeBits, bitpackedValue));

              break;
            }
            default:
              throw new Error(`Unsupported field compression: ${recordFieldInfo.fieldCompression}`);
          }

          // Reinterpret field correctly for compression types other than None
          if (recordFieldInfo.fieldCompression !== CompressionType.None) {
            if (!Array.isArray(type)) {
              castBuffer.seek(0);
              if ((out[prop] as number | bigint) < 0) castBuffer.writeBigInt64LE(BigInt(out[prop] as number | bigint));
              else castBuffer.writeBigUInt64LE(BigInt(out[prop] as number | bigint));

              castBuffer.seek(0);
              switch (fieldType) {
                case FieldType.String:
                  throw new Error('Compressed string arrays currently not used/supported.');

                case FieldType.Int8: out[prop] = castBuffer.readInt8(); break;
                case FieldType.UInt8: out[prop] = castBuffer.readUInt8(); break;
                case FieldType.Int16: out[prop] = castBuffer.readInt16LE(); break;
                case FieldType.UInt16: out[prop] = castBuffer.readUInt16LE(); break;
                case FieldType.Int32: out[prop] = castBuffer.readInt32LE(); break;
                case FieldType.UInt32: out[prop] = castBuffer.readUInt32LE(); break;
                case FieldType.Int64: out[prop] = castBuffer.readInt64LE(); break;
                case FieldType.UInt64: out[prop] = castBuffer.readUInt64LE(); break;
                case FieldType.Float: out[prop] = castBuffer.readFloatLE(); break;
                default: throw new Error(`Unsupported field type: ${fieldType}`);
              }
            } else {
              const arr = out[prop] as number[];
              for (let j = 0; j < recordFieldInfo.fieldCompressionPacking[2]; j++) {
                castBuffer.seek(0);
                // Note: original checks `out[prop] < 0` (the array), which always
                // coerces to false, so the unsigned branch is always taken.
                castBuffer.writeBigUInt64LE(BigInt(arr[j]));

                castBuffer.seek(0);
                switch (fieldType) {
                  case FieldType.String:
                    throw new Error('Compressed string arrays currently not used/supported.');

                  case FieldType.Int8: arr[j] = castBuffer.readInt8(); break;
                  case FieldType.UInt8: arr[j] = castBuffer.readUInt8(); break;
                  case FieldType.Int16: arr[j] = castBuffer.readInt16LE(); break;
                  case FieldType.UInt16: arr[j] = castBuffer.readUInt16LE(); break;
                  case FieldType.Int32: arr[j] = castBuffer.readInt32LE(); break;
                  case FieldType.UInt32: arr[j] = castBuffer.readUInt32LE(); break;
                  case FieldType.Int64: arr[j] = castBuffer.readInt64LE() as unknown as number; break;
                  case FieldType.UInt64: arr[j] = castBuffer.readUInt64LE() as unknown as number; break;
                  case FieldType.Float: arr[j] = castBuffer.readFloatLE(); break;
                  default: throw new Error(`Unsupported field type: ${fieldType}`);
                }
              }
            }
          }

          // Round floats correctly
          if (fieldType === FieldType.Float) {
            if (count! > 0) {
              const arr = out[prop] as number[];
              for (let j = 0; j < count!; j++) arr[j] = Math.round(arr[j] * 100) / 100;
            } else {
              out[prop] = Math.round((out[prop] as number) * 100) / 100;
            }
          }

          if (!hasIDMap && fieldIndex === idIndex) {
            recordID = out[prop] as number;
            this.idField = prop;
          }

          fieldIndex++;
        }

        this.rows.set(recordID!, out);

        if (section.relationshipMap && section.relationshipMap.has(i)) {
          const foreignID = section.relationshipMap.get(i)!;
          if (!this.relationshipLookup.has(foreignID)) this.relationshipLookup.set(foreignID, []);

          this.relationshipLookup.get(foreignID)!.push(recordID!);
        }
      }
    }

    write('Parsed %s with %d rows', this.fileName, this.size);
    this.isLoaded = true;
  }
}

export default WDCReader;
