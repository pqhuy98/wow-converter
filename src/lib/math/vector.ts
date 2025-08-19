import { EulerRotation, QuaternionRotation, Vector3 } from './common';

export class V3 {
  static all(value: number): Vector3 {
    return [value, value, value];
  }

  static sum(a: Vector3, b: Vector3): Vector3 {
    return [a[0] + b[0], a[1] + b[1], a[2] + b[2]];
  }

  static sub(a: Vector3, b: Vector3): Vector3 {
    return [a[0] - b[0], a[1] - b[1], a[2] - b[2]];
  }

  static mean(a: Vector3, b: Vector3): Vector3 {
    return [
      (a[0] + b[0]) / 2,
      (a[1] + b[1]) / 2,
      (a[2] + b[2]) / 2,
    ];
  }

  static negative(a: Vector3): Vector3 {
    return [-a[0], -a[1], -a[2]];
  }

  static scale(a: Vector3, b: number): Vector3 {
    return [a[0] * b, a[1] * b, a[2] * b];
  }

  static mul(a: Vector3, b: Vector3): Vector3 {
    return [a[0] * b[0], a[1] * b[1], a[2] * b[2]];
  }

  static rotate(v: Vector3, eulerAngleRadians: Vector3): Vector3 {
    return rotateVector(v, eulerAngleRadians);
  }

  static normalize(v: Vector3): Vector3 {
    const mag = Math.hypot(v[0], v[1], v[2]);
    return [v[0] / mag, v[1] / mag, v[2] / mag];
  }

  static min(a: Vector3, b: Vector3): Vector3 {
    return [Math.min(a[0], b[0]), Math.min(a[1], b[1]), Math.min(a[2], b[2])];
  }

  static max(a: Vector3, b: Vector3): Vector3 {
    return [Math.max(a[0], b[0]), Math.max(a[1], b[1]), Math.max(a[2], b[2])];
  }

  static distance(a: Vector3, b: Vector3): number {
    return Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2]);
  }

  static lerp(v0: Vector3, v1: Vector3, t: number): Vector3 {
    return [
      v0[0] + t * (v1[0] - v0[0]),
      v0[1] + t * (v1[1] - v0[1]),
      v0[2] + t * (v1[2] - v0[2]),
    ];
  }

  static lerpScalar(v0: number, v1: number, t: number): number {
    return v0 + t * (v1 - v0);
  }

  // eslint-disable-next-line
  static noInterp<T extends number[]>(v0: T, _v1: T, _t: number): T {
    return [...v0] as T;
  }

  static slerp(q0: QuaternionRotation, q1: QuaternionRotation, t: number): QuaternionRotation {
    // Compute the dot product (cosine of the angle between the quaternions)
    let dot = q0[0] * q1[0] + q0[1] * q1[1] + q0[2] * q1[2] + q0[3] * q1[3];
    dot = Math.max(-1.0, Math.min(1.0, dot));

    // If the dot product is negative, the quaternions have opposite handed-ness
    // and slerp won't take the shorter path. So we negate one quaternion.
    if (dot < 0.0) {
      // eslint-disable-next-line no-param-reassign
      q1 = [-q1[0], -q1[1], -q1[2], -q1[3]];
      dot = -dot;
    }

    const DOT_THRESHOLD = 0.9995;
    if (dot > DOT_THRESHOLD) {
      // If the quaternions are nearly parallel, use linear interpolation
      // to avoid division by 0
      const result = [
        q0[0] + t * (q1[0] - q0[0]),
        q0[1] + t * (q1[1] - q0[1]),
        q0[2] + t * (q1[2] - q0[2]),
        q0[3] + t * (q1[3] - q0[3]),
      ];
      // Normalize the result
      const mag = Math.sqrt(result[0] * result[0] + result[1] * result[1] + result[2] * result[2] + result[3] * result[3]);
      return [result[0] / mag, result[1] / mag, result[2] / mag, result[3] / mag];
    }

    // Calculate the angle between the quaternions
    const theta_0 = Math.acos(dot); // θ₀ = angle between input quaternions
    const theta = theta_0 * t; // theta = angle between q0 and the result
    const sin_theta = Math.sin(theta); // Compute this once
    const sin_theta_0 = Math.sin(theta_0); // Compute this once

    const s0 = Math.cos(theta) - dot * sin_theta / sin_theta_0; // == sin(theta_0 - theta) / sin(theta_0)
    const s1 = sin_theta / sin_theta_0;

    return [
      (s0 * q0[0]) + (s1 * q1[0]),
      (s0 * q0[1]) + (s1 * q1[1]),
      (s0 * q0[2]) + (s1 * q1[2]),
      (s0 * q0[3]) + (s1 * q1[3]),
    ];
  }
}

export function rotateVector(
  position: Vector3,
  angle: EulerRotation,
): Vector3 {
  const [x, y, z] = position;
  const rx = angle[0];
  const ry = angle[1];
  const rz = angle[2];

  // Rotation matrices
  const R_x = [
    [1, 0, 0],
    [0, Math.cos(rx), -Math.sin(rx)],
    [0, Math.sin(rx), Math.cos(rx)],
  ];

  const R_y = [
    [Math.cos(ry), 0, Math.sin(ry)],
    [0, 1, 0],
    [-Math.sin(ry), 0, Math.cos(ry)],
  ];

  const R_z = [
    [Math.cos(rz), -Math.sin(rz), 0],
    [Math.sin(rz), Math.cos(rz), 0],
    [0, 0, 1],
  ];

  // Helper function to multiply matrices
  function multiplyMatrixVector(matrix: number[][], vector: Vector3): Vector3 {
    return [
      matrix[0][0] * vector[0] + matrix[0][1] * vector[1] + matrix[0][2] * vector[2],
      matrix[1][0] * vector[0] + matrix[1][1] * vector[1] + matrix[1][2] * vector[2],
      matrix[2][0] * vector[0] + matrix[2][1] * vector[1] + matrix[2][2] * vector[2],
    ];
  }

  // Combine rotations
  const intermediateVector1 = multiplyMatrixVector(R_x, [x, y, z]);
  const intermediateVector2 = multiplyMatrixVector(R_y, intermediateVector1);
  const rotatedVector = multiplyMatrixVector(R_z, intermediateVector2);

  return rotatedVector;
}
