import { QuaternionRotation, Vector3 } from '@/lib/math/common';

import { Animation, animationToString } from './animation';
import { f, fVector } from './formatter';
import { Geoset, GeosetAnim } from './geoset';
import { Sequence } from './sequence';

export enum NodeFlag {
  DONTINHERIT_TRANSLATION = 'DontInherit { Translation },',
  DONTINHERIT_SCALING = 'DontInherit { Scaling },',
  DONTINHERIT_ROTATION = 'DontInherit { Rotation },',
  BILLBOARDED = 'Billboarded,',
  BILLBOARD_LOCK_X = 'BillboardedLockX,',
  BILLBOARD_LOCK_Y = 'BillboardedLockY,',
  BILLBOARD_LOCK_Z = 'BillboardedLockZ,',
}

export interface IdObject {
  name: string;
  objectId?: number;
  pivotPoint: Vector3;
}

export interface Node extends IdObject {
  parent?: Node;
  flags: NodeFlag[];
  translation?: Animation<Vector3>;
  scaling?: Animation<Vector3>;
  rotation?: Animation<QuaternionRotation>;
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

export function bonesToString(bones: Bone[]): string {
  return bones.map((bone) => `
    Bone "${bone.name}" {
      ObjectId ${bone.objectId},

      ${bone.parent != null ? `Parent ${bone.parent.objectId},` : ''}

      ${bone.geoset != null ? `GeosetId ${bone.geoset === 'Multiple' ? bone.geoset : bone.geoset.id},` : ''}

      ${bone.geosetAnim != null ? `GeosetAnimId ${bone.geosetAnim.id},` : 'GeosetAnimId None,'}

      ${bone.flags.join('\n\t')}

      ${animationToString('Translation', bone.translation)}

      ${animationToString('Rotation', bone.rotation)}

      ${animationToString('Scaling', bone.scaling)}
    }`).join('\n');
}

export function attachmentPointsToString(attachmentPoints: AttachmentPoint[]): string {
  return attachmentPoints.map((attachment) => `
    Attachment "${attachment.name}" {

      ObjectId ${attachment.objectId},

      ${attachment.parent != null ? `Parent ${attachment.parent.objectId},` : ''}

      AttachmentID ${attachment.attachmentId},

      ${animationToString('Translation', attachment.translation)}

      ${animationToString('Rotation', attachment.rotation)}

      ${animationToString('Scaling', attachment.scaling)}
    }`).join('\n');
}

export function collisionShapesToString(collisionShapes: CollisionShape[]): string {
  return collisionShapes.map((shape) => `
    CollisionShape "${shape.name}" {

      ObjectId ${shape.objectId},

      ${shape.type},

      Vertices ${shape.vertices.length} {
        ${shape.vertices.map((v) => `{ ${fVector(v)} },`).join('\n')}
      }
  
      BoundsRadius ${f(shape.boundRadius)},
    }`).join('\n');
}

export function eventObjectsToString(eventObjects: EventObject[]): string {
  eventObjects.forEach((e) => e.track.sort((a, b) => a.sequence.interval[0] - b.sequence.interval[0]));
  return eventObjects.map((event) => `
    EventObject "${event.name}" {

      ObjectId ${event.objectId},

      EventTrack ${event.track.length} {
        ${event.track.map((e) => `${e.sequence.interval[0] + e.offset},`).join('\n')}
      }
    }`).join('\n');
}

export function pivotPointsToString(objects: IdObject[]): string {
  return `
    PivotPoints ${objects.length} {
      ${objects.map(({ pivotPoint }) => `{ ${fVector(pivotPoint)} },`).join('\n')}
    }`;
}
