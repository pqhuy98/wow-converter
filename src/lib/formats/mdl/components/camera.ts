import { QuaternionRotation, Vector3 } from '@/lib/math/common';

import { Animation, animationToString } from './animation';
import { fVector } from './formatter';

export interface Camera {
  name: string
  position: Vector3
  fieldOfView: number
  farClip: number
  nearClip: number
  target: {
    position: Vector3
  }
  translation?: Animation<Vector3>;
  scaling?: Animation<Vector3>;
  rotation?: Animation<QuaternionRotation>;
}

export function camerasToString(cameras: Camera[]): string {
  return cameras.map((cam) => `
    Camera "${cam.name}" {
      Position { ${fVector(cam.position)} },
      FieldOfView ${cam.fieldOfView},
      FarClip ${cam.farClip},
      NearClip ${cam.nearClip},
      Target {
        Position { ${fVector(cam.target.position)} },
      }
      ${animationToString('Translation', cam.translation)}
      ${animationToString('Rotation', cam.rotation)}
      ${animationToString('Scaling', cam.scaling)}
    }`).join('\n');
}
