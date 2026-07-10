import { QuaternionRotation, Vector3 } from '@/lib/math/common';

import { Animation, animationToString } from './animation';
import { f, fVector } from './formatter';

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
      FieldOfView ${formatCameraFloat(cam.fieldOfView)},
      FarClip ${formatCameraClipFloat(cam.farClip)},
      NearClip ${formatCameraClipFloat(cam.nearClip)},
      Target {
        Position { ${fVector(cam.target.position)} },
      }
      ${animationToString('Translation', cam.translation)}
      ${animationToString('Rotation', cam.rotation)}
      ${animationToString('Scaling', cam.scaling)}
    }`).join('\n');
}

function formatCameraFloat(value: number): string {
  return f(value);
}

function formatCameraClipFloat(value: number): string {
  return formatCameraFloat(nextAfterTowardZero(value));
}

function nextAfterTowardZero(value: number): number {
  if (value === 0 || Number.isNaN(value)) return value;

  const view = new DataView(new ArrayBuffer(8));
  view.setFloat64(0, value, true);
  const bits = view.getBigUint64(0, true);
  view.setBigUint64(0, bits - 1n, true);
  return view.getFloat64(0, true);
}
