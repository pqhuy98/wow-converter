// Runs the retail MDL parity suite case-by-case (TS vs Go) and prints only failing diffs.
// This is used to iterate exporter parity regressions without flooding logs on passing cases.
import { parsers } from '@pqhuy98/mdx-m3-viewer';
import { spawnSync } from 'child_process';
import { readFileSync } from 'fs';
import path from 'path';

import {
  ensureRandomMdlCasesFile,
  type RandomMdlParityCase,
} from './mdl-parity-random-cases';

interface TestCase {
  base: string;
  weaponR: string;
  weaponL: string;
  size: string;
}

interface ParityRunCase {
  modelName: string;
  label: string;
}

const HELP_TEXT = `
Usage:
  bun scripts/retail-mdl-parity-loop.ts [flags]

Modes:
  - Default (loop): exports cases one-by-one and prints ONLY failures + diff.
  - Debug (--debug): exports ONE case and prints a structured MDL summary + diff stat + diff head.

Flags:
  --suite=retail|classic
    Test suite to run (default: retail). In classic mode, Go exporter uses -classic.

  --ts-url=<url>
    TS wow-data-server base URL (default: env TS_WOW_DATA_SERVER_URL or http://127.0.0.1:17753)

  --go-url=<url>
    Go wow-data-server base URL (default: env GO_WOW_DATA_SERVER_URL or http://127.0.0.1:17754)

  --case-idx=<n>
    Run a single test case index (0-based). Same as setting env TEST_CASE_IDX/CASE_IDX.
    When set, default --limit becomes 1.

  --start=<n>
    Start index (0-based) when running a range (default: env TEST_START or 0).

  --limit=<n>
    Number of cases to run (default: env TEST_LIMIT; 0 means “no limit”).

  --continue
    Do not stop on first failure; keep going and exit non-zero if any failures occurred.
    (env CONTINUE_ON_FAIL=1 also enables this)

  --diff-lines=<n>
    Number of diff lines to print (default: env DIFF_LINES; debug defaults to 140, loop defaults to 240).
    Use 0 to print the full diff.

  --ts-out=<dir>
    Output directory for TS exported MDLs (default: .parity-artifacts/loop-ts-retail-mdl)

  --go-out=<dir>
    Output directory for Go exported MDLs (default: .parity-artifacts/loop-go-retail-mdl)

  --random-mdl, --random-m2
    Pick random .m2 and .wmo models from the listfile instead of hard-coded cases.
    M2 selections also pick a random skin when available. Off by default.

  --random-count=<n>
    Number of random models to compare (default: 10).

  --random-seed=<n>
    Seed for reproducible random model/skin selection (default: 1).

  --cases-file=<path>
    JSON file for generated random cases (default: .parity-artifacts/random-mdl-cases-<seed>.json).

  --regenerate-cases
    Regenerate the random cases file even if it already exists.

  --debug
    Debug single-case output:
    - prints structured MDL summary (global sequences + texture animation frames)
    - prints diff stat
    - prints first --diff-lines lines of unified diff

  -h, --help
    Show this help text.

Environment variables (alternatives to flags):
  TS_WOW_DATA_SERVER_URL, GO_WOW_DATA_SERVER_URL
  TEST_SUITE, TEST_CASE_IDX, CASE_IDX, TEST_START, TEST_LIMIT, DIFF_LINES, CONTINUE_ON_FAIL

Examples:
  # Run full retail loop, only printing failures
  bun run parity:retail-mdl-loop

  # Continue through all failures, print shorter diffs
  bun run parity:retail-mdl-loop -- --continue --diff-lines=80

  # Debug a specific retail case (0-based index)
  bun run debug:mdl-parity -- --case-idx=1

  # Debug classic case 0
  bun run debug:mdl-parity -- --suite=classic --case-idx=0

  # Compare 10 random listfile .m2/.wmo models (TS vs Go)
  bun scripts/retail-mdl-parity-loop.ts --random-mdl --go-url=http://127.0.0.1:17753
`.trim();

const args = new Map<string, string>();
for (let i = 2; i < process.argv.length; i++) {
  const [key, value = ''] = process.argv[i]!.split('=', 2);
  args.set(key, value);
}

if (args.has('--help') || args.has('-h')) {
  // eslint-disable-next-line no-console
  console.log(HELP_TEXT);
  process.exit(0);
}

const tsUrl = args.get('--ts-url') ?? process.env.TS_WOW_DATA_SERVER_URL ?? 'http://127.0.0.1:17753';
const goUrl = args.get('--go-url') ?? process.env.GO_WOW_DATA_SERVER_URL ?? 'http://127.0.0.1:17754';
const suite = (args.get('--suite') ?? process.env.TEST_SUITE ?? 'retail') === 'classic' ? 'classic' : 'retail';
const randomMdl = args.has('--random-mdl') || args.has('--random-m2') || process.env.RANDOM_MDL_PARITY === '1';
const randomCount = Number(args.get('--random-count') ?? process.env.RANDOM_MDL_COUNT ?? 10);
const randomSeed = Number(args.get('--random-seed') ?? process.env.RANDOM_MDL_SEED ?? 1);
const regenerateCases = args.has('--regenerate-cases');
const debugMode = args.has('--debug');
const caseIdxArg = args.get('--case-idx') ?? process.env.TEST_CASE_IDX ?? process.env.CASE_IDX;
const caseIdx = caseIdxArg == null || caseIdxArg === '' ? undefined : Number(caseIdxArg);

const start = caseIdx ?? Number(args.get('--start') ?? process.env.TEST_START ?? 0);
const limit = Number(args.get('--limit') ?? process.env.TEST_LIMIT ?? (caseIdx != null ? 1 : 0));
const diffLines = Number(args.get('--diff-lines') ?? process.env.DIFF_LINES ?? (debugMode ? 140 : 240));
const defaultTsOut = randomMdl ? '.parity-artifacts/loop-ts-random-mdl' : '.parity-artifacts/loop-ts-retail-mdl';
const defaultGoOut = randomMdl ? '.parity-artifacts/loop-go-random-mdl' : '.parity-artifacts/loop-go-retail-mdl';
const tsOut = args.get('--ts-out') ?? defaultTsOut;
const goOut = args.get('--go-out') ?? defaultGoOut;
const casesFile = args.get('--cases-file') ?? `.parity-artifacts/random-mdl-cases-${randomSeed}.json`;
const continueOnFail = args.has('--continue') || process.env.CONTINUE_ON_FAIL === '1';

let runCases: ParityRunCase[] = [];
if (randomMdl) {
  const payload = await ensureRandomMdlCasesFile(tsUrl, casesFile, randomCount, randomSeed, regenerateCases);
  const allRandomCases = payload.cases.map((testCase) => ({
    modelName: testCase.outputName,
    label: formatRandomCaseLabel(testCase),
  }));
  const end = limit > 0 ? Math.min(allRandomCases.length, start + limit) : allRandomCases.length;
  runCases = allRandomCases.slice(start, end);
  if (!debugMode) {
    const m2Count = payload.cases.filter((c) => c.modelType === 'm2').length;
    const wmoCount = payload.cases.filter((c) => c.modelType === 'wmo').length;
    console.log(`Random MDL parity: ${runCases.length}/${allRandomCases.length} cases (${m2Count} m2, ${wmoCount} wmo, seed=${payload.seed}) -> ${casesFile}`);
  }
} else {
  const allCases = readCasesFromGo(suite);
  const end = limit > 0 ? Math.min(allCases.length, start + limit) : allCases.length;
  runCases = allCases.slice(start, end).map((testCase, offsetIndex) => ({
    modelName: deriveName(testCase.base),
    label: `${start + offsetIndex + 1}/${allCases.length}: ${deriveName(testCase.base)}`,
  }));
}

const failures: string[] = [];

for (let caseIndex = 0; caseIndex < runCases.length; caseIndex++) {
  const { modelName, label } = runCases[caseIndex]!;
  const caseNo = randomMdl ? `${caseIndex + 1}/${runCases.length}` : label;

  const run = debugMode ? runVerbose : runQuiet;

  if (debugMode) {
    // eslint-disable-next-line no-console
    console.log(`\n== ${randomMdl ? 'RANDOM MDL' : suite.toUpperCase()} MDL debug ${caseNo}: ${modelName} ==`);
  }

  const exportOffset = randomMdl ? start + caseIndex : start + caseIndex;
  const tsExportCmd = randomMdl
    ? `bun scripts/export-parity-artifacts.ts --cases-file=${quote(casesFile)} --out=${quote(tsOut)} --fresh --offset=${exportOffset} --limit=1 --format=mdl`
    : `bun scripts/export-parity-artifacts.ts --suite=${suite} --out=${quote(tsOut)} --fresh --offset=${exportOffset} --limit=1 --format=mdl`;
  const goExportCmd = randomMdl
    ? `go run ./test/cmd/test-export -cases-file ${quote(path.join('..', casesFile))} -out ${quote(path.join('..', goOut))} -format mdl -fresh -offset ${exportOffset} -limit 1`
    : `go run ./test/cmd/test-export ${suite === 'classic' ? '-classic ' : ''}-out ${quote(path.join('..', goOut))} -format mdl -fresh -offset ${exportOffset} -limit 1`;

  run('TS export', tsExportCmd, {
    WOW_DATA_SERVER_URL: tsUrl,
    WOW_DATA_SERVER_PORT: new URL(tsUrl).port,
  });

  run('Go export', goExportCmd, {
    WOW_DATA_SERVER_URL: goUrl,
    WOW_DATA_SERVER_PORT: new URL(goUrl).port,
  }, 'golang');

  const tsMdl = path.join(tsOut, `${modelName}.mdl`);
  const goMdl = path.join(goOut, `${modelName}.mdl`);

  if (debugMode) {
    const tsSummary = summarizeMdl(tsMdl);
    const goSummary = summarizeMdl(goMdl);
    // eslint-disable-next-line no-console
    console.log('\n== Structured Summary ==');
    // eslint-disable-next-line no-console
    console.log('ts', JSON.stringify(tsSummary));
    // eslint-disable-next-line no-console
    console.log('go', JSON.stringify(goSummary));
    // eslint-disable-next-line no-console
    console.log('globalSeqEqual', JSON.stringify(tsSummary.globalSeq) === JSON.stringify(goSummary.globalSeq));
    // eslint-disable-next-line no-console
    console.log('texAnimEqual', JSON.stringify(tsSummary.texAnims) === JSON.stringify(goSummary.texAnims));

    // eslint-disable-next-line no-console
    console.log('\n== MDL Diff Stat ==');
    printDiffStat(tsMdl, goMdl);

    // eslint-disable-next-line no-console
    console.log(`\n== MDL Diff First ${diffLines} Lines ==`);
    printDiff(tsMdl, goMdl, diffLines);
    process.exit(0);
  }

  const compare = spawnSync('git', ['diff', '--no-index', '--quiet', '--', tsMdl, goMdl], {
    env: process.env,
    encoding: 'utf8',
  });

  if (compare.status === 0) {
    continue;
  }

  if (compare.status !== 1) {
    if (compare.stdout) process.stdout.write(compare.stdout);
    if (compare.stderr) process.stderr.write(compare.stderr);
    console.error(`Compare failed for ${modelName} with status ${compare.status}`);
    process.exit(compare.status ?? 1);
  }

  console.log(`FAIL ${caseNo}: ${modelName}${randomMdl ? ` (${label})` : ''}`);
  printDiff(tsMdl, goMdl, diffLines);
  failures.push(`${caseNo}: ${modelName}`);
  if (!continueOnFail) {
    process.exit(1);
  }
}

if (failures.length > 0) {
  process.exit(1);
}

process.exit(0);

function formatRandomCaseLabel(testCase: RandomMdlParityCase): string {
  const skin = testCase.skinId ? ` skin=${testCase.skinId}` : '';
  return `${testCase.modelType}:${testCase.fileName}${skin}`;
}

function runQuiet(label: string, command: string, env: Record<string, string>, cwd = '.'): void {
  const result = spawnSync(command, {
    cwd,
    env: { ...process.env, ...env },
    shell: true,
    encoding: 'utf8',
  });
  if (result.status !== 0) {
    console.error(`${label} failed with status ${result.status}`);
    if (result.stdout) process.stdout.write(result.stdout);
    if (result.stderr) process.stderr.write(result.stderr);
    process.exit(result.status ?? 1);
  }
}

function runVerbose(label: string, command: string, env: Record<string, string>, cwd = '.'): void {
  // eslint-disable-next-line no-console
  console.log(`\n== ${label} ==`);
  const result = spawnSync(command, {
    cwd,
    env: { ...process.env, ...env },
    shell: true,
    stdio: 'inherit',
  });
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

function printDiffStat(left: string, right: string): void {
  const result = spawnSync('git', ['diff', '--no-index', '--stat', '--', left, right], {
    env: process.env,
    encoding: 'utf8',
  });
  if (result.stderr) process.stderr.write(result.stderr);
  if (result.stdout) process.stdout.write(result.stdout);
  if (result.status != null && result.status > 1) {
    process.exit(result.status);
  }
}

function printDiff(left: string, right: string, maxLines: number): void {
  const result = spawnSync('git', ['diff', '--no-index', '--unified=2', '--', left, right], {
    env: process.env,
    encoding: 'utf8',
  });
  if (result.stderr) process.stderr.write(result.stderr);
  if (result.stdout) {
    const output = maxLines > 0
      ? `${result.stdout.split(/\r?\n/).slice(0, maxLines).join('\n')}\n`
      : result.stdout;
    process.stdout.write(output);
  }
  if (result.status != null && result.status > 1) {
    console.error(`Diff failed with status ${result.status}`);
    process.exit(result.status);
  }
}

function readCasesFromGo(suiteName: 'retail' | 'classic'): TestCase[] {
  const source = readFileSync('golang/internal/testcases/cases.go', 'utf8').replaceAll('\r\n', '\n');
  const varName = suiteName === 'classic' ? 'classicCases' : 'retailCases';
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

function unescapeGoString(value: string): string {
  return (JSON.parse(`"${value}"`) as string).replaceAll('\\\\', '\\');
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

function quote(value: string): string {
  return `"${value.replaceAll('"', '\\"')}"`;
}

function summarizeMdl(filePath: string) {
  const model = new parsers.mdlx.Model();
  model.loadMdl(readFileSync(filePath, 'utf8'));
  return {
    globalSeq: model.globalSequences,
    texAnims: model.textureAnimations.map((textureAnim, i) => ({
      i,
      anims: textureAnim.animations.map((animation) => ({
        gs: animation.globalSequenceId,
        frames: animation.frames,
      })),
    })),
  };
}
