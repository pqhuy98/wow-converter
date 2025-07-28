import { Vector2, Vector3 } from './common';

/**
 * @returns slope angle in degree
 */
export function calculateTriangleSlope([A, B, C]: Vector3[]): number {
  // Calculate vectors AB and AC
  const AB = [B[0] - A[0], B[1] - A[1], B[2] - A[2]];
  const AC = [C[0] - A[0], C[1] - A[1], C[2] - A[2]];

  // Calculate the cross product AB x AC
  const crossProduct = [
    AB[1] * AC[2] - AB[2] * AC[1], // n_x
    AB[2] * AC[0] - AB[0] * AC[2], // n_y
    AB[0] * AC[1] - AB[1] * AC[0], // n_z
  ];

  // Length of the normal vector
  const normalLength = Math.sqrt(
    crossProduct[0] ** 2 + crossProduct[1] ** 2 + crossProduct[2] ** 2,
  );

  // Angle with respect to z-axis (vertical direction)
  const cosTheta = crossProduct[2] / normalLength;

  // Convert angle from radians to degrees
  const angleInRadians = Math.acos(cosTheta);
  const angleInDegrees = angleInRadians * (180 / Math.PI);

  return angleInDegrees;
}

export function getZProjectionOfXyInTriangle(v1: Vector3, v2: Vector3, v3: Vector3, x: number, y: number): number {
  // Create vectors from v1 to v2 and v1 to v3
  const vectorA = {
    x: v2[0] - v1[0],
    y: v2[1] - v1[1],
    z: v2[2] - v1[2],
  };

  const vectorB = {
    x: v3[0] - v1[0],
    y: v3[1] - v1[1],
    z: v3[2] - v1[2],
  };

  // Compute the normal vector (a, b, c) via cross product of vectorA and vectorB
  const normal = {
    a: vectorA.y * vectorB.z - vectorA.z * vectorB.y,
    b: vectorA.z * vectorB.x - vectorA.x * vectorB.z,
    c: vectorA.x * vectorB.y - vectorA.y * vectorB.x,
  };

  // Check if the plane is vertical (c === 0)
  if (normal.c === 0) {
    throw new Error('The plane is vertical relative to the Z-axis; Z cannot be uniquely determined.');
  }

  // Calculate d using the plane equation: a*x + b*y + c*z + d = 0 => d = - (a*x0 + b*y0 + c*z0)
  const d = -(normal.a * v1[0] + normal.b * v1[1] + normal.c * v1[2]);

  // Solve for z: z = (-a*x - b*y - d) / c
  const z = (-normal.a * x - normal.b * y - d) / normal.c;

  return z;
}

// Function to calculate the area of the triangle using vertices
export function triangleArea(A: Vector2, B: Vector2, C: Vector2): number {
  return Math.abs((A[0] * (B[1] - C[1]) + B[0] * (C[1] - A[1]) + C[0] * (A[1] - B[1])) / 2.0);
}

// Function to check if a point P is inside the triangle ABC
export function isInsideTriangle(A: Vector2, B: Vector2, C: Vector2, P: Vector2): boolean {
  const fullArea = triangleArea(A, B, C);
  const area1 = triangleArea(P, B, C);
  const area2 = triangleArea(A, P, C);
  const area3 = triangleArea(A, B, P);

  // Check if the sum of P's area with the sides of the triangle equals the full area
  return Math.abs(fullArea - (area1 + area2 + area3)) < 1;
}

// Function to find integer points inside the triangle
export function findIntegerPointsInTriangle(A: Vector2, B: Vector2, C: Vector2): Vector2[] {
  const points: Vector2[] = [];
  // Determine the bounding box of the triangle
  const minX = Math.min(A[0], B[0], C[0]);
  const maxX = Math.max(A[0], B[0], C[0]);
  const minY = Math.min(A[1], B[1], C[1]);
  const maxY = Math.max(A[1], B[1], C[1]);

  // Iterate over the bounding box and check each point
  for (let x = Math.floor(minX); x <= Math.ceil(maxX); x++) {
    for (let y = Math.floor(minY); y <= Math.ceil(maxY); y++) {
      const point: Vector2 = [x, y];
      if (isInsideTriangle(A, B, C, point)) {
        points.push(point);
      }
    }
  }

  return points;
}
