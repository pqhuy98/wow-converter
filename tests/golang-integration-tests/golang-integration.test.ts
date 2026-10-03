/**
 * Runs Go tests marked `//go:build integration_tests`.
 *
 * Those files are left out of `go test ./...`. This suite starts wow-data-server
 * on port 18753 (dev default is 17753) and points the tests at it.
 *
 *   bun test --max-concurrency=1 tests/golang-integration-tests
 */
import {
  afterAll, beforeAll, setDefaultTimeout, test,
} from 'bun:test';
import { spawnSync } from 'child_process';
import { readdirSync, readFileSync } from 'fs';
import path from 'path';

import {
  killPorts, repoRoot, spawnManaged, stopChildren, waitForEndpoint,
} from '../helpers';

const port = 18753;
const baseURL = `http://127.0.0.1:${port}`;
const golangDir = path.join(repoRoot, 'golang');
const timeoutMs = 60 * 60_000;

const dataServerEnv = {
  WOW_DATA_SERVER_PORT: String(port),
  WOW_DATA_SERVER_URL: baseURL,
  WOW_DATA_SERVER_SOCKET: '',
  WOW_DATA_TRANSPORT: '',
  WOW_CONVERTER_BUNDLED: '',
};

setDefaultTimeout(timeoutMs);

beforeAll(async () => {
  killPorts([port]);
  const child = spawnManaged(
    'wow-data-server',
    'go',
    ['run', './cmd/wow-data-server'],
    golangDir,
    { ...dataServerEnv, WOW_LOG_PREFIX: 'go-integration' },
    port,
  );
  await waitForEndpoint(child, `${baseURL}/rest/getCascInfo`, 10 * 60_000, cascInfoReady);
}, 10 * 60_000);

afterAll(() => {
  stopChildren();
});

test('integration-tagged go tests', () => {
  const { packages, pattern } = integrationTests(golangDir);
  const result = spawnSync('go', [
    'test', '-tags', 'integration_tests', '-count=1', '-run', pattern, ...packages,
  ], {
    cwd: golangDir,
    stdio: 'inherit',
    env: { ...process.env, ...dataServerEnv },
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`go test exited ${result.status ?? 'unknown'}`);
});

function cascInfoReady(value: unknown): boolean {
  return typeof value === 'object' && value !== null && 'id' in value && value.id === 'CASC_INFO';
}

function integrationTests(root: string): { packages: string[]; pattern: string } {
  const names = new Set<string>();
  const packages = new Set<string>();
  for (const file of walkGoTests(root)) {
    const text = readFileSync(file, 'utf8');
    const firstLine = text.split(/\r?\n/, 1)[0]?.trim();
    if (firstLine !== '//go:build integration_tests') continue;
    const found = [...text.matchAll(/^func (Test\w+)\(/gm)].map((match) => match[1] ?? '');
    const tests = found.filter((name) => name !== '' && name !== 'TestMain');
    if (tests.length === 0) continue;
    for (const name of tests) names.add(name);
    const rel = path.relative(root, path.dirname(file)).replaceAll('\\', '/');
    packages.add(`./${rel}`);
  }
  if (names.size === 0) throw new Error('no //go:build integration_tests tests found');
  return { packages: [...packages].sort(), pattern: [...names].sort().join('|') };
}

function walkGoTests(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === 'vendor' || entry.name.startsWith('.')) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) out.push(...walkGoTests(full));
    else if (entry.name.endsWith('_test.go')) out.push(full);
  }
  return out;
}
