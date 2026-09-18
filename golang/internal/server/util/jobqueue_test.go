package util

import (
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
	secondRan := make(chan struct{}, 1)
	queue := NewJobQueue(QueueConfig[string, string]{
		Concurrency: 1,
		JobTTL:      time.Minute,
		JobTimeout:  time.Second,
	}, func(job *Job[string, string]) (string, error) {
		if job.ID == "first" {
			<-releaseFirst
		} else {
			secondRan <- struct{}{}
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
	close(releaseFirst)
	waitForJobStatus(t, queue, "pending", JobCancelled)
	select {
	case <-secondRan:
		t.Fatal("cancelled pending job ran")
	case <-time.After(10 * time.Millisecond):
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
