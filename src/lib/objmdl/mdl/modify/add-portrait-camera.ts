import { Vector3 } from "@/lib/math/common";
import { V3 } from "@/lib/math/vector";
import { MDLModify } from ".";
import { iterateNodesAtTimestamp } from "../mdl-traverse";

export function addPortraitCamera(this: MDLModify, standSequenceName: string = 'Stand') {
  const cameraBone = this.mdl.bones.find((b) => b.name === 'Head')
    || this.mdl.bones.find((b) => b.name === 'Chest')
    || this.mdl.bones.find((b) => b.name === 'Root');
  if (!cameraBone) {
    // Generate default camera
    this.mdl.cameras.push({
      name: 'Portrait_Camera',
      fieldOfView: 1,
      nearClip: 0.1,
      farClip: 10000,
      target: {
        position: V3.mean(this.mdl.model.minimumExtent, this.mdl.model.maximumExtent),
      },
      position: [...V3.scale([
        this.mdl.model.minimumExtent[0],
        this.mdl.model.maximumExtent[1],
        this.mdl.model.maximumExtent[2],
      ], 1.1)],
    });

    return this;
  }

  let nodePos: Vector3 = cameraBone.pivotPoint;
  const standSequence = this.mdl.sequences.find((seq) => seq.name === standSequenceName);

  // Find actual position of cameraBone during Stand animation
  if (standSequence) {
    iterateNodesAtTimestamp(this.mdl, standSequence, standSequence.interval[0], (node, value) => {
      if (node === cameraBone) {
        nodePos = value.position;
      }
    });
  }

  const distanceScale = {
    Head: 3,
    Chest: 2,
    Root: 1,
  }[cameraBone.name]!;

  const cameraPosition = V3.sum(nodePos, [
    distanceScale * this.mdl.model.maximumExtent[0],
    0.5 * (Math.random() - 0.5) * this.mdl.model.maximumExtent[1],
    (Math.random() * 0.2 - 0.1) * this.mdl.model.maximumExtent[2],
  ]);
  // console.log('Add portrait camera looking at bone', cameraBone.name);
  // console.log('Target position', nodePos);
  // console.log('Camera position', cameraPosition);

  this.mdl.cameras.push({
    name: 'Portrait_Camera',
    fieldOfView: 1,
    nearClip: 0.1,
    farClip: 10000,
    target: {
      position: nodePos,
    },
    position: cameraPosition,
  });
  return this;
}