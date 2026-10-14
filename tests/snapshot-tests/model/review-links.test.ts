import { expect, test } from 'bun:test';
import {
  existsSync, readFileSync, unlinkSync, writeFileSync,
} from 'fs';

import { readSnapshotCases, snapshotExportPath, type SnapshotSuite } from './catalog';
import { writeReview } from './write-review';

test('review links retain product paths and mounted filenames', () => {
  expect(snapshotExportPath('retail', 'nythendra')).toBe('retails/nythendra');
  expect(snapshotExportPath('classic', 'mage')).toBe('classic/mage');
  expect(snapshotExportPath('mount', 'lich-king')).toBe('retails/mount/lich-king');
  const root = import.meta.dir;
  const files = [`${root}/review.html`, `${root}/review-failures.json`];
  const saved = files.map((file) => (existsSync(file) ? readFileSync(file) : undefined));
  const temporaryShots: string[] = [];
  const suites: SnapshotSuite[] = ['retail', 'classic', 'mount'];
  try {
    for (const suite of suites) {
      const fixture = readSnapshotCases(suite).find((c) => existsSync(`${root}/${suite}/${c.slug}/${c.slug}.expected.png`));
      if (!fixture) throw new Error(`missing ${suite} fixture`);
      const stem = `${root}/${suite}/${fixture.slug}/${fixture.slug}`;
      if (!existsSync(`${stem}.actual.png`)) {
        writeFileSync(`${stem}.actual.png`, readFileSync(`${stem}.expected.png`));
        temporaryShots.push(`${stem}.actual.png`);
      }
    }
    writeReview([]);
    const generated = readFileSync(`${root}/review.html`, 'utf8');
    const script = generated.split('<script>')[1].split('</script>')[0];
    // Parse the emitted browser code without executing DOM operations.
    new Bun.Transpiler({ loader: 'js' }).transformSync(script);
    const match = generated.match(/const CASES = (\[[\s\S]*?\]);/);
    if (!match) throw new Error('missing review cases');
    const cases: {suite: string;slug: string;converterUrl: string;wowViewerUrl?: string}[] = JSON.parse(match[1]);
    const retail = cases.find((c) => c.suite === 'retail');
    const classic = cases.find((c) => c.suite === 'classic');
    const mount = cases.find((c) => c.suite === 'mount');
    if (!retail || !classic || !mount) throw new Error('missing catalog suite');
    expect(new URL(retail.converterUrl).searchParams.get('model')).toBe(`retails/${retail.slug}.mdx`);
    expect(new URL(classic.converterUrl).searchParams.get('model')).toBe(`classic/${classic.slug}.mdx`);
    expect(new URL(mount.converterUrl).searchParams.get('model')).toBe(`retails/mount/${mount.slug}_mount.mdx`);
    expect(new URL(classic.wowViewerUrl ?? '').searchParams.get('product')).toBe('wow_classic');
    expect(new URL(retail.wowViewerUrl ?? '').searchParams.get('product')).toBe('wow');
  } finally {
    temporaryShots.forEach((file) => unlinkSync(file));
    files.forEach((file, index) => {
      const data = saved[index];
      if (data) writeFileSync(file, data);
      else if (existsSync(file)) unlinkSync(file);
    });
  }
});
