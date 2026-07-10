/**
 * Builds the OBJ-equivalent in-memory structure (IResult) straight from
 * M2Loader + Skin data, replicating exactly:
 *  - M2Exporter.exportAsOBJ's mesh emission (geoset mask, texture/material
 *    resolution incl. data-texture patching),
 *  - OBJWriter.getContent's used-index renumbering,
 *  - OBJFile.parse's structure (groups, faces, 1-based indices, w=0 UVs).
 * The numbers are copied directly; the legacy float64 -> shortest-repr string
 * -> parseFloat round-trip is lossless, so structures are identical.
 */
import {
  IFace, IGroup, IModel, IResult, ITextureVertex, IVertex,
} from '@/lib/converter/wow-model/bundle/obj';
import { getGeosetName } from '@/lib/wow/export/m2/geoset-mapper';
import type { GeosetMaskEntry, TextureManifestEntry, VariantTexture } from '@/lib/wow/export/m2/m2-exporter';
import { M2Loader } from '@/lib/wow/formats/m2/m2-loader';
import { Skin } from '@/lib/wow/formats/m2/skin';

export interface ObjMesh {
  name: string;
  triangles: number[];
  matName?: string;
}

/** Mirror of the writeOBJ submesh loop in M2Exporter.exportAsOBJ. */
export function buildMeshes(
  m2: M2Loader,
  skin: Skin,
  geosetMask: GeosetMaskEntry[] | undefined,
  validTextures: Map<VariantTexture, TextureManifestEntry>,
  dataTextures: Set<number>,
): ObjMesh[] {
  const meshes: ObjMesh[] = [];

  for (let mI = 0, mC = skin.subMeshes.length; mI < mC; mI++) {
    if (geosetMask && !geosetMask[mI].checked) continue;

    const mesh = skin.subMeshes[mI];
    const verts = new Array<number>(mesh.triangleCount);
    for (let vI = 0; vI < mesh.triangleCount; vI++) {
      const vert = skin.indices[skin.triangles[mesh.triangleStart + vI]];
      verts[vI] = vert;
    }

    let texture: { fileDataID: VariantTexture } | null = null;
    const texUnit = skin.textureUnits.find((tex) => tex.skinSectionIndex === mI);
    if (texUnit) {
      texture = m2.textures[m2.textureCombos[texUnit.textureComboIndex]] as unknown as { fileDataID: VariantTexture };
      // Patch for data textures
      const texType = m2.textureTypes[m2.textureCombos[texUnit.textureComboIndex]];
      if (dataTextures.has(texType)) texture.fileDataID = `data-${texType}`;
    }

    let matName: string | undefined;
    if (texture?.fileDataID && validTextures.has(texture.fileDataID)) matName = validTextures.get(texture.fileDataID)!.matName;

    meshes.push({ name: getGeosetName(mI, mesh.submeshID), triangles: verts, matName });
  }

  return meshes;
}

/**
 * Mirror of OBJWriter.getContent + OBJFile.parse: renumber used vertex
 * attributes and emit the parsed-OBJ structure directly.
 */
export function buildRawObjResult(
  verts: number[],
  normals: number[],
  uvLayers: number[][],
  meshes: ObjMesh[],
  modelName: string,
  mtlLib: string | undefined,
): IResult {
  const usedIndices = new Set<number>();
  meshes.forEach((mesh) => mesh.triangles.forEach((index) => usedIndices.add(index)));

  const vertMap = new Map<number, number>();
  const normalMap = new Map<number, number>();
  const uvMap = new Map<number, number>();

  const vertices: IVertex[] = [];
  for (let i = 0, j = 0, u = 0, n = verts.length; i < n; j++, i += 3) {
    if (usedIndices.has(j)) {
      vertMap.set(j, u++);
      vertices.push({ x: verts[i], y: verts[i + 1], z: verts[i + 2] });
    }
  }

  const vertexNormals: IVertex[] = [];
  for (let i = 0, j = 0, u = 0, n = normals.length; i < n; j++, i += 3) {
    if (usedIndices.has(j)) {
      normalMap.set(j, u++);
      vertexNormals.push({ x: normals[i], y: normals[i + 1], z: normals[i + 2] });
    }
  }

  const hasUV = uvLayers.length > 0;
  const textureCoords: ITextureVertex[] = [];
  const textureCoords2: ITextureVertex[] = [];
  for (let uvIndex = 0; uvIndex < uvLayers.length; uvIndex++) {
    const uv = uvLayers[uvIndex];
    const target = uvIndex === 0 ? textureCoords : textureCoords2;
    for (let i = 0, j = 0, u = 0, n = uv.length; i < n; j++, i += 2) {
      if (usedIndices.has(j)) {
        if (uvIndex === 0) uvMap.set(j, u++);
        target.push({ u: uv[i], v: uv[i + 1], w: 0 });
      }
    }
  }

  // Faces: replicate the OBJ text semantics, including usemtl persistence
  // across meshes that have no material of their own.
  const initialGroup: IGroup = { name: '' };
  const faces: IFace[] = [];
  let currentMaterial = '';
  for (const mesh of meshes) {
    const group: IGroup = { name: mesh.name };
    if (mesh.matName) currentMaterial = mesh.matName;

    const triangles = mesh.triangles;
    for (let i = 0, n = triangles.length; i < n; i += 3) {
      const face: IFace = {
        group,
        material: currentMaterial,
        smoothingGroup: 1,
        vertices: [],
      };
      for (let k = 0; k < 3; k++) {
        const t = triangles[i + k];
        face.vertices.push({
          vertexIndex: vertMap.get(t)! + 1,
          textureCoordsIndex: hasUV ? uvMap.get(t)! + 1 : 0,
          vertexNormalIndex: normalMap.get(t)! + 1,
        });
      }
      faces.push(face);
    }
  }

  const model: IModel = {
    name: modelName,
    group: [initialGroup],
    vertices,
    textureCoords,
    textureCoords2,
    vertexNormals,
    faces,
  };

  return {
    models: [model],
    materialLibraries: mtlLib ? [mtlLib] : [],
  };
}

/** OBJ-equivalent structure for the visible model. */
export function buildObjResult(
  m2: M2Loader,
  meshes: ObjMesh[],
  modelName: string,
  mtlLib: string | undefined,
): IResult {
  // UV layers: layer 0 = m2.uv (always exported), layer 1 = m2.uv2 (modelsExportUV2 always on).
  return buildRawObjResult(m2.vertices, m2.normals, [m2.uv, m2.uv2], meshes, modelName, mtlLib);
}

/** Mirror of the legacy `.phys.obj` collision export (OBJWriter, no UVs/MTL). */
export function buildCollisionObjResult(m2: M2Loader, modelName: string): IResult {
  return buildRawObjResult(
    m2.collisionPositions,
    m2.collisionNormals,
    [],
    [{ name: 'Collision', triangles: m2.collisionIndices }],
    modelName,
    undefined,
  );
}
