import type MdxModelInstance from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/modelinstance';
import type Scene from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/scene';

/** Attachment particles use their own instance's clock, independently of the parent. */
export function freezeShotPose(inst: MdxModelInstance): void {
  for (const instance of shotInstances(inst)) instance.timeScale = 0;
}

/** Replay the entire attachment tree at 60Hz, then freeze every particle clock. */
export function settleShotPose(scene: Scene, inst: MdxModelInstance, targetFrame: number, elapsedSeconds?: number): void {
  const instances = shotInstances(inst);
  for (const instance of instances) {
    instance.clearEmittedObjects();
    instance.timeScale = 0;
    instance.counter = 0;
    for (const emitter of instance.particleEmitters2) emitter.lastEmissionKey = -1;
  }
  scene.emittedObjectUpdater.update(0);
  inst.setSequence(inst.sequence);
  const random = Math.random;
  Math.random = seededRandom(1);
  try {
    let guard = 0;
    let elapsed = 0;
    while ((elapsedSeconds === undefined ? inst.frame + 1e-3 < targetFrame : elapsed < elapsedSeconds) && guard < 20000) {
      const remaining = elapsedSeconds === undefined ? (targetFrame - inst.frame) / 1000 : elapsedSeconds - elapsed;
      const dt = Math.min(1 / 60, remaining);
      if (dt <= 0) break;
      // updateAnimations traverses attachment nodes; their update() applies timeScale.
      for (const instance of instances) instance.timeScale = 1;
      inst.updateAnimations(dt);
      scene.emittedObjectUpdater.update(dt);
      for (const instance of instances) instance.timeScale = 0;
      elapsed += dt;
      guard += 1;
    }
    if (elapsedSeconds === undefined) inst.frame = targetFrame;
    inst.forced = true;
    inst.updateAnimations(0);
  } finally {
    for (const instance of instances) instance.timeScale = 0;
    Math.random = random;
  }
}

function shotInstances(inst: MdxModelInstance): MdxModelInstance[] {
  return [inst, ...inst.attachments.flatMap((attachment) => shotInstances(attachment.internalInstance))];
}

function seededRandom(seed: number): () => number {
  let state = seed;
  return () => {
    state = (state * 9301 + 49297) % 233280;
    return state / 233280;
  };
}
