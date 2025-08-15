import { parsers } from '@pqhuy98/mdx-m3-viewer';

import { Vector3 } from '../../math/common';
import { Animation } from './components/animation';
import { Camera, camerasToString } from './components/camera';
import { Bound } from './components/extent';
import { f } from './components/formatter';
import {
  Geoset, GeosetAnim, geosetAnimsToString, geosetsToString,
} from './components/geoset';
import { GlobalSequence, globalSequencesToString } from './components/global-sequence';
import { Material, materialsToString } from './components/material';
import {
  AttachmentPoint, attachmentPointsToString, Bone, bonesToString, CollisionShape, collisionShapesToString, EventObject, eventObjectsToString, pivotPointsToString,
} from './components/node';
import { Sequence, sequencesToString } from './components/sequence';
import { Texture, texturesToString } from './components/texture';
import { TextureAnim, textureAnimsToString } from './components/texture-anim';
import { MDLModify } from './modify';

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
  } & Bound;

  globalSequences: GlobalSequence[] = [];

  sequences: Sequence[] = [];

  textures: Texture[] = [];

  materials: Material[] = [];

  textureAnims: TextureAnim[] = [];

  geosets: Geoset[] = [];

  geosetAnims: GeosetAnim[] = [];

  bones: Bone[] = [];

  attachments: AttachmentPoint[] = [];

  cameras: Camera[] = [];

  eventObjects: EventObject[] = [];

  collisionShapes: CollisionShape[] = [];

  modify: MDLModify;

  boundsOverriden?: (obj: Bound) => void;

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

  private versionToString() {
    return `
      Version {
        FormatVersion ${this.version.formatVersion},
      }`;
  }

  private modelToString() {
    return `Model "${this.model.name}" {
      NumGeosets ${this.geosets.length},
      NumBones ${this.bones.length},
      NumAttachments ${this.attachments.length},
      BlendTime ${this.model.blendTime},
      MinimumExtent { ${this.model.minimumExtent.map(f).join(', ')} },
      MaximumExtent { ${this.model.maximumExtent.map(f).join(', ')} },
      BoundsRadius ${f(this.model.boundsRadius)},
    }`;
  }

  getNodes() {
    return [
      ...this.bones,
      ...this.attachments,
      ...this.eventObjects,
      ...this.collisionShapes,
    ];
  }

  getAnimated(): Animation<number[] | number>[] {
    return [
      ...this.getNodes().flatMap((node) => [node.translation, node.rotation, node.scaling]),
      ...this.cameras.flatMap((cam) => [cam.translation, cam.rotation, cam.scaling]),
      ...this.textureAnims.flatMap((texAnim) => [texAnim.translation, texAnim.rotation, texAnim.scaling]),
      ...this.materials.flatMap((mat) => mat.layers.flatMap((layer) => [
        layer.alpha && 'keyFrames' in layer.alpha ? layer.alpha : null,
        layer.tvertexAnim?.translation,
        layer.tvertexAnim?.rotation,
        layer.tvertexAnim?.scaling,
      ])),
      ...this.geosetAnims.flatMap((geosetAnim) => [
        geosetAnim.alpha && 'keyFrames' in geosetAnim.alpha ? geosetAnim.alpha : null,
        geosetAnim.color && 'keyFrames' in geosetAnim.color ? geosetAnim.color : null,
      ]),
    ].filter((anim) => anim != null);
  }

  private updateIds() {
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
      ...this.attachments,
      ...this.eventObjects,
      ...this.collisionShapes,
    ].forEach((node, i) => node.objectId = i);
    this.attachments.forEach((p, i) => p.attachmentId = i);
  }

  private toString() {
    this.updateIds();

    // Override extends before write to String
    if (this.boundsOverriden) {
      this.boundsOverriden(this.model);
      this.geosets.forEach((geoset) => {
        this.boundsOverriden!(geoset);
      });
      this.sequences.forEach((seq) => {
        this.boundsOverriden!(seq);
      });
    }

    let result = `// Exported by Huy's wow-converter
      ${this.versionToString()}
      ${this.modelToString()}
      ${sequencesToString(this.sequences)}
      ${globalSequencesToString(this.globalSequences)}
      ${texturesToString(this.textures)}
      ${materialsToString(this.version.formatVersion, this.materials)}
      ${textureAnimsToString(this.textureAnims)}
      ${geosetsToString(this.version.formatVersion, this.geosets, this.bones, this.sequences)}
      ${geosetAnimsToString(this.geosetAnims)}
      ${bonesToString(this.bones)}
      ${attachmentPointsToString(this.attachments)}
      ${camerasToString(this.cameras)}
      ${eventObjectsToString(this.eventObjects)}
      ${collisionShapesToString(this.collisionShapes)}
      ${pivotPointsToString([
    ...this.bones,
    ...this.attachments,
    ...this.eventObjects,
    ...this.collisionShapes,
  ])}
    `;

    let depth = 0;
    result = result.split('\n')
      .map((l) => l.trim())
      .filter((l) => l.length > 0)
      .map((l) => {
        if (l.endsWith('}')) {
          depth--;
        }
        try {
          const newLine = Array(depth).fill('\t').join('') + l;
          if (l.endsWith('{')) {
            depth++;
          }
          return newLine;
        } catch (e) {
          console.error({ depth });
          return l;
        }
      })
      .join('\n');

    // Restore
    if (this.boundsOverriden) {
      this.syncExtents();
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
    return this.toString();
  }

  sync() {
    this.syncExtents();

    // Update global sequence durations to max of all key-frames timestamps
    const updateGlobalSeq = (globalSeq?: GlobalSequence, ...values: number[]) => {
      if (globalSeq) {
        globalSeq.duration = Math.max(globalSeq.duration, ...values);
      }
    };
    this.textureAnims.forEach((texAnim) => {
      updateGlobalSeq(texAnim.translation?.globalSeq, ...texAnim.translation?.keyFrames.keys() ?? []);
      updateGlobalSeq(texAnim.rotation?.globalSeq, ...texAnim.rotation?.keyFrames.keys() ?? []);
      updateGlobalSeq(texAnim.scaling?.globalSeq, ...texAnim.scaling?.keyFrames.keys() ?? []);
    });
    this.bones.forEach((bone) => {
      updateGlobalSeq(bone.translation?.globalSeq, ...bone.translation?.keyFrames.keys() ?? []);
      updateGlobalSeq(bone.rotation?.globalSeq, ...bone.rotation?.keyFrames.keys() ?? []);
      updateGlobalSeq(bone.scaling?.globalSeq, ...bone.scaling?.keyFrames.keys() ?? []);
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
          twoSided: false,
          layers: [
            {
              filterMode: 'None',
              texture: this.textures[0],
              unshaded: false,
              sphereEnvMap: false,
              twoSided: false,
              unfogged: false,
              unlit: false,
              noDepthTest: false,
              noDepthSet: false,
              alpha: {
                static: true,
                value: 1,
              },
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
  }

  syncExtents() {
    // Remove empty geosets
    this.geosets = this.geosets.filter((geoset) => geoset.vertices.length > 0 && geoset.faces.length > 0);

    // Compute geoset's bound
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

    // Compute model's bound
    if (this.geosets.length > 0) {
      this.model.minimumExtent = this.geosets.map((geoset) => geoset.minimumExtent).reduce((a, b) => [Math.min(a[0], b[0]), Math.min(a[1], b[1]), Math.min(a[2], b[2])]);
      this.model.maximumExtent = this.geosets.map((geoset) => geoset.maximumExtent).reduce((a, b) => [Math.max(a[0], b[0]), Math.max(a[1], b[1]), Math.max(a[2], b[2])]);
      this.model.boundsRadius = this.geosets.reduce((a, b) => Math.max(a, b.boundsRadius), 0);
    }

    // Update sequence's bound
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
