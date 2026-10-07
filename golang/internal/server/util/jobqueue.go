package util

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/pqhuy98/wow-converter/internal/ansi"
)

// JobStatus is the lifecycle state of a queued job.
type JobStatus string

const (
	JobPending    JobStatus = "pending"
	JobProcessing JobStatus = "processing"
	JobDone       JobStatus = "done"
	JobFailed     JobStatus = "failed"
	JobCancelled  JobStatus = "cancelled"
)

// Job is a unit of work in the queue.
type Job[T, V any] struct {
	ID            string        `json:"id"`
	Request       T             `json:"request"`
	Status        JobStatus     `json:"status"`
	Result        *V            `json:"result,omitempty"`
	Error         string        `json:"error,omitempty"`
	SubmittedAt   int64         `json:"submittedAt"`
	StartedAt     *int64        `json:"startedAt,omitempty"`
	FinishedAt    *int64        `json:"finishedAt,omitempty"`
	AddToRecent   bool          `json:"addToRecent,omitempty"`
	NoTimeout     bool          `json:"noTimeout,omitempty"`
	Timeout       time.Duration `json:"-"`
	resultOnError bool
	ctx           context.Context
	cancel        context.CancelFunc
}

// Context is cancelled on the processing deadline or when CancelJob is called.
func (j *Job[T, V]) Context() context.Context {
	if j.ctx == nil {
		return context.Background()
	}
	return j.ctx
}

// PreserveResultOnError asks the queue to publish the handler's returned
// result even when the handler also returns an error.
func (j *Job[T, V]) PreserveResultOnError() {
	j.resultOnError = true
}

// JobStatusView is the public status payload for API responses.
type JobStatusView[V any] struct {
	ID          string    `json:"id"`
	Status      JobStatus `json:"status"`
	Position    *int      `json:"position,omitempty"`
	Result      *V        `json:"result,omitempty"`
	Error       string    `json:"error,omitempty"`
	SubmittedAt int64     `json:"submittedAt"`
	StartedAt   *int64    `json:"startedAt,omitempty"`
	FinishedAt  *int64    `json:"finishedAt,omitempty"`
}

// QueueConfig configures queue behavior.
type QueueConfig[T, V any] struct {
	Concurrency    int
	MaxPendingJobs int
	JobTTL         time.Duration
	JobTimeout     time.Duration
	OnCompleted    func(*Job[T, V])
	// OnTimeout is for job-local cleanup only. It must not reset or restart the
	// wow-data-server because bundled mode shares its in-process CASC runtime.
	OnTimeout func()
}

// JobQueue processes jobs with bounded concurrency.
type JobQueue[T, V any] struct {
	config QueueConfig[T, V]
	handle func(*Job[T, V]) (V, error)

	mu                  sync.Mutex
	pending             []*Job[T, V]
	queueHead           int
	pendingIndex        map[string]int
	jobs                map[string]*Job[T, V]
	activeJobs          int
	RecentCompletedJobs []*Job[T, V]
}

// NewJobQueue creates a job queue and starts TTL cleanup.
func NewJobQueue[T, V any](config QueueConfig[T, V], handle func(*Job[T, V]) (V, error)) *JobQueue[T, V] {
	q := &JobQueue[T, V]{
		config:       config,
		handle:       handle,
		pendingIndex: make(map[string]int),
		jobs:         make(map[string]*Job[T, V]),
	}
	go q.cleanupLoop()
	return q
}

func (q *JobQueue[T, V]) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now().UnixMilli()
		q.mu.Lock()
		for id, job := range q.jobs {
			if (job.Status == JobDone || job.Status == JobFailed || job.Status == JobCancelled) && job.FinishedAt != nil {
				if now-*job.FinishedAt > q.config.JobTTL.Milliseconds() {
					delete(q.jobs, id)
				}
			}
		}
		q.mu.Unlock()
	}
}

// AddJob enqueues a job and starts processing when possible.
func (q *JobQueue[T, V]) AddJob(job *Job[T, V]) {
	q.mu.Lock()
	job.ctx, job.cancel = context.WithCancel(context.Background())
	q.pending = append(q.pending, job)
	q.pendingIndex[job.ID] = len(q.pending) - 1
	q.jobs[job.ID] = job
	q.mu.Unlock()
	q.tryProcessQueue()
}

// CancelJob cancels a pending or processing job.
func (q *JobQueue[T, V]) CancelJob(id string) bool {
	q.mu.Lock()
	job, ok := q.jobs[id]
	if !ok || (job.Status != JobPending && job.Status != JobProcessing) {
		q.mu.Unlock()
		return false
	}
	job.cancel()
	if job.Status == JobPending {
		delete(q.pendingIndex, id)
		job.Status = JobCancelled
		job.Error = "Export halted"
		now := time.Now().UnixMilli()
		job.FinishedAt = &now
	}
	q.mu.Unlock()
	q.tryProcessQueue()
	return true
}

// GetJob returns a job by ID.

// CompletedResult also finds retained recent exports after the short status TTL.
func (q *JobQueue[T, V]) CompletedResult(id string) *V {
	q.mu.Lock()
	defer q.mu.Unlock()
	if job := q.jobs[id]; job != nil && job.Status == JobDone {
		return job.Result
	}
	for _, job := range q.RecentCompletedJobs {
		if job.ID == id && job.Status == JobDone {
			return job.Result
		}
	}
	return nil
}

func (q *JobQueue[T, V]) OutstandingCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.activeJobs + len(q.pendingIndex)
}

func (q *JobQueue[T, V]) GetJob(id string) *Job[T, V] {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.jobs[id]
}

// GetJobStatus returns the public status view for a job.
func (q *JobQueue[T, V]) GetJobStatus(id string) *JobStatusView[V] {
	q.mu.Lock()
	defer q.mu.Unlock()
	job, ok := q.jobs[id]
	if !ok {
		return nil
	}
	pos := q.jobPositionLocked(id)
	return &JobStatusView[V]{
		ID:          id,
		Status:      job.Status,
		Position:    pos,
		Result:      job.Result,
		Error:       job.Error,
		SubmittedAt: job.SubmittedAt,
		StartedAt:   job.StartedAt,
		FinishedAt:  job.FinishedAt,
	}
}

func (q *JobQueue[T, V]) jobPositionLocked(id string) *int {
	idx, ok := q.pendingIndex[id]
	if !ok {
		return nil
	}
	pos := 0
	for i := q.queueHead; i <= idx; i++ {
		if q.pending[i].Status == JobPending {
			pos++
		}
	}
	return &pos
}

// GetQueueSnapshot returns pending and processing counts.
func (q *JobQueue[T, V]) GetQueueSnapshot() (pendingCount, processingCount int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	pendingCount = len(q.pendingIndex)
	return pendingCount, q.activeJobs
}

// ListActiveJobIDs returns IDs of pending or processing jobs sorted by submission time.
func (q *JobQueue[T, V]) ListActiveJobIDs() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var ids []string
	for _, job := range q.jobs {
		if job.Status == JobPending || job.Status == JobProcessing {
			ids = append(ids, job.ID)
		}
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if q.jobs[ids[i]].SubmittedAt > q.jobs[ids[j]].SubmittedAt {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	return ids
}

func (q *JobQueue[T, V]) tryProcessQueue() {
	q.mu.Lock()
	if q.queueHead > q.config.MaxPendingJobs {
		q.pending = q.pending[q.queueHead:]
		q.queueHead = 0
		q.pendingIndex = make(map[string]int)
		for i, job := range q.pending {
			if job.Status == JobPending {
				q.pendingIndex[job.ID] = i
			}
		}
	}
	for q.activeJobs < q.config.Concurrency && q.queueHead < len(q.pending) {
		job := q.pending[q.queueHead]
		q.queueHead++
		delete(q.pendingIndex, job.ID)
		if job.Status != JobPending {
			continue
		}
		q.activeJobs++
		job.Status = JobProcessing
		now := time.Now().UnixMilli()
		job.StartedAt = &now
		if !job.NoTimeout {
			timeout := q.config.JobTimeout
			if job.Timeout > 0 {
				timeout = job.Timeout
			}
			parentCancel := job.cancel
			var deadlineCancel context.CancelFunc
			job.ctx, deadlineCancel = context.WithTimeout(job.ctx, timeout)
			job.cancel = func() {
				parentCancel()
				deadlineCancel()
			}
		}
		go q.runJob(job)
	}
	q.mu.Unlock()
}

func (q *JobQueue[T, V]) runJob(job *Job[T, V]) {
	// Cancellation requests stopping; it does not release the worker slot.
	// Native encoders and other uninterruptible stages must finish before a
	// replacement job can start or a terminal status can be published.
	finished := make(chan struct{})
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		select {
		case <-job.Context().Done():
			q.mu.Lock()
			if errors.Is(job.Context().Err(), context.DeadlineExceeded) {
				job.Error = "Job timeout; stopping export"
			} else {
				job.Error = "Stopping export"
			}
			q.mu.Unlock()
			if errors.Is(job.Context().Err(), context.DeadlineExceeded) && q.config.OnTimeout != nil {
				q.config.OnTimeout()
			}
		case <-finished:
		}
	}()
	result, err := q.handle(job)
	close(finished)
	<-watcherDone
	q.mu.Lock()
	defer q.mu.Unlock()
	contextErr := job.Context().Err()
	job.cancel() // Release the deadline timer after physical work completes.
	if errors.Is(contextErr, context.DeadlineExceeded) {
		err = errJobTimeout
	}

	now := time.Now().UnixMilli()
	job.FinishedAt = &now
	if errors.Is(contextErr, context.Canceled) {
		job.Status = JobCancelled
		job.Error = "Export halted"
		log.Printf("Job cancelled: %s", job.ID)
	} else if err != nil {
		job.Status = JobFailed
		job.Error = err.Error()
		if job.resultOnError {
			job.Result = &result
		}
		log.Printf("%s", ansi.Redf("Job failed: %s: %v", job.ID, err))
	} else {
		job.Status = JobDone
		job.Error = ""
		job.Result = &result
		if job.AddToRecent {
			q.RecentCompletedJobs = append([]*Job[T, V]{job}, q.RecentCompletedJobs...)
			if len(q.RecentCompletedJobs) > 500 {
				q.RecentCompletedJobs = q.RecentCompletedJobs[:500]
			}
		}
		if q.config.OnCompleted != nil {
			q.config.OnCompleted(job)
		}
	}
	q.activeJobs--
	go q.tryProcessQueue()
}

var errJobTimeout = &timeoutError{}

type timeoutError struct{}

func (e *timeoutError) Error() string { return "Job timeout" }
