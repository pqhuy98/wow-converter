import _ from 'lodash';
import { parsers } from 'mdx-m3-viewer';

import { QuaternionRotation, Vector2, Vector3 } from '../../math/common';
import { sortMapByKeyAsc } from '../../math/utils';
import { SequenceData } from '../animation/animation_mapper';
import { MDLModify } from './mdl-modify';

export type Interpolation = 'Linear' | 'DontInterp'
export function wowToWc3Interpolation(wowInterpolation: number): Interpolation {
  return wowInterpolation === 1 ? 'Linear' : 'DontInterp';
}

export type BlendMode = 'None' | 'Transparent' | 'Blend' | 'Additive' | 'AddAlpha' | 'Modulate' | 'Modulate2x'
export function m2BlendModeToWc3FilterMode(m2BlendMode: number): BlendMode {
  switch (m2BlendMode) {
    // https://wowdev.wiki/M2/Rendering#M2BLEND
    case 0: // GxBlend_Opaque
      return 'None';
    case 1: // GxBlend_AlphaKey
      return 'Transparent';
    case 2: // GxBlend_Alpha
      return 'Blend';
    case 3: // GxBlend_NoAlphaAdd
      return 'Blend';
    case 4: // GxBlend_Add
      return 'Additive';
    case 5: // GxBlend_Mod
      return 'Modulate';
    case 6: // GxBlend_Mod2x
      return 'Modulate2x';
    case 7: // GxBlend_BlendAdd
      return 'Additive';
    default:
      throw new Error('Unknown blend mode');
  }
}

export enum NodeFlag {
  DONTINHERIT_TRANSLATION = 'DontInherit { Translation },',
  DONTINHERIT_SCALING = 'DontInherit { Scaling },',
  DONTINHERIT_ROTATION = 'DontInherit { Rotation },',
  BILLBOARDED = 'Billboarded,',
  BILLBOARD_LOCK_X = 'BillboardedLockX,',
  BILLBOARD_LOCK_Y = 'BillboardedLockY,',
  BILLBOARD_LOCK_Z = 'BillboardedLockZ,',
}

export interface Extents {
  minimumExtent: Vector3;
  maximumExtent: Vector3;
  boundsRadius: number;
}

export interface GlobalSequence {
  id: number
  duration: number;
}

export interface Sequence extends Extents {
  name: string;
  interval: [number, number];
  nonLooping: boolean;
  movementSpeed: number,
  data: SequenceData,
  rarity?: number
  keep?: boolean;
}

export interface Texture {
  id: number
  image: string;
  wrapWidth: boolean;
  wrapHeight: boolean;
}

export interface Material {
  id: number
  constantColor: boolean;
  layers: {
    filterMode: BlendMode;
    texture: Texture;
    twoSided: boolean;
    unfogged: boolean;
    unlit: boolean;
    noDepthTest: boolean;
    noDepthSet: boolean;
    tvertexAnim?: TextureAnim;
  }[];
}

export interface TransformAnimation<T> {
  interpolation: Interpolation;
  globalSeq?: GlobalSequence;
  keyFrames: Map<number, T>;
}

export interface TextureAnim {
  id: number
  translation?: TransformAnimation<Vector3>;
  scaling?: TransformAnimation<Vector3>;
  rotation?: TransformAnimation<QuaternionRotation>;
}

export interface GeosetVertex {
  id: number
  position: Vector3
  normal: Vector3
  texPosition: Vector2
  matrix?: Matrix
  skinWeights?: SkinWeight[]
}

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

export interface Geoset extends Extents {
  id: number
  name: string
  vertices: GeosetVertex[];
  faces: Face[];
  material: Material;
  matrices: Matrix[]
  selectionGroup: number;
}

export type AnimatedValue<T> = {
  static: true;
  value: T;
} | {
  interpolation: Interpolation;
  keyFrames: Map<number, T>;
};

export interface GeosetAnim {
  id: number
  geoset: Geoset;
  alpha?: AnimatedValue<number>;
  color?: AnimatedValue<Vector3>;
}

export interface IdObject {
  name: string;
  objectId?: number;
  pivotPoint: Vector3;
}

export interface Node extends IdObject {
  parent?: Node;
  flags: NodeFlag[];
  translation?: TransformAnimation<Vector3>;
  scaling?: TransformAnimation<Vector3>;
  rotation?: TransformAnimation<QuaternionRotation>;
}

export interface Bone extends Node {
  type: 'Bone'
  geoset?: Geoset | 'Multiple';
  geosetAnim?: GeosetAnim;
}

export interface AttachmentPoint extends Node {
  type: 'AttachmentPoint'
  attachmentId: number;
}

export interface EventObject extends IdObject {
  track: {sequence: Sequence, offset: number}[] // which sequence, and duration offset from sequence's start time
}

export interface CollisionShape extends IdObject {
  type: 'Sphere' | 'Cylinder'
  vertices: Vector3[]
  boundRadius: number
}

export interface Camera {
  name: string
  position: Vector3
  fieldOfView: number
  farClip: number
  nearClip: number
  target: {
    position: Vector3
  }
}
export interface WowAttachment {
  wowAttachmentId: number;
  bone: Bone;
  pivotPoint: Vector3;
}

export class MDL {
  version: {
    formatVersion: number;
  };

  model: {
    name: string;
    blendTime: number;
  } & Extents;

  globalSequences: GlobalSequence[] = [];

  sequences: Sequence[] = [];

  textures: Texture[] = [];

  materials: Material[] = [];

  textureAnims: TextureAnim[] = [];

  geosets: Geoset[] = [];

  geosetAnims: GeosetAnim[] = [];

  bones: Bone[] = [];

  attachmentPoints: AttachmentPoint[] = [];

  cameras: Camera[] = [];

  eventObjects: EventObject[] = [];

  collisionShapes: CollisionShape[] = [];

  modify: MDLModify;

  extendsOverriden?: (obj: Extents) => void;

  wowAttachments: WowAttachment[] = [];

  constructor(props: {formatVersion: number, name: string}) {
    this.version = { formatVersion: props.formatVersion };
    this.model = {
      name: props.name,
      blendTime: 150,
      minimumExtent: [0, 0, 0],
      maximumExtent: [0, 0, 0],
      boundsRadius: 0,
    };
    this.modify = new MDLModify(this);
  }

  versionToString() {
    return `
Version {
  FormatVersion ${this.version.formatVersion},
}`;
  }

  modelToString() {
    return `Model "${this.model.name}" {
  NumGeosets ${this.geosets.length},
  NumBones ${this.bones.length},
  NumAttachments ${this.attachmentPoints.length},
  BlendTime ${this.model.blendTime},
  MinimumExtent { ${this.model.minimumExtent.map(f).join(', ')} },
  MaximumExtent { ${this.model.maximumExtent.map(f).join(', ')} },
  BoundsRadius ${f(this.model.boundsRadius)},
}`;
  }

  globalSequencesToString() {
    if (this.globalSequences.length === 0) return '';
    return `GlobalSequences ${this.globalSequences.length} {
${this.globalSequences.map((gs) => `\tDuration ${gs.duration},`).join('\n')}
}`;
  }

  sequencesToString() {
    if (this.sequences.length === 0) return '';

    // Add number suffix to sequences with same name
    const animNameCount = new Map<string, number>();
    const seqName = new Map<Sequence, string>();
    this.sequences.forEach((seq) => {
      animNameCount.set(seq.name, (animNameCount.get(seq.name) ?? 0) + 1);
      seqName.set(seq, `${seq.name} ${animNameCount.get(seq.name)}`);
    });

    return `Sequences ${this.sequences.length} {
${this.sequences.map((sequence) => `
  Anim "${seqName.get(sequence)}" {
    Interval { ${sequence.interval[0]}, ${sequence.interval[1]} },
    ${sequence.nonLooping ? 'NonLooping,' : ''}
    ${sequence.movementSpeed > 0 ? `MoveSpeed ${sequence.movementSpeed},` : ''}
    ${(sequence.rarity ?? 0) > 0 ? `Rarity ${sequence.rarity},` : ''}
    MinimumExtent { ${sequence.minimumExtent.map(f).join(', ')} },
    MaximumExtent { ${sequence.maximumExtent.map(f).join(', ')} },
    BoundsRadius ${f(sequence.boundsRadius)},
  }`).join('\n')}
}`;
  }

  texturesToString() {
    if (this.textures.length === 0) return '';
    return `Textures ${this.textures.length} {
${this.textures.map((texture) => `
  Bitmap {
    Image "${texture.image}",
    ${texture.wrapWidth ? 'WrapWidth,' : ''}
    ${texture.wrapHeight ? 'WrapHeight,' : ''}
  }`).join('\n')}
}`;
  }

  materialsToString() {
    if (this.materials.length === 0) return '';
    return `Materials ${this.materials.length} {
${this.materials.map((material) => `
  Material {
    ${material.constantColor ? 'ConstantColor,' : ''}
    ${material.layers.map((layer) => `
    Layer {
      FilterMode ${layer.filterMode},
      static TextureID ${layer.texture.id},
      ${layer.twoSided ? 'TwoSided,' : ''}
      ${layer.unfogged ? 'Unfogged,' : ''}
      ${layer.noDepthTest ? 'NoDepthTest,' : ''}
      ${layer.noDepthSet ? 'NoDepthSet,' : ''}
      ${layer.unlit ? 'Unlit,' : ''}
      ${layer.tvertexAnim != null ? `TVertexAnimId ${layer.tvertexAnim.id},` : ''}
    }`).join('\n')}
  }`).join('\n')}
}`;
  }

  textureAnimsToString() {
    if (this.textureAnims.length === 0) return '';
    return `TextureAnims ${this.textureAnims.length} {
${this.textureAnims.map((texAnim) => `\tTVertexAnim {
      ${texAnim.translation && texAnim.translation.keyFrames.size > 0 ? `
    Translation ${[...texAnim.translation.keyFrames.keys()].length} {
      ${texAnim.translation.interpolation},
      ${texAnim.translation.globalSeq != null ? `GlobalSeqId ${texAnim.translation.globalSeq.id},` : ''}
      ${[...sortMapByKeyAsc(texAnim.translation.keyFrames).entries()].map(([timestamp, translation]) => `
      ${timestamp}: { ${translation.map(f).join(', ')} },`).join('\n')}
    }` : ''}
    
    ${texAnim.rotation && texAnim.rotation.keyFrames.size > 0 ? `
    Rotation ${[...texAnim.rotation.keyFrames.keys()].length} {
      ${texAnim.rotation.interpolation},
      ${texAnim.rotation.globalSeq != null ? `GlobalSeqId ${texAnim.rotation.globalSeq.id},` : ''}
      ${[...sortMapByKeyAsc(texAnim.rotation.keyFrames).entries()].map(([timestamp, rotations]) => `
      ${timestamp}: { ${rotations.map(f).join(', ')} },`).join('\n')}
    }` : ''}
  
    ${texAnim.scaling && texAnim.scaling.keyFrames.size > 0 ? `
    Scaling ${[...texAnim.scaling.keyFrames.keys()].length} {
      ${texAnim.scaling.interpolation},
      ${texAnim.scaling.globalSeq != null ? `GlobalSeqId ${texAnim.scaling.globalSeq.id},` : ''}
      ${[...sortMapByKeyAsc(texAnim.scaling.keyFrames).entries()].map(([timestamp, scaling]) => `
      ${timestamp}: { ${scaling.map(f).join(', ')} },`).join('\n')}
    }` : ''}
  }`).join('\n')}
}`;
  }

  geosetsToString() {
    const getSkinWeight = (vertex: GeosetVertex) => {
      const boneIndices = Array(4).fill(0).map((__, i) => (vertex.skinWeights![i] ? vertex.skinWeights![i].bone.objectId : 0));
      const weights = Array(4).fill(0).map((__, i) => (vertex.skinWeights![i] ? vertex.skinWeights![i].weight : 0));
      return `\t\t${[...boneIndices, ...weights.map(Math.round)].join(', ')},`;
    };

    return this.geosets.map((geoset) => {
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

      return `
Geoset {
  Vertices ${geoset.vertices.length} {
${geoset.vertices.map((vertex) => `		{ ${f(vertex.position[0])}, ${f(vertex.position[1])}, ${f(vertex.position[2])} },`).join('\n')}
  }
  Normals ${geoset.vertices.length} {
${geoset.vertices.map((vertex) => `		{ ${f(vertex.normal[0])}, ${f(vertex.normal[1])}, ${f(vertex.normal[2])} },`).join('\n')}
  }
  TVertices ${geoset.vertices.length} {
${geoset.vertices.map((vertex) => `		{ ${f(vertex.texPosition[0])}, ${f(vertex.texPosition[1])} },`).join('\n')}
  }
  ${useVertexGroup ? `VertexGroup {
${geoset.vertices.map((v) => `\t\t${v.matrix!.id},`).join('\n')}
  }` : ''}
  ${useSkinWeights ? `Tangents ${geoset.vertices.length} {
${geoset.vertices.map((v) => `\t\t{ ${v.normal.map(f).join(', ')}, ${f(Math.sign(Math.abs(_.sum(v.normal))))} },`).join('\n')}
  }
  SkinWeights ${geoset.vertices.length} {
${geoset.vertices.map(getSkinWeight).join('\n')}
  }
  Groups ${this.bones.length} ${this.bones.length} {
${this.bones.map((bone) => `\t\tMatrices { ${bone.objectId} },`).join('\n')}
  }
  ` : `
  Groups ${geoset.matrices.length} ${_.sum(geoset.matrices.flatMap((matrix) => matrix.bones.length))} {
${geoset.matrices.map((matrix) => `\t\tMatrices { ${matrix.bones.map((n) => n.objectId).join(', ')} },`).join('\n')}
  }
  `}
  Faces 1 ${geoset.faces.length * 3} {
    Triangles {
      { ${geoset.faces.flatMap((face) => face.vertices.map((v) => v.id)).join(', ')} },
    }
  }
  MinimumExtent { ${geoset.minimumExtent.map(f).join(', ')} },
  MaximumExtent { ${geoset.maximumExtent.map(f).join(', ')} },
  BoundsRadius ${f(geoset.boundsRadius)},
${this.sequences.map((seq) => `\tAnim {
    MinimumExtent { ${seq.minimumExtent.map(f).join(', ')} },
    MaximumExtent { ${seq.maximumExtent.map(f).join(', ')} },
    BoundsRadius ${f(seq.boundsRadius)},
  }`).join('\n')}
  MaterialID ${geoset.material.id},
  SelectionGroup ${geoset.selectionGroup},
  LevelOfDetail 0,
  Name "${geoset.name}",
}`;
    }).join('\n');
  }

  geosetAnimsToString() {
    return this.geosetAnims.map((geosetAnim) => {
      let colorBlock = '';
      if (geosetAnim.color) {
        if ('static' in geosetAnim.color) {
          colorBlock = `static Color { ${[
            // MDL color order is blue, green, red
            geosetAnim.color.value[2],
            geosetAnim.color.value[1],
            geosetAnim.color.value[0],
          ].map(f).join(', ')} },`;
        } else {
          const color = geosetAnim.color;
          colorBlock = `Color ${color.keyFrames.size} {
    ${color.interpolation},
${[...sortMapByKeyAsc(color.keyFrames).entries()].map(([timestamp, rgb]) => `\t\t${timestamp}: {${rgb.map(f).join(', ')}},`).join('\n')}
  }`;
        }
      }
      let alphaBlock = '';
      if (geosetAnim.alpha) {
        if ('static' in geosetAnim.alpha) {
          alphaBlock = `static Alpha ${f(geosetAnim.alpha.value / 32767)},`;
        } else {
          const alpha = geosetAnim.alpha;
          alphaBlock = `Alpha ${alpha.keyFrames.size} {
    ${alpha.interpolation},
${[...sortMapByKeyAsc(alpha.keyFrames).entries()].map(([timestamp, value]) => `\t\t${timestamp}: ${f(value / 32767)},`).join('\n')}
  }`;
        }
      }
      return `GeosetAnim {
  GeosetId ${geosetAnim.geoset.id},
  ${colorBlock}
  ${alphaBlock}
}`;
    }).join('\n');
  }

  bonesToString() {
    return this.bones.map((bone) => `Bone "${bone.name}" {
  ObjectId ${bone.objectId},
  ${bone.parent != null ? `Parent ${bone.parent.objectId},` : ''}
  ${bone.geoset != null ? `GeosetId ${bone.geoset === 'Multiple' ? bone.geoset : bone.geoset.id},` : ''}
  ${bone.geosetAnim != null ? `GeosetAnimId ${bone.geosetAnim.id},` : 'GeosetAnimId None,'}
  ${bone.flags.join('\n\t')}
  ${bone.translation && bone.translation.keyFrames.size > 0 ? `
  Translation ${[...bone.translation.keyFrames.keys()].length} {
    ${bone.translation.interpolation},
    ${bone.translation.globalSeq != null ? `GlobalSeqId ${bone.translation.globalSeq.id},` : ''}
    ${[...sortMapByKeyAsc(bone.translation.keyFrames).entries()].map(([timestamp, translation]) => `
    ${timestamp}: { ${translation.map(f).join(', ')} },`).join('\n')}
  }` : ''}

  ${bone.rotation && bone.rotation.keyFrames.size > 0 ? `
  Rotation ${[...bone.rotation.keyFrames.keys()].length} {
    ${bone.rotation.interpolation},
    ${bone.rotation.globalSeq != null ? `GlobalSeqId ${bone.rotation.globalSeq.id},` : ''}
    ${[...sortMapByKeyAsc(bone.rotation.keyFrames).entries()].map(([timestamp, rotations]) => `
    ${timestamp}: { ${rotations.map(f).join(', ')} },`).join('\n')}
  }` : ''}

  ${bone.scaling && bone.scaling.keyFrames.size > 0 ? `
  Scaling ${[...bone.scaling.keyFrames.keys()].length} {
    ${bone.scaling.interpolation},
    ${bone.scaling.globalSeq != null ? `GlobalSeqId ${bone.scaling.globalSeq.id},` : ''}
    ${[...sortMapByKeyAsc(bone.scaling.keyFrames).entries()].map(([timestamp, scaling]) => `
    ${timestamp}: { ${scaling.map(f).join(', ')} },`).join('\n')}
  }` : ''}
}`).join('\n');
  }

  attachmentPointsToString() {
    return this.attachmentPoints.map((attachment) => `Attachment "${attachment.name}" {
  ObjectId ${attachment.objectId},
  ${attachment.parent != null ? `Parent ${attachment.parent.objectId},` : ''}
  AttachmentID ${attachment.attachmentId},
}`).join('\n');
  }

  pivotPointsToString() {
    return `PivotPoints ${this.bones.length + this.attachmentPoints.length + this.eventObjects.length + this.collisionShapes.length} {
${[...this.bones, ...this.attachmentPoints, ...this.eventObjects, ...this.collisionShapes]
    .map(({ pivotPoint }) => `	{ ${f(pivotPoint[0])}, ${f(pivotPoint[1])}, ${f(pivotPoint[2])} },`).join('\n')}
}`;
  }

  collisionShapesToString() {
    return this.collisionShapes.map((shape) => `CollisionShape "${shape.name}" {
  ObjectId ${shape.objectId},
  ${shape.type},
  Vertices ${shape.vertices.length} {
${shape.vertices.map((v) => `\t\t{ ${v.map(f).join(', ')} },`).join('\n')}
  }
  BoundsRadius ${f(shape.boundRadius)},
}`).join('\n');
  }

  eventObjectsToString() {
    this.eventObjects.forEach((e) => e.track.sort((a, b) => a.sequence.interval[0] - b.sequence.interval[1]));
    return this.eventObjects.map((event) => `EventObject "${event.name}" {
  ObjectId ${event.objectId},
  EventTrack ${event.track.length} {
${event.track.map((e) => `\t\t${e.sequence.interval[0] + e.offset},`).join('\n')}
  }
}`).join('\n');
  }

  camerasToString() {
    const res: string[] = [];
    this.cameras.forEach((cam) => {
      res.push(`Camera "${cam.name}" {`);
      res.push(`\tPosition { ${cam.position.map(f).join(', ')} },`);
      res.push(`\tFieldOfView ${cam.fieldOfView},`);
      res.push(`\tFarClip ${cam.farClip},`);
      res.push(`\tNearClip ${cam.nearClip},`);
      res.push('\tTarget {');
      res.push(`\t\tPosition { ${cam.target.position.map(f).join(', ')} },`);
      res.push('\t}');
      res.push('}');
    });
    return res.join('\n');
  }

  toString() {
    // Reindex everything with `id`
    this.globalSequences.forEach((v, i) => v.id = i);
    this.textures.forEach((v, i) => v.id = i);
    this.materials.forEach((v, i) => v.id = i);
    this.textureAnims.forEach((v, i) => v.id = i);
    this.geosetAnims.forEach((v, i) => v.id = i);
    this.geosets.forEach((geoset, i) => {
      geoset.id = i;
    });
    [
      ...this.bones,
      ...this.attachmentPoints,
      ...this.eventObjects,
      ...this.collisionShapes,
    ].forEach((node, i) => node.objectId = i);
    this.attachmentPoints.forEach((p, i) => p.attachmentId = i);

    // Override extends before write to String
    if (this.extendsOverriden) {
      this.extendsOverriden(this.model);
      this.geosets.forEach((geoset) => {
        this.extendsOverriden!(geoset);
      });
      this.sequences.forEach((seq) => {
        this.extendsOverriden!(seq);
      });
    }

    const result = `// Saved by Huy's custom OBJ-MDL converter
${this.versionToString()}
${this.modelToString()}
${this.sequencesToString()}
${this.globalSequencesToString()}
${this.texturesToString()}
${this.materialsToString()}
${this.textureAnimsToString()}
${this.geosetsToString()}
${this.geosetAnimsToString()}
${this.bonesToString()}
${this.attachmentPointsToString()}
${this.camerasToString()}
${this.eventObjectsToString()}
${this.collisionShapesToString()}
${this.pivotPointsToString()}
`.split('\n').filter((l) => l.trim().length > 0).join('\n').replaceAll('  ', '\t');

    // Restore
    if (this.extendsOverriden) {
      this.syncExtends();
    }

    return result;
  }

  toMdx() {
    const m = new parsers.mdlx.Model();
    const mdlStr = this.toString();
    m.loadMdl(mdlStr);
    return m.saveMdx();
  }

  toMdl() {
    // const m = new parsers.mdlx.Model();
    // m.loadMdx(this.toMdx());
    // return m.saveMdl();
    return this.toString();
  }

  sync() {
    this.syncExtends();

    // Update global sequence timestamps
    this.textureAnims.forEach((texAnim) => {
      if (texAnim.translation && texAnim.translation.globalSeq != null) {
        this.modify.updateGlobalSequenceDuration(texAnim.translation.globalSeq, ...texAnim.translation.keyFrames.keys());
      }
      if (texAnim.rotation && texAnim.rotation.globalSeq != null) {
        this.modify.updateGlobalSequenceDuration(texAnim.rotation.globalSeq, ...texAnim.rotation.keyFrames.keys());
      }
      if (texAnim.scaling && texAnim.scaling.globalSeq != null) {
        this.modify.updateGlobalSequenceDuration(texAnim.scaling.globalSeq, ...texAnim.scaling.keyFrames.keys());
      }
    });

    this.bones.forEach((bone) => {
      if (bone.translation?.globalSeq != null) {
        this.modify.updateGlobalSequenceDuration(bone.translation.globalSeq, ...bone.translation.keyFrames.keys());
      }
      if (bone.rotation?.globalSeq != null) {
        this.modify.updateGlobalSequenceDuration(bone.rotation.globalSeq, ...bone.rotation.keyFrames.keys());
      }
      if (bone.scaling?.globalSeq != null) {
        this.modify.updateGlobalSequenceDuration(bone.scaling.globalSeq, ...bone.scaling.keyFrames.keys());
      }
    });

    // Compute bone's GeosetId. It can be ID of one single geoset, or "Multiple" if
    // the bone is shared between multiple geosets.
    const geosetsPerBone = new Map<Bone, Set<Geoset>>();
    this.geosets.forEach((geoset) => geoset.matrices.forEach((matrix) => matrix.bones.forEach((node) => {
      if (!geosetsPerBone.has(node)) {
        geosetsPerBone.set(node, new Set());
      }
      geosetsPerBone.get(node)!.add(geoset);
    })));

    this.bones.forEach((bone) => {
      if (geosetsPerBone.has(bone)) {
        const geosets = geosetsPerBone.get(bone)!;
        bone.geoset = geosets.size > 1 ? 'Multiple' : geosets.values().next().value;
      }
    });

    // Compute simple collision shape
    this.collisionShapes = [
      {
        name: 'Collision Sphere01',
        type: 'Sphere',
        vertices: [[0, 0, this.model.boundsRadius / 2]],
        boundRadius: this.model.boundsRadius / 2,
        pivotPoint: [0, 0, 0],
      },
    ];

    // If no material is defined, create a default one
    if (this.materials.length === 0) {
      this.textures = [
        {
          id: 0,
          image: '',
          wrapWidth: false,
          wrapHeight: false,
        },
      ];
      this.materials = [
        {
          id: 0,
          layers: [
            {
              filterMode: 'None',
              texture: this.textures[0],
              twoSided: false,
              unfogged: false,
              unlit: false,
              noDepthTest: false,
              noDepthSet: false,
            },
          ],
          constantColor: false,
        },
      ];
    }

    this.geosets.forEach((geoset) => {
      if (!geoset.material) {
        geoset.material = this.materials[0];
      }
    });

    // this.materials.forEach((material) => {
    //   if (material.layers.length === 1) {
    //     material.layers.push({ ...material.layers[0] });
    //   }
    // });
  }

  syncExtends() {
    this.geosets = this.geosets.filter((geoset) => geoset.vertices.length > 0 && geoset.faces.length > 0);
    this.geosets.forEach((geoset) => {
      const min: Vector3 = [Infinity, Infinity, Infinity];
      const max: Vector3 = [-Infinity, -Infinity, -Infinity];
      geoset.vertices.forEach(({ position: [x, y, z] }) => {
        min[0] = Math.min(min[0], x);
        min[1] = Math.min(min[1], y);
        min[2] = Math.min(min[2], z);
        max[0] = Math.max(max[0], x);
        max[1] = Math.max(max[1], y);
        max[2] = Math.max(max[2], z);
      });
      geoset.minimumExtent = min;
      geoset.maximumExtent = max;
      geoset.boundsRadius = calculateBoundRadius(geoset.vertices.map((v) => v.position));
    });

    if (this.geosets.length > 0) {
      this.model.minimumExtent = this.geosets.map((geoset) => geoset.minimumExtent).reduce((a, b) => [Math.min(a[0], b[0]), Math.min(a[1], b[1]), Math.min(a[2], b[2])]);
      this.model.maximumExtent = this.geosets.map((geoset) => geoset.maximumExtent).reduce((a, b) => [Math.max(a[0], b[0]), Math.max(a[1], b[1]), Math.max(a[2], b[2])]);
      this.model.boundsRadius = this.geosets.reduce((a, b) => Math.max(a, b.boundsRadius), 0);
    }

    this.sequences.forEach((s) => {
      s.minimumExtent = [...this.model.minimumExtent];
      s.maximumExtent = [...this.model.maximumExtent];
      s.boundsRadius = this.model.boundsRadius;
    });
  }
}

function calculateBoundRadius(vertices: Vector3[]) {
  let maxDistance = 0;
  for (let i = 0; i < vertices.length; i++) {
    const [x, y, z] = vertices[i];
    const distance = Math.sqrt(x * x + y * y + z * z);
    if (distance > maxDistance) {
      maxDistance = distance;
    }
  }

  return maxDistance;
}

export function f(x?: number | null) {
  let num = x ?? 0;
  if (Math.abs(num) > 999999) {
    num = Math.sign(num) * 999999;
  }
  return parseFloat(num.toFixed(4)).toString();
}
