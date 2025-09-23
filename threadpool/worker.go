package threadpool

import (
	"context"
	"github.com/oddbit-project/blueprint/log"
	"sync"
)

type Worker struct {
	jobQueue       chan Job
	ctx            context.Context
	requestCounter uint64
	counterMutex   sync.Mutex
	// Could add a logger field for panic logging
	// logger         *log.Logger
}

type WorkerGroup struct {
	workers  []*Worker
	ctx      context.Context
	cancelFn context.CancelFunc
	wg       *sync.WaitGroup
	stop     *sync.Once
}

func NewWorker(jobQueue chan Job, ctx context.Context) *Worker {
	return &Worker{
		jobQueue:       jobQueue,
		ctx:            ctx,
		requestCounter: 0,
	}
}

func (w *Worker) Start(wg *sync.WaitGroup, logger *log.Logger) {
	go func() {
		defer wg.Done()
		for {
			select {
			case job := <-w.jobQueue:
				// Recover from any panics in job execution to prevent worker crash
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Only log if logger is provided
							if logger != nil {
								logger.Warnf("ThreadPool Worker panic: %v", r)
							}
							// Otherwise silently recover
						}
					}()
					job.Run(w.ctx)
				}()

				// Update counter after job completion
				w.counterMutex.Lock()
				w.requestCounter++
				w.counterMutex.Unlock()

			case <-w.ctx.Done():
				return
			}
		}
	}()
}

func (w *Worker) RequestCounter() uint64 {
	w.counterMutex.Lock()
	defer w.counterMutex.Unlock()
	return w.requestCounter
}

// NewWorkerGroup creates a new group of workers
// If logger is nil, panics will be recovered silently
func NewWorkerGroup(workerCount int, jobQueue chan Job, parentCtx context.Context, logger *log.Logger) (*WorkerGroup, error) {
	if workerCount < 1 {
		return nil, ErrInvalidWorkerCount
	}
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancelFn := context.WithCancel(parentCtx)
	group := &WorkerGroup{
		workers:  make([]*Worker, workerCount),
		ctx:      ctx,
		cancelFn: cancelFn,
		wg:       &sync.WaitGroup{},
		stop:     &sync.Once{},
	}
	// Start workers
	for i := 0; i < workerCount; i++ {
		// First create and add to WaitGroup before starting the worker goroutine
		group.workers[i] = NewWorker(jobQueue, group.ctx)
		group.wg.Add(1)
		group.workers[i].Start(group.wg, logger)
	}
	return group, nil
}

func (w *WorkerGroup) RequestCount() uint64 {
	var totalRequests uint64
	// Use a lock to ensure consistent reading of values across all workers
	var mutex sync.Mutex
	var wg sync.WaitGroup

	wg.Add(len(w.workers))
	for _, worker := range w.workers {
		go func(worker *Worker) {
			defer wg.Done()
			count := worker.RequestCounter()
			mutex.Lock()
			totalRequests += count
			mutex.Unlock()
		}(worker)
	}
	wg.Wait()
	return totalRequests
}

func (w *WorkerGroup) Stop() {
	w.stop.Do(func() {
		w.cancelFn()
		w.wg.Wait()
	})
}
