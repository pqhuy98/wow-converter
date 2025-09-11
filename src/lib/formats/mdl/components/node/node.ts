import { QuaternionRotation, Vector3 } from '@/lib/math/common';

import { WowAttachment } from '../../mdl';
import { Animation, animationToString } from '../animation';
import { f, fVector } from '../formatter';
import { Geoset, GeosetAnim } from '../geoset';
import { Sequence } from '../sequence';

export enum NodeFlag {
  DONTINHERIT_TRANSLATION = 'DontInherit { Translation },',
  DONTINHERIT_SCALING = 'DontInherit { Scaling },',
  DONTINHERIT_ROTATION = 'DontInherit { Rotation },',
  BILLBOARDED = 'Billboarded,',
  BILLBOARD_LOCK_X = 'BillboardedLockX,',
  BILLBOARD_LOCK_Y = 'BillboardedLockY,',
  BILLBOARD_LOCK_Z = 'BillboardedLockZ,',
}

export interface Node {
  name: string;
  objectId?: number;
  pivotPoint: Vector3;
  parent?: Node;
  flags: NodeFlag[];
  translation?: Animation<Vector3>;
  scaling?: Animation<Vector3>;
  rotation?: Animation<QuaternionRotation>;
  type: string
}

export interface Bone extends Node {
  type: 'Bone'
  parent?: Bone;
  geoset?: Geoset | 'Multiple';
  geosetAnim?: GeosetAnim;
}

export interface AttachmentPoint extends Node {
  type: 'AttachmentPoint'
  path?: string;
  attachmentId: number;
  data?: {
    wowAttachment: WowAttachment;
  }
}

export interface EventObject extends Node {
  type: 'EventObject'
  track: {sequence: Sequence, offset: number}[] // which sequence, and duration offset from sequence's start time
}

export interface Helper extends Node {
  type: 'Helper'
}

export interface CollisionShape extends Node {
  type: 'Sphere' | 'Box'
  vertices: Vector3[]
  boundRadius: number
}

export function bonesToString(bones: Bone[]): string {
  return bones.map((bone) => `
    Bone "${bone.name}" {
      ${nodeHeaders(bone)}
      ${bone.geoset != null ? `GeosetId ${bone.geoset === 'Multiple' ? bone.geoset : bone.geoset.id},` : ''}
      ${bone.geosetAnim ? `GeosetAnimId ${bone.geosetAnim.id},` : 'GeosetAnimId None,'}
      ${nodeAnimations(bone)}
    }`).join('\n');
}

export function attachmentPointsToString(attachmentPoints: AttachmentPoint[]): string {
  return attachmentPoints.map((attachment) => `
    Attachment "${attachment.name}" {
      ${nodeHeaders(attachment)}
      AttachmentID ${attachment.attachmentId},
      ${attachment.path ? `Path "${attachment.path}",` : ''}
      ${nodeAnimations(attachment)}
    }`).join('\n');
}

export function collisionShapesToString(collisionShapes: CollisionShape[]): string {
  return collisionShapes.map((shape) => `
    CollisionShape "${shape.name}" {
      ${nodeHeaders(shape)}
      ${shape.type},
      BoundsRadius ${f(shape.boundRadius)},
      Vertices ${shape.vertices.length} {
        ${shape.vertices.map((v) => `{ ${fVector(v)} },`).join('\n')}
      }

      ${nodeAnimations(shape)}
    }`).join('\n');
}

export function eventObjectsToString(eventObjects: EventObject[]): string {
  eventObjects.forEach((e) => e.track.sort((a, b) => a.sequence.interval[0] - b.sequence.interval[0]));
  return eventObjects.map((event) => `
    EventObject "${event.name}" {
      ${nodeHeaders(event)}
      EventTrack ${event.track.length} {
        ${event.track.map((e) => `${e.sequence.interval[0] + e.offset},`).join('\n')}
      }
      ${nodeAnimations(event)}
    }`).join('\n');
}

export function helpersToString(helpers: Helper[]): string {
  return helpers.map((helper) => `
    Helper "${helper.name}" {
      ${nodeHeaders(helper)}
      ${nodeAnimations(helper)}
    }`).join('\n');
}

export function pivotPointsToString(nodes: Node[]): string {
  return `
    PivotPoints ${nodes.length} {
      ${nodes.map(({ pivotPoint }) => `{ ${fVector(pivotPoint)} },`).join('\n')}
    }`;
}

export function nodeHeaders(node: Node): string {
  return `
      ObjectId ${node.objectId},

      ${node.parent != null ? `Parent ${node.parent.objectId},` : ''}

      ${node.flags.join('\n')}
  `;
}

export function nodeAnimations(node: Node): string {
  return `
    ${animationToString('Translation', node.translation)}
    ${animationToString('Rotation', node.rotation)}
    ${animationToString('Scaling', node.scaling)}
  `;
}
