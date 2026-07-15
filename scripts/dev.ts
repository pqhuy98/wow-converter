/** Starts the selected converter development stack and stops all children on exit. */
import { type ChildProcess, spawn, spawnSync } from 'child_process';
import path from 'path';

const repoRoot = path.resolve(import.meta.dir, '..');
const legacyTS = process.argv.includes('--ts');
const children: ChildProcess[] = [];

const kill = spawnSync('bun', ['scripts/kill-dev-ports.ts'], {
  cwd: repoRoot,
  shell: process.platform === 'win32',
  stdio: 'inherit',
});
if (kill.status !== 0) process.exit(kill.status ?? 1);

if (legacyTS) {
  start('TS converter', 'bun', ['--watch', 'src/server/index.ts'], repoRoot, {
    NODE_ENV: 'development',
  });
  start('TS wow-data-server', 'bun', ['--watch', 'src/wow-data-server/index.ts'], repoRoot);
} else {
  start('Go bundled converter', 'go', ['tool', 'air'], path.join(repoRoot, 'golang'), {
    NODE_ENV: 'development',
    WOW_CONVERTER_BUNDLED: '1',
  });
}
start('Web UI', 'bun', ['run', 'dev'], path.join(repoRoot, 'webui'));

for (const signal of ['SIGINT', 'SIGTERM'] as const) {
  process.on(signal, () => {
    stopChildren();
    process.exit(0);
  });
}

function start(
  label: string,
  command: string,
  args: readonly string[],
  cwd: string,
  env: Readonly<Record<string, string>> = {},
): void {
  console.log(`Starting ${label}`);
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
    } else if (signal != null) {
      console.error(`${label} exited with signal ${signal}`);
    }
  });
  children.push(child);
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
