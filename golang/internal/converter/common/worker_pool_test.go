package common

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPoolCancellationJoinsRunningWorkAndSkipsQueuedTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	var queuedRan atomic.Bool
	result := make(chan error, 1)
	go func() {
		result <- WorkerPoolContext(ctx, 1, []func() error{
			func() error { close(started); <-release; return nil },
			func() error { queuedRan.Store(true); return nil },
		})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first worker did not start")
	}
	cancel()
	select {
	case <-result:
		t.Fatal("pool returned before running worker completed")
	default:
	}
	close(release)
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("pool error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pool did not finish after worker returned")
	}
	if queuedRan.Load() {
		t.Fatal("queued work ran after cancellation")
	}
}
