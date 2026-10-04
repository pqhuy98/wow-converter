/** Compatibility entry point; capture and labels are owned by the Go server. */
import { spawnSync } from 'child_process';
import { fileURLToPath } from 'url';

const result = spawnSync('go', ['run', './cmd/shot-wowhead', ...process.argv.slice(2)], {
  cwd: fileURLToPath(new URL('../../../golang/', import.meta.url)),
  stdio: 'inherit',
  windowsHide: true,
});
if (result.error) console.error(result.error.message);
process.exit(result.status ?? 1);
