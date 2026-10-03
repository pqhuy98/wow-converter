/**
 * Copies models into maps/test-regression-{retail,mount,classic}.w3x.
 *
 *   bun test --max-concurrency=1 tests/regression-maps
 *   fresh=1 bun test --max-concurrency=1 tests/regression-maps
 *   suite=mount bun test --max-concurrency=1 tests/regression-maps
 *
 * suite missing runs retail, then mount, then classic.
 * Both modes read the converter's exported-assets (`{suite}/{slug}.mdx`). fresh=1 clears that folder, then exports.
 * A fresh run exports retail and mount on wow, then switches to wow_classic for classic and restores the original product.
 */
import { expect, test } from 'bun:test';
import { existsSync } from 'fs';
import { join } from 'path';

import {
  clearExportedAssets, converterUrl, ensureWowProduct, readWowProduct,
} from '../helpers';
import { readSnapshotCases, type SnapshotSuite } from '../snapshot-tests/model/catalog';
import {
  copyRegressionModels, ensureRegressionMap, isFresh, limitedCases, placeCharacters, placeMounts,
} from './util';

const allSuites: readonly SnapshotSuite[] = ['retail', 'mount', 'classic'];

function selectedSuites(): readonly SnapshotSuite[] {
  const raw = process.env.suite ?? '';
  if (raw === '') return allSuites;
  if (raw === 'retail' || raw === 'mount' || raw === 'classic') return [raw];
  throw new Error('suite is retail, mount, or classic');
}

test('regression maps', async () => {
  const suites = selectedSuites();
  const fresh = isFresh();
  const base = converterUrl();
  if (fresh) await clearExportedAssets(base);
  const original = fresh ? (await readWowProduct(base)).product : '';
  try {
    let exportedProduct = '';
    for (const suite of suites) {
      if (fresh) {
        const product = suite === 'classic' ? 'wow_classic' : 'wow';
        await ensureWowProduct(base, product);
        // Shared texture filenames can hold different bytes in each product.
        if (exportedProduct !== '' && exportedProduct !== product) await clearExportedAssets(base);
        exportedProduct = product;
      }
      const mapDir = ensureRegressionMap(suite);
      const placed = await copyRegressionModels(mapDir, suite, limitedCases(readSnapshotCases(suite)));
      if (suite === 'mount') placeMounts(mapDir, placed);
      else placeCharacters(mapDir, placed);
      expect(existsSync(join(mapDir, 'war3map.w3i'))).toBe(true);
      expect(existsSync(join(mapDir, 'war3mapUnits.doo'))).toBe(true);
      for (const npc of placed) {
        expect(existsSync(join(mapDir, `${npc.model}.mdx`))).toBe(true);
      }
    }
  } finally {
    if (original !== '') await ensureWowProduct(base, original);
  }
}, 2 * 60 * 60 * 1000);
