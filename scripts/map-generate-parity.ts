/**
 * Runs the full Valiance Keep map parity test with fresh TS and Go servers.
 *
 * Usage:
 *   bun run parity:map
 *
 * Requires CASC_LOCAL_WOW in .env. The default case matches examples/convert.ts:
 * Northrend tiles 21,27 through 22,28 at quality 4096, including creatures.
 */
import { type ChildProcess, spawn, spawnSync } from 'child_process';
import path from 'path';

const repoRoot = path.resolve(import.meta.dir, '..');
const tsDataPort = 17753;
const goDataPort = 17754;
const tsConverterPort = 3001;
const goConverterPort = 3102;
const tsExportDir = path.join(repoRoot, '.parity-artifacts/map/ts-wow-export');
const goExportDir = path.join(repoRoot, '.parity-artifacts/map/go-wow-export');
const children: ChildProcess[] = [];

const baseEnv: NodeJS.ProcessEnv = { ...process.env };
delete baseEnv.WOW_CONVERTER_BUNDLED;
delete baseEnv.WOW_CONVERTER_BUNDLE;
baseEnv.IS_SHARED_HOSTING = 'false';
baseEnv.NODE_ENV = 'production';

try {
  killParityPorts();

  const tsData = spawnManaged(
    'TS wow-data-server',
    'bun',
    ['src/wow-data-server/index.ts'],
    repoRoot,
    {
      WOW_DATA_SERVER_PORT: String(tsDataPort),
      WOW_EXPORT_DIR: tsExportDir,
      WOW_LOG_PREFIX: 'map-ts-data',
    },
  );
  const goData = spawnManaged(
    'Go wow-data-server',
    'go',
    ['run', './cmd/wow-data-server'],
    path.join(repoRoot, 'golang'),
    {
      WOW_DATA_SERVER_PORT: String(goDataPort),
      WOW_EXPORT_DIR: goExportDir,
      WOW_LOG_PREFIX: 'map-go-data',
    },
  );

  await Promise.all([
    waitForCasc(tsData, `http://127.0.0.1:${tsDataPort}`),
    waitForCasc(goData, `http://127.0.0.1:${goDataPort}`),
  ]);

  const tsConverter = spawnManaged(
    'TS converter',
    'bun',
    ['src/server/index.ts'],
    repoRoot,
    {
      PORT: String(tsConverterPort),
      WOW_DATA_SERVER_URL: `http://127.0.0.1:${tsDataPort}`,
      WOW_DATA_SERVER_PORT: String(tsDataPort),
      WOW_EXPORT_DIR: tsExportDir,
    },
  );
  const goConverter = spawnManaged(
    'Go converter',
    'go',
    ['run', './cmd/wow-converter'],
    path.join(repoRoot, 'golang'),
    {
      PORT: String(goConverterPort),
      WOW_DATA_SERVER_URL: `http://127.0.0.1:${goDataPort}`,
      WOW_DATA_SERVER_PORT: String(goDataPort),
      WOW_EXPORT_DIR: goExportDir,
    },
  );

  await Promise.all([
    waitForConverter(tsConverter, `http://127.0.0.1:${tsConverterPort}`),
    waitForConverter(goConverter, `http://127.0.0.1:${goConverterPort}`),
  ]);

  const test = spawnSync(
    'go',
    [
      'test',
      '-tags=integration',
      '-v',
      '-count=1',
      '-timeout=4h',
      '-run=^TestMapGenerateParityValianceKeep$',
      './test/integration/',
    ],
    {
      cwd: path.join(repoRoot, 'golang'),
      env: {
        ...baseEnv,
        WOW_TS_CONVERTER_URL: `http://127.0.0.1:${tsConverterPort}`,
        WOW_GO_CONVERTER_URL: `http://127.0.0.1:${goConverterPort}`,
      },
      shell: process.platform === 'win32',
      stdio: 'inherit',
    },
  );
  if (test.error) throw test.error;
  if (test.status !== 0) {
    throw new Error(`Valiance Keep map parity failed with exit code ${test.status ?? 'unknown'}`);
  }
} finally {
  stopChildren();
}

function killParityPorts(): void {
  const result = spawnSync(
    'bun',
    [
      'scripts/kill-dev-ports.ts',
      String(tsDataPort),
      String(goDataPort),
      String(tsConverterPort),
      String(goConverterPort),
    ],
    {
      cwd: repoRoot,
      shell: process.platform === 'win32',
      stdio: 'inherit',
    },
  );
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`Failed to release parity ports: exit code ${result.status ?? 'unknown'}`);
  }
}

function spawnManaged(
  label: string,
  command: string,
  args: readonly string[],
  cwd: string,
  env: Readonly<Record<string, string>>,
): ChildProcess {
  console.log(`Starting ${label}`);
  const child = spawn(command, [...args], {
    cwd,
    env: { ...baseEnv, ...env },
    shell: process.platform === 'win32',
    stdio: 'inherit',
  });
  child.on('exit', (code, signal) => {
    if (code != null && code !== 0) {
      console.error(`${label} exited with code ${code}`);
    } else if (signal != null) {
      console.error(`${label} exited with signal ${signal}`);
    }
  });
  children.push(child);
  return child;
}

async function waitForCasc(child: ChildProcess, baseURL: string): Promise<void> {
  await waitForEndpoint(child, `${baseURL}/rest/getCascInfo`, 15 * 60_000, (value) => (
    isRecord(value) && value.id === 'CASC_INFO'
  ));
  console.log(`CASC ready at ${baseURL}`);
}

async function waitForConverter(child: ChildProcess, baseURL: string): Promise<void> {
  await waitForEndpoint(child, `${baseURL}/api/get-config`, 2 * 60_000, () => true);
  console.log(`Converter ready at ${baseURL}`);
}

async function waitForEndpoint(
  child: ChildProcess,
  url: string,
  timeoutMs: number,
  accept: (value: unknown) => boolean,
): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (child.exitCode != null) {
      throw new Error(`Process exited before ${url} became ready`);
    }
    try {
      const response = await fetch(url);
      if (response.ok && accept(await response.json())) return;
    } catch {
      // Startup commonly rejects connections until the listener is ready.
    }
    await Bun.sleep(2_000);
  }
  throw new Error(`Timed out waiting for ${url}`);
}

function stopChildren(): void {
  for (const child of children.reverse()) {
    if (child.pid == null || child.exitCode != null) continue;
    if (process.platform === 'win32') {
      spawnSync('taskkill', ['/PID', String(child.pid), '/T', '/F'], { stdio: 'ignore' });
    } else {
      child.kill('SIGTERM');
    }
  }
}

function isRecord(value: unknown): value is Readonly<Record<string, unknown>> {
  return typeof value === 'object' && value != null;
}
