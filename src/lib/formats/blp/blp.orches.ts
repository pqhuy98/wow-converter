import fs from 'fs';
import { cpus } from 'os';
import { Worker } from 'worker_threads';

type Task = {
  id: number,
  pngBuffer: Buffer,
  blpPath: string,
  resolve: () => void,
  reject: (e: Error) => void,
};

type WorkerTaskMessage = {
  type: 'task',
  id: number,
  pngArrayBuffer: ArrayBuffer,
  byteOffset: number,
  byteLength: number,
  blpPath: string,
};
type WorkerDoneMessage = { type: 'done', id: number, success: boolean, error?: string };
type WorkerShutdownAckMessage = { type: 'shutdown-ack' };
type WorkerInboundMessage = WorkerDoneMessage | WorkerShutdownAckMessage;

function isWorkerInboundMessage(msg: unknown): msg is WorkerInboundMessage {
  if (!msg || typeof msg !== 'object' || !('type' in msg)) return false;
  const t = (msg as { type: unknown }).type;
  return t === 'done' || t === 'shutdown-ack';
}

type WorkerWithState = Worker & {
  __currentTask: Task | null,
  busy: boolean,
};

const defaultPoolSize = (() => {
  try {
    const count = cpus().length;
    return Math.max(1, count - 1);
  } catch {
    return 4;
  }
})();

function resolveWorkerPath(): string {
  let workerPath = './blp.worker.ts';
  workerPath = fs.existsSync(workerPath) ? workerPath : new URL(workerPath, import.meta.url).href;
  return workerPath;
}

export class BlpWorkerPool {
  private readonly workers: WorkerWithState[] = [];

  private readonly queue: Task[] = [];

  private nextTaskId = 1;

  private readonly workerPath: string;

  constructor(private readonly size: number) {
    this.workerPath = resolveWorkerPath();
    for (let i = 0; i < this.size; i++) {
      this.workers[i] = this.spawnWorker(i);
    }
  }

  getSize(): number {
    return this.workers.length;
  }

  private spawnWorker(index: number): WorkerWithState {
    const worker = new Worker(this.workerPath) as unknown as WorkerWithState;
    worker.__currentTask = null;
    worker.busy = false;

    worker.on('message', (msg: unknown) => {
      if (!isWorkerInboundMessage(msg)) return;
      if (msg.type !== 'done') return;
      const current = worker.__currentTask;
      if (current && current.id === msg.id) {
        if (msg.success) current.resolve();
        else current.reject(new Error(`${msg.error ?? 'Unknown error'}\nblpPath:${current.blpPath}`));
      }
      worker.__currentTask = null;
      worker.busy = false;
      this.pump();
    });

    worker.on('error', () => {
      // Requeue the in-flight task if any, then respawn
      if (worker.__currentTask) {
        this.queue.unshift(worker.__currentTask);
      }
      this.workers[index] = this.spawnWorker(index);
      this.pump();
    });

    worker.on('exit', (code) => {
      if (code === 0) return;
      if (worker.__currentTask) {
        this.queue.unshift(worker.__currentTask);
      }
      this.workers[index] = this.spawnWorker(index);
      this.pump();
    });

    return worker;
  }

  private pump() {
    for (let i = 0; i < this.workers.length; i++) {
      const w = this.workers[i];
      if (w.busy) continue;
      const task = this.queue.shift();
      if (!task) return;
      w.__currentTask = task;
      w.busy = true;
      const arrayBuffer: ArrayBuffer = task.pngBuffer.buffer as ArrayBuffer;
      const message: WorkerTaskMessage = {
        type: 'task',
        id: task.id,
        pngArrayBuffer: arrayBuffer,
        byteOffset: task.pngBuffer.byteOffset,
        byteLength: task.pngBuffer.byteLength,
        blpPath: task.blpPath,
      };
      // Transfer ownership of the ArrayBuffer to avoid copying memory
      w.postMessage(message, [arrayBuffer]);
    }
  }

  submit(pngBuffer: Buffer, blpPath: string): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      const task: Task = {
        id: this.nextTaskId++,
        pngBuffer,
        blpPath,
        resolve,
        reject,
      };
      this.queue.push(task);
      this.pump();
    });
  }

  async shutdown(): Promise<void> {
    await Promise.all(this.workers.map((w) => new Promise<void>((res) => {
      w.postMessage({ type: 'shutdown' });
      const timeout = setTimeout(() => {
        try {
          void w.terminate();
        } catch {
          /* ignore */
        }
        res();
      }, 200);
      w.on('message', (msg: unknown) => {
        if (isWorkerInboundMessage(msg) && msg.type === 'shutdown-ack') {
          clearTimeout(timeout);
          res();
        }
      });
    })));
  }
}

let singletonPool: BlpWorkerPool | null = null;

export function ensureBlpWorkerPool(desiredSize?: number): BlpWorkerPool {
  if (!singletonPool) {
    const size = Math.max(1, desiredSize ?? defaultPoolSize);
    singletonPool = new BlpWorkerPool(size);
  }
  return singletonPool;
}

export function getBlpWorkerPoolSize(): number {
  return singletonPool ? singletonPool.getSize() : 0;
}

export function submitBlpTask(pngBuffer: Buffer, blpPath: string): Promise<void> {
  const pool = ensureBlpWorkerPool();
  return pool.submit(pngBuffer, blpPath);
}

export async function shutdownBlpWorkerPool(): Promise<void> {
  if (!singletonPool) return;
  const pool = singletonPool;
  singletonPool = null;
  await pool.shutdown();
}
