package util

import (
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
)

// Job is a unit of work in the queue.
type Job[T, V any] struct {
	ID          string
	Request     T
	Status      JobStatus
	Result      *V
	Error       string
	SubmittedAt int64
	StartedAt   *int64
	FinishedAt  *int64
	AddToRecent bool
	NoTimeout   bool
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
	Concurrency          int
	MaxPendingJobs       int
	JobTTL               time.Duration
	JobTimeout           time.Duration
	OnCompleted          func(*Job[T, V])
	OnTimeout            func()
}

// JobQueue processes jobs with bounded concurrency.
type JobQueue[T, V any] struct {
	config QueueConfig[T, V]
	handle func(*Job[T, V]) (V, error)

	mu                 sync.Mutex
	pending            []*Job[T, V]
	queueHead          int
	pendingIndex       map[string]int
	jobs               map[string]*Job[T, V]
	activeJobs         int
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
			if (job.Status == JobDone || job.Status == JobFailed) && job.FinishedAt != nil {
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
	q.pending = append(q.pending, job)
	q.pendingIndex[job.ID] = len(q.pending) - 1
	q.jobs[job.ID] = job
	q.mu.Unlock()
	q.tryProcessQueue()
}

// GetJob returns a job by ID.
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
	pos := idx - q.queueHead + 1
	return &pos
}

// GetQueueSnapshot returns pending and processing counts.
func (q *JobQueue[T, V]) GetQueueSnapshot() (pendingCount, processingCount int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	pendingCount = len(q.pending) - q.queueHead
	if pendingCount < 0 {
		pendingCount = 0
	}
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
			q.pendingIndex[job.ID] = i
		}
	}
	for q.activeJobs < q.config.Concurrency && q.queueHead < len(q.pending) {
		job := q.pending[q.queueHead]
		q.queueHead++
		delete(q.pendingIndex, job.ID)
		q.activeJobs++
		job.Status = JobProcessing
		now := time.Now().UnixMilli()
		job.StartedAt = &now
		go q.runJob(job)
	}
	q.mu.Unlock()
}

func (q *JobQueue[T, V]) runJob(job *Job[T, V]) {
	var result V
	var err error

	if job.NoTimeout {
		result, err = q.handle(job)
	} else {
		type outcome struct {
			result V
			err    error
		}
		ch := make(chan outcome, 1)
		go func() {
			r, e := q.handle(job)
			ch <- outcome{r, e}
		}()
		select {
		case out := <-ch:
			result, err = out.result, out.err
		case <-time.After(q.config.JobTimeout):
			err = errJobTimeout
			if q.config.OnTimeout != nil {
				q.config.OnTimeout()
			}
		}
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now().UnixMilli()
	job.FinishedAt = &now
	if err != nil {
		job.Status = JobFailed
		job.Error = err.Error()
		log.Printf("%s", ansi.Redf("Job failed: %s: %v", job.ID, err))
	} else {
		job.Status = JobDone
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
