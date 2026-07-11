/**
 * Direct port of M2Exporter.exportTextures naming/override logic, minus pixel
 * output: instead of decoding BLP->PNG on the server, each texture's relative
 * PNG path is registered in the converter-side texture-source registry
 * (fileDataID for raw BLPs, RGBA for baked data textures). The names, MTL
 * materials and variant-texture overrides are byte-for-byte identical to the
 * legacy path so downstream MDL assembly produces identical output.
 */
import path from 'path';

import { registerTextureSource, TextureSource } from '@/lib/converter/common/texture-source';
import type { ObjMaterial } from '@/lib/converter/wow-model/bundle/mtl';
import { getFileNameByID } from '@/lib/wow/archive/client/name-client';
import { getRawWowFile } from '@/lib/wow/archive/client/raw-client';
import type { TextureManifestEntry, VariantTexture } from '@/lib/wow/export/m2/m2-exporter';
import { replaceExtension } from '@/lib/wow/export/writers/export-helper';
import { M2Loader } from '@/lib/wow/formats/m2/m2-loader';

/** Data texture provided by the character pipeline (composited RGBA). */
export interface DirectDataTexture {
  /** Original CASC path to preserve in export layout, or null for generic naming. */
  filename: string | null;
  source: TextureSource;
}

export interface ResolvedTextures {
  validTextures: Map<VariantTexture, TextureManifestEntry>;
  mtlMaterials: ObjMaterial[];
}

/** Mirror of getExportPath (removePathSpaces always on for our config). */
function virtualExportPath(exportRoot: string, file: string): string {
  return path.normalize(path.join(exportRoot, file.replace(/\s/g, '')));
}

/**
 * Resolve all textures of an M2 model.
 * Mutates m2.textures[].fileDataID with variant overrides, exactly like the
 * legacy exporter (writeMeta and the mesh builder depend on this patching).
 *
 * @param outDir directory of the virtual OBJ output (under exportRoot)
 * @param exportRoot converter-side export root (config.exportAssetDir)
 */
export async function resolveTextures(
  m2: M2Loader,
  variantTextures: VariantTexture[],
  dataTextures: Map<number, DirectDataTexture>,
  outDir: string,
  exportRoot: string,
): Promise<ResolvedTextures> {
  const validTextures = new Map<VariantTexture, TextureManifestEntry>();
  const mtlMaterials: ObjMaterial[] = [];

  const addMaterial = (name: string, file: string) => {
    mtlMaterials.push({ name, map_Kd: file });
  };

  const registerSource = (texPath: string, source: TextureSource) => {
    registerTextureSource(path.relative(exportRoot, texPath), source);
  };

  // NOTE: textureIndex deliberately keeps counting across the data-texture
  // loop before indexing m2.textureTypes, replicating the legacy exporter's
  // behaviour exactly (bug-for-bug parity).
  let textureIndex = 0;

  // Data textures first (character bakes).
  for (const [textureType, dataTexture] of dataTextures) {
    const { filename, source } = dataTexture;

    let texPath: string;
    let texFile: string;
    let matName: string;

    if (filename) {
      const originalPath = filename.replace(/\\/g, '/');
      const fileNamePNG = replaceExtension(originalPath, '.png');
      texPath = virtualExportPath(exportRoot, fileNamePNG);
      texFile = path.relative(outDir, texPath);
      matName = `mat_${path.basename(fileNamePNG, '.png')}`;
    } else {
      texFile = `data-${textureType}.png`;
      texPath = path.join(outDir, texFile);
      matName = `mat_${textureType}`;
    }

    registerSource(texPath, source);

    addMaterial(matName, texFile);
    validTextures.set(`data-${textureType}`, {
      matName,
      matPathRelative: texFile,
      matPath: texPath,
    });

    textureIndex++;
  }

  for (const texture of m2.textures) {
    const textureType = m2.textureTypes[textureIndex];
    let texFileDataID: VariantTexture = texture.fileDataID;

    // Skip data textures; they're already processed above.
    if (dataTextures.has(textureType)) {
      textureIndex++;
      continue;
    }

    if (textureType > 0) {
      let targetFileDataID: VariantTexture = 0;

      if (textureType >= 11 && textureType < 14) {
        // Creature textures.
        targetFileDataID = variantTextures[textureType - 11];
      } else if (textureType > 1 && textureType < 5) {
        targetFileDataID = variantTextures[textureType - 2];
      }

      // Only override if a valid replacement is provided; otherwise keep original.
      const hasValidReplacement = (typeof targetFileDataID === 'string') || (Number.isInteger(targetFileDataID) && targetFileDataID > 0);
      if (hasValidReplacement) {
        texFileDataID = targetFileDataID;
        // Backward patch the variant texture into the M2 instance (writeMeta
        // and the mesh builder read the patched value).
        (texture as { fileDataID: VariantTexture }).fileDataID = targetFileDataID;
      }
    }

    if ((typeof texFileDataID === 'string' && texFileDataID.startsWith('data-')) || (typeof texFileDataID === 'number' && !Number.isNaN(texFileDataID) && texFileDataID > 0)) {
      try {
        let texFile = `${texFileDataID}.png`;
        let texPath = path.join(outDir, texFile);

        let matName = `mat_${texFileDataID}`;
        let fileName = typeof texFileDataID === 'number' ? await getFileNameByID(texFileDataID) : undefined;

        if (fileName === undefined && typeof texFileDataID === 'string' && !texFileDataID.startsWith('data-')) {
          fileName = `${texFileDataID}.blp`;
          matName = `mat_${texFileDataID}`;
        } else if (fileName !== undefined) {
          matName = `mat_${path.basename(fileName.toLowerCase(), '.blp')}`;
          // Remove spaces from material name for MTL compatibility.
          matName = matName.replace(/\s/g, '');
        }

        // Shared textures layout (enableSharedTextures always on).
        if (fileName !== undefined) {
          fileName = replaceExtension(fileName, '.png');
        } else {
          fileName = `unknown/${texFile}`;
        }
        texPath = virtualExportPath(exportRoot, fileName);
        texFile = path.relative(outDir, texPath);

        if (typeof texFileDataID === 'number') {
          // Verify the file is actually fetchable (parity with the legacy
          // path, which skipped textures whose CASC read failed). The raw
          // bytes land in the shared cache and are reused for BLP encoding.
          await getRawWowFile(texFileDataID);
          registerSource(texPath, { kind: 'blp', fileDataID: texFileDataID });
        }

        addMaterial(matName, texFile);
        validTextures.set(texFileDataID, {
          matName,
          matPathRelative: texFile,
          matPath: texPath,
        });
      } catch (e) {
        console.warn(`Failed to resolve texture ${texFileDataID} for M2:`, e instanceof Error ? e.message : String(e));
      }
    }

    textureIndex++;
  }

  return { validTextures, mtlMaterials };
}
