export interface GlobalSequence {
  id: number
  duration: number;
}

export function globalSequencesToString(globalSequences: GlobalSequence[]): string {
  if (globalSequences.length === 0) return '';
  return `GlobalSequences ${globalSequences.length} {
    ${globalSequences.map((gs) => `Duration ${gs.duration},`).join('\n')}
  }`;
}
