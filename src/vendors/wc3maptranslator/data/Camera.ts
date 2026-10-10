import { type angle } from '../CommonInterfaces';

interface Camera {
  dofDistance?: number
  dofScale?: number
  posAbsoluteZ?: number
  cameraType?: number
  target: CameraTarget
  offsetZ: number
  rotation: angle
  aoa: angle // angle of attack
  distance: number
  roll: number
  fov: angle // field of view
  farClipping: number
  nearClipping: number
  localPitch: number
  localYaw: number
  localRoll: number
  name: string
}

interface CameraTarget {
  x: number
  y: number
}

export type { Camera, CameraTarget };
