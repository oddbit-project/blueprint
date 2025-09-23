package threadpool

import (
	"context"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/utils"
	"time"
)

const (
	ErrInvalidWorkerCount = utils.Error("Invalid workerCount value")
	ErrInvalidQueueSize   = utils.Error("Invalid queueSize value")
	ErrPoolNotStarted     = utils.Error("ThreadPool not started")
	ErrPoolAlreadyStarted = utils.Error("ThreadPool already started")
)

type Pool interface {
	Stop() error
	Dispatch(j Job)
	TryDispatch(j Job) bool
	DispatchWithTimeout(j Job, timeout time.Duration) bool
	DispatchWithContext(ctx context.Context, j Job) error
	Start(ctx context.Context) error
}

type ThreadPool struct {
	workers     *WorkerGroup
	workerCount int
	jobQueue    chan Job
	logger      *log.Logger
}

type OptionsFn func(*ThreadPool)

// WithLogger adds a logger to the threadpool
func WithLogger(logger *log.Logger) OptionsFn {
	return func(t *ThreadPool) {
		t.logger = logger
	}
}

// NewThreadPool is a constructor function that creates a new ThreadPool instance. It takes in parameters:
// - workerCount: the number of workers to be created in the ThreadPool. Must be greater than 0. If it's less than 1, it returns ErrInvalidWorkerCount.
// - queueSize: the size of the job queue in the ThreadPool. Must be greater than 0. If it's less than 1, it returns ErrInvalidQueueSize.
// - opts: optional functional options like WithLogger
// It returns a pointer to the created ThreadPool and an error.
//
// Example usage:
// pool, err := NewThreadPool(5, 10)
//
//	if err != nil {
//	  // handle error
//	}
//
// // Or with options:
// logger := log.NewLogger()
// pool, err := NewThreadPool(5, 10, WithLogger(logger))
//
// pool.Dispatch(job)
// ...
func NewThreadPool(workerCount int, queueSize int, opts ...OptionsFn) (*ThreadPool, error) {
	if workerCount < 1 {
		return nil, ErrInvalidWorkerCount
	}
	if queueSize < 1 {
		return nil, ErrInvalidQueueSize
	}
	pool := &ThreadPool{
		workers:     nil,
		workerCount: workerCount,
		jobQueue:    make(chan Job, queueSize),
	}
	for _, opt := range opts {
		opt(pool)
	}
	return pool, nil
}

// GetRequestCount returns the total number of requests handled by the ThreadPool.
// If the ThreadPool has not been started, it returns 0.
// It internally calls the RequestCount method of the workers in the ThreadGroup to calculate the total number of requests.
func (t *ThreadPool) GetRequestCount() uint64 {
	if t.workers == nil {
		return 0
	}
	return t.workers.RequestCount()
}

// GetQueueLen returns the number of jobs currently in the jobQueue of the ThreadPool.
// It calculates the size of the jobQueue by using the len() function on the jobQueue slice.
// The returned value represents the number of pending jobs waiting to be processed by the workers.
func (t *ThreadPool) GetQueueLen() int {
	return len(t.jobQueue)
}

// GetQueueCapacity returns the capacity of the job queue in the ThreadPool.
func (t *ThreadPool) GetQueueCapacity() int {
	return cap(t.jobQueue)
}

// GetWorkerCount returns the current number of workers in the ThreadPool.
// It retrieves the value of workerCount from the ThreadPool and returns it.
// This count represents the number of workers that are actively processing jobs.
// Note that this count does not include idle or terminated workers.
func (t *ThreadPool) GetWorkerCount() int {
	if t.workers == nil {
		return 0
	}
	return len(t.workers.workers)
}

// Start starts the execution of the ThreadPool. It returns an error if the ThreadPool
// has already been started. If the given context is nil, it will default to the background context.
// It creates a new WorkerGroup with the specified workerCount
func (t *ThreadPool) Start(ctx context.Context) error {
	if t.workers != nil {
		return ErrPoolAlreadyStarted
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if t.logger != nil {
		t.logger.Info("Starting threadpool...", log.KV{"workerCount": t.workerCount, "queueSize": cap(t.jobQueue)})
	}
	var err error
	// Create a worker group with the logger
	t.workers, err = NewWorkerGroup(t.workerCount, t.jobQueue, ctx, t.logger)
	return err
}

// Stop stops the execution of the ThreadPool. It returns an error if the ThreadPool
// has not been started yet. It cancels the context and waits for all workers to finish
// their current jobs. After that, it cleans the worker list and sets the started flag to false.
// Note: this function is blocking
func (t *ThreadPool) Stop() error {
	if t.workers == nil {
		return ErrPoolNotStarted
	}
	if t.logger != nil {
		t.logger.Info("Shutting down threadpool...")
	}
	t.workers.Stop()
	t.workers = nil
	return nil
}

// Dispatch adds a new job to the jobQueue of the ThreadPool.
// The job will be executed by one of the worker goroutines in the ThreadPool.
// The job must implement the Job interface with a Run method that takes a context.Context parameter.
//
// Example usage:
//
//	job := MyJob{}
//	threadPool.Dispatch(job)
//
// Note: This function is blocking if jobQueue is full
func (t *ThreadPool) Dispatch(j Job) {
	t.jobQueue <- j
}

// TryDispatch attempts to dispatch a job to the ThreadPool without blocking.
// It returns true if the job was successfully dispatched, false if the queue is full.
//
// Example usage:
//
//	job := MyJob{}
//	if !threadPool.TryDispatch(job) {
//	  // Handle job rejection (queue full)
//	}
func (t *ThreadPool) TryDispatch(j Job) bool {
	select {
	case t.jobQueue <- j:
		return true
	default:
		return false
	}
}

// DispatchWithTimeout attempts to dispatch a job with a specified timeout.
// It returns true if the job was successfully dispatched, false if the timeout was reached.
//
// Example usage:
//
//	job := MyJob{}
//	if !threadPool.DispatchWithTimeout(job, 100*time.Millisecond) {
//	  // Handle job timeout
//	}
func (t *ThreadPool) DispatchWithTimeout(j Job, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case t.jobQueue <- j:
		return true
	case <-timer.C:
		return false
	}
}

// DispatchWithContext attempts to dispatch a job respecting context cancellation.
// It returns nil if the job was successfully dispatched, or an error if the context was canceled.
//
// Example usage:
//
//	job := MyJob{}
//	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
//	defer cancel()
//
//	if err := threadPool.DispatchWithContext(ctx, job); err != nil {
//	  // Handle dispatch error (context canceled or deadline exceeded)
//	}
func (t *ThreadPool) DispatchWithContext(ctx context.Context, j Job) error {
	select {
	case t.jobQueue <- j:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
