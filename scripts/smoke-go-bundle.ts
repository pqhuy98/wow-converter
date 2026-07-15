/** Verifies that the built dist-go bundle starts in bundled mode and serves API requests. */
import { spawn, spawnSync } from 'child_process';
import { existsSync } from 'fs';
import path from 'path';

const repoRoot = path.resolve(import.meta.dir, '..');
const executable = path.join(
  repoRoot,
  'dist-go',
  process.platform === 'win32' ? 'wow-converter.exe' : 'wow-converter',
);
if (!existsSync(executable)) {
  throw new Error(`Missing ${executable}; run bun run build first`);
}

const port = 18081;
spawnSync('bun', ['scripts/kill-dev-ports.ts', String(port)], {
  cwd: repoRoot,
  shell: process.platform === 'win32',
  stdio: 'inherit',
});
const app = spawn(executable, [], {
  cwd: path.dirname(executable),
  env: {
    ...process.env,
    CASC_LOCAL_WOW: '',
    CASC_REMOTE_REGION: '',
    NODE_ENV: 'production',
    PORT: String(port),
    WOW_CONVERTER_BUNDLED: '1',
  },
  stdio: 'inherit',
});

let ready = false;
try {
  const deadline = Date.now() + 2 * 60_000;
  while (Date.now() < deadline) {
    if (app.exitCode != null) throw new Error(`Go bundle exited with code ${app.exitCode}`);
    try {
      const response = await fetch(`http://127.0.0.1:${port}/api/get-config`);
      if (response.ok) {
        console.log('Go bundled binary API smoke: PASS');
        ready = true;
        break;
      }
    } catch {
      // Retry until the listener starts.
    }
    await Bun.sleep(500);
  }
  if (!ready) {
    throw new Error('Timed out waiting for Go bundled binary');
  }
} finally {
  if (app.pid != null && app.exitCode == null) {
    if (process.platform === 'win32') {
      spawnSync('taskkill', ['/PID', String(app.pid), '/T', '/F'], { stdio: 'ignore' });
    } else {
      app.kill('SIGTERM');
    }
  }
}
