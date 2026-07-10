package blp

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// TaskInput is one BLP conversion task.
type TaskInput struct {
	Data     []byte
	Kind     string // "png" or "blp2"
	ResizeTo *Size
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
	p.sem <- struct{}{}
	defer func() { <-p.sem }()

	encode := EncodeInput{ResizeTo: input.ResizeTo}
	switch input.Kind {
	case "png":
		encode.PNG = input.Data
	case "blp2":
		encode.BLP2 = input.Data
	}
	return ConvertTextureToBlp(encode, blpPath)
}

// SubmitBlpTask submits a task through the worker pool.
func SubmitBlpTask(input TaskInput, blpPath string) error {
	return EnsureWorkerPool(0).Submit(input, blpPath)
}

// ShutdownWorkerPool stops native bridge workers used by the encoder.
func ShutdownWorkerPool() {
	ShutdownNativePool()
}

// EnsureInlinePool is an alias for EnsureWorkerPool (legacy name).
func EnsureInlinePool(desiredSize int) *WorkerPool {
	return EnsureWorkerPool(desiredSize)
}

// GetInlinePoolSize is an alias for GetWorkerPoolSize (legacy name).
func GetInlinePoolSize() int {
	return GetWorkerPoolSize()
}

// ShutdownInlinePool is an alias for ShutdownWorkerPool (legacy name).
func ShutdownInlinePool() {
	ShutdownWorkerPool()
}
