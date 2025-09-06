import { Vector3 } from '@/lib/math/common';
import { V3 } from '@/lib/math/vector';

import { Node } from '../components/node/node';
import { iterateNodesAtTimestamp } from '../mdl-traverse';
import { MDLModify } from '.';

const debug = false;

// TODO: this doesn't work with Undead character!!
export function computeWalkMovespeed(this: MDLModify) {
  this.mdl.sequences.forEach((seq) => {
    if (seq.moveSpeed === 0 && ([
      'Walk', 'Run', 'Sprint', 'FlyWalk',
    ].includes(seq.data.wowName))) {
      debug && console.log(this.mdl.model.name, 'calculating missing movespeed for', `"${seq.name}" (${seq.data.wowName})`);
      const SAMPLE_STEPS = 30;

        // -------------------------------------------------------------------
        // 1. Sample frames and capture node positions
        // -------------------------------------------------------------------
        type FrameEntry = { node: Node; position: Vector3 };
        type FrameInfo = { entries: FrameEntry[]; time: number };

        const frames: FrameInfo[] = [];

        let globalMin: Vector3 = [Number.POSITIVE_INFINITY, Number.POSITIVE_INFINITY, Number.POSITIVE_INFINITY];
        let globalMax: Vector3 = [Number.NEGATIVE_INFINITY, Number.NEGATIVE_INFINITY, Number.NEGATIVE_INFINITY];

        for (let i = 0; i <= SAMPLE_STEPS; i += 1) {
          const t = seq.interval[0] + ((seq.interval[1] - seq.interval[0]) * i) / SAMPLE_STEPS;
          const entries: FrameEntry[] = [];

          iterateNodesAtTimestamp(this.mdl, seq, t, (node: Node, value) => {
            entries.push({ node, position: value.position });
            globalMin = V3.min(globalMin, value.position);
            globalMax = V3.max(globalMax, value.position);
          });

          frames.push({ entries, time: t });
        }

        // -------------------------------------------------------------------
        // 2. Derive tolerances
        // -------------------------------------------------------------------
        const diag = Math.hypot(
          globalMax[0] - globalMin[0],
          globalMax[1] - globalMin[1],
          globalMax[2] - globalMin[2],
        );
        const groundTolerance = diag * 0.02; // 10% of model size
        const zStableTolerance = diag * 0.02; // 2% of model size

        const legs = this.mdl.bones.filter((b) => b.name.startsWith('Leg'));
        const legChildren = new Set<Node>(legs);
        if (legChildren.size > 0) {
          for (const bone of this.mdl.bones) {
            if (legChildren.has(bone)) continue;
            let cur = bone.parent;
            while (cur) {
              if (legChildren.has(cur)) {
                legChildren.add(bone);
                break;
              }
              cur = cur.parent;
            }
          }
        }
        debug && console.log(this.mdl.model.name, 'bones below legs:', legChildren.size);

        // -------------------------------------------------------------------
        // 3. Detect contacts with Z-stability check
        // -------------------------------------------------------------------
        const contactXs: number[] = [];
        const contactFrameTimes: number[] = [];
        const contactFrameIndices: number[] = [];
        const contactFrameContactCount: number[] = [];
        const prevZ = new Map<Node, number>();

        const contactFlags: boolean[] = [];

        const contactBones = new Set<Node>();

        let prevTime = 0;
        frames.forEach(({ entries, time }, frameIdx) => {
          const timeDeltaS = (time - prevTime) / 1000;
          entries.forEach(({ node, position }) => {
            const isLeg = legChildren.size === 0 || legChildren.has(node);
            const stableZ = !prevZ.has(node) || Math.abs(position[2] - prevZ.get(node)!) / timeDeltaS <= zStableTolerance;
            if (stableZ && Math.abs(position[2]) <= groundTolerance && isLeg) {
              contactXs.push(position[0]);
              contactBones.add(node);
              if (!contactFlags[frameIdx]) {
                contactFlags[frameIdx] = true;
                contactFrameTimes.push(time);
                contactFrameIndices.push(frameIdx);
                contactFrameContactCount[frameIdx] = 1;
              } else {
                contactFrameContactCount[frameIdx] += 1;
              }
            }
            prevZ.set(node, position[2]);
          });
          prevTime = time;
        });

        debug && console.log(this.mdl.model.name, 'contact bones:', [...contactBones].map((b) => b.name));

        // -------------------------------------------------------------------
        // 4. Outlier removal (trim 5 % at each end)
        // -------------------------------------------------------------------
        let minContactX = Number.POSITIVE_INFINITY;
        let maxContactX = Number.NEGATIVE_INFINITY;

        if (contactXs.length >= 2) {
          const sorted = [...contactXs].sort((a, b) => a - b);
          const trim = Math.floor(sorted.length * 0.05);
          const trimmed = sorted.slice(trim, sorted.length - trim || undefined);
          minContactX = Math.min(...trimmed);
          maxContactX = Math.max(...trimmed);
        }

        const strideLength = maxContactX - minContactX;

        // -------------------------------------------------------------------
        // 5. Duration – sum of intervals where consecutive frames both have contact
        // -------------------------------------------------------------------
        let durationMs = 0;
        if (contactFrameTimes.length >= 2) {
          for (let i = 0; i < contactFrameIndices.length - 1; i += 1) {
            if (contactFrameIndices[i + 1] === contactFrameIndices[i] + 1) {
              durationMs += contactFrameTimes[i + 1] - contactFrameTimes[i];
            }
          }
        }
        if (durationMs === 0) {
          durationMs = seq.interval[1] - seq.interval[0];
        }
        const strideDurationSeconds = durationMs / 2 / 1000; // divide by 2 because each feet moves forward then backward
        debug && console.log(this.mdl.model.name, seq.name, 'contact durationMs', durationMs, 'stride duration S', strideDurationSeconds);

        const moveSpeed = strideDurationSeconds > 0 ? strideLength / strideDurationSeconds : 0;
        if (debug) {
          console.log(seq.interval);
          console.log(contactFrameTimes);
          console.log(contactFrameContactCount);
          console.log({
            seqName: seq.name,
            wowName: seq.data.wowName,
            diag,
            groundTolerance,
            zStableTolerance,
            minContactX,
            maxContactX,
            strideLength,
            strideDurationSeconds,
            moveSpeed,
          });
        }

        const sizeX = globalMax[0] - globalMin[0];
        if (moveSpeed > 0.5 * sizeX && moveSpeed < sizeX * 2) {
          debug && console.log('setting moveSpeed', seq.name, { moveSpeed, sizeX });
          seq.moveSpeed = moveSpeed;
        } else {
          debug && console.log(this.mdl.model.name, seq.name, 'setting moveSpeed to 0');
          seq.moveSpeed = 0;
        }
    }
  });
  return this;
}
