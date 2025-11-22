import { Face, GeosetVertex } from '../components/geoset';
import { MDLModify } from '.';

export function removeSmallFaceComponents(this: MDLModify, minComponentArea: number): MDLModify {
  const deletedFaces = new Set<Face>();

  this.mdl.geosets.forEach((geoset) => {
    if (geoset.faces.length === 0) return;

    const faces = geoset.faces;
    const faceCount = faces.length;

    const vertexToFaces = new Map<GeosetVertex, number[]>();
    for (let i = 0; i < faceCount; i++) {
      const face = faces[i];
      face.vertices.forEach((vertex) => {
        const existing = vertexToFaces.get(vertex);
        if (existing !== undefined) {
          existing.push(i);
        } else {
          vertexToFaces.set(vertex, [i]);
        }
      });
    }

    const neighbours: number[][] = Array.from({ length: faceCount }, () => []);
    vertexToFaces.forEach((indices) => {
      for (let i = 0; i < indices.length; i++) {
        for (let j = i + 1; j < indices.length; j++) {
          const a = indices[i];
          const b = indices[j];
          neighbours[a].push(b);
          neighbours[b].push(a);
        }
      }
    });

    const visited: boolean[] = Array.from({ length: faceCount }, () => false);

    for (let i = 0; i < faceCount; i++) {
      if (visited[i]) continue;

      const stack: number[] = [i];
      visited[i] = true;
      const componentFaces: number[] = [];
      let componentArea = 0;

      while (stack.length > 0) {
        const idx = stack.pop()!;
        componentFaces.push(idx);
        componentArea += getFaceArea(faces[idx]);

        neighbours[idx].forEach((next) => {
          if (!visited[next]) {
            visited[next] = true;
            stack.push(next);
          }
        });
      }

      if (componentArea < minComponentArea) {
        componentFaces.forEach((idx) => {
          deletedFaces.add(faces[idx]);
        });
      }
    }
  });

  console.log({ deletedFaces: deletedFaces.size });
  this.deleteFacesIf((f) => deletedFaces.has(f));
  return this;
}

function getFaceArea(face: Face): number {
  const a = face.vertices[0].position;
  const b = face.vertices[1].position;
  const c = face.vertices[2].position;

  const abX = b[0] - a[0];
  const abY = b[1] - a[1];
  const abZ = b[2] - a[2];

  const acX = c[0] - a[0];
  const acY = c[1] - a[1];
  const acZ = c[2] - a[2];

  const crossX = abY * acZ - abZ * acY;
  const crossY = abZ * acX - abX * acZ;
  const crossZ = abX * acY - abY * acX;

  const crossLength = Math.sqrt(crossX * crossX + crossY * crossY + crossZ * crossZ);
  return 0.5 * crossLength;
}
