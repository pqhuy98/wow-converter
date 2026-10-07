package blp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCancelledSubmissionDoesNotWaitForEncoderCapacityOrWrite(t *testing.T) {
	pool := &WorkerPool{sem: make(chan struct{}, 1)}
	pool.sem <- struct{}{} // An existing encoder owns the only slot.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "cancelled.blp")
	result := make(chan error, 1)
	go func() { result <- pool.SubmitContext(ctx, TaskInput{Kind: "png"}, path) }()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("submission error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled submission waited for encoder capacity")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled output exists: %v", err)
	}
	if len(pool.sem) != 1 {
		t.Fatal("cancelled submission released another encoder's slot")
	}
}
