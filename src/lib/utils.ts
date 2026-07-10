export const nArray = <T>(height: number, width: number, v: T): T[][] => Array.from({ length: (height) }, () => Array<T>(width).fill(v));

export function toMap<T, K extends(keyof T)>(array: T[], key: K) {
  return new Map(array.map((item) => [item[key], item]));
}

export async function waitUntil(condition: () => boolean | Promise<boolean>) {
  if (await condition()) return true;
  return new Promise<boolean>((resolve) => {
    // eslint-disable-next-line @typescript-eslint/no-misused-promises
    const interval = setInterval(async () => {
      if (await condition()) {
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
