// Compares two artifact directories (MDX/BLP) by size+sha256 to validate export parity.
// Used to quickly spot byte-level mismatches between TS and Go export outputs.
import { createHash } from 'crypto';
import { readdir, readFile, stat } from 'fs/promises';
import path from 'path';

const leftRoot = process.argv[2];
const rightRoot = process.argv[3];

if (!leftRoot || !rightRoot) {
  console.error('Usage: bun scripts/compare-artifact-dirs.ts <leftDir> <rightDir>');
  process.exit(1);
}

const leftFiles = await collectArtifacts(leftRoot);
const rightFiles = await collectArtifacts(rightRoot);
const all = [...new Set([...leftFiles.keys(), ...rightFiles.keys()])].sort();
const failures: string[] = [];

for (const rel of all) {
  const left = leftFiles.get(rel);
  const right = rightFiles.get(rel);
  if (!left) {
    failures.push(`missing left: ${rel}`);
    continue;
  }
  if (!right) {
    failures.push(`missing right: ${rel}`);
    continue;
  }
  if (left.size !== right.size || left.sha256 !== right.sha256) {
    failures.push(`bytes differ: ${rel} left=${left.size}/${left.sha256.slice(0, 12)} right=${right.size}/${right.sha256.slice(0, 12)}`);
  }
}

console.log(`Compared ${all.length} artifacts (${leftFiles.size} left, ${rightFiles.size} right)`);
if (failures.length > 0) {
  console.error(`Artifact parity failed (${failures.length})`);
  for (const failure of failures.slice(0, 100)) console.error(failure);
  if (failures.length > 100) console.error(`... ${failures.length - 100} more`);
  process.exit(1);
}
console.log('Artifact parity passed');
process.exit(0);

async function collectArtifacts(root: string): Promise<Map<string, { size: number; sha256: string }>> {
  const out = new Map<string, { size: number; sha256: string }>();
  await walk(root, root, out);
  return out;
}

async function walk(root: string, dir: string, out: Map<string, { size: number; sha256: string }>) {
  let entries;
  try {
    entries = await readdir(dir, { withFileTypes: true });
  } catch {
    return;
  }
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      await walk(root, full, out);
      continue;
    }
    const ext = path.extname(entry.name).toLowerCase();
    if (ext !== '.mdx' && ext !== '.blp') continue;
    const data = await readFile(full);
    const info = await stat(full);
    out.set(path.relative(root, full).replaceAll('\\', '/'), {
      size: info.size,
      sha256: createHash('sha256').update(data).digest('hex'),
    });
  }
}
