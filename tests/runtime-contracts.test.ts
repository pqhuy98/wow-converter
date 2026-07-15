import {
  describe, expect, test,
} from 'bun:test';
import { existsSync, readFileSync } from 'fs';
import path from 'path';

const repoRoot = path.resolve(import.meta.dir, '..');

describe('runtime command contracts', () => {
  test('exposes only the essential developer workflows', () => {
    const packageJson = JSON.parse(readFileSync(path.join(repoRoot, 'package.json'), 'utf8')) as {
      scripts: Record<string, string>;
    };
    const { scripts } = packageJson;

    for (const name of [
      'lint', 'dev', 'dev:ts', 'build', 'build:linux', 'test', 'test:go',
      'test:ts', 'parity:map', 'parity:mdl', 'generate:acore-sqlite', 'clean',
      'smoke', 'smoke:hmr',
    ]) {
      expect(scripts[name]).toBeString();
    }
    expect(scripts.dev).toBe('bun scripts/dev.ts');
    expect(scripts.test).toBe('bun run test:go && bun run test:ts');
    expect(scripts['test:ts']).toBe('bun test tests');
    for (const removed of [
      'dev:split', 'dev:data-server', 'dev:converter', 'dev:app:converter',
      'dev:ts:run', 'dev:ts:server', 'dev:ts:data-server', 'dev:webui',
      'parity:servers', 'parity:map:compare', 'debug:mdl-parity',
      'parity:retail-mdl-loop', 'parity:random-mdl-loop', 'parity:mount-mdl-loop',
      'build:bundle', 'build:server', 'build:webui', 'build:ts',
      'start', 'start:ts', 'wow-data-server', 'wow-data-server:ts',
      'test:unit:ts', 'test:export:ts', 'test:go:integration', 'test:go:bench',
    ]) {
      expect(scripts[removed]).toBeUndefined();
    }
  });

  test('defines a bundled Go VPS deployment on golang-port', () => {
    const deploy = readFileSync(path.join(repoRoot, 'deploy/vps/scripts/deploy-app.sh'), 'utf8');
    const service = readFileSync(path.join(repoRoot, 'deploy/vps/systemd/wow-converter.service'), 'utf8');

    expect(deploy).toMatch(/WOW_DEPLOY_BRANCH:-golang-port/);
    expect(deploy).toMatch(/"\x24\{BUN\}" run build:linux/);
    expect(service).toContain('Description=wow-converter (Go bundled)');
    expect(service).toContain('Environment=WOW_CONVERTER_BUNDLED=1');
    expect(existsSync(path.join(repoRoot, 'deploy/vps/systemd/wow-data-server.service'))).toBe(false);
  });
});
