import { Vector3 } from '@/lib/math/common';

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
    }`).join('\n');
}
