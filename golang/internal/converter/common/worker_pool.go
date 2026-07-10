package common

import "sync"

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
	concurrency = boundedConcurrency(concurrency, len(tasks))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for _, task := range tasks {
		wg.Add(1)
		sem <- struct{}{}
		go func(fn func() error) {
			defer wg.Done()
			defer func() { <-sem }()
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
	return firstErr
}
