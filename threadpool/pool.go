package threadpool

import (
	"context"
	"github.com/oddbit-project/blueprint/utils"
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
	Start() error
}

type ThreadPool struct {
	workers     *WorkerGroup
	workerCount int
	jobQueue    chan Job
}

// NewThreadPool is a constructor function that creates a new ThreadPool instance. It takes in two parameters:
// - workerCount: the number of workers to be created in the ThreadPool. Must be greater than 0. If it's less than 1, it returns ErrInvalidWorkerCount.
// - queueSize: the size of the job queue in the ThreadPool. Must be greater than 0. If it's less than 1, it returns ErrInvalidQueueSize.
// It returns a pointer to the created ThreadPool and an error.
//
// Example usage:
// pool, err := NewThreadPool(5, 10)
//
//	if err != nil {
//	  // handle error
//	}
//
// pool.Dispatch(job)
// ...
func NewThreadPool(workerCount int, queueSize int) (*ThreadPool, error) {
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
	var err error
	t.workers, err = NewWorkerGroup(t.workerCount, t.jobQueue, ctx)
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
