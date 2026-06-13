import fs from 'fs';
import { cpus } from 'os';
import path from 'path';
import { fileURLToPath } from 'url';
import { Worker } from 'worker_threads';

import { bundledAppRoot } from '@/lib/wow-data-server/transport';

export type BlpTaskInput = {
  data: Buffer,
  kind: 'png' | 'blp2',
  resizeTo?: { width: number, height: number },
};

type Task = {
  id: number,
  input: BlpTaskInput,
  blpPath: string,
  resolve: () => void,
  reject: (e: Error) => void,
};

type WorkerTaskMessage = {
  type: 'task',
  id: number,
  arrayBuffer: ArrayBuffer,
  byteOffset: number,
  byteLength: number,
  kind: 'png' | 'blp2',
  resizeTo?: { width: number, height: number },
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
  const candidates = [
    path.join(process.cwd(), 'blp.worker.js'),
    path.join(bundledAppRoot(), 'blp.worker.js'),
    path.join(path.dirname(fileURLToPath(import.meta.url)), 'blp.worker.js'),
    path.join(process.cwd(), 'blp.worker.ts'),
    path.join(bundledAppRoot(), 'blp.worker.ts'),
    path.join(path.dirname(fileURLToPath(import.meta.url)), 'blp.worker.ts'),
  ];

  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }

  return path.join(bundledAppRoot(), 'blp.worker.js');
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

    worker.on('error', (err) => {
      console.log('BLB worker restarted due to error:', err);

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
      const { data } = task.input;
      const arrayBuffer: ArrayBuffer = data.buffer as ArrayBuffer;
      const message: WorkerTaskMessage = {
        type: 'task',
        id: task.id,
        arrayBuffer,
        byteOffset: data.byteOffset,
        byteLength: data.byteLength,
        kind: task.input.kind,
        resizeTo: task.input.resizeTo,
        blpPath: task.blpPath,
      };
      // Transfer ownership of the ArrayBuffer to avoid copying memory
      w.postMessage(message, [arrayBuffer]);
    }
  }

  submit(input: BlpTaskInput, blpPath: string): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      const task: Task = {
        id: this.nextTaskId++,
        input,
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

export function submitBlpTask(input: BlpTaskInput, blpPath: string): Promise<void> {
  const pool = ensureBlpWorkerPool();
  return pool.submit(input, blpPath);
}

export async function shutdownBlpWorkerPool(): Promise<void> {
  if (!singletonPool) return;
  const pool = singletonPool;
  singletonPool = null;
  await pool.shutdown();
}
