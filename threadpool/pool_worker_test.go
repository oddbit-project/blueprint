package threadpool

import (
	"context"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

type testJob struct {
	handler func()
}

func newTestJob(handler func()) *testJob {
	return &testJob{
		handler: handler,
	}
}

func (t *testJob) Run(ctx context.Context) {
	t.handler()
}

func runPool(t *testing.T, jobCount int, pool *ThreadPool) {
	require.NoError(t, pool.Start(context.Background()))
	defer pool.Stop()

	counter := 0
	var lock sync.Mutex

	wg := &sync.WaitGroup{}
	wg.Add(jobCount)

	// queue & run jobs
	for i := 0; i < jobCount; i++ {
		job := newTestJob(func() {
			defer wg.Done()
			lock.Lock()
			defer lock.Unlock()
			counter += 1
		})
		pool.Dispatch(job)
	}
	wg.Wait()
	require.Equal(t, uint64(jobCount), pool.GetRequestCount())
	require.Equal(t, counter, jobCount)
	require.Equal(t, pool.GetWorkerCount(), pool.workerCount)
	require.Equal(t, pool.GetQueueLen(), 0)
	require.Equal(t, pool.GetRequestCount(), uint64(jobCount))
}

func TestThreadPool_work(t *testing.T) {
	pool, err := NewThreadPool(5, 10)
	require.NoError(t, err)

	tests := []struct {
		name     string
		jobCount int
	}{
		{
			name:     "SmallJobCount",
			jobCount: 32,
		},
		{
			name:     "HugeJobCount",
			jobCount: 32000,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runPool(t, tt.jobCount, pool)
		})
	}
}
