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
