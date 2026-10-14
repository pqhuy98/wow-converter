import type MdxModelInstance from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/handlers/mdx/modelinstance';
import type Scene from '@pqhuy98/mdx-m3-viewer/dist/cjs/viewer/scene';
import { expect, test } from 'bun:test';

import { freezeShotPose, settleShotPose } from './shot-pose';

test('nested particle clocks stay frozen and replay resets global time', () => {
  const grandchild = instance();
  const child = instance([grandchild]);
  const root = instance([child]);
  const instances = [root, child, grandchild];
  // The SDK's emitted-object updater applies the emitting instance's own timeScale.
  const updater = { update(dt: number) { for (const inst of instances) inst.particleAge += dt * inst.timeScale; } };
  // Structural SDK doubles avoid requiring a WebGL context for the clock regression.
  const model = root as unknown as MdxModelInstance;
  const scene = { emittedObjectUpdater: updater } as unknown as Scene;
  freezeShotPose(model);
  expect(instances.map((inst) => inst.timeScale)).toEqual([0, 0, 0]);
  const random = Math.random;
  for (let replay = 0; replay < 2; replay++) {
    settleShotPose(scene, model, 1000);
    for (const inst of instances) {
      expect(inst.counter).toBeCloseTo(1000, 6);
      expect(inst.particleAge).toBeCloseTo(1, 6);
      expect(inst.timeScale).toBe(0);
    }
    updater.update(5); // Simulate time spent switching views and capturing screenshots.
    for (const inst of instances) expect(inst.particleAge).toBeCloseTo(1, 6);
    expect(Math.random).toBe(random);
  }
});

test('explicit elapsed replay warms particles across looping sequence boundaries', () => {
  const root = instance();
  const advance = root.updateAnimations;
  root.updateAnimations = function (dt: number): void {
    advance.call(this, dt);
    this.frame %= 1000;
  };
  const model = root as unknown as MdxModelInstance;
  const scene = { emittedObjectUpdater: { update(dt: number) { root.particleAge += dt * root.timeScale; } } } as unknown as Scene;
  settleShotPose(scene, model, 0, 2.5);
  expect(root.counter).toBeCloseTo(2500, 6);
  expect(root.frame).toBeCloseTo(500, 6);
  expect(root.particleAge).toBeCloseTo(2.5, 6);
  expect(root.timeScale).toBe(0);
});

function instance(children: {
  timeScale: number;
  setSequence(sequence: number): void;
  updateAnimations(dt: number): void;
}[] = []) {
  return {
    frame: 0,
    counter: 123,
    sequence: 0,
    timeScale: 1,
    forced: false,
    particleAge: 0,
    particleEmitters2: [{ lastEmissionKey: 5 }],
    attachments: children.map((internalInstance) => ({ internalInstance })),
    clearEmittedObjects() { this.particleAge = 0; },
    setSequence(sequence: number) {
      this.sequence = sequence;
      this.frame = 0;
      for (const child of children) child.setSequence(0);
    },
    updateAnimations(dt: number) {
      this.frame += dt * 1000;
      this.counter += dt * 1000;
      for (const child of children) child.updateAnimations(dt * child.timeScale);
    },
  };
}
