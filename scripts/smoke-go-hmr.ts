/**
 * Starts the default Go dev stack and verifies Next.js HMR upgrades through
 * the Go server at ws://127.0.0.1:3001/_next/webpack-hmr.
 */
import { spawn, spawnSync } from 'child_process';

const repoRoot = `${import.meta.dir}/..`;
const dev = spawn('bun', ['run', 'dev'], {
  cwd: repoRoot,
  env: process.env,
  shell: process.platform === 'win32',
  stdio: 'inherit',
});

try {
  await waitForHTTP('http://127.0.0.1:3001/api/get-config', 10 * 60_000);
  await waitForWebSocket('ws://127.0.0.1:3001/_next/webpack-hmr', 30_000);
  console.log('Go reverse proxy HMR WebSocket: PASS');
} finally {
  if (dev.pid != null && dev.exitCode == null) {
    if (process.platform === 'win32') {
      spawnSync('taskkill', ['/PID', String(dev.pid), '/T', '/F'], { stdio: 'ignore' });
    } else {
      dev.kill('SIGTERM');
    }
  }
}

async function waitForHTTP(url: string, timeoutMs: number): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    if (dev.exitCode != null) throw new Error(`Dev stack exited with code ${dev.exitCode}`);
    try {
      const response = await fetch(url);
      if (response.ok) return;
    } catch {
      // Retry while Air and the bundled server start.
    }
    await Bun.sleep(1_000);
  }
  throw new Error(`Timed out waiting for ${url}`);
}

function waitForWebSocket(url: string, timeoutMs: number): Promise<void> {
  return new Promise((resolve, reject) => {
    const socket = new WebSocket(url);
    const timeout = setTimeout(() => {
      socket.close();
      reject(new Error(`Timed out waiting for ${url}`));
    }, timeoutMs);
    socket.addEventListener('open', () => {
      clearTimeout(timeout);
      socket.close();
      resolve();
    }, { once: true });
    socket.addEventListener('error', () => {
      clearTimeout(timeout);
      reject(new Error(`WebSocket upgrade failed for ${url}`));
    }, { once: true });
  });
}
