import { describe, expect, test } from 'bun:test';

import { AnimationFile, type Data } from '@/lib/converter/wow-model/bundle/animation';
import { getDefaultConfig } from '@/lib/global-config';

describe('AnimationFile quaternion conversion', () => {
  test('keeps xyzw order while remapping axes', async () => {
    const config = await getDefaultConfig();
    const file = new AnimationFile('inline', config);

    const bone: Data.Bone = {
      boneID: -1,
      flags: 0,
      parentBone: -1,
      subMeshID: 0,
      boneNameCRC: 0,
      translation: {
        globalSeq: -1,
        interpolation: 0,
        timestamps: [[0]],
        values: [[[0, 0, 0]]],
      },
      rotation: {
        globalSeq: -1,
        interpolation: 0,
        timestamps: [[0]],
        values: [[[1, 2, 3, 4]]],
      },
      scale: {
        globalSeq: -1,
        interpolation: 0,
        timestamps: [[0]],
        values: [[[1, 1, 1]]],
      },
      pivot: [0, 0, 0],
    };

    file.loadFromData({
      bones: [bone],
      animations: [{
        id: 0,
        variationIndex: 0,
        duration: 1,
        movespeed: 0,
        flags: 0,
        frequency: 0,
        padding: 0,
        replayMin: 0,
        replayMax: 0,
        blendTimeIn: 0,
        blendTimeOut: 0,
        boxPosMin: [0, 0, 0],
        boxPosMax: [0, 0, 0],
        boxRadius: 0,
        variationNext: 0,
        aliasNext: 0,
      }],
      boneWeights: [255, 0, 0, 0],
      boneIndicies: [0, 0, 0, 0],
      attachments: [],
    });

    const mdl = file.toMdl([]);
    const rotation = mdl.bones[0]?.rotation?.keyFrames.get(0);
    expect(rotation).toEqual([1, -3, 2, 4]);
  });
});
