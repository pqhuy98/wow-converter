import path from 'path';
import { WorkerOptions } from 'worker_threads';

function detectEnvironment(): 'tsx' | 'pkg' | 'node' {
  if (process.execArgv.some((a) => a.includes('tsx'))
    || process.argv.some((a) => a.includes('tsx'))
    || process.env.npm_lifecycle_event === 'dev') {
    return 'tsx';
  }

  if ((process as unknown as { pkg?: unknown }).pkg) {
    return 'pkg';
  }

  return 'node';
}

// One switch to pick the right worker entry + safe execArgv per environment
export function resolveWorkerEntry(callerMetaUrl: string, workerPath: string): {
  entry: string | URL;
  options: WorkerOptions;
  mode: 'tsx' | 'pkg' | 'node'
} {
  const environment = detectEnvironment();
  const workerPathNoExt = workerPath.replace(/\.(ts|js|cjs|mjs)$/i, '');
  const workerPathTsx = `${workerPathNoExt}.ts`;
  const workerPathCjs = `${workerPathNoExt}.cjs`;

  if (environment === 'tsx') {
    // tsx dev: run TS worker with tsx loader; preserve only loader flags
    // Must have .ts because Worker use node with type stripping
    return {
      entry: new URL(workerPathTsx, callerMetaUrl),
      options: { execArgv: ['--import', 'tsx'] },
      mode: 'tsx',
    };
  }

  if (environment === 'pkg') {
    // pkg: worker must be a real file next to the exe; clear execArgv (pkg V8 flags will break Workers)
    const exeDir = path.dirname(process.execPath);
    const fileName = path.basename(workerPathCjs);
    const entry = path.join(exeDir, fileName);
    return {
      entry,
      options: { execArgv: [] },
      mode: 'pkg',
    };
  }

  // node server.bundle.cjs (rollup) or plain node build: point to emitted JS; clear execArgv
  return {
    entry: new URL(workerPathCjs, callerMetaUrl),
    options: { execArgv: [] },
    mode: 'node',
  };
}
