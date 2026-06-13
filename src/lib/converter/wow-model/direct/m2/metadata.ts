/**
 * Direct port of M2Exporter's writeMeta: builds the `.json`-equivalent
 * metadata object in memory. Values mirror the legacy JSON exactly (the
 * caller JSON-round-trips the result to normalize undefined/NaN/BigInt the
 * same way JSONWriter + JSON.parse did).
 */
import { getFileNameByID } from '@/lib/wow/archive/client/name-client';
import type { GeosetMaskEntry, TextureManifestEntry, VariantTexture } from '@/lib/wow/export/m2/m2-exporter';
import { M2Loader } from '@/lib/wow/formats/m2/m2-loader';
import { Skin } from '@/lib/wow/formats/m2/skin';

export async function buildMetadataObject(
  m2: M2Loader,
  skin: Skin,
  fileDataID: number,
  fileName: string | undefined,
  geosetMask: GeosetMaskEntry[] | undefined,
  validTextures: Map<VariantTexture, TextureManifestEntry>,
  dataTextures: Set<number>,
): Promise<Record<string, unknown>> {
  // Clone the submesh array and add the 'enabled' property.
  const subMeshes = new Array<unknown>(skin.subMeshes.length);
  for (let i = 0, n = subMeshes.length; i < n; i++) {
    const subMeshEnabled = !geosetMask || geosetMask[i].checked;
    subMeshes[i] = { enabled: subMeshEnabled, ...skin.subMeshes[i] };
  }

  // Clone M2 textures array and expand entries with internal/external names.
  const textures = new Array<unknown>(m2.textures.length);
  for (let i = 0, n = textures.length; i < n; i++) {
    const texture = m2.textures[i];
    const texType = m2.textureTypes[i];
    let textureEntry = validTextures.get(texture.fileDataID);

    // Fallback for data textures whose fileDataID may be 0 or not mapped yet.
    if (!textureEntry && dataTextures.has(texType)) textureEntry = validTextures.get(`data-${texType}`);

    textures[i] = {
      fileNameInternal: typeof texture.fileDataID === 'number' ? await getFileNameByID(texture.fileDataID) : undefined,
      fileNameExternal: textureEntry?.matPathRelative,
      mtlName: textureEntry?.matName,
      ...texture,
    };
  }

  return {
    fileType: 'm2',
    fileDataID,
    fileName,
    internalName: m2.name,
    textures,
    textureTypes: m2.textureTypes,
    materials: m2.materials,
    textureCombos: m2.textureCombos,
    skeletonFileID: m2.skeletonFileID,
    boneFileIDs: m2.boneFileIDs,
    animFileIDs: m2.animFileIDs,
    m2Animations: m2.animations,
    colors: m2.colors,
    textureWeights: m2.textureWeights,
    transparencyLookup: m2.transparencyLookup,
    textureTransforms: m2.textureTransforms,
    textureTransformsLookup: m2.textureTransformsLookup,
    boundingBox: m2.boundingBox,
    boundingSphereRadius: m2.boundingSphereRadius,
    collisionBox: m2.collisionBox,
    collisionSphereRadius: m2.collisionSphereRadius,
    lights: m2.lights || [],
    skin: {
      subMeshes,
      textureUnits: skin.textureUnits,
      fileName: skin.fileName,
      fileDataID: skin.fileDataID,
    },
    cameras: m2.cameras || [],
    cameraLookup: m2.cameraLookup || [],
    ribbonEmitters: m2.ribbonEmitters || [],
    particleEmitters: m2.particleEmitters || [],
  };
}
