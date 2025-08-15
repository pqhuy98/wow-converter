import { Vector2, Vector3 } from "@/lib/math/common";
import { MDLModify } from ".";
import { GeosetVertex, SkinWeight } from "../components/geoset";
import { Face } from "../components/geoset";
import { V3 } from "@/lib/math/vector";
import { Bone } from "../components/node";


type Plane = { axis: 0 | 1 | 2; min: boolean; value: number };


export function deleteVerticesIf(
  this: MDLModify,
  shouldDeleteVert: (v: GeosetVertex) => boolean,
  resolvePartialFace?: (f: Face) => Face[],
) {
  this.mdl.geosets.forEach((geoset) => {
    let verts = new Set<GeosetVertex>(geoset.vertices);
    const faces = new Set<Face>(geoset.faces);
    geoset.vertices.forEach((vert) => {
      if (shouldDeleteVert?.(vert)) verts.delete(vert);
    });
    geoset.faces.forEach((face) => {
      if (face.vertices.some((v) => !verts.has(v))) {
        faces.delete(face);
        const newFaces = resolvePartialFace?.(face);
        newFaces?.forEach((newFace) => {
          faces.add(newFace);
          newFace.vertices.forEach((v) => verts.add(v));
        });
      }
    });
    verts = new Set([...faces].flatMap((face) => face.vertices));

    geoset.vertices = [...verts];
    geoset.faces = [...faces];
  });
  return this;
}

export function deleteVerticesOutsideBox(this: MDLModify, low: Vector3, high: Vector3) {
  const shouldDeleteVert = (vert: GeosetVertex) => vert.position[0] < low[0]
    || vert.position[1] < low[1]
    || vert.position[2] < low[2]
    || vert.position[0] > high[0]
    || vert.position[1] > high[1]
    || vert.position[2] > high[2];

  const resolvePartialFace = (face: Face): Face[] => {
    const input = [...face.vertices];
    const planes: Plane[] = [
      { axis: 0, min: true, value: low[0] },
      { axis: 0, min: false, value: high[0] },
      { axis: 1, min: true, value: low[1] },
      { axis: 1, min: false, value: high[1] },
      { axis: 2, min: true, value: low[2] },
      { axis: 2, min: false, value: high[2] },
    ];
    let poly = input;
    for (const plane of planes) {
      poly = clipPolygon(poly, plane);
      if (poly.length === 0) return [];
    }
    const outFaces: Face[] = [];
    for (let i = 1; i < poly.length - 1; i++) {
      outFaces.push({ vertices: [poly[0], poly[i], poly[i + 1]] });
    }
    return outFaces;
  };

  this.deleteVerticesIf(shouldDeleteVert, resolvePartialFace);
  return this;
}

export function deleteVerticesInsideBox(this: MDLModify, low: Vector3, high: Vector3) {
  const shouldDeleteVertInside = (vert: GeosetVertex) => vert.position[0] >= low[0]
    && vert.position[1] >= low[1]
    && vert.position[2] >= low[2]
    && vert.position[0] <= high[0]
    && vert.position[1] <= high[1]
    && vert.position[2] <= high[2];

  const resolvePartialFaceInside = (face: Face): Face[] => {
    const planes: Plane[] = [
      { axis: 0, min: false, value: low[0] }, // keep x < low[0]
      { axis: 0, min: true, value: high[0] }, // keep x > high[0]
      { axis: 1, min: false, value: low[1] }, // keep y < low[1]
      { axis: 1, min: true, value: high[1] }, // keep y > high[1]
      { axis: 2, min: false, value: low[2] }, // keep z < low[2]
      { axis: 2, min: true, value: high[2] }, // keep z > high[2]
    ];
    const outFaces: Face[] = [];

    for (const plane of planes) {
      const poly = clipPolygon(face.vertices, plane);
      if (poly.length < 3) continue;
      for (let i = 1; i < poly.length - 1; i++) {
        outFaces.push({ vertices: [poly[0], poly[i], poly[i + 1]] });
      }
    }

    return outFaces;
  };

  this.deleteVerticesIf(shouldDeleteVertInside, resolvePartialFaceInside);
  return this;
}

export function cut1DimOutside(this: MDLModify, dimension: number, lowPercent: number, highPercent: number) {
  const diff = this.mdl.model.maximumExtent[dimension] - this.mdl.model.minimumExtent[dimension];
  const low = this.mdl.model.minimumExtent[dimension] + diff * lowPercent;
  const high = this.mdl.model.minimumExtent[dimension] + diff * highPercent;
  const vLow: Vector3 = [-Infinity, -Infinity, -Infinity];
  const vHigh: Vector3 = [Infinity, Infinity, Infinity];
  vLow[dimension] = low;
  vHigh[dimension] = high;
  return this.deleteVerticesOutsideBox(vLow, vHigh);
}

export function cutInsidePercent(this: MDLModify, [[x0, x1], [y0, y1], [z0, z1]]: [[number, number], [number, number], [number, number]]) {
  const vLow: Vector3 = [
    V3.lerpScalar(this.mdl.model.minimumExtent[0], this.mdl.model.maximumExtent[0], x0),
    V3.lerpScalar(this.mdl.model.minimumExtent[1], this.mdl.model.maximumExtent[1], y0),
    V3.lerpScalar(this.mdl.model.minimumExtent[2], this.mdl.model.maximumExtent[2], z0),
  ];
  const vHigh: Vector3 = [
    V3.lerpScalar(this.mdl.model.minimumExtent[0], this.mdl.model.maximumExtent[0], x1),
    V3.lerpScalar(this.mdl.model.minimumExtent[1], this.mdl.model.maximumExtent[1], y1),
    V3.lerpScalar(this.mdl.model.minimumExtent[2], this.mdl.model.maximumExtent[2], z1),
  ];
  return this.deleteVerticesInsideBox(vLow, vHigh);
}

export function cutOutsidePercent(this: MDLModify, [[x0, x1], [y0, y1], [z0, z1]]: [[number, number], [number, number], [number, number]]) {
  this.mdl.sync();
  const vLow: Vector3 = [
    V3.lerpScalar(this.mdl.model.minimumExtent[0], this.mdl.model.maximumExtent[0], x0),
    V3.lerpScalar(this.mdl.model.minimumExtent[1], this.mdl.model.maximumExtent[1], y0),
    V3.lerpScalar(this.mdl.model.minimumExtent[2], this.mdl.model.maximumExtent[2], z0),
  ];
  const vHigh: Vector3 = [
    V3.lerpScalar(this.mdl.model.minimumExtent[0], this.mdl.model.maximumExtent[0], x1),
    V3.lerpScalar(this.mdl.model.minimumExtent[1], this.mdl.model.maximumExtent[1], y1),
    V3.lerpScalar(this.mdl.model.minimumExtent[2], this.mdl.model.maximumExtent[2], z1),
  ];
  return this.deleteVerticesOutsideBox(vLow, vHigh);
}

export function cropVerticesOneDimension(this: MDLModify, dimension: number, low: number, high: number) {
  this.mdl.sync();
  const vLow: Vector3 = [-Infinity, -Infinity, -Infinity];
  const vHigh: Vector3 = [Infinity, Infinity, Infinity];
  vLow[dimension] = low;
  vHigh[dimension] = high;
  return this.deleteVerticesOutsideBox(vLow, vHigh);
}

export function deleteFacesIf(this: MDLModify, shouldDeleteFace: (face: Face) => boolean) {
  this.mdl.geosets.forEach((geoset) => {
    let verts = new Set<GeosetVertex>(geoset.vertices);
    const faces = new Set<Face>(geoset.faces);
    geoset.faces.forEach((face) => {
      if (shouldDeleteFace(face)) {
        faces.delete(face);
      }
    });
    verts = new Set([...faces].flatMap((face) => face.vertices));

    geoset.vertices = [...verts];
    geoset.faces = [...faces];
  });
  return this;
}

function interpolateVertex(v1: GeosetVertex, v2: GeosetVertex, t: number): GeosetVertex {
  const lerp = (a: number, b: number) => a + (b - a) * t;
  const interpVec3 = (a: Vector3, b: Vector3) => [lerp(a[0], b[0]), lerp(a[1], b[1]), lerp(a[2], b[2])] as Vector3;
  const interpVec2 = (a: Vector2, b: Vector2): Vector2 => [lerp(a[0], b[0]), lerp(a[1], b[1])];

  const skinWeights: SkinWeight[] = [];
  if (v1.skinWeights || v2.skinWeights) {
    const map = new Map<Bone, number>();
    (v1.skinWeights || []).forEach((sw) => map.set(sw.bone, sw.weight * (1 - t)));
    (v2.skinWeights || []).forEach((sw) => map.set(sw.bone, (map.get(sw.bone) || 0) + sw.weight * t));
    map.forEach((weight, bone) => {
      if (weight > 0) skinWeights.push({ bone, weight });
    });
  }

  return {
    id: -1,
    position: interpVec3(v1.position, v2.position),
    normal: interpVec3(v1.normal, v2.normal),
    texPosition: interpVec2(v1.texPosition, v2.texPosition),
    matrix: v1.matrix && v2.matrix
      ? /* simple lerp or choose one */ v1.matrix
      : v1.matrix || v2.matrix,
    skinWeights: skinWeights.length ? skinWeights : undefined,
  };
}

function clipPolygon(
  verts: GeosetVertex[],
  plane: Plane,
): GeosetVertex[] {
  const inside = (v: GeosetVertex) => (plane.min
    ? v.position[plane.axis] >= plane.value
    : v.position[plane.axis] <= plane.value);

  const out: GeosetVertex[] = [];
  for (let i = 0; i < verts.length; i++) {
    const curr = verts[i];
    const next = verts[(i + 1) % verts.length];
    const currIn = inside(curr);
    const nextIn = inside(next);
    if (currIn) out.push(curr);
    if (currIn !== nextIn) {
      const delta = (plane.value - curr.position[plane.axis])
        / (next.position[plane.axis] - curr.position[plane.axis]);
      out.push(interpolateVertex(curr, next, delta));
    }
  }
  return out;
}
