import { MDLModify } from ".";
import { Node } from "../components/node";
import { Animation } from "../components/animation";
import { GlobalSequence } from "../components/global-sequence";
import { Material } from "../components/material";
import { Texture } from "../components/texture";
import { buildChildrenLists } from "../mdl-traverse";


export function removeUnusedMaterialsTextures(this: MDLModify) {
  // Deduplicate textures
  this.mdl.materials = [...new Set(this.mdl.geosets.map((geoset) => geoset.material))];
  const textureKey = (tex: Texture) => JSON.stringify(tex);
  const usedTextures = new Map<string, Texture>();
  this.mdl.materials.forEach((mat) => {
    mat.layers.forEach((layer) => {
      if (layer.texture.image === '') {
        console.log('Empty texture', mat.id, layer.texture.image);
      }
      const texKey = textureKey(layer.texture);
      if (!usedTextures.has(texKey)) {
        usedTextures.set(texKey, layer.texture);
      } else {
        layer.texture = usedTextures.get(texKey)!;
      }
    });
  });
  this.mdl.textures = [...usedTextures.values()];

  // Deduplicate materials
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
  const usedNodes = new Set<Node>([...this.mdl.attachments]);
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
  this.mdl.sequences = this.mdl.sequences.filter((seq) => !seq.name.includes('Cinematic'));
  return this;
}


export function optimizeKeyFrames(this: MDLModify) {
  // Pre-compute sequence intervals once so every key-frame test is O(1)
  const seqIntervals = this.mdl.sequences
    .map((s) => [s.interval[0], s.interval[1]] as const)
    .sort((a, b) => a[0] - b[0]);

  // Cursor-based helper – much cheaper than Array.some/Array.find each time.
  const inSequence = (timestamp: number, cursor: { idx: number }): boolean => {
    let i = cursor.idx;
    while (i < seqIntervals.length && seqIntervals[i][1] < timestamp) i++;
    cursor.idx = i;
    return i < seqIntervals.length && seqIntervals[i][0] <= timestamp;
  };

  const optimiseAnim = <T extends number[] | number>(anim: Animation<T>, threshold: number) => {
    if (!anim || anim.keyFrames.size <= 2) return; // nothing to prune

    const times = Array.from(anim.keyFrames.keys()).sort((a, b) => a - b);
    let prevT = times[0];
    const cursor = { idx: 0 };

    for (let k = 1; k < times.length; k++) {
      const t = times[k];
      const v1 = anim.keyFrames.get(t)!;
      const v0 = anim.keyFrames.get(prevT)!;

      const inside = inSequence(t, cursor);
      if (!inside) { // always drop keys that sit in no sequence
        anim.keyFrames.delete(t);
        continue;
      }

      // Early-exit diff calculation
      let diff = 0;
      if (Array.isArray(v1)) {
        for (let j = 0; j < v1.length && diff < threshold; j++) diff += Math.abs(v1[j] - v0[j]);
      } else if (typeof v1 === 'number' && typeof v0 === 'number') {
        diff = Math.abs(v1 - v0);
      }

      if (diff >= threshold) {
        prevT = t;
        continue; // keep – movement above threshold
      }

      let firstFrame = false;
      for (let sIdx = 0; sIdx < seqIntervals.length; sIdx++) {
        const sStart = seqIntervals[sIdx][0];
        if (prevT < sStart && sStart <= t) { firstFrame = true; break; }
      }

      const nextT = k + 1 < times.length ? times[k + 1] : Number.POSITIVE_INFINITY;
      let lastFrame = k === times.length - 1;
      if (!lastFrame) {
        for (let sIdx = 0; sIdx < seqIntervals.length; sIdx++) {
          const sEnd = seqIntervals[sIdx][1];
          if (t <= sEnd && sEnd < nextT) { lastFrame = true; break; }
        }
      }

      if (!inside || (!firstFrame && !lastFrame)) {
        // Prune – insignificant change inside idle segment
        anim.keyFrames.delete(t);
      } else {
        prevT = t; // keep – important boundary frame
      }
    }
  };

  const usedGlobalSequences = new Set<GlobalSequence>();

  this.mdl.getAnimated().forEach((anim) => {
    if (anim.globalSeq) {
      usedGlobalSequences.add(anim.globalSeq);
      return;
    }

    let threshold = 0.005;
    switch (anim.type) {
      case 'translation':
        threshold = 0.005;
        break;
      case 'rotation':
        threshold = 0.001;
        break;
      case 'scaling':
        threshold = 0.01;
        break;
      case 'alpha':
        threshold = 0.01;
        break;
      case 'color':
        threshold = 0.01;
        break;
      case 'tvertex':
        threshold = 0.001;
        break;
      case 'tvertexAnim':
        threshold = 0.001;
        break;
    }
    optimiseAnim(anim, threshold);
  });

  this.mdl.globalSequences = this.mdl.globalSequences.filter((gs) => usedGlobalSequences.has(gs));

  return this;
}