/**
 * End-to-end export timing (UI request → wow-data-server → converter → MDX).
 * Enable with EXPORT_PROFILE=1.
 */
import { AsyncLocalStorage } from 'async_hooks';

export interface ProfileNode {
  name: string;
  ms: number;
  meta?: Record<string, string | number>;
  children?: ProfileNode[];
}

export interface ExportPipelineSummary {
  /** Native M2 → OBJ (+ sidecar JSON/PNG) on wow-data-server. */
  m2ToObjMs: number;
  /** Memory/disk handoff: download export files to converter. */
  prefetchMs: number;
  /** OBJ/JSON → in-memory MDL on converter. */
  objToMdxMs: number;
  /** Per-item export, body texture composite, MDL merge / attach. */
  gearAttachMs: number;
  /** MDL post-process + write MDX/BLP. */
  finalizeMs: number;
  /** Everything else (metadata fetch, skin pick, REST overhead, …). */
  otherMs: number;
}

export interface ExportProfileSnapshot {
  totalMs: number;
  tree: ProfileNode[];
  summary: Record<string, number>;
  pipeline: ExportPipelineSummary;
}

const als = new AsyncLocalStorage<ExportProfile>();

export function isExportProfileEnabled(): boolean {
  const v = process.env.EXPORT_PROFILE;
  return v === '1' || v === 'true';
}

export function getExportProfile(): ExportProfile | null {
  return als.getStore() ?? null;
}

export async function runWithExportProfile<T>(
  fn: () => Promise<T>,
): Promise<{ result: T; profile: ExportProfileSnapshot | null }> {
  if (!isExportProfileEnabled()) {
    return { result: await fn(), profile: null };
  }
  const profile = new ExportProfile();
  const result = await als.run(profile, fn);
  return { result, profile: profile.snapshot() };
}

export async function profileScope<T>(
  name: string,
  fn: () => Promise<T>,
  meta?: Record<string, string | number>,
): Promise<T> {
  const profile = getExportProfile();
  if (!profile) return fn();
  return profile.scope(name, fn, meta);
}

export function profileMark(name: string, ms: number, meta?: Record<string, string | number>): void {
  getExportProfile()?.mark(name, ms, meta);
}

/** Like profileScope for synchronous work (e.g. MDL merge helpers). */
export function profileSync<T>(name: string, fn: () => T, meta?: Record<string, string | number>): T {
  if (!getExportProfile()) return fn();
  const t0 = performance.now();
  try {
    return fn();
  } finally {
    profileMark(name, performance.now() - t0, meta);
  }
}

interface Frame {
  name: string;
  start: number;
  children: ProfileNode[];
  meta?: Record<string, string | number>;
}

export class ExportProfile {
  private readonly root: ProfileNode[] = [];

  private stack: Frame[] = [];

  private readonly start = performance.now();

  async scope<T>(name: string, fn: () => Promise<T>, meta?: Record<string, string | number>): Promise<T> {
    const frame: Frame = {
      name, start: performance.now(), children: [], meta,
    };
    this.stack.push(frame);
    try {
      return await fn();
    } finally {
      this.stack.pop();
      const ms = performance.now() - frame.start;
      const node: ProfileNode = { name, ms, meta: frame.meta };
      if (frame.children.length > 0) node.children = frame.children;
      this.append(node);
    }
  }

  mark(name: string, ms: number, meta?: Record<string, string | number>): void {
    this.append({ name, ms, meta });
  }

  merge(snapshot: ExportProfileSnapshot | null | undefined, prefix: string): void {
    if (!snapshot) return;
    for (const node of snapshot.tree) {
      this.append(cloneWithPrefix(node, prefix));
    }
  }

  snapshot(): ExportProfileSnapshot {
    const totalMs = performance.now() - this.start;
    const tree = this.root.map((n) => ({ ...n }));
    const summary = flattenSummary(tree);
    return {
      totalMs,
      tree,
      summary,
      pipeline: classifyPipeline(summary, totalMs),
    };
  }

  private append(node: ProfileNode): void {
    const parent = this.stack[this.stack.length - 1];
    if (parent) parent.children.push(node);
    else this.root.push(node);
  }
}

function cloneWithPrefix(node: ProfileNode, prefix: string): ProfileNode {
  const name = prefix ? `${prefix}/${node.name}` : node.name;
  const cloned: ProfileNode = { name, ms: node.ms, meta: node.meta };
  if (node.children?.length) {
    cloned.children = node.children.map((c) => cloneWithPrefix(c, prefix));
  }
  return cloned;
}

function flattenSummary(nodes: ProfileNode[], prefix = ''): Record<string, number> {
  const out: Record<string, number> = {};
  for (const node of nodes) {
    const key = prefix ? `${prefix}/${node.name}` : node.name;
    out[key] = (out[key] ?? 0) + node.ms;
    if (node.children?.length) {
      const child = flattenSummary(node.children, key);
      for (const [k, v] of Object.entries(child)) {
        out[k] = (out[k] ?? 0) + v;
      }
    }
  }
  return out;
}

function classifyPipeline(summary: Record<string, number>, totalMs: number): ExportPipelineSummary {
  let m2ToObjMs = 0;
  let prefetchMs = 0;
  let objToMdxMs = 0;
  let gearAttachMs = 0;
  let finalizeMs = 0;
  let accounted = 0;

  const keys = Object.keys(summary);
  const isLeaf = (key: string) => !keys.some((k) => k !== key && k.startsWith(`${key}/`));

  for (const [key, ms] of Object.entries(summary)) {
    if (!isLeaf(key)) continue;
    const k = key.toLowerCase();
    if (k.includes('exportasobj') || k.includes('m2.load') || k.includes('m2/exporttextures')
      || k.includes('server/exportmodels') || k.includes('server/exportcharacter')
      || k.includes('writeobj') || k.includes('writebones') || k.includes('writemeta')
      || k.includes('bakematerials') || k.includes('initmodelcaches') || k.includes('initcharactercaches')) {
      m2ToObjMs += ms;
      accounted += ms;
    } else if (k.includes('prefetch') || k.includes('/download')) {
      prefetchMs += ms;
      accounted += ms;
    } else if (k.includes('objtomdx') || k.includes('convertwowexportmodel') || k.includes('parseobj')
      || k.includes('parseanimation') || k.includes('parsemetadata') || k.includes('assetmanager.parse')
      || k.includes('parsedirect') || k.includes('exportbasedirect') || k.includes('exportbaselegacy')
      || k.includes('buildbones') || k.includes('m2tomdx')) {
      objToMdxMs += ms;
      accounted += ms;
    } else if (k.includes('attachequipment') || k.includes('attachgear') || k.includes('mergecollection')
      || k.includes('mergebone') || k.includes('mergecustomization') || k.includes('applyequipmentbody')
      || k.includes('compositebodytexture') || k.includes('processequipment') || k.includes('processitem/')
      || k.includes('/exportitem') || k.includes('cloneitem') || k.includes('applycustomzation')
      || k.includes('exportitemvisual') || k.includes('exportcustomizationcollection')
      || k.includes('exportoverlaytextures') || k.includes('writebodytexture') || k.includes('applycloaktexture')
      || k.includes('preparecharacterexport') || k.includes('applyreplaceabletextures')
      || k.includes('reparenttobone') || k.includes('mergeitemobjects') || k.includes('remapbones')
      || k.includes('cachecollection') || k.includes('clonedeep') || k.includes('filtercollectiongeosets')
      || k.includes('assigngeosets') || k.includes('forkcollection')) {
      gearAttachMs += ms;
      accounted += ms;
    } else if (k.includes('writetextures') || k.includes('writemodels') || k.includes('postprocess')
      || k.includes('optimizemdl') || k.includes('pngstoblps') || k.includes('recomputenormals')
      || k.includes('optimizekeyframes')) {
      finalizeMs += ms;
      accounted += ms;
    }
  }

  return {
    m2ToObjMs,
    prefetchMs,
    objToMdxMs,
    gearAttachMs,
    finalizeMs,
    otherMs: Math.max(0, totalMs - accounted),
  };
}

export function formatExportProfile(snapshot: ExportProfileSnapshot): string {
  const lines = [
    `Export profile (${snapshot.totalMs.toFixed(0)} ms total)`,
    `  pipeline: m2→obj ${snapshot.pipeline.m2ToObjMs.toFixed(0)} ms | prefetch ${snapshot.pipeline.prefetchMs.toFixed(0)} ms | obj→mdx ${snapshot.pipeline.objToMdxMs.toFixed(0)} ms | gear ${snapshot.pipeline.gearAttachMs.toFixed(0)} ms | finalize ${snapshot.pipeline.finalizeMs.toFixed(0)} ms | other ${snapshot.pipeline.otherMs.toFixed(0)} ms`,
  ];
  const top = Object.entries(snapshot.summary)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 20);
  if (top.length > 0) {
    lines.push('  top phases:');
    for (const [name, ms] of top) {
      lines.push(`    ${ms.toFixed(0).padStart(6)} ms  ${name}`);
    }
  }
  return lines.join('\n');
}
