package threadpool

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/oddbit-project/blueprint/log"
)

type Worker struct {
	jobQueue chan Job
	ctx      context.Context
	// stopCh asks the worker to finish what is already queued and exit. It is
	// set by a WorkerGroup; a Worker built directly has none and exits as soon
	// as its context is done, discarding whatever was queued.
	stopCh         <-chan struct{}
	requestCounter atomic.Uint64
}

type WorkerGroup struct {
	workers  []*Worker
	ctx      context.Context
	cancelFn context.CancelFunc
	wg       *sync.WaitGroup
	stop     *sync.Once
	// stopCh is closed by Drain: the workers then empty the queue and exit,
	// rather than abandoning jobs that were accepted but not yet started.
	stopCh chan struct{}
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
				w.run(job, logger)

			case <-w.stopCh:
				// graceful: run what was already accepted, then leave. The
				// jobs run on the worker's own context, which is still live --
				// cancelling it here would abandon them just as surely as
				// dropping them.
				w.drain(logger)
				return

			case <-w.ctx.Done():
				return
			}
		}
	}()
}

// drain runs every job already in the queue and returns once it is empty.
func (w *Worker) drain(logger *log.Logger) {
	for {
		select {
		case job := <-w.jobQueue:
			w.run(job, logger)
		default:
			return
		}
	}
}

// run executes one job, surviving a panic in it.
func (w *Worker) run(job Job, logger *log.Logger) {
	defer func() {
		if r := recover(); r != nil {
			if logger != nil {
				logger.Warnf("ThreadPool Worker panic: %v", r)
			}
		}
	}()
	job.Run(w.ctx)
	w.requestCounter.Add(1)
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
		stopCh:   make(chan struct{}),
	}
	// Start workers
	for i := 0; i < workerCount; i++ {
		// First create and add to WaitGroup before starting the worker goroutine
		group.workers[i] = NewWorker(jobQueue, group.ctx)
		group.workers[i].stopCh = group.stopCh
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

// Stop cancels the workers' context and waits for the jobs already running.
// Anything still queued is abandoned; use Drain to finish it.
func (w *WorkerGroup) Stop() {
	w.stop.Do(func() {
		w.cancelFn()
		w.wg.Wait()
	})
}

// Drain asks the workers to finish the queue and exit, and waits for them. A
// job that was accepted has been promised a run; abandoning it loses work the
// caller believes was handed over.
func (w *WorkerGroup) Drain() {
	w.stop.Do(func() {
		close(w.stopCh)
		w.wg.Wait()
		w.cancelFn()
	})
}
