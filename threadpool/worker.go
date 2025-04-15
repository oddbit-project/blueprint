package threadpool

import (
	"context"
	"sync"
)

type Worker struct {
	jobQueue       chan Job
	ctx            context.Context
	requestCounter uint64
	counterMutex   sync.Mutex
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

func (w *Worker) Start(wg *sync.WaitGroup) {
	go func() {
		defer wg.Done()
		for {
			select {
			case job := <-w.jobQueue:
				job.Run(w.ctx)
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

func NewWorkerGroup(workerCount int, jobQueue chan Job, parentCtx context.Context) (*WorkerGroup, error) {
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
		group.workers[i] = NewWorker(jobQueue, group.ctx)
		group.workers[i].Start(group.wg)
		group.wg.Add(1)
	}
	return group, nil
}

func (w *WorkerGroup) RequestCount() uint64 {
	var totalRequests uint64
	for _, worker := range w.workers {
		totalRequests += worker.RequestCounter()
	}
	return totalRequests
}

func (w *WorkerGroup) Stop() {
	w.stop.Do(func() {
		w.cancelFn()
		w.wg.Wait()
	})
}