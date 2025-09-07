import chalk from 'chalk';

import { Animation } from '../components/animation';
import { GlobalSequence } from '../components/global-sequence';
import { Material } from '../components/material';
import { Node } from '../components/node/node';
import { Texture } from '../components/texture';
import { buildChildrenLists } from '../mdl-traverse';
import { MDLModify } from '.';

export function removeUnusedMaterialsTextures(this: MDLModify) {
  // Deduplicate textures
  this.mdl.materials = [...new Set(this.mdl.geosets.map((geoset) => geoset.material))];
  const textureKey = (tex: Texture) => JSON.stringify(tex);
  const usedTextures = new Map<string, Texture>();

  const getTexture = (tex: Texture) => {
    const key = textureKey(tex);
    if (!usedTextures.has(key)) {
      usedTextures.set(key, tex);
    }
    return usedTextures.get(key)!;
  };

  this.mdl.materials.forEach((mat, i) => {
    mat.layers.forEach((layer) => {
      if (layer.texture.image === '') {
        console.log(chalk.red(`Empty texture, i: ${i}, wow type: ${layer.texture.wowData.type}`));
      }
      layer.texture = getTexture(layer.texture);
    });
  });
  this.mdl.particleEmitter2s.forEach((e) => {
    e.texture = getTexture(e.texture);
  });
  this.mdl.textures = [...usedTextures.values()];

  // Deduplicate materials
  // set id of texture anims so that materialKey works correct.
  // Because textureAnims[].XXX.keyframes cannot be serialized since it's a Map
  this.mdl.textureAnims.forEach((ta, i) => ta.id = i);
  const materialKey = (mat: Material) => JSON.stringify(mat);

  const usedMaterials = new Map<string, Material>();
  this.mdl.geosets.forEach((geoset) => {
    const matKey = materialKey(geoset.material);
    if (!usedMaterials.has(matKey)) {
      usedMaterials.set(matKey, geoset.material);
    } else {
      geoset.material = usedMaterials.get(matKey)!;
    }
  });
  this.mdl.materials = [...usedMaterials.values()];
  return this;
}

export function removeUnusedNodes(this: MDLModify) {
  const usedNodes = new Set<Node>([
    ...this.mdl.attachments,
    // ...this.mdl.particleEmitters,
    ...this.mdl.particleEmitter2s,
    // ...this.mdl.particleEmitterPopcorns,
    ...this.mdl.eventObjects,
    ...this.mdl.collisionShapes,
  ]);
  this.mdl.geosets.forEach((geoset) => geoset.vertices.forEach((v) => {
    v.skinWeights?.forEach((sw) => usedNodes.add(sw.bone));
    v.matrix?.bones.forEach((b) => usedNodes.add(b));
  }));

  const childrenList = buildChildrenLists(this.mdl);
  const dfs = (cur: Node) => {
    let isUsed = usedNodes.has(cur);
    // console.log(cur.name, "children:", childrenList.get(cur)!.map(n => n.name).join(", "))
    for (const child of childrenList.get(cur)!) {
      const childUsed = dfs(child);
      isUsed = childUsed || isUsed;
    }
    if (isUsed) usedNodes.add(cur);
    return isUsed;
  };
  this.mdl.bones.forEach((b) => {
    if (!b.parent) dfs(b);
  });
  this.mdl.bones = this.mdl.bones.filter((b) => usedNodes.has(b));
  return this;
}

export function removeUnusedVertices(this: MDLModify) {
  this.mdl.geosets.forEach((geoset) => {
    const usedNodes = new Set(geoset.faces.flatMap((face) => face.vertices));
    geoset.vertices = geoset.vertices.filter((v) => usedNodes.has(v));
  });
  this.mdl.syncExtents();
  return this;
}

export function removeCinematicSequences(this: MDLModify) {
  this.mdl.sequences = this.mdl.sequences.filter((seq) => !seq.name.includes('Cinematic') || seq.keep);
  return this;
}

export function optimizeKeyFrames(this: MDLModify) {
  // Pre-compute sequence intervals once so every key-frame test is O(1)
  const seqIntervals = this.mdl.sequences
    .map((s) => [s.interval[0], s.interval[1]] as const)
    .sort((a, b) => a[0] - b[0]);

  // Cursor-based helper – much cheaper than Array.some/Array.find each time.
  const inSequence = (anim: Animation<unknown>, timestamp: number, cursor: { idx: number }): boolean => {
    if (anim.globalSeq) {
      return timestamp < anim.globalSeq.duration;
    }
    let i = cursor.idx;
    while (i < seqIntervals.length && seqIntervals[i][1] < timestamp) i++;
    cursor.idx = i;
    return i < seqIntervals.length && seqIntervals[i][0] <= timestamp;
  };

  const thresholds = {
    translation: 0.005,
    rotation: 0.001,
    scaling: 0.01,
    alpha: 0.01,
    color: 0.01,
    tvertex: 0.01,
    tvertexAnim: 0.01,
    default: 0.01,
  } as const;

  const diffBetween = (a: number | number[], b: number | number[]): number => {
    if (typeof a === 'number' && typeof b === 'number') return Math.abs(a - b);
    if (Array.isArray(a) && Array.isArray(b)) {
      let acc = 0;
      for (let i = 0; i < a.length; i++) acc += Math.abs(a[i] - b[i]);
      return acc;
    }
    return Number.POSITIVE_INFINITY;
  };

  let initialKfCount = 0;
  let deletedKfCount = 0;

  const optimiseAnim = <T extends number[] | number>(anim: Animation<T>, threshold: number) => {
    initialKfCount += anim.keyFrames.size;
    if (!anim || anim.keyFrames.size <= 2) return; // nothing to prune

    const times = Array.from(anim.keyFrames.keys()).sort((a, b) => a - b);
    let t0 = times[0];
    const cursor = { idx: 0 };

    for (let k = 1; k < times.length; k++) {
      const v0 = anim.keyFrames.get(t0)!;
      const t1 = times[k];
      const v1 = anim.keyFrames.get(t1)!;
      const t2 = times[k + 1];
      const v2 = anim.keyFrames.get(t2);

      const inside = inSequence(anim, t1, cursor);
      if (!inside) { // always drop keys that sit in no sequence
        anim.keyFrames.delete(t1);
        deletedKfCount++;
        continue;
      }

      if (k < times.length - 1) {
        if (diffBetween(v0, v1) >= threshold) {
          t0 = t1;
          continue; // keep – movement above threshold
        }
      }

      let firstFrame = false;
      for (let sIdx = 0; sIdx < seqIntervals.length; sIdx++) {
        const sStart = seqIntervals[sIdx][0];
        if (t0 < sStart && sStart <= t1) { firstFrame = true; break; }
      }

      const nextT = k + 1 < times.length ? times[k + 1] : Number.POSITIVE_INFINITY;
      let lastFrame = k === times.length - 1;
      if (!lastFrame) {
        for (let sIdx = 0; sIdx < seqIntervals.length; sIdx++) {
          const sEnd = seqIntervals[sIdx][1];
          if (t1 <= sEnd && sEnd < nextT) { lastFrame = true; break; }
        }
      }

      if ((!inside || (!firstFrame && !lastFrame)) && v2 && diffBetween(v0, v2) < threshold) {
        anim.keyFrames.delete(t1);
        deletedKfCount++;
        continue;
      }

      // keep – important boundary frame
      t0 = t1;
    }

    if (!inSequence(anim, times[0], { idx: 0 })) {
      anim.keyFrames.delete(times[0]);
      deletedKfCount++;
    }
  };

  const usedGlobalSequences = new Set<GlobalSequence>();

  this.mdl.getAnimated().forEach((anim) => {
    if (anim.globalSeq) {
      usedGlobalSequences.add(anim.globalSeq);
    }

    const threshold = thresholds[anim.type] ?? thresholds.default;
    optimiseAnim(anim, threshold);
  });

  const debug = false;
  const reduction = deletedKfCount / Math.max(1, initialKfCount);
  debug && console.log(chalk.green(`Reduced key frames by ${(reduction * 100).toFixed(2)}% `
  + `after deleting ${deletedKfCount}/${initialKfCount} key frames`));

  this.mdl.globalSequences = this.mdl.globalSequences.filter((gs) => usedGlobalSequences.has(gs))
    .sort((a, b) => a.duration - b.duration);

  const neverVisible = (obj: {name: string, visibility?: Animation<number>}) => {
    const visibility = [...(obj.visibility?.keyFrames.values() ?? [])];
    return visibility.length > 0 && visibility.every((v) => !v);
  };
  this.mdl.particleEmitter2s = this.mdl.particleEmitter2s.filter((e) => !neverVisible(e));
  this.mdl.ribbonEmitters = this.mdl.ribbonEmitters.filter((e) => !neverVisible(e));
  this.mdl.lights = this.mdl.lights.filter((e) => !neverVisible(e));

  return this;
}
