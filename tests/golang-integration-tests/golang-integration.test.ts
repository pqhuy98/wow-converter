/**
 * Runs Go tests marked `//go:build integration_tests`.
 *
 * Those files are left out of `go test ./...`. This suite does not start a
 * server. It uses the boot converter's data server (WOW_DATA_SERVER_SOCKET /
 * WOW_DATA_SERVER_URL), then the bun-dev sockets, then localhost:17753.
 * Texture-bake API tests use WOW_CONVERTER_URL (default http://127.0.0.1:3001).
 *
 *   bun test --max-concurrency=1 tests/golang-integration-tests
 */
import { beforeAll, setDefaultTimeout, test } from 'bun:test';
import { spawnSync } from 'child_process';
import { readdirSync, readFileSync } from 'fs';
import path from 'path';

import { detectDataServer, waitForCascInfo } from '../api-tests/client';
import { converterUrl, repoRoot } from '../helpers';

const golangDir = path.join(repoRoot, 'golang');

setDefaultTimeout(60 * 60_000);

beforeAll(async () => {
  const endpoint = await detectDataServer();
  await waitForCascInfo(endpoint);
  if (endpoint.unix) {
    process.env.WOW_DATA_SERVER_SOCKET = endpoint.unix;
    process.env.WOW_DATA_TRANSPORT = 'socket';
    delete process.env.WOW_DATA_SERVER_URL;
    delete process.env.WOW_DATA_SERVER_PORT;
  } else {
    process.env.WOW_DATA_SERVER_URL = endpoint.base;
    delete process.env.WOW_DATA_SERVER_SOCKET;
    delete process.env.WOW_DATA_TRANSPORT;
  }
  if (!process.env.WOW_CONVERTER_URL?.trim()) {
    process.env.WOW_CONVERTER_URL = converterUrl();
  }
  console.log(`wow-data-server ${endpoint.label}; converter ${process.env.WOW_CONVERTER_URL}`);
}, 10 * 60_000);

test('integration-tagged go tests', () => {
  const { packages, pattern } = integrationTests(golangDir);
  const result = spawnSync('go', [
    'test', '-tags', 'integration_tests', '-count=1', '-run', pattern, ...packages,
  ], {
    cwd: golangDir,
    stdio: 'inherit',
    env: process.env,
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`go test exited ${result.status ?? 'unknown'}`);
});

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
