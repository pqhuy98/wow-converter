// Exports a slice of parity test cases using the TS pipeline into a stable artifact directory.
// Used by parity runners to generate TS baselines for diffing against the Go exporter.
import { readFile } from 'fs/promises';
import { emptyDir, ensureDir } from 'fs-extra';
import path from 'path';

import {
  AttachItem, CharacterExporter, local, Size, wowhead,
} from '@/lib/converter/character';
import { AttackTag } from '@/lib/converter/wow-model/animation/animation-mapper';
import { WoWAttachmentID } from '@/lib/converter/wow-model/animation/bones-mapper';
import { Config, getDefaultConfig } from '@/lib/global-config';
import { wowDataClient } from '@/lib/wow-data-client/wow-data-client';

import {
  loadRandomMdlCasesFile,
  type RandomMdlParityCase,
} from './mdl-parity-random-cases';

interface TestCase {
  base: string;
  weaponR: string;
  weaponL: string;
  size: string;
  skinId?: string;
  outputName?: string;
}

interface MountCase {
  riderBase: string;
  mountPath: string;
  mountScale: number;
  seatOffset: number[];
  size: string;
}

const args = new Map<string, string>();
for (let i = 2; i < process.argv.length; i++) {
  const [key, value = ''] = process.argv[i]!.split('=', 2);
  args.set(key, value);
}

const suite = args.get('--suite') === 'classic' ? 'classic' : 'retail';
const mountMode = args.has('--mount');
const casesFile = args.get('--cases-file') ?? '';
const outDir = args.get('--out') ?? `.parity-artifacts/ts/${suite}`;
const limit = Number(args.get('--limit') ?? process.env.TEST_LIMIT ?? 0);
const offset = Number(args.get('--offset') ?? 0);
const fresh = args.has('--fresh');
const format = args.get('--format') === 'mdl' ? 'mdl' : 'mdx';

await wowDataClient.waitUntilReady();

const randomCases = casesFile ? (await loadRandomMdlCasesFile(casesFile)).cases : null;
const allCases = randomCases
  ? randomCases.map(randomCaseToTestCase)
  : mountMode
    ? await readMountCasesFromGo()
    : await readCasesFromGo(suite);
const offsetCases = offset > 0 ? allCases.slice(offset) : allCases;
const cases = limit > 0 ? offsetCases.slice(0, limit) : offsetCases;

if (fresh) {
  await emptyDir(outDir);
} else {
  await ensureDir(outDir);
}

const ceConfig: Config = {
  ...(await getDefaultConfig()),
  overrideModels: true,
  maxTextureSize: 512,
};

for (let i = 0; i < cases.length; i++) {
  const testCase = cases[i]!;
  const name = testCase.outputName
    ?? (mountMode ? `mount-case-${offset + i}` : deriveName(testCase.base));
  console.log(`Exporting ${i + 1}/${cases.length}: ${name}`);

  const exporter = new CharacterExporter(ceConfig);
  const mountCase = mountMode ? (testCase as unknown as MountCase) : undefined;
  await exporter.exportCharacter({
    base: resolveCharacterBase(testCase.base),
    attachItems: buildAttachItems(testCase),
    attackTag: chooseAttackTag(testCase),
    inGameMovespeed: 270,
    size: testCase.size === '' ? undefined : testCase.size as Size,
    scale: 1.5,
    particlesDensity: 0.5,
    mount: mountCase ? {
      path: wowhead(mountCase.mountPath),
      scale: mountCase.mountScale || undefined,
      seatOffset: mountCase.seatOffset?.length === 3 ? mountCase.seatOffset as [number, number, number] : undefined,
    } : undefined,
  }, name, testCase.skinId ? { localModelSkinId: testCase.skinId } : undefined);
  exporter.optimizeModelsTextures();
  await exporter.writeAllTextures(outDir);
  await exporter.writeAllModels(outDir, format);
}

console.log(`TS parity export complete: ${cases.length} cases -> ${outDir}`);
process.exit(0);

async function readCasesFromGo(which: 'retail' | 'classic'): Promise<TestCase[]> {
  const source = await readFile('golang/internal/testcases/cases.go', 'utf8');
  const varName = which === 'retail' ? 'retailCases' : 'classicCases';
  const match = source.match(new RegExp(`var ${varName} = \\[\\]TestCase\\{([\\s\\S]*?)\\n\\}`));
  if (!match) throw new Error(`Could not find ${varName} in cases.go`);

  const out: TestCase[] = [];
  const rowPattern = /Base: "((?:\\.|[^"\\])*)", WeaponR: "((?:\\.|[^"\\])*)", WeaponL: "((?:\\.|[^"\\])*)", Size: "((?:\\.|[^"\\])*)"/g;
  for (const row of match[1]!.matchAll(rowPattern)) {
    out.push({
      base: unescapeGoString(row[1]!),
      weaponR: unescapeGoString(row[2]!),
      weaponL: unescapeGoString(row[3]!),
      size: unescapeGoString(row[4]!),
    });
  }
  return out;
}

async function readMountCasesFromGo(): Promise<Array<TestCase & { mountPath: string; mountScale: number; seatOffset: number[] }>> {
  const source = await readFile('golang/internal/testcases/mount_cases.go', 'utf8');
  const blocks = [...source.matchAll(/\{\s*RiderBase: "((?:\\.|[^"\\])*)",\s*MountPath: "((?:\\.|[^"\\])*)"(?:,\s*MountScale: ([0-9.]+))?(?:,\s*SeatOffset: \[\]float64\{([^}]*)\})?,\s*Size: "((?:\\.|[^"\\])*)"\s*\}/g)];
  return blocks.map((row) => ({
    base: unescapeGoString(row[1]!),
    weaponR: '',
    weaponL: '',
    size: unescapeGoString(row[5]!),
    mountPath: unescapeGoString(row[2]!),
    mountScale: row[3] ? Number(row[3]) : 0,
    seatOffset: row[4] ? row[4].split(',').map((v) => Number(v.trim())) : [],
  }));
}

function unescapeGoString(value: string): string {
  return (JSON.parse(`"${value}"`) as string).replaceAll('\\\\', '\\');
}

function buildAttachItems(testCase: TestCase): Record<string, AttachItem> {
  const attachItems: Record<string, AttachItem> = {};
  if (testCase.weaponR) {
    attachItems[WoWAttachmentID.HandRight] = { path: wowhead(testCase.weaponR), scale: 1 };
  }
  if (testCase.weaponL) {
    attachItems[WoWAttachmentID.HandLeft] = { path: wowhead(testCase.weaponL), scale: 1 };
  }
  return attachItems;
}

function deriveName(base: string): string {
  if (base.startsWith('local::')) {
    const name = path.win32.basename(base.replace('local::', '')).replace(/\.(m2|wmo|obj)$/i, '');
    return `local-${name}`;
  }
  if (base.includes('npc=')) {
    const id = base.split('npc=').pop()?.split('/').shift();
    const name = base.split('/').pop()!.split('#')[0];
    return `npc-${name}-${id}`;
  }
  if (base.includes('object=')) {
    const id = base.split('object=').pop()?.split('/').shift();
    const name = base.split('/').pop()!.split('#')[0];
    return `object-${name}-${id}`;
  }
  if (base.includes('item=')) {
    const id = base.split('item=').pop()?.split('/').shift();
    const name = base.split('/').pop()!.split('#')[0];
    return `item-${name}-${id}`;
  }
  if (base.includes('dressing-room')) {
    return `dressing-room-${base.split('?').at(-1)!.split('#')[0]}`;
  }
  return `export-${base.replaceAll('/', '_')}`;
}

function randomCaseToTestCase(testCase: RandomMdlParityCase): TestCase {
  return {
    base: testCase.localRef,
    weaponR: '',
    weaponL: '',
    size: '',
    skinId: testCase.skinId || undefined,
    outputName: testCase.outputName,
  };
}

function chooseAttackTag(testCase: TestCase): AttackTag {
  if (testCase.weaponR && !testCase.weaponL || !testCase.weaponR && testCase.weaponL) return '2H';
  if (testCase.weaponR && testCase.weaponL) return '1H';
  return 'Auto';
}

function resolveCharacterBase(base: string) {
  if (base.startsWith('local::')) return local(base.replace('local::', ''));
  if (base.startsWith('http') || base.includes('npc=') || base.includes('object=') || base.includes('item=') || base.includes('dressing-room')) {
    return wowhead(base);
  }
  return local(base);
}
