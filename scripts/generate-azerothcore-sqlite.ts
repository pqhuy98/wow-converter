import { Database } from 'bun:sqlite';
import { mkdirSync, unlinkSync } from 'fs';
import mysql from 'mysql2/promise';
import { dirname, resolve } from 'path';
import pg from 'pg';

import {
  ACORE_SQLITE_OUTPUT,
  ACORE_SQLITE_TABLES,
  type AcoreTableSpec,
  normalizeSqliteColumnValue,
} from './acore-sqlite-tables';

const BATCH_SIZE = 2000;
const DEFAULT_SOURCE_URL = 'mysql://acore:acore@127.0.0.1:3306/acore_world';

interface SourceDb {
  kind: 'mysql' | 'postgres';
  label: string;
  queryRows: (table: AcoreTableSpec) => AsyncGenerator<unknown[]>;
  close: () => Promise<void>;
}

async function createMysqlSource(url: string): Promise<SourceDb> {
  const connection = await mysql.createConnection(url);
  return {
    kind: 'mysql',
    label: url.replace(/\/\/([^:@/]+):([^@/]+)@/, '//$1:***@'),
    close: async () => { await connection.end(); },
    async* queryRows(table) {
      const columnList = table.columns.map((c) => `\`${c}\``).join(', ');
      const [rows] = await connection.query(`SELECT ${columnList} FROM \`${table.name}\``);
      for (const row of rows as Record<string, unknown>[]) {
        yield table.columns.map((column) => row[column]);
      }
    },
  };
}

async function createPostgresSource(url: string): Promise<SourceDb> {
  const connection = new pg.Client({ connectionString: url });
  await connection.connect();
  return {
    kind: 'postgres',
    label: url.replace(/\/\/([^:@/]+):([^@/]+)@/, '//$1:***@'),
    close: async () => { await connection.end(); },
    async* queryRows(table) {
      const columnList = table.columns.map((c) => `"${c}"`).join(', ');
      const result = await connection.query(`SELECT ${columnList} FROM "${table.name}"`);
      for (const row of result.rows) {
        yield table.columns.map((column) => row[column]);
      }
    },
  };
}

async function createSource(url: string): Promise<SourceDb> {
  if (url.startsWith('mysql://')) return createMysqlSource(url);
  if (url.startsWith('postgres://') || url.startsWith('postgresql://')) {
    return createPostgresSource(url);
  }
  throw new Error(`Unsupported ACORE_SOURCE_DATABASE_URL protocol: ${url.split(':')[0]}`);
}

function resolveSourceUrl(): string {
  const url = process.env.ACORE_SOURCE_DATABASE_URL ?? DEFAULT_SOURCE_URL;
  if (url.startsWith('file:')) {
    throw new Error('ACORE_SOURCE_DATABASE_URL must be a MySQL or PostgreSQL connection string');
  }
  return url;
}

function normalizeRow(table: AcoreTableSpec, row: unknown[]): unknown[] {
  return table.columns.map((column, index) => normalizeSqliteColumnValue(table.name, column, row[index]));
}

function insertRows(db: Database, table: AcoreTableSpec, rows: unknown[][]) {
  if (rows.length === 0) return;

  const placeholders = table.columns.map(() => '?').join(', ');
  const insert = db.prepare(
    `INSERT INTO ${table.name} (${table.columns.join(', ')}) VALUES (${placeholders})`,
  );

  db.run('BEGIN');
  try {
    for (const row of rows) insert.run(...normalizeRow(table, row));
    db.run('COMMIT');
  } catch (error) {
    db.run('ROLLBACK');
    throw error;
  }
}

function createSqlite(outputPath: string): Database {
  try {
    unlinkSync(outputPath);
  } catch {
    // fresh build
  }

  mkdirSync(dirname(outputPath), { recursive: true });
  const db = new Database(outputPath, { create: true });
  db.run('PRAGMA journal_mode = WAL');
  db.run('PRAGMA synchronous = NORMAL');
  return db;
}

function initTable(db: Database, table: AcoreTableSpec) {
  db.run(`DROP TABLE IF EXISTS ${table.name}`);
  db.run(table.createSql);
  for (const indexSql of table.indexes ?? []) {
    db.run(indexSql);
  }
}

async function importTableFromSource(
  db: Database,
  table: AcoreTableSpec,
  source: SourceDb,
) {
  let batch: unknown[][] = [];
  let count = 0;

  for await (const row of source.queryRows(table)) {
    batch.push(row);
    if (batch.length >= BATCH_SIZE) {
      insertRows(db, table, batch);
      count += batch.length;
      batch = [];
    }
  }

  if (batch.length > 0) {
    insertRows(db, table, batch);
    count += batch.length;
  }

  return count;
}

async function main() {
  const sourceUrl = resolveSourceUrl();
  const source = await createSource(sourceUrl);
  const outputPath = resolve(import.meta.dir, '..', ACORE_SQLITE_OUTPUT);
  const db = createSqlite(outputPath);

  console.log(`Writing ${outputPath}`);
  console.log(`Tables: ${ACORE_SQLITE_TABLES.map((t) => t.name).join(', ')}`);
  console.log(`Source: live ${source.kind} (${source.label})`);

  try {
    for (const table of ACORE_SQLITE_TABLES) {
      initTable(db, table);
      const count = await importTableFromSource(db, table, source);
      console.log(`  ${table.name}: ${count.toLocaleString()} rows`);
    }
  } finally {
    await source.close();
  }

  db.run('ANALYZE');
  db.close();
  console.log('Done.');
  process.exit(0);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
