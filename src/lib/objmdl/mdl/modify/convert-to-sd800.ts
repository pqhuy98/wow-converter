import { MDLModify } from ".";
import { Bone } from "../components/node";
import { Geoset, GeosetVertex, Matrix } from "../components/geoset";
import { Face } from "../components/geoset";

export function convertToSd800(this: MDLModify) {
  const mdl = this.mdl;
  if (mdl.version.formatVersion === 800) return this;
  mdl.version.formatVersion = 800;

  const boneOriginalOrder = new Map<Bone, number>(mdl.bones.map((b, idx) => [b, idx]));

  // For each geoset, generate classic (matrix based) skinning data
  mdl.geosets.forEach((geoset) => {
    const matrixMap = new Map<string, Matrix>();
    const matrices: Matrix[] = [];

    const getOrCreateMatrix = (bones: Bone[]): Matrix => {
      const key = bones.map((b) => boneOriginalOrder.get(b)!).sort((a, b) => a - b).join(',');
      let m = matrixMap.get(key);
      if (!m) {
        m = { id: matrices.length, bones };
        matrices.push(m);
        matrixMap.set(key, m);
      }
      return m;
    };

    geoset.vertices.forEach((v) => {
      // Compile influencing bones – take up to 4 strongest weights
      let bones: Bone[] = [];
      if (v.skinWeights && v.skinWeights.length > 0) {
        bones = v.skinWeights
          .filter((sw) => sw.weight > 0)
          .sort((a, b) => {
            const diff = b.weight - a.weight;
            if (diff !== 0) return diff;
            return (boneOriginalOrder.get(a.bone) ?? 0) - (boneOriginalOrder.get(b.bone) ?? 0);
          })
          .slice(0, 4)
          .map((sw) => sw.bone);
      }
      // Fallback: if still empty pick first bone to avoid empty set
      if (bones.length === 0) bones = [mdl.bones[0]];

      // Ensure deterministic order by original order
      bones.sort((a, b) => (boneOriginalOrder.get(a) ?? 0) - (boneOriginalOrder.get(b) ?? 0));

      const mat = getOrCreateMatrix(bones);
      v.matrix = mat;
      delete v.skinWeights;
    });

    // If we exceeded 255 matrices, split the geoset into chunks that each fit the limit
    if (matrices.length > 255) {
      const parts: Geoset[] = [];
      const partMatrices: Matrix[][] = [];
      const matrixPartInfo = new Map<Matrix, { partIdx: number; newMat: Matrix }>();

      matrices.forEach((m, idx) => {
        const partIdx = Math.floor(idx / 255);
        if (!partMatrices[partIdx]) partMatrices[partIdx] = [];
        const localMat: Matrix = { id: partMatrices[partIdx].length, bones: m.bones };
        partMatrices[partIdx].push(localMat);
        matrixPartInfo.set(m, { partIdx, newMat: localMat });
      });

      const partVertices: GeosetVertex[][] = partMatrices.map(() => []);
      const vertexCloneMaps: Map<GeosetVertex, GeosetVertex>[] = partMatrices.map(() => new Map());

      // Clone vertices into their parts with remapped matrix
      geoset.vertices.forEach((v) => {
        const info = matrixPartInfo.get(v.matrix!);
        if (!info) return; // should not happen
        const clone: GeosetVertex = { ...v, matrix: info.newMat };
        partVertices[info.partIdx].push(clone);
        vertexCloneMaps[info.partIdx].set(v, clone);
      });

      // Prepare faces per part
      const partFaces: Face[][] = partMatrices.map(() => []);
      geoset.faces.forEach((f) => {
        const info0 = matrixPartInfo.get(f.vertices[0].matrix!);
        const info1 = matrixPartInfo.get(f.vertices[1].matrix!);
        const info2 = matrixPartInfo.get(f.vertices[2].matrix!);
        if (!info0 || !info1 || !info2) return;
        // Only keep face if all verts in same part
        if (info0.partIdx === info1.partIdx && info1.partIdx === info2.partIdx) {
          const partIdx = info0.partIdx;
          const clonedFace: Face = {
            vertices: [
              vertexCloneMaps[partIdx].get(f.vertices[0])!,
              vertexCloneMaps[partIdx].get(f.vertices[1])!,
              vertexCloneMaps[partIdx].get(f.vertices[2])!,
            ],
          };
          partFaces[partIdx].push(clonedFace);
        }
      });

      // Build new geosets
      partMatrices.forEach((mats, idx) => {
        const newGs: Geoset = {
          ...geoset,
          id: -1,
          vertices: partVertices[idx],
          faces: partFaces[idx],
          matrices: mats,
          name: `${geoset.name}_${idx}`,
        };
        parts.push(newGs);
      });

      // Replace old geoset with first part and push others
      const firstPart = parts.shift()!;
      Object.assign(geoset, firstPart);
      mdl.geosets.push(...parts);
    } else {
      // Within limit
      geoset.matrices = matrices;
    }
  });

  mdl.sync();

  return this;
}