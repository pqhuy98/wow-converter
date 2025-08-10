import { Vector3 } from '@/lib/math/common';

export interface Bound {
  minimumExtent: Vector3;
  maximumExtent: Vector3;
  boundsRadius: number;
}
