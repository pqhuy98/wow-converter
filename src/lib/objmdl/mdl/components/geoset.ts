import _ from 'lodash';

import { Vector2, Vector3 } from '@/lib/math/common';

import { animationToString, Interpolation } from './animation';
import { Bound } from './extent';
import { f, fVector } from './formatter';
import { Material } from './material';
import { Bone } from './node';
import { Sequence } from './sequence';

export interface Face {
  vertices: [GeosetVertex, GeosetVertex, GeosetVertex];
}

export interface Matrix {
  id: number
  bones: Bone[]
}

export interface SkinWeight {
  bone: Bone,
  weight: number
}

export interface Geoset extends Bound {
  id: number
  name: string
  vertices: GeosetVertex[];
  faces: Face[];
  material: Material;
  matrices: Matrix[]
  selectionGroup: number;
}

export interface GeosetVertex {
  id: number
  position: Vector3
  normal: Vector3
  texPosition: Vector2
  matrix?: Matrix
  skinWeights?: SkinWeight[]
}

export interface GeosetAnim {
  id: number
  geoset: Geoset;
  alpha?: AnimatedValue<number>;
  color?: AnimatedValue<Vector3>;
}

export type AnimatedValue<T> = {
  static: true;
  value: T;
} | {
  interpolation: Interpolation;
  keyFrames: Map<number, T>;
};

export function geosetsToString(geosets: Geoset[], bones: Bone[], sequences: Sequence[]): string {
  const getSkinWeight = (vertex: GeosetVertex) => {
    const boneIndices = Array(4).fill(0).map((__, i) => (vertex.skinWeights![i] ? vertex.skinWeights![i].bone.objectId : 0));
    const weights = Array(4).fill(0).map((__, i) => (vertex.skinWeights![i] ? vertex.skinWeights![i].weight : 0));
    return `\t\t${[...boneIndices, ...weights.map(Math.round)].join(', ')},`;
  };

  return geosets.map((geoset) => {
    const useSkinWeights = geoset.vertices.some((v) => v.skinWeights);
    const useVertexGroup = geoset.vertices.some((v) => v.matrix);
    if (useSkinWeights === useVertexGroup) {
      throw new Error('Geoset must not use skin weight and vertex group at the same time.');
    }
    if (useSkinWeights) {
      const missingSwVert = geoset.vertices.filter((v) => !v.skinWeights);
      if (missingSwVert.length > 0) {
        throw new Error(`Geoset ${geoset.name} has ${missingSwVert.length} vertices without skin weights.`);
      }
    }
    if (useVertexGroup) {
      const missingMatVert = geoset.vertices.filter((v) => !v.matrix);
      if (missingMatVert.length > 0) {
        throw new Error(`Geoset ${geoset.name} has ${missingMatVert.length} vertices without vertex group.`);
      }
    }

    geoset.vertices.forEach((v, j) => v.id = j);
    geoset.matrices.forEach((mat, j) => mat.id = j);

    let vertexGroupBlock = '';
    if (useVertexGroup) {
      vertexGroupBlock = `
        VertexGroup {
          ${geoset.vertices.map((v) => `${v.matrix!.id},`).join('\n')}
        }
        Groups ${geoset.matrices.length} ${_.sum(geoset.matrices.flatMap((matrix) => matrix.bones.length))} {
          ${geoset.matrices.map((matrix) => `Matrices { ${matrix.bones.map((n) => n.objectId).join(', ')} },`)
    .join('\n')}
        }
      `;
    }

    let skinWeightsBlock = '';
    if (useSkinWeights) {
      const normalToTangent = (normal: Vector3) => [
        normal[0],
        normal[1],
        normal[2],
        Math.sign(Math.abs(_.sum(normal))),
      ];

      skinWeightsBlock = `
        Tangents ${geoset.vertices.length} {
          ${geoset.vertices.map((v) => `{ ${fVector(normalToTangent(v.normal))} },`).join('\n')}
        }
        SkinWeights ${geoset.vertices.length} {
          ${geoset.vertices.map(getSkinWeight).join('\n')}
        }
        Groups ${bones.length} ${bones.length} {
          ${bones.map((bone) => `Matrices { ${bone.objectId} },`).join('\n')}
        }
      `;
    }

    return `Geoset {
      Vertices ${geoset.vertices.length} {
        ${geoset.vertices.map((vertex) => `{ ${fVector(vertex.position)} },`).join('\n')}
      }

      Normals ${geoset.vertices.length} {
        ${geoset.vertices.map((vertex) => `{ ${fVector(vertex.normal)} },`).join('\n')}
      }

      TVertices ${geoset.vertices.length} {
        ${geoset.vertices.map((vertex) => `{ ${fVector(vertex.texPosition)} },`).join('\n')}
      }

      ${vertexGroupBlock}
      ${skinWeightsBlock}

      Faces 1 ${geoset.faces.length * 3} {
        Triangles {
          { ${geoset.faces.flatMap((face) => face.vertices.map((v) => v.id)).join(', ')} },
        }
      }

      MinimumExtent { ${fVector(geoset.minimumExtent)} },
      MaximumExtent { ${fVector(geoset.maximumExtent)} },
      BoundsRadius ${f(geoset.boundsRadius)},

      ${sequences.map((seq) => `Anim {
        MinimumExtent { ${fVector(seq.minimumExtent)} },
        MaximumExtent { ${fVector(seq.maximumExtent)} },
        BoundsRadius ${f(seq.boundsRadius)},
      }`).join('\n')}

      MaterialID ${geoset.material.id},
      SelectionGroup ${geoset.selectionGroup},
      LevelOfDetail 0,
      Name "${geoset.name}",
    }`;
  }).join('\n');
}

export function geosetAnimsToString(geosetAnims: GeosetAnim[]): string {
  return geosetAnims.map((geosetAnim) => {
    let colorBlock = '';
    if (geosetAnim.color) {
      if ('static' in geosetAnim.color) {
        colorBlock = `
          static Color { ${fVector([
    // MDL color order is blue, green, red
    geosetAnim.color.value[2],
    geosetAnim.color.value[1],
    geosetAnim.color.value[0],
  ])} },`;
      } else {
        const color = geosetAnim.color;
        colorBlock = animationToString('Color', color);
      }
    }

    let alphaBlock = '';
    if (geosetAnim.alpha) {
      if ('static' in geosetAnim.alpha) {
        alphaBlock = `static Alpha ${f(geosetAnim.alpha.value / 32767)},`;
      } else {
        const alpha = geosetAnim.alpha;
        alphaBlock = animationToString('Alpha', alpha);
      }
    }

    return `
    GeosetAnim {
      GeosetId ${geosetAnim.geoset.id},
      ${colorBlock}
      ${alphaBlock}
    }`;
  }).join('\n');
}
