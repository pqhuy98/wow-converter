import { expect, test } from 'bun:test';
import { spawnSync } from 'child_process';
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'fs';
import { tmpdir } from 'os';
import path from 'path';
import { fileURLToPath } from 'url';

import { ShotBrowser } from '../../../.cursor/skills/shot-export-wow-converter/shot-export';

// Installed-browser check; no converter or model exports needed.
test.skipIf(process.env.REPORT_SHOT_BROWSER_TEST !== '1')('snapshot browser cleans its own temp files on close, error and process exit', async () => {
  const parent = mkdtempSync(path.join(tmpdir(), 'shot-browser-test-'));
  const previous = { TEMP: process.env.TEMP, TMP: process.env.TMP, TMPDIR: process.env.TMPDIR };
  try {
    Object.assign(process.env, { TEMP: parent, TMP: parent, TMPDIR: parent });
    const sentinel = path.join(parent, 'unrelated.txt');
    writeFileSync(sentinel, 'keep');
    for (const mode of ['success', 'capture-error']) {
      const browser = await ShotBrowser.open(640, 400);
      let bitsJob = '';
      try {
        const roots = readdirSync(parent).filter((name) => name.startsWith('wow-shot-'));
        expect(roots.length).toBe(1);
        expect(readdirSync(path.join(parent, roots[0]))).toContain('profile');
        mkdirSync(path.join(parent, roots[0], 'msedge_url_fetcher_test'));
        if (process.platform === 'win32') {
          // A real service-owned job must be cancelled even if its directory is removable already.
          const created = spawnSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', "$ErrorActionPreference='Stop'; Import-Module BitsTransfer; (Start-BitsTransfer -Source 'http://127.0.0.1:1/component.crx' -Destination $env:SHOT_TEST_DEST -DisplayName 'wow-converter snapshot cleanup regression' -Asynchronous -Suspended).JobId.ToString()"], {
            env: { ...process.env, SHOT_TEST_DEST: path.join(parent, roots[0], 'component.crx') },
            encoding: 'utf8', windowsHide: true, timeout: 15_000,
          });
          if (created.error || created.status !== 0) throw new Error(`BITS test job failed: ${created.error?.message ?? created.stderr}`);
          bitsJob = created.stdout.trim();
        }
        if (mode === 'capture-error') {
          const server = Bun.serve({ port: 0, hostname: '127.0.0.1', fetch: () => new Response('<html data-viewer-error="capture failed"></html>', { headers: { 'Content-Type': 'text/html' } }) });
          try {
            let failure = '';
            try {
              await browser.capture({
                base: server.url.toString(), model: 'missing.mdx', seq: 'Stand',
                width: 640, height: 400, views: ['front'], readyMs: 1000,
              });
            } catch (err: unknown) {
              failure = err instanceof Error ? err.message : String(err);
            }
            expect(failure).toBe('capture failed');
          } finally {
            await server.stop(true);
          }
        }
      } finally {
        try {
          await browser.close();
          if (bitsJob) {
            const remaining = spawnSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', "$ErrorActionPreference='Stop'; Import-Module BitsTransfer; @(Get-BitsTransfer | Where-Object { $_.JobId.ToString() -eq $env:SHOT_TEST_ID }).Count"], {
              env: { ...process.env, SHOT_TEST_ID: bitsJob }, encoding: 'utf8', windowsHide: true, timeout: 15_000,
            });
            expect(remaining.status).toBe(0);
            expect(remaining.stdout.trim()).toBe('0');
          }
        } finally {
          if (bitsJob) {
            // Test teardown also scopes cancellation to the one new job created above.
            spawnSync('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', 'Import-Module BitsTransfer; Get-BitsTransfer | Where-Object { $_.JobId.ToString() -eq $env:SHOT_TEST_ID } | Remove-BitsTransfer -Confirm:$false'], {
              env: { ...process.env, SHOT_TEST_ID: bitsJob }, windowsHide: true, timeout: 15_000,
            });
          }
        }
      }
      await browser.close(); // Closing twice must not target another browser or temp directory.
      expect(readdirSync(parent)).toEqual(['unrelated.txt']);
    }
    const modulePath = fileURLToPath(new URL('../../../.cursor/skills/shot-export-wow-converter/shot-export.ts', import.meta.url));
    // Emit the Node signal event: Windows process.kill uses forced termination instead.
    for (const [finish, code] of [['process.exit(0)', 0], ["process.emit('SIGINT')", 130], ["process.emit('SIGTERM')", 143]]) {
      const child = spawnSync(process.execPath, ['--eval', `import { ShotBrowser } from ${JSON.stringify(modulePath)}; await ShotBrowser.open(640, 400); ${finish};`], {
        env: process.env, encoding: 'utf8', windowsHide: true, timeout: 30_000,
      });
      if (child.error || child.status !== code) throw new Error(`exit cleanup failed: ${child.error?.message ?? child.stderr}`);
      expect(child.stderr).toBe('');
      expect(readdirSync(parent)).toEqual(['unrelated.txt']);
    }
    expect(readFileSync(sentinel, 'utf8')).toBe('keep');
  } finally {
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
    rmSync(parent, { recursive: true, force: true, maxRetries: 20, retryDelay: 100 });
  }
}, 60_000);
