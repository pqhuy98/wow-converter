/**
 * Remove build artifacts. On Windows, stops wow-converter.exe first so the
 * unix socket under dist/.cache is not locked (EACCES during shx rm -rf dist).
 */
import { spawnSync } from 'child_process';
import { existsSync } from 'fs';
import { mkdir, rm } from 'fs/promises';
import path from 'path';

const repoRoot = path.resolve(import.meta.dir, '..');

function stopBundledAppIfRunning(): void {
  if (process.platform !== 'win32') return;
  spawnSync('taskkill', ['/F', '/IM', 'wow-converter.exe', '/T'], { stdio: 'ignore' });
}

async function removePath(target: string): Promise<void> {
  if (!existsSync(target)) return;
  await rm(target, {
    recursive: true,
    force: true,
    maxRetries: 5,
    retryDelay: 200,
  });
}

async function main(): Promise<void> {
  stopBundledAppIfRunning();
  if (process.platform === 'win32') {
    await new Promise((resolve) => { setTimeout(resolve, 300); });
  }

  await removePath(path.join(repoRoot, 'build'));
  await removePath(path.join(repoRoot, 'dist'));
  await removePath(path.join(repoRoot, 'wow-converter.zip'));

  await mkdir(path.join(repoRoot, 'build'), { recursive: true });
  await mkdir(path.join(repoRoot, 'dist'), { recursive: true });
}

main().catch((e: Error) => {
  console.error(e.message);
  console.error('If dist/.cache/wow-data-server.sock is locked, close wow-converter.exe and retry.');
  process.exit(1);
});
