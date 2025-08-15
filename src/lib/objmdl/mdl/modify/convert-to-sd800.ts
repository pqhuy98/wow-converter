import { MDLModify } from ".";
import { Bone } from "../components/node";
import { Geoset, GeosetVertex, Matrix } from "../components/geoset";
import { Face } from "../components/geoset";

export function convertToSd800(this: MDLModify) {
  const mdl = this.mdl;
  if (mdl.version.formatVersion === 800) return this;
  mdl.version.formatVersion = 800;

  // Stable order for existing and newly created (phantom) bones
  const boneOriginalOrder = new Map<Bone, number>(mdl.bones.map((b, idx) => [b, idx]));
  let nextBoneOrder = mdl.bones.length;

  // Phantom bone management per original bone
  const phantomBonesPerOriginal = new Map<Bone, Bone[]>();
  const createdPhantomBones = new Set<Bone>();
  const nextPhantomIndexPerOriginal = new Map<Bone, number>();

  const ensurePhantomBones = (original: Bone, requiredCount: number): Bone[] => {
    if (!phantomBonesPerOriginal.has(original)) {
      phantomBonesPerOriginal.set(original, []);
      nextPhantomIndexPerOriginal.set(original, 1);
    }
    const list = phantomBonesPerOriginal.get(original)!;
    while (list.length < requiredCount) {
      const idx = nextPhantomIndexPerOriginal.get(original)!;
      const phantom: Bone = {
        type: 'Bone',
        name: `${original.name}__${idx}`,
        pivotPoint: [...original.pivotPoint],
        parent: original,
        flags: [],
      };
      mdl.bones.push(phantom);
      createdPhantomBones.add(phantom);
      boneOriginalOrder.set(phantom, nextBoneOrder++);
      list.push(phantom);
      nextPhantomIndexPerOriginal.set(original, idx + 1);
    }
    return list;
  };

  // For each geoset, generate classic (matrix based) skinning data
  mdl.geosets.forEach((geoset) => {
    const matrixMap = new Map<string, Matrix>();
    const matrices: Matrix[] = [];

    const getOrCreateMatrix = (bones: Bone[]): Matrix => {
      const key = bones
        .map((b) => boneOriginalOrder.get(b)!)
        .sort((a, b) => a - b)
        .join(',');
      let m = matrixMap.get(key);
      if (!m) {
        m = { id: matrices.length, bones };
        matrices.push(m);
        matrixMap.set(key, m);
      }
      return m;
    };

    geoset.vertices.forEach((v) => {
      // Build weight-aware matrix using phantom bones to approximate weights
      const resolution = 8;
      const weights = (v.skinWeights ?? [])
        .filter((sw) => sw.weight > 0)
        .map((sw) => ({ bone: sw.bone, weight: sw.weight }));

      let bonesForMatrix: Bone[] = [];
      if (weights.length > 0) {
        // Normalize
        const sum = weights.reduce((a, b) => a + b.weight, 0);
        const normalized = weights.map((w) => ({ bone: w.bone, weight: w.weight / (sum || 1) }));

        // Quantize with largest-remainder to sum exactly to <= resolution, dropping tiny contributions
        const raw = normalized.map((w) => ({ bone: w.bone, raw: w.weight * resolution }));
        const floor = raw.map((r) => ({ bone: r.bone, count: Math.floor(r.raw), frac: r.raw - Math.floor(r.raw) }));
        let total = floor.reduce((a, b) => a + b.count, 0);

        // Distribute remaining tokens by fractional part, but cap at resolution
        if (total < resolution) {
          const remaining = resolution - total;
          floor
            .slice()
            .sort((a, b) => b.frac - a.frac)
            .slice(0, remaining)
            .forEach((item) => item.count++);
          total = floor.reduce((a, b) => a + b.count, 0);
        }

        // Remove zero-count small contributors
        const counts = floor.filter((c) => c.count > 0);
        // Edge-case: if all counts are zero (extreme tiny weights), keep strongest one
        if (counts.length === 0) {
          const strongest = normalized.slice().sort((a, b) => b.weight - a.weight)[0];
          counts.push({ bone: strongest.bone, count: 1, frac: 0 });
        }

        // For each bone: include original bone once, plus (count-1) phantom bones
        counts.forEach(({ bone, count }) => {
          if (count <= 0) return;
          bonesForMatrix.push(bone);
          if (count > 1) {
            const phantoms = ensurePhantomBones(bone, count - 1);
            bonesForMatrix.push(...phantoms.slice(0, count - 1));
          }
        });
      } else {
        // Fallback: no skin weights -> bind to first bone only
        bonesForMatrix = [mdl.bones[0]];
      }

      // Deterministic ordering
      bonesForMatrix.sort((a, b) => (boneOriginalOrder.get(a) ?? 0) - (boneOriginalOrder.get(b) ?? 0));

      const mat = getOrCreateMatrix(bonesForMatrix);
      v.matrix = mat;
      delete v.skinWeights;
    });

    // If we exceeded 255 matrices, split the geoset into chunks that each fit the limit
    if (matrices.length > 255) {
      type Part = {
        matrices: Matrix[];
        matrixMap: Map<Matrix, Matrix>;
        vertices: GeosetVertex[];
        vertexMap: Map<GeosetVertex, GeosetVertex>;
        faces: Face[];
      };

      const parts: Part[] = [];

      const canFitFaceIntoPart = (part: Part, required: Set<Matrix>) => {
        let needed = 0;
        required.forEach((m) => { if (!part.matrixMap.has(m)) needed++; });
        return part.matrices.length + needed <= 255;
      };

      const ensureMatrixInPart = (part: Part, m: Matrix): Matrix => {
        let mapped = part.matrixMap.get(m);
        if (!mapped) {
          mapped = { id: part.matrices.length, bones: m.bones };
          part.matrices.push(mapped);
          part.matrixMap.set(m, mapped);
        }
        return mapped;
      };

      geoset.faces.forEach((f) => {
        const required = new Set<Matrix>([
          f.vertices[0].matrix!,
          f.vertices[1].matrix!,
          f.vertices[2].matrix!,
        ]);

        let assigned: Part | undefined = parts.find((p) => canFitFaceIntoPart(p, required));
        if (!assigned) {
          assigned = {
            matrices: [],
            matrixMap: new Map(),
            vertices: [],
            vertexMap: new Map(),
            faces: [],
          };
          parts.push(assigned);
        }

        // Ensure matrices and clone vertices into the part
        const vClones: GeosetVertex[] = [0, 1, 2].map((i) => {
          const origV = f.vertices[i];
          let clone = assigned!.vertexMap.get(origV);
          if (!clone) {
            const mappedMatrix = ensureMatrixInPart(assigned!, origV.matrix!);
            clone = { ...origV, matrix: mappedMatrix };
            assigned!.vertices.push(clone);
            assigned!.vertexMap.set(origV, clone);
          } else {
            // Ensure its matrix mapping exists
            clone.matrix = ensureMatrixInPart(assigned!, origV.matrix!);
          }
          return clone;
        });

        assigned.faces.push({ vertices: [vClones[0], vClones[1], vClones[2]] });
      });

      // Materialize new geosets
      const newGeosets: Geoset[] = parts.map((p, idx) => ({
        ...geoset,
        id: -1,
        vertices: p.vertices,
        faces: p.faces,
        matrices: p.matrices,
        name: `${geoset.name}_${idx}`,
      }));

      // Replace old geoset with first part and push others
      const firstPart = newGeosets.shift()!;
      Object.assign(geoset, firstPart);
      mdl.geosets.push(...newGeosets);
    } else {
      // Within limit
      geoset.matrices = matrices;
    }
  });

  // Prune unused phantom bones
  const usedBones = new Set<Bone>();
  mdl.geosets.forEach((gs) => gs.matrices.forEach((m) => m.bones.forEach((b) => usedBones.add(b))));
  mdl.bones = mdl.bones.filter((b) => !createdPhantomBones.has(b) || usedBones.has(b));

  // Assert matrices not exceed length 255
  if (mdl.geosets.some((gs) => gs.matrices.length > 255)) {
    throw new Error('Geoset matrices exceed length 255');
  }

  // Assert matrice contents are unique
  if (mdl.geosets.some((gs) => gs.matrices.length !== new Set(gs.matrices).size)) {
    throw new Error('Geoset matrices are not unique');
  }

  console.log("Largest matrix size", mdl.geosets.reduce((max, gs) =>
    Math.max(max, ...gs.matrices.map((m) => m.bones.length)), 0));

  mdl.sync();

  return this;
}