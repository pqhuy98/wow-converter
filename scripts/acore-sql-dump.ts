import { createReadStream } from 'fs';
import { join } from 'path';
import { createInterface } from 'readline';

import type { AcoreTableSpec } from './acore-sqlite-tables';

/** Reads AzerothCore's one-row-per-line MySQL base dumps without executing SQL. */
export async function* readAcoreSqlDump(directory: string, table: AcoreTableSpec): AsyncGenerator<unknown[]> {
  const lines = createInterface({ input: createReadStream(join(directory, `${table.name}.sql`)), crlfDelay: Infinity });
  const columns: string[] = [];
  let readingSchema = false;
  let readingRows = false;
  let indexes: number[] = [];
  try {
    for await (const line of lines) {
      if (line.startsWith(`CREATE TABLE \`${table.name}\``)) readingSchema = true;
      if (readingSchema) {
        const column = /^\s+`([^`]+)`/.exec(line)?.[1];
        if (column) columns.push(column);
        if (line.startsWith(')')) {
          readingSchema = false;
          indexes = table.columns.map((name) => columns.indexOf(name));
          if (indexes.some((index) => index < 0)) throw new Error(`Missing required columns in ${table.name} dump`);
        }
      }
      if (line.startsWith(`INSERT INTO \`${table.name}\` VALUES`)) {
        if (indexes.length === 0) throw new Error(`Missing schema for ${table.name}`);
        readingRows = true;
        continue;
      }
      if (!readingRows) continue;
      if (!line.startsWith('(')) throw new Error(`Unsupported INSERT layout in ${table.name}`);
      const row = parseDumpRow(line);
      if (row.length !== columns.length) throw new Error(`Column count mismatch in ${table.name}`);
      yield indexes.map((index) => row[index]);
      if (line.endsWith(';')) readingRows = false;
    }
    if (readingRows) throw new Error(`Unterminated INSERT in ${table.name}`);
  } finally {
    lines.close();
  }
}

function parseDumpRow(line: string): unknown[] {
  const values: unknown[] = [];
  let index = 1;
  while (index < line.length) {
    let value = '';
    if (line[index] === "'") {
      index++;
      let closed = false;
      while (index < line.length) {
        const char = line[index++];
        if (char === '\\') {
          const escaped = line[index++];
          const escapes: Record<string, string> = { n: '\n', r: '\r', t: '\t', '0': '\0', b: '\b', Z: '\x1a' };
          value += escapes[escaped] ?? escaped;
        } else if (char === "'") {
          if (line[index] === "'") { value += "'"; index++; }
          else { closed = true; break; }
        } else value += char;
      }
      if (!closed) throw new Error('Unterminated SQL string');
      values.push(value);
    } else {
      while (index < line.length && line[index] !== ',' && line[index] !== ')') value += line[index++];
      if (value === 'NULL') values.push(null);
      else {
        const number = Number(value);
        if (value.trim() === '' || !Number.isFinite(number)) throw new Error(`Invalid SQL number ${value}`);
        values.push(number);
      }
    }
    if (line[index] === ')') return values;
    if (line[index++] !== ',') throw new Error('Invalid SQL row separator');
  }
  throw new Error('Unterminated SQL row');
}
