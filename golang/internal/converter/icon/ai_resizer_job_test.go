package icon

import (
	"sync"
	"testing"
	"time"
)

func TestResizeJobBroadcastsToAllWaiters(t *testing.T) {
	job := &resizeJob{done: make(chan struct{})}

	const waiters = 8
	var wg sync.WaitGroup
	wg.Add(waiters)
	errs := make(chan error, waiters)

	for i := 0; i < waiters; i++ {
		go func() {
			defer wg.Done()
			path, err := job.wait()
			if err != nil {
				errs <- err
				return
			}
			if path != "/tmp/out.png" {
				errs <- err
			}
		}()
	}

	time.Sleep(10 * time.Millisecond)
	job.finish(resizeResult{path: "/tmp/out.png"})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("waiters did not all unblock")
	}

	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("waiter error: %v", err)
		}
	}
}
