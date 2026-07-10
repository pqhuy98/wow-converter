// Runs mount MDL parity cases (TS vs Go) one-by-one.
import { spawnSync } from 'child_process';
import path from 'path';

const tsUrl = process.env.TS_WOW_DATA_SERVER_URL ?? 'http://127.0.0.1:17753';
const goUrl = process.env.GO_WOW_DATA_SERVER_URL ?? 'http://127.0.0.1:17754';
const tsOut = '.parity-artifacts/loop-ts-mount-mdl';
const goOut = '.parity-artifacts/loop-go-mount-mdl';

const failures: string[] = [];
for (let i = 0; i < 2; i++) {
  const label = `mount-case-${i}`;
  console.log(`\n== MOUNT MDL parity ${i + 1}/2: ${label} ==`);

  const ts = spawnSync(
    'bun',
    ['scripts/export-parity-artifacts.ts', '--mount', `--out=${tsOut}`, '--fresh', `--offset=${i}`, '--limit=1', '--format=mdl'],
    { env: { ...process.env, WOW_DATA_SERVER_URL: tsUrl, WOW_DATA_SERVER_PORT: new URL(tsUrl).port }, stdio: 'inherit', shell: true },
  );
  if (ts.status !== 0) {
    failures.push(`${label}: TS export failed`);
    continue;
  }

  const go = spawnSync(
    'go',
    ['run', './test/cmd/test-export', '-mount', `-out=${goOut}`, '-offset', String(i), '-limit', '1', '-format', 'mdl'],
    { cwd: path.join(process.cwd(), 'golang'), env: { ...process.env, WOW_DATA_SERVER_URL: goUrl }, stdio: 'inherit', shell: true },
  );
  if (go.status !== 0) {
    failures.push(`${label}: Go export failed`);
    continue;
  }

  const diff = spawnSync(
    'git',
    ['diff', '--no-index', '--stat', path.join(tsOut, `${label}.mdl`), path.join(goOut, `${label}.mdl`)],
    { stdio: 'pipe', shell: true },
  );
  if (diff.status !== 0 && diff.stdout?.length) {
    console.log(diff.stdout.toString());
    failures.push(`${label}: MDL differs`);
  } else {
    console.log(`${label}: OK`);
  }
}

if (failures.length) {
  console.error('\nMount parity failures:\n' + failures.join('\n'));
  process.exit(1);
}
console.log('\nAll mount parity cases passed.');
