package threadpool

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/oddbit-project/blueprint/log"
)

type Worker struct {
	jobQueue       chan Job
	ctx            context.Context
	requestCounter atomic.Uint64
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
		jobQueue: jobQueue,
		ctx:      ctx,
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

				w.requestCounter.Add(1)

			case <-w.ctx.Done():
				return
			}
		}
	}()
}

func (w *Worker) RequestCounter() uint64 {
	return w.requestCounter.Load()
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
	var total uint64
	for _, worker := range w.workers {
		total += worker.RequestCounter()
	}
	return total
}

func (w *WorkerGroup) Stop() {
	w.stop.Do(func() {
		w.cancelFn()
		w.wg.Wait()
	})
}
