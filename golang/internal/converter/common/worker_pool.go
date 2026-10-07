package common

import (
	"context"
	"sync"
)

// boundedConcurrency caps a worker limit to the number of jobs (min 1 when jobs > 0).
func boundedConcurrency(limit, jobs int) int {
	if jobs < 1 {
		return 1
	}
	if limit < 1 {
		limit = 1
	}
	if limit > jobs {
		return jobs
	}
	return limit
}

// WorkerPool runs tasks with bounded concurrency.
func WorkerPool(concurrency int, tasks []func() error) error {
	return WorkerPoolContext(context.Background(), concurrency, tasks)
}

// WorkerPoolContext stops scheduling on cancellation and waits for started tasks.
func WorkerPoolContext(ctx context.Context, concurrency int, tasks []func() error) error {
	concurrency = boundedConcurrency(concurrency, len(tasks))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
schedule:
	for _, task := range tasks {
		if ctx.Err() != nil {
			break
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break schedule
		}
		wg.Add(1)
		go func(fn func() error) {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			if err := fn(); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}(task)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	return firstErr
}
