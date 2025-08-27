import { Bound } from './extent';
import { f } from './formatter';

export interface Sequence extends Bound {
  name: string;
  interval: [number, number];
  nonLooping: boolean;
  moveSpeed: number,
  data: {
    wc3Name: string
    wowName: string
    wowVariant: number
    wowFrequency: number
    attackTag: string
  }
  ,
  rarity?: number
  keep?: boolean;
}

export type SequenceData = Sequence['data']

export function sequencesToString(sequences: Sequence[]) {
  if (sequences.length === 0) return '';

  // Add number suffix to sequences with same name
  const animNameCount = new Map<string, number>();
  const seqName = new Map<Sequence, string>();
  sequences.forEach((seq) => {
    animNameCount.set(seq.name, (animNameCount.get(seq.name) ?? 0) + 1);
    seqName.set(seq, `${seq.name} ${animNameCount.get(seq.name)}`);
  });

  return `Sequences ${sequences.length} {
    ${sequences.map((sequence) => `
      Anim "${seqName.get(sequence)}" {
        Interval { ${sequence.interval[0]}, ${sequence.interval[1]} },
        ${sequence.nonLooping ? 'NonLooping,' : ''}
        ${sequence.moveSpeed > 0 ? `MoveSpeed ${sequence.moveSpeed},` : ''}
        ${(sequence.rarity ?? 0) > 0 ? `Rarity ${sequence.rarity},` : ''}
        MinimumExtent { ${sequence.minimumExtent.map(f).join(', ')} },
        MaximumExtent { ${sequence.maximumExtent.map(f).join(', ')} },
        BoundsRadius ${f(sequence.boundsRadius)},
      }`).join('\n')}
  }`;
}
