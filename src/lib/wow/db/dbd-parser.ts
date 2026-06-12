/**
 * DBD (database definition) parser, ported from wow.export (src/js/db/DBDParser.js).
 */
/* eslint-disable max-classes-per-file */
import type { BufferWrapper } from '../formats/buffer';

/** Pattern to match column definitions in a DBD document. */
const PATTERN_COLUMN = /^(int|float|locstring|string)(<[^:]+::[^>]+>)?\s([^\s]+)/;

/** Pattern to match build identifiers in a DBD document. */
const PATTERN_BUILD = /^BUILD\s(.*)/;

/** Pattern to match build range identifiers in a DBD document. */
const PATTERN_BUILD_RANGE = /([^-]+)-(.*)/;

/** Pattern to match comment entries in a DBD document. */
const PATTERN_COMMENT = /^COMMENT\s/;

/** Pattern to match layout hash identifiers in a DBD document. */
const PATTERN_LAYOUT = /^LAYOUT\s(.*)/;

/** Pattern to match a field entry in a DBD document. */
const PATTERN_FIELD = /^(\$([^$]+)\$)?([^<[]+)(<(u|)(\d+)>)?(\[(\d+)\])?$/;

/** Pattern to match the components of a build ID. */
const PATTERN_BUILD_ID = /(\d+).(\d+).(\d+).(\d+)/;

export type DBDColumnType = 'int' | 'float' | 'locstring' | 'string';

interface BuildID {
  major: number;
  minor: number;
  patch: number;
  rev: number;
}

/** Parse a build ID into components. */
function parseBuildID(buildID: string): BuildID {
  const parts = buildID.match(PATTERN_BUILD_ID);
  const entry: BuildID = {
    major: 0, minor: 0, patch: 0, rev: 0,
  };

  if (parts !== null) {
    entry.major = parseInt(parts[1], 10);
    entry.minor = parseInt(parts[2], 10);
    entry.patch = parseInt(parts[3], 10);
    entry.rev = parseInt(parts[4], 10);
  }

  return entry;
}

/** Returns true if the provided build falls within the provided range. */
function isBuildInRange(buildStr: string, minStr: string, maxStr: string): boolean {
  const build = parseBuildID(buildStr);
  const min = parseBuildID(minStr);
  const max = parseBuildID(maxStr);

  if (build.major < min.major || build.major > max.major) return false;
  if (build.minor < min.minor || build.minor > max.minor) return false;
  if (build.patch < min.patch || build.patch > max.patch) return false;
  if (build.rev < min.rev || build.rev > max.rev) return false;

  return true;
}

export class DBDField {
  type: DBDColumnType;

  name: string;

  isSigned = true;

  isID = false;

  isInline = true;

  isRelation = false;

  arrayLength = -1;

  size = -1;

  constructor(fieldName: string, fieldType: DBDColumnType) {
    this.type = fieldType;
    this.name = fieldName;
  }
}

export class DBDEntry {
  builds = new Set<string>();

  buildRanges = new Set<{ min: string; max: string }>();

  layoutHashes = new Set<string>();

  fields = new Set<DBDField>();

  /** Add a build to this DBD entry. */
  addBuild(min: string, max?: string): void {
    if (max !== undefined) this.buildRanges.add({ min, max });
    else this.builds.add(min);
  }

  /** Add a layout hash to this DBD entry. */
  addLayoutHashes(...hashes: string[]): void {
    for (const hash of hashes) this.layoutHashes.add(hash);
  }

  /** Add a field to this DBD entry. */
  addField(field: DBDField): void {
    this.fields.add(field);
  }

  /** Check if this entry is valid for the provided buildID or layout hash. */
  isValidFor(buildID: string, layoutHash: string): boolean {
    // Layout hash takes priority, being the quickest to check.
    if (this.layoutHashes.has(layoutHash)) return true;

    // Check for a single build ID.
    if (this.builds.has(buildID)) return true;

    // Fallback to checking build ranges.
    for (const range of this.buildRanges) {
      if (isBuildInRange(buildID, range.min, range.max)) return true;
    }

    return false;
  }
}

export class DBDParser {
  entries = new Set<DBDEntry>();

  columns = new Map<string, DBDColumnType>();

  constructor(data: BufferWrapper) {
    this.parse(data);
  }

  /** Get a DBD structure for the provided buildID and layoutHash. */
  getStructure(buildID: string, layoutHash: string): DBDEntry | null {
    for (const entry of this.entries) {
      if (entry.isValidFor(buildID, layoutHash)) return entry;
    }

    return null;
  }

  /** Parse the contents of a DBD document. */
  private parse(data: BufferWrapper): void {
    const lines = data.readLines();

    // Separate the file into chunks separated by empty lines.
    let chunk: string[] = [];
    for (const line of lines) {
      if (line.trim().length > 0) {
        chunk.push(line);
      } else {
        this.parseChunk(chunk);
        chunk = [];
      }
    }

    // Ensure last chunk is accounted for.
    if (chunk.length > 0) this.parseChunk(chunk);

    if (this.columns.size === 0) throw new Error('Invalid DBD: No columns defined.');
  }

  /** Parse a chunk from this DBD document. */
  private parseChunk(chunk: string[]): void {
    if (chunk[0] === 'COLUMNS') {
      this.parseColumnChunk(chunk);
    } else {
      const entry = new DBDEntry();
      for (const line of chunk) {
        // Build IDs.
        const buildMatch = line.match(PATTERN_BUILD);
        if (buildMatch !== null) {
          // BUILD 1.7.0.4671-1.8.0.4714
          // BUILD 0.9.1.3810
          // BUILD 1.13.6.36231, 1.13.6.36310
          const builds = buildMatch[1].split(',');
          for (const build of builds) {
            const buildRange = build.match(PATTERN_BUILD_RANGE);
            if (buildRange !== null) entry.addBuild(buildRange[1], buildRange[2]);
            else entry.addBuild(build.trim());
          }

          continue;
        }

        // Skip comments.
        if (line.match(PATTERN_COMMENT) !== null) continue;

        // Layout hashes.
        const layoutMatch = line.match(PATTERN_LAYOUT);
        if (layoutMatch !== null) {
          // LAYOUT 0E84A21C, 35353535
          entry.addLayoutHashes(...(layoutMatch[1].split(',').map((e) => e.trim())));
          continue;
        }

        const fieldMatch = line.match(PATTERN_FIELD);
        if (fieldMatch !== null) {
          const fieldName = fieldMatch[3];
          const fieldType = this.columns.get(fieldName);

          if (fieldType === undefined) throw new Error(`Invalid DBD: No field type defined for ${fieldName}`);

          const field = new DBDField(fieldName, fieldType);

          // Parse annotations, (eg 'id,noninline,relation').
          if (fieldMatch[2] !== undefined) {
            const annotations = fieldMatch[2].split(',');
            if (annotations.includes('id')) field.isID = true;
            if (annotations.includes('noninline')) field.isInline = false;
            if (annotations.includes('relation')) field.isRelation = true;
          }

          // Parse signedness, either 'u' or undefined.
          if (fieldMatch[5] && fieldMatch[5].length > 0) field.isSigned = false;

          // Parse data size (eg '32').
          if (fieldMatch[6] !== undefined) {
            const dataSize = parseInt(fieldMatch[6], 10);
            if (!Number.isNaN(dataSize)) field.size = dataSize;
          }

          // Parse array size (eg '2').
          if (fieldMatch[8] !== undefined) {
            const arrayLength = parseInt(fieldMatch[8], 10);
            if (!Number.isNaN(arrayLength)) field.arrayLength = arrayLength;
          }

          entry.addField(field);
        }
      }

      this.entries.add(entry);
    }
  }

  /** Parse the column definition of a DBD document. */
  private parseColumnChunk(chunk: string[]): void {
    if (chunk === undefined) throw new Error('Invalid DBD: Missing column definitions.');

    // Remove the COLUMNS header.
    chunk.shift();

    for (const entry of chunk) {
      const match = entry.match(PATTERN_COLUMN);
      if (match !== null) {
        const columnType = match[1] as DBDColumnType; // int|float|locstring|string
        // const columnForeignKey = match[2]; // <TableName::ColumnName> or undefined
        const columnName = match[3].replace('?', ''); // Field_6_0_1_18179_000?

        this.columns.set(columnName, columnType);
      }
    }
  }
}

export default DBDParser;
