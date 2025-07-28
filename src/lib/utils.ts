export const nArray = <T>(height: number, width: number, v: T): T[][] => Array.from({ length: (height + 1) }, () => Array<T>(width + 1).fill(v));

export function reverseFourCC(code: number): string {
  // Extract each character from the 32-bit integer
  const char1 = String.fromCharCode((code >>> 24) & 0xFF);
  const char2 = String.fromCharCode((code >>> 16) & 0xFF);
  const char3 = String.fromCharCode((code >>> 8) & 0xFF);
  const char4 = String.fromCharCode(code & 0xFF);

  // Combine the characters into a string
  return char1 + char2 + char3 + char4;
}

export function toMap<T, K extends(keyof T)>(array: T[], key: K) {
  return new Map(array.map((item) => [item[key], item]));
}

export function waitUntil(condition: () => boolean) {
  if (condition()) return Promise.resolve(true);
  return new Promise((resolve) => {
    const interval = setInterval(() => {
      if (condition()) {
        clearInterval(interval);
        resolve(true);
      }
    }, 100);
  });
}
