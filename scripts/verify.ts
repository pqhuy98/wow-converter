/**
 * Release checks: Go unit tests, Go integration tests, dist-go build, boot that binary, API, then
 * snapshots for retail, mount, and classic. Regression maps export each
 * product separately so shared texture paths contain the correct bytes.
 *
 * dist-go is the production bundle. CASC comes from POST /api/wow-config/apply
 * using the repo .env; that file is never copied into dist-go. A successful
 * run deletes runtime artifacts left under dist-go.
 *
 *   bun scripts/verify.ts
 */
import { spawnSync } from 'child_process';
import { readFileSync } from 'fs';
import { rm } from 'fs/promises';
import path from 'path';

import {
  killPorts,
  repoRoot,
  spawnManaged,
  stopChildren,
  waitForEndpoint,
} from '../tests/helpers';

type Status = 'PASS' | 'FAIL' | 'SKIP';

interface StepResult {
  readonly name: string;
  readonly status: Status;
  readonly seconds: number;
}

interface CascApplyBody {
  readonly mode: 'local' | 'remote';
  readonly product: string;
  readonly installDirectory?: string;
  readonly regionTag?: string;
}

const port = '18081';
const baseURL = `http://127.0.0.1:${port}`;
const distGo = path.join(repoRoot, 'dist-go');
const exeName = process.platform === 'win32' ? 'wow-converter.exe' : 'wow-converter';
const dataSocket = path.join(distGo, '.cache', 'wow-data-server.sock');
const distArtifacts = [
  '.env',
  '.cache',
  'exported-assets',
  'exported-assets-browse',
  'recent-exports.json',
];

main().then((code) => {
  process.exit(code);
}).catch((error: unknown) => {
  console.error(error);
  process.exit(1);
});

async function main(): Promise<number> {
  const results: StepResult[] = [];

  results.push(await runStep('unit', () => run('go', ['test', './...'], path.join(repoRoot, 'golang'))));
  results.push(await runStep('integration', () => run(
    'bun',
    ['test', '--max-concurrency=1', 'tests/golang-integration-tests'],
    repoRoot,
  )));

  const build = await runStep('build', () => run('bun', ['run', process.platform === 'win32' ? 'build' : 'build:linux'], repoRoot, {
    NODE_ENV: 'production',
  }));
  results.push(build);

  try {
    const boot = await runStep('boot', async () => {
      if (build.status !== 'PASS') return 'SKIP';
      killPorts([Number(port)]);
      const child = spawnManaged(exeName, path.join(distGo, exeName), [], distGo, {
        NODE_ENV: 'production',
        PORT: port,
        WOW_CONVERTER_BUNDLED: '1',
        WOW_DATA_SERVER_SOCKET: dataSocket,
        // Bun loads the repo .env. Only the explicit API apply should open CASC.
        CASC_LOCAL_WOW: '',
        CASC_REMOTE_REGION: '',
      }, Number(port));
      await waitForEndpoint(child, `${baseURL}/api/wow-config/status`, 10 * 60_000, serverListening);
      await applyCascFromEnv(baseURL);
      await waitForEndpoint(child, `${baseURL}/api/wow-config/status`, 10 * 60_000, cascReady);
      return 'PASS';
    });
    results.push(boot);

    const againstServer = { WOW_CONVERTER_URL: baseURL };
    results.push(await runStep('api', () => {
      if (boot.status !== 'PASS') return 'SKIP';
      return run('bun', ['test', 'tests/api-tests'], repoRoot, {
        ...againstServer,
        WOW_DATA_SERVER_SOCKET: dataSocket,
        WOW_DATA_SERVER_URL: '',
      });
    }));
    results.push(await runStep('snapshot', () => {
      if (boot.status !== 'PASS') return 'SKIP';
      return run('bun', ['test', '--max-concurrency=1', 'tests/snapshot-tests'], repoRoot, {
        ...againstServer,
        SNAPSHOT_SUITE: '',
        SNAPSHOT_SLUG: '',
        SNAPSHOT_UPDATE: '',
      });
    }));
    results.push(await runStep('regression', () => {
      if (boot.status !== 'PASS') return 'SKIP';
      return run('bun', ['test', '--max-concurrency=1', 'tests/regression-maps'], repoRoot, {
        ...againstServer,
        fresh: '1',
        suite: '',
        TEST_LIMIT: '',
      });
    }));
  } finally {
    stopChildren();
  }

  let failed = results.some((result) => result.status === 'FAIL');
  if (!failed) {
    try {
      await cleanDistArtifacts();
    } catch (error: unknown) {
      console.error(error);
      failed = true;
    }
  }

  for (const result of results) {
    console.log(`${result.status} ${result.name} (${result.seconds}s)`);
  }
  return failed ? 1 : 0;
}

async function runStep(name: string, fn: () => Promise<Status> | Status): Promise<StepResult> {
  const start = Date.now();
  try {
    return { name, status: await fn(), seconds: elapsedSeconds(start) };
  } catch (error: unknown) {
    console.error(error);
    return { name, status: 'FAIL', seconds: elapsedSeconds(start) };
  }
}

function run(
  command: string,
  args: readonly string[],
  cwd: string,
  env?: Readonly<Record<string, string>>,
): Status {
  const result = spawnSync(command, [...args], {
    cwd,
    stdio: 'inherit',
    shell: process.platform === 'win32',
    env: { ...process.env, ...env },
  });
  if (result.error) throw result.error;
  if (result.status !== 0) throw new Error(`${command} exited ${result.status ?? 'unknown'}`);
  return 'PASS';
}

function serverListening(value: unknown): boolean {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function cascReady(value: unknown): boolean {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) return false;
  if (!('cascLoaded' in value) || !('cascLoading' in value) || !('wowDataServerReachable' in value)) return false;
  return value.cascLoaded === true && value.cascLoading === false && value.wowDataServerReachable === true;
}

async function applyCascFromEnv(base: string): Promise<void> {
  const body = readCascEnv(path.join(repoRoot, '.env'));
  const response = await fetch(`${base}/api/wow-config/apply`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify(body),
    signal: AbortSignal.timeout(10 * 60_000),
  });
  if (!response.ok) {
    throw new Error(`wow-config apply failed (${response.status}): ${await response.text()}`);
  }
}

function readCascEnv(file: string): CascApplyBody {
  const values = parseEnvFile(readFileSync(file, 'utf8'));
  const localDir = cascValue(values.CASC_LOCAL_WOW);
  if (localDir !== '') {
    return {
      mode: 'local',
      product: cascValue(values.CASC_LOCAL_PRODUCT) || 'wow',
      installDirectory: localDir,
    };
  }
  const region = cascValue(values.CASC_REMOTE_REGION);
  if (region !== '') {
    return {
      mode: 'remote',
      product: cascValue(values.CASC_REMOTE_PRODUCT) || 'wow',
      regionTag: region,
    };
  }
  throw new Error('repo .env has no CASC_LOCAL_WOW or CASC_REMOTE_REGION');
}

function parseEnvFile(text: string): Record<string, string> {
  const values: Record<string, string> = {};
  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (line === '' || line.startsWith('#')) continue;
    const eq = line.indexOf('=');
    if (eq <= 0) continue;
    values[line.slice(0, eq).trim()] = line.slice(eq + 1).trim();
  }
  return values;
}

function cascValue(value: string | undefined): string {
  if (value == null) return '';
  return value.trim().replace(/^"|"$/g, '').replaceAll('\\\\', '\\');
}

async function cleanDistArtifacts(): Promise<void> {
  for (const name of distArtifacts) {
    await rm(path.join(distGo, name), { recursive: true, force: true });
  }
}

function elapsedSeconds(start: number): number {
  return Math.round((Date.now() - start) / 1000);
}
