/**
 * Smoke test for the bundled wow-converter.exe: waits for CASC + HTTP, exports Lich King.
 *
 * Usage:
 *   bun scripts/smoke-test-bundle.ts [distDir] [--in-place]
 */
import { existsSync, readdirSync } from 'fs';
import { cp, mkdir, rm } from 'fs/promises';
import path from 'path';

const repoRoot = path.resolve(import.meta.dir, '..');
const args = process.argv.slice(2).filter((a) => !a.startsWith('-'));
const distDir = path.resolve(args[0] ?? path.join(repoRoot, 'dist'));
const useInPlace = process.argv.includes('--in-place');
const tmpDir = path.join(repoRoot, `.tmp-bundle-test-${Date.now()}`);
const runDir = useInPlace ? distDir : tmpDir;
const exePath = path.join(runDir, 'wow-converter.exe');
const port = Number(process.env.PORT || 3099);
const timeoutMs = Number(process.env.SMOKE_TIMEOUT_MS || 600_000);

const lichKingExport = {
  character: {
    base: { type: 'wowhead', value: 'https://www.wowhead.com/wotlk/npc=36597/the-lich-king' },
    size: 'hero',
    attackTag: '2H',
    inGameMovespeed: 270,
    attachItems: {
      1: {
        path: { type: 'wowhead', value: 'https://www.wowhead.com/classic/item=231885/frostmourne' },
      },
    },
    portraitCameraSequenceName: 'Stand',
  },
  optimization: {
    sortSequences: true,
    allMaterialsUnshaded: false,
    removeUnusedVertices: true,
    removeUnusedNodes: true,
    removeUnusedMaterialsTextures: true,
  },
  outputFileName: 'smoke-lichking',
  format: 'mdx',
};

async function copyDistToTmp(): Promise<void> {
  await mkdir(tmpDir, { recursive: true });
  console.log('Copying dist to', tmpDir);
  if (process.platform === 'win32') {
    const { spawnSync } = await import('child_process');
    const r = spawnSync('robocopy', [distDir, tmpDir, '/E', '/XD', '.cache', '/NFL', '/NDL', '/NJH', '/NJS'], { stdio: 'inherit' });
    if (r.status !== undefined && r.status >= 8) {
      throw new Error(`robocopy failed with code ${r.status}`);
    }
    return;
  }
  for (const name of readdirSync(distDir)) {
    await cp(path.join(distDir, name), path.join(tmpDir, name), { recursive: true });
  }
}

async function waitForHttp(url: string, label: string): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(url);
      if (res.ok || res.status === 404) {
        console.log(`${label} ready`);
        return;
      }
    } catch {
      // retry
    }
    await new Promise((r) => { setTimeout(r, 2000); });
  }
  throw new Error(`${label} not ready within ${timeoutMs}ms`);
}

async function waitForExport(jobId: string): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const res = await fetch(`http://127.0.0.1:${port}/api/export/character/status/${jobId}`);
    const json = await res.json() as { status?: string; error?: string; result?: { exportedModels?: { path: string }[] } };
    if (json.status === 'done' || json.status === 'completed') {
      const models = json.result?.exportedModels ?? [];
      if (models.length === 0) throw new Error('Export completed but no models returned');
      console.log('Export completed:', models.map((m) => m.path).join(', '));
      return;
    }
    if (json.status === 'failed') {
      throw new Error(json.error ?? 'Export failed');
    }
    await new Promise((r) => { setTimeout(r, 3000); });
  }
  throw new Error(`Export job ${jobId} timed out`);
}

async function main(): Promise<void> {
  if (!existsSync(path.join(distDir, 'wow-converter.exe'))) {
    throw new Error(`Missing ${path.join(distDir, 'wow-converter.exe')} — run bun run build:ts first`);
  }

  if (!useInPlace) {
    await copyDistToTmp();
  } else {
    console.log('Running in-place from', distDir);
  }

  if (!existsSync(exePath)) {
    throw new Error(`Missing ${exePath}`);
  }

  const envPath = path.join(repoRoot, '.env');
  if (existsSync(envPath)) {
    await cp(envPath, path.join(runDir, '.env'));
    console.log('Copied .env into test directory');
  } else {
    console.warn('No .env found — CASC must be configured in test dir for export to succeed');
  }

  console.log('Starting wow-converter.exe on port', port, 'from', runDir);
  const proc = Bun.spawn({
    cmd: [exePath],
    cwd: runDir,
    env: {
      ...process.env,
      PORT: String(port),
      NODE_ENV: 'production',
    },
    stdout: 'pipe',
    stderr: 'pipe',
  });

  const pipe = async (stream: ReadableStream<Uint8Array>, prefix: string) => {
    const reader = stream.getReader();
    const decoder = new TextDecoder();
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      const text = decoder.decode(value);
      text.split(/\r?\n/).filter(Boolean).forEach((line) => {
        console.log(`${prefix}${line}`);
      });
    }
  };
  void pipe(proc.stdout, '[exe] ');
  void pipe(proc.stderr, '[exe!] ');

  try {
    await waitForHttp(`http://127.0.0.1:${port}/`, 'UI');

    const postRes = await fetch(`http://127.0.0.1:${port}/api/export/character`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(lichKingExport),
    });
    const postJson = await postRes.json() as { id?: string; error?: string };
    if (!postRes.ok || !postJson.id) {
      throw new Error(postJson.error ?? `Export POST failed (${postRes.status})`);
    }
    console.log('Export job queued:', postJson.id);
    await waitForExport(postJson.id);
    console.log('SMOKE TEST PASSED');
  } finally {
    proc.kill();
    await proc.exited;
    if (!useInPlace) {
      await rm(tmpDir, { recursive: true, force: true }).catch(() => {});
    }
  }
}

main().catch((e) => {
  console.error('SMOKE TEST FAILED:', e);
  process.exit(1);
});
