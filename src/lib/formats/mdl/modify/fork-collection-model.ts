import { Model } from '@/lib/converter/common/models';
import { profileSync } from '@/lib/export-profile';

import { Geoset } from '../components/geoset';
import { MDL } from '../mdl';

/**
 * Immutable attach copy for collection armor: deep-clone only mesh/material data
 * (typically 2–3 geosets), not the full character SKEL bone animation graph.
 *
 * Bone objects on geoset skin weights stay as shared references into the pristine
 * template — safe because collection merge only reads bone.name for remap, then
 * clears item.bones without mutating the template skeleton.
 */
export function forkCollectionModel(template: Model, enabledGeosets: Geoset[]): Model {
  const src = template.mdl;
  const enabledSet = new Set(enabledGeosets);
  const materials = [...new Set(enabledGeosets.map((g) => g.material))];

  return profileSync('forkCollection', () => {
    const cloned = structuredClone({
      geosets: enabledGeosets,
      materials,
      textures: src.textures,
      textureAnims: src.textureAnims,
      geosetAnims: src.geosetAnims.filter((ga) => enabledSet.has(ga.geoset)),
      globalSequences: src.globalSequences,
      sequences: src.sequences,
      attachments: src.attachments,
      lights: src.lights,
      ribbonEmitters: src.ribbonEmitters,
      particleEmitter2s: src.particleEmitter2s,
      helpers: src.helpers,
      cameras: src.cameras,
      eventObjects: src.eventObjects,
    });

    const mdl = new MDL({ formatVersion: src.version.formatVersion, name: src.model.name });
    mdl.model = { ...src.model };
    mdl.accumScale = src.accumScale;
    mdl.geosets = cloned.geosets;
    mdl.materials = cloned.materials;
    mdl.textures = cloned.textures;
    mdl.textureAnims = cloned.textureAnims;
    mdl.geosetAnims = cloned.geosetAnims;
    mdl.globalSequences = cloned.globalSequences;
    mdl.sequences = cloned.sequences;
    mdl.attachments = cloned.attachments;
    mdl.lights = cloned.lights;
    mdl.ribbonEmitters = cloned.ribbonEmitters;
    mdl.particleEmitter2s = cloned.particleEmitter2s;
    mdl.helpers = cloned.helpers;
    mdl.cameras = cloned.cameras;
    mdl.eventObjects = cloned.eventObjects;
    mdl.bones = src.bones;
    mdl.wowAttachments = src.wowAttachments;

    return { relativePath: template.relativePath, mdl };
  }, {
    geosets: enabledGeosets.length,
    vertices: enabledGeosets.reduce((s, g) => s + g.vertices.length, 0),
  });
}
