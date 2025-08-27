export function f(x?: number | null) {
  let num = x ?? 0;
  if (Math.abs(num) > 999999) {
    num = Math.sign(num) * 999999;
  }
  return parseFloat(num.toFixed(4)).toString();
}

export function fVector(vector: number[]): string {
  return vector.map(f).join(', ');
}
