type JobStatus = 'pending' | 'processing' | 'done' | 'failed';

export interface Job<T, V> {
  id: string;
  request: T;
  status: JobStatus;
  result?: V;
  error?: string;
  submittedAt: number;
  startedAt?: number;
  finishedAt?: number; // timestamp when job reached a terminal state
  isDemo?: boolean;
}

export interface QueueConfig<T, V> {
  concurrency: number;
  maxPendingJobs: number;
  jobTTL: number;
  jobTimeout: number;
  jobCompletedCallback?: (job: Job<T, V>) => void;
}

export class JobQueue<T, V> {
  private pendingQueue: Job<T, V>[] = [];

  private queueHead = 0;

  private pendingIndexMap = new Map<string, number>();

  private jobsMap = new Map<string, Job<T, V>>();

  private activeJobs = 0;

  private jobsDone = 0;

  private jobsFailed = 0;

  public recentJobs: Job<T, V>[] = [];

  constructor(
    private config: QueueConfig<T, V>,
    private handler: (job: Job<T, V>) => Promise<V>,
  ) {
    setInterval(() => {
      const now = Date.now();
      for (const [id, job] of this.jobsMap) {
        if (job.isDemo) {
          continue;
        }
        if ((job.status === 'done' || job.status === 'failed') && job.finishedAt && now - job.finishedAt > this.config.jobTTL) {
          this.jobsMap.delete(id);
        }
      }
    }, 60_000);
  }

  public addJob(job: Job<T, V>) {
    this.pendingQueue.push(job);
    this.pendingIndexMap.set(job.id, this.pendingQueue.length - 1);
    this.jobsMap.set(job.id, job);
    this.tryProcessQueue();
  }

  public getJobStatus(jobId: string) {
    const job = this.jobsMap.get(jobId);
    if (!job) {
      return undefined;
    }

    return {
      id: jobId,
      status: job.status,
      position: this.getJobPosition(jobId),
      result: job.result,
      error: job.error,
      submittedAt: job.submittedAt,
      startedAt: job.startedAt,
      finishedAt: job.finishedAt,
    };
  }

  public getJob(jobId: string) {
    return this.jobsMap.get(jobId);
  }

  public getJobPosition(jobId: string) {
    const index = this.pendingIndexMap.get(jobId);
    return index ? index - this.queueHead + 1 : undefined;
  }

  private tryProcessQueue() {
    if (this.queueHead > this.config.maxPendingJobs) {
      this.pendingQueue = this.pendingQueue.slice(this.queueHead);
      this.queueHead = 0;
    }
    while (this.activeJobs < this.config.concurrency && this.queueHead < this.pendingQueue.length) {
      const job = this.pendingQueue[this.queueHead];
      this.queueHead++;
      this.pendingIndexMap.delete(job.id);
      this.activeJobs++;
      job.status = 'processing';

      void (async () => {
        job.startedAt = Date.now();
        try {
          if (job.isDemo) {
            job.result = await this.handler(job);
          } else {
            job.result = await Promise.race<V>([
              this.handler(job),
              new Promise<V>((_, reject) => {
                setTimeout(() => reject(new Error('Job timeout')), this.config.jobTimeout);
              }),
            ]);
          }
          job.status = 'done';
          job.finishedAt = Date.now();
          this.jobsDone++;
          if (this.config.jobCompletedCallback) {
            this.config.jobCompletedCallback(job);
          }
        } catch (err) {
          job.status = 'failed';
          job.error = err instanceof Error ? err.message : String(err);
          job.finishedAt = Date.now();
          console.error(err);
          this.jobsFailed++;
        } finally {
          this.recentJobs.push(job);
          this.recentJobs.sort((a, b) => b.submittedAt - a.submittedAt);
          if (this.recentJobs.length > 50) {
            this.recentJobs.shift();
          }
          this.activeJobs--;
          this.tryProcessQueue();
        }
      })();
    }
  }
}
