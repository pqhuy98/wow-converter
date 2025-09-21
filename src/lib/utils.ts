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

// Produce a stable, order-independent JSON string.
export function stableStringify(value: unknown): string {
  const sorter = (val: unknown): unknown => {
    if (Array.isArray(val)) {
      return val.map(sorter);
    }
    if (val && typeof val === 'object' && !(val instanceof Date)) {
      const sorted: Record<string, unknown> = {};
      for (const key of Object.keys(val).sort()) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        sorted[key] = sorter((val as any)[key]);
      }
      return sorted;
    }
    return val;
  };
  return JSON.stringify(sorter(value));
}

export async function workerPool<T>(workerCount: number, tasks: (() => Promise<T>)[]) {
  const results: T[] = [];
  const worker = async () => {
    while (tasks.length > 0) {
      const task = tasks.shift();
      if (task) {
        results.push(await task());
      }
    }
  };
  const workers = Array.from({ length: workerCount }, worker);
  await Promise.all(workers);
  return results;
}
