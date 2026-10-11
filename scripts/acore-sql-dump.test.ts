import { expect, test } from 'bun:test';
import { mkdtempSync, rmSync, writeFileSync } from 'fs';
import { join } from 'path';

import { readAcoreSqlDump } from './acore-sql-dump';

test('dump reader projects requested columns and decodes MySQL string escapes', async () => {
  const directory = mkdtempSync(join(import.meta.dir, '.acore-test-'));
  try {
    writeFileSync(join(directory, 'objects.sql'), String.raw`CREATE TABLE ` + '`objects`' + String.raw` (
  ` + '`id`' + String.raw` int,
  ` + '`name`' + String.raw` text,
  ` + '`unused`' + String.raw` text
);
INSERT INTO ` + '`objects`' + String.raw` VALUES
(1,'Pipe, valve\'s','NULL'),
(2,'World\\IceCrown\\pipe.mdx',NULL);
`);
    const rows: unknown[][] = [];
    for await (const row of readAcoreSqlDump(directory, { name: 'objects', columns: ['name', 'id'], createSql: '' })) rows.push(row);
    expect(rows).toEqual([["Pipe, valve's", 1], ['World\\IceCrown\\pipe.mdx', 2]]);
  } finally {
    rmSync(directory, { recursive: true });
  }
});
