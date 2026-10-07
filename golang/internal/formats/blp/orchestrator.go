package blp

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// TaskInput is one BLP conversion task.
type TaskInput struct {
	Data          []byte
	Kind          string // "png" or "blp2"
	ResizeTo      *Size
	Opaque        bool
	PreserveAlpha bool
	IgnoreAlpha   bool
}

// WorkerPool runs BLP conversions with bounded parallelism.
type WorkerPool struct {
	sem chan struct{}
}

var (
	singletonPool *WorkerPool
	poolOnce      sync.Once
)

func defaultPoolSize() int {
	n := runtime.NumCPU()
	if n <= 1 {
		return 1
	}
	return n - 1
}

func workersFromEnv() int {
	raw := strings.TrimSpace(os.Getenv("BLP_WORKERS"))
	if raw == "" {
		return defaultPoolSize()
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// EnsureWorkerPool returns the singleton conversion pool.
func EnsureWorkerPool(desiredSize int) *WorkerPool {
	poolOnce.Do(func() {
		size := desiredSize
		if size < 1 {
			size = workersFromEnv()
		}
		singletonPool = &WorkerPool{
			sem: make(chan struct{}, size),
		}
	})
	return singletonPool
}

// GetWorkerPoolSize returns the configured pool size.
func GetWorkerPoolSize() int {
	if singletonPool == nil {
		return 0
	}
	return cap(singletonPool.sem)
}

// Submit runs one conversion task with pool concurrency limits.
func (p *WorkerPool) Submit(input TaskInput, blpPath string) error {
	return p.SubmitContext(context.Background(), input, blpPath)
}

// SubmitContext cancels waiting tasks; an encoder already running is joined.
func (p *WorkerPool) SubmitContext(ctx context.Context, input TaskInput, blpPath string) error {
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-p.sem }()
	if err := ctx.Err(); err != nil {
		return err
	}

	encode := EncodeInput{ResizeTo: input.ResizeTo, Opaque: input.Opaque, PreserveAlpha: input.PreserveAlpha, IgnoreAlpha: input.IgnoreAlpha}
	switch input.Kind {
	case "png":
		encode.PNG = input.Data
	case "blp2":
		encode.BLP2 = input.Data
	}
	err := ConvertTextureToBlp(encode, blpPath)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// SubmitBlpTask submits a task through the worker pool.
func SubmitBlpTask(input TaskInput, blpPath string) error {
	return EnsureWorkerPool(0).Submit(input, blpPath)
}

// SubmitBlpTaskContext submits cancellable work through the shared encoder pool.
func SubmitBlpTaskContext(ctx context.Context, input TaskInput, blpPath string) error {
	return EnsureWorkerPool(0).SubmitContext(ctx, input, blpPath)
}

// ShutdownWorkerPool stops native bridge workers used by the encoder.
func ShutdownWorkerPool() {
	ShutdownNativePool()
}
