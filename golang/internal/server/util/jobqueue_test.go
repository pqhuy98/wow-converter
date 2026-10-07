package util

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFailedJobCanPreserveReturnedResult(t *testing.T) {
	queue := NewJobQueue(QueueConfig[string, string]{
		Concurrency: 1,
		JobTTL:      time.Minute,
		JobTimeout:  time.Second,
	}, func(job *Job[string, string]) (string, error) {
		job.PreserveResultOnError()
		return "partial result", errors.New("partial failure")
	})
	queue.AddJob(&Job[string, string]{
		ID: "test", Request: "request", Status: JobPending, SubmittedAt: time.Now().UnixMilli(),
	})

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := queue.GetJobStatus("test")
		if status != nil && status.Status == JobFailed {
			if status.Result == nil || *status.Result != "partial result" {
				t.Fatalf("failed job result = %v, want partial result", status.Result)
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("job did not fail before deadline")
}

func TestCancelJobStopsProcessingJob(t *testing.T) {
	started := make(chan struct{})
	queue := NewJobQueue(QueueConfig[string, string]{
		Concurrency: 1,
		JobTTL:      time.Minute,
		JobTimeout:  time.Second,
	}, func(job *Job[string, string]) (string, error) {
		close(started)
		<-job.Context().Done()
		return "", job.Context().Err()
	})
	queue.AddJob(&Job[string, string]{
		ID: "processing", Request: "request", Status: JobPending, SubmittedAt: time.Now().UnixMilli(),
	})
	<-started

	if !queue.CancelJob("processing") {
		t.Fatal("CancelJob returned false for processing job")
	}
	waitForJobStatus(t, queue, "processing", JobCancelled)
}

func TestCancelJobSkipsPendingJob(t *testing.T) {
	releaseFirst := make(chan struct{})
	lastStarted := make(chan struct{})
	secondRan := make(chan struct{}, 1)
	queue := NewJobQueue(QueueConfig[string, string]{
		Concurrency: 1,
		JobTTL:      time.Minute,
		JobTimeout:  time.Second,
	}, func(job *Job[string, string]) (string, error) {
		if job.ID == "first" {
			<-releaseFirst
		} else if job.ID == "pending" {
			secondRan <- struct{}{}
		} else {
			close(lastStarted)
		}
		return "", nil
	})
	queue.AddJob(&Job[string, string]{
		ID: "first", Request: "request", Status: JobPending, SubmittedAt: time.Now().UnixMilli(),
	})
	queue.AddJob(&Job[string, string]{
		ID: "pending", Request: "request", Status: JobPending, SubmittedAt: time.Now().UnixMilli(),
	})

	if !queue.CancelJob("pending") {
		t.Fatal("CancelJob returned false for pending job")
	}
	queue.AddJob(&Job[string, string]{ID: "last", Status: JobPending})
	if status := queue.GetJobStatus("last"); status.Position == nil || *status.Position != 1 {
		t.Fatalf("cancelled pending job inflated queue position: %+v", status)
	}
	if pending, processing := queue.GetQueueSnapshot(); pending != 1 || processing != 1 {
		t.Fatalf("queue snapshot counted cancelled pending job: (%d, %d)", pending, processing)
	}
	close(releaseFirst)
	waitSignal(t, lastStarted)
	waitForJobStatus(t, queue, "pending", JobCancelled)
	select {
	case <-secondRan:
		t.Fatal("cancelled pending job ran")
	default:
	}
}

func TestJobTimeoutOverride(t *testing.T) {
	queue := NewJobQueue(QueueConfig[string, string]{
		Concurrency: 1,
		JobTTL:      time.Minute,
		JobTimeout:  time.Second,
	}, func(job *Job[string, string]) (string, error) {
		<-job.Context().Done()
		return "done", nil // A late success must not override the deadline.
	})
	queue.AddJob(&Job[string, string]{
		ID: "short", Request: "request", Status: JobPending, SubmittedAt: time.Now().UnixMilli(),
		Timeout: 50 * time.Millisecond,
	})
	waitForJobStatus(t, queue, "short", JobFailed)
	status := queue.GetJobStatus("short")
	if status == nil || status.Error != "Job timeout" {
		t.Fatalf("short job error = %v, want Job timeout", status)
	}
}

func TestTimeoutRetainsSlotUntilHandlerReturns(t *testing.T) {
	started := make(chan struct{})
	timedOut := make(chan struct{})
	release := make(chan struct{})
	nextStarted := make(chan struct{})
	completed := make(chan string, 2)
	queue := NewJobQueue(QueueConfig[string, string]{
		Concurrency: 1, MaxPendingJobs: 10, JobTTL: time.Minute, JobTimeout: time.Hour,
		OnTimeout:   func() { close(timedOut) },
		OnCompleted: func(job *Job[string, string]) { completed <- job.ID },
	}, func(job *Job[string, string]) (string, error) {
		if job.ID == "slow" {
			if _, ok := job.Context().Deadline(); !ok {
				t.Error("handler context has no deadline")
			}
			close(started)
			<-job.Context().Done()
			if !errors.Is(job.Context().Err(), context.DeadlineExceeded) {
				t.Errorf("context error = %v, want deadline exceeded", job.Context().Err())
			}
			<-release // Simulates an encoder that cannot interrupt physical work.
			return "late success", nil
		}
		close(nextStarted)
		return "retry succeeded", nil
	})
	queue.AddJob(&Job[string, string]{ID: "slow", Status: JobPending, Timeout: 10 * time.Millisecond, AddToRecent: true})
	waitSignal(t, started)
	queue.AddJob(&Job[string, string]{ID: "retry", Status: JobPending})
	waitSignal(t, timedOut)
	status := queue.GetJobStatus("slow")
	if status.Status != JobProcessing || status.FinishedAt != nil || status.Error != "Job timeout; stopping export" {
		t.Fatalf("timed-out handler still running: %+v", status)
	}
	if pending, processing := queue.GetQueueSnapshot(); pending != 1 || processing != 1 {
		t.Fatalf("queue snapshot = (%d, %d), want (1, 1)", pending, processing)
	}
	select {
	case <-nextStarted:
		t.Fatal("retry started before original handler returned")
	default:
	}
	close(release)
	waitSignal(t, nextStarted)
	status = queue.GetJobStatus("slow")
	if status.Status != JobFailed || status.Error != "Job timeout" || status.Result != nil || status.FinishedAt == nil {
		t.Fatalf("late success overwrote timeout: %+v", status)
	}
	select {
	case id := <-completed:
		if id != "retry" {
			t.Fatalf("completion callback for %q, want retry only", id)
		}
	case <-time.After(time.Second):
		t.Fatal("retry did not complete")
	}
	if retry := queue.GetJobStatus("retry"); retry.Status != JobDone || retry.Result == nil || *retry.Result != "retry succeeded" {
		t.Fatalf("retry result: %+v", retry)
	}
	if pending, processing := queue.GetQueueSnapshot(); pending != 0 || processing != 0 {
		t.Fatalf("completed queue retained work: (%d, %d)", pending, processing)
	}
	if queue.CompletedResult("slow") != nil || len(queue.RecentCompletedJobs) != 0 {
		t.Fatal("timeout was retained as a successful export")
	}
}

func TestCancelRetainsSlotUntilHandlerReturns(t *testing.T) {
	started, cancelled, release, nextStarted := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	queue := NewJobQueue(QueueConfig[string, string]{Concurrency: 1, JobTTL: time.Minute, JobTimeout: time.Hour}, func(job *Job[string, string]) (string, error) {
		if job.ID == "cancel" {
			if _, ok := job.Context().Deadline(); ok {
				t.Error("NoTimeout handler has a deadline")
			}
			close(started)
			<-job.Context().Done()
			close(cancelled)
			<-release
			return "late success", nil
		}
		close(nextStarted)
		return "done", nil
	})
	queue.AddJob(&Job[string, string]{ID: "cancel", Status: JobPending, NoTimeout: true})
	waitSignal(t, started)
	queue.AddJob(&Job[string, string]{ID: "next", Status: JobPending})
	if !queue.CancelJob("cancel") {
		t.Fatal("processing job could not be cancelled")
	}
	waitSignal(t, cancelled)
	if status := queue.GetJobStatus("cancel"); status.Status != JobProcessing || status.FinishedAt != nil {
		t.Fatalf("cancelled handler finished before returning: %+v", status)
	}
	select {
	case <-nextStarted:
		t.Fatal("next handler overlapped cancelled physical work")
	default:
	}
	close(release)
	waitSignal(t, nextStarted)
	if status := queue.GetJobStatus("cancel"); status.Status != JobCancelled || status.Result != nil || status.Error != "Export halted" {
		t.Fatalf("cancelled late success status: %+v", status)
	}
}

func waitSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for handler synchronization")
	}
}

func waitForJobStatus[T, V any](t *testing.T, queue *JobQueue[T, V], id string, want JobStatus) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status := queue.GetJobStatus(id)
		if status != nil && status.Status == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("job %q did not reach status %q", id, want)
}
