/**
 * Kill stale dev servers listening on the standard local ports.
 * Used before `dev` / `dev:go` / `dev:goapp` so restarts do not fail with EADDRINUSE.
 */
import { spawnSync } from 'child_process';

const defaultPorts = [
  3000, // Next.js webui
  Number(process.env.PORT || 3001), // wow-converter API
  Number(process.env.WOW_DATA_SERVER_PORT || 17753), // wow-data-server
];

const ports = process.argv.length > 2
  ? process.argv.slice(2).map((value) => Number(value)).filter((port) => Number.isFinite(port) && port > 0)
  : defaultPorts;

for (const port of [...new Set(ports)]) {
  killPort(port);
}

function killPort(port: number): void {
  const pids = process.platform === 'win32' ? windowsPortPids(port) : unixPortPids(port);
  for (const pid of pids) {
    if (pid === process.pid) continue;
    console.log(`Killing process ${pid} on port ${port}`);
    const result = process.platform === 'win32'
      ? spawnSync('taskkill', ['/PID', String(pid), '/F'], { stdio: 'inherit' })
      : spawnSync('kill', ['-TERM', String(pid)], { stdio: 'inherit' });
    if (result.status != null && result.status !== 0) {
      process.exit(result.status);
    }
  }
}

function windowsPortPids(port: number): number[] {
  const result = spawnSync('netstat', ['-ano'], { encoding: 'utf8' });
  if (result.status !== 0) return [];
  const pids = new Set<number>();
  for (const line of result.stdout.split(/\r?\n/)) {
    const parts = line.trim().split(/\s+/);
    if (parts.length < 5) continue;
    const localAddress = parts[1]!;
    const state = parts[3]!;
    const pid = Number(parts[4]);
    if (!Number.isFinite(pid) || state !== 'LISTENING') continue;
    if (localAddress.endsWith(`:${port}`)) {
      pids.add(pid);
    }
  }
  return [...pids];
}

function unixPortPids(port: number): number[] {
  const result = spawnSync('sh', ['-lc', `lsof -ti tcp:${port} -sTCP:LISTEN`], { encoding: 'utf8' });
  if (result.status !== 0 && result.status !== 1) return [];
  return result.stdout
    .split(/\s+/)
    .map((value) => Number(value))
    .filter((pid) => Number.isFinite(pid));
}
