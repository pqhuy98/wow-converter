// Starts the TS and Go wow-data-server on fixed parity ports, killing any existing listeners first.
// This keeps parity runs reproducible and avoids “stale server code” when iterating on Go/TS exporters.
import { spawn, spawnSync } from 'child_process';

const tsPort = Number(process.env.TS_WOW_DATA_SERVER_PORT ?? 17753);
const goPort = Number(process.env.GO_WOW_DATA_SERVER_PORT ?? 17754);

spawnSync('bun', ['scripts/kill-dev-ports.ts', String(tsPort), String(goPort)], {
  cwd: import.meta.dir + '/..',
  stdio: 'inherit',
  shell: process.platform === 'win32',
});

const children = [
  spawnServer('TS wow-data-server', 'bun', ['src/wow-data-server/index.ts'], '.', {
    WOW_DATA_SERVER_PORT: String(tsPort),
    WOW_LOG_PREFIX: 'ts',
  }),
  spawnServer('Go wow-data-server', 'go', ['run', './cmd/wow-data-server'], 'golang', {
    WOW_DATA_SERVER_PORT: String(goPort),
    WOW_LOG_PREFIX: 'go',
  }),
];

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
  process.on(signal, () => {
    for (const child of children) {
      child.kill(signal);
    }
    process.exit(0);
  });
}

function spawnServer(label: string, command: string, args: string[], cwd: string, env: Record<string, string>) {
  console.log(`Starting ${label} on port ${env.WOW_DATA_SERVER_PORT}`);
  const child = spawn(command, args, {
    cwd,
    env: { ...process.env, ...env },
    shell: process.platform === 'win32',
    stdio: 'inherit',
  });
  child.on('exit', (code, signal) => {
    if (code != null && code !== 0) {
      console.error(`${label} exited with code ${code}`);
      process.exitCode = code;
    } else if (signal) {
      console.error(`${label} exited with signal ${signal}`);
    }
  });
  return child;
}

