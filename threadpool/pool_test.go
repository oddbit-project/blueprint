package threadpool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewThreadPool(t *testing.T) {
	tests := []struct {
		name        string
		workerCount int
		queueSize   int
		expectErr   bool
		errorTypeIs error
	}{
		{
			name:        "ErrorWithZeroWorkerCount",
			workerCount: 0,
			queueSize:   10,
			expectErr:   true,
			errorTypeIs: ErrInvalidWorkerCount,
		},
		{
			name:        "ErrorWithZeroQueueSize",
			workerCount: 1,
			queueSize:   0,
			expectErr:   true,
			errorTypeIs: ErrInvalidQueueSize,
		},
		{
			name:        "SuccessWithOneWorkerAndQueueSize",
			workerCount: 1,
			queueSize:   1,
			expectErr:   false,
		},
		{
			name:        "SuccessWithMultipleWorkerAndQueueSize",
			workerCount: 12,
			queueSize:   128,
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotThreadPool, gotErr := NewThreadPool(tt.workerCount, tt.queueSize)
			if tt.expectErr {
				require.Error(t, gotErr)
				require.Equal(t, tt.errorTypeIs, gotErr)
				return
			}
			require.NoError(t, gotErr)
			require.NotNil(t, gotThreadPool)
			require.Equal(t, gotThreadPool.workerCount, tt.workerCount)
			require.Equal(t, tt.queueSize, cap(gotThreadPool.jobQueue))
			require.Equal(t, gotThreadPool.GetWorkerCount(), 0)
			require.Equal(t, gotThreadPool.GetRequestCount(), uint64(0))
			require.Equal(t, gotThreadPool.GetQueueLen(), 0)
			require.Equal(t, gotThreadPool.GetQueueCapacity(), tt.queueSize)
		})
	}
}

func TestThreadPool_MultipleStartStop(t *testing.T) {
	var err error
	var pool *ThreadPool
	pool, err = NewThreadPool(8, 32)
	require.NoError(t, err)
	_ = pool.Start(context.Background())
	err = pool.Start(context.Background())
	require.Equal(t, ErrPoolAlreadyStarted, err)

	_ = pool.Stop()
	err = pool.Stop()
	require.Equal(t, ErrPoolNotStarted, err)
}

func TestThreadPool_TryDispatch(t *testing.T) {
	// Use a synchronization channel to control job execution
	jobStarted := make(chan struct{})
	jobRelease := make(chan struct{})

	// Create a pool with 1 worker and queue size of 1
	pool, err := NewThreadPool(1, 1)
	require.NoError(t, err)
	require.NoError(t, pool.Start(context.Background()))
	defer pool.Stop()

	// First job - blocks until we signal
	require.True(t, pool.TryDispatch(newTestJob(func() {
		jobStarted <- struct{}{} // Signal job started
		<-jobRelease             // Wait for release signal
	})))

	// Wait for the job to start processing
	<-jobStarted

	// Second job goes to queue
	require.True(t, pool.TryDispatch(newTestJob(func() {})))

	// Third job should fail (queue is full)
	require.False(t, pool.TryDispatch(newTestJob(func() {})))

	// Release the job being processed
	jobRelease <- struct{}{}

	// Wait for first job to complete and second job to start
	time.Sleep(100 * time.Millisecond)

	// Now the queue should have space
	require.True(t, pool.TryDispatch(newTestJob(func() {})))
}

func TestThreadPool_DispatchWithContext(t *testing.T) {
	// Use a synchronization channel to control job execution
	jobStarted := make(chan struct{})
	jobRelease := make(chan struct{})

	// Create a pool with 1 worker and queue size of 1
	pool, err := NewThreadPool(1, 1)
	require.NoError(t, err)
	require.NoError(t, pool.Start(context.Background()))
	defer pool.Stop()

	// First job - blocks until we signal
	require.True(t, pool.TryDispatch(newTestJob(func() {
		jobStarted <- struct{}{} // Signal job started
		<-jobRelease             // Wait for release signal
	})))

	// Wait for the job to start processing
	<-jobStarted

	// Second job goes to queue
	require.True(t, pool.TryDispatch(newTestJob(func() {})))

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Should fail due to timeout (queue full)
	require.Error(t, pool.DispatchWithContext(ctx, newTestJob(func() {})))

	// Same with timeout dispatch method
	require.False(t, pool.DispatchWithTimeout(newTestJob(func() {}), 10*time.Millisecond))

	// Release the job being processed
	jobRelease <- struct{}{}

	// Wait for first job to complete and second job to start
	time.Sleep(100 * time.Millisecond)

	// Now a dispatch with context should succeed
	ctx2 := context.Background()
	require.NoError(t, pool.DispatchWithContext(ctx2, newTestJob(func() {})))
}

func TestThreadPool_PanicRecovery(t *testing.T) {
	pool, err := NewThreadPool(1, 2)
	require.NoError(t, err)
	require.NoError(t, pool.Start(context.Background()))
	defer pool.Stop()

	// Job that panics
	pool.Dispatch(newTestJob(func() {
		panic("test panic")
	}))

	// Give time for job to execute
	time.Sleep(50 * time.Millisecond)

	// Worker should still be alive and processing jobs
	processed := make(chan bool)
	pool.Dispatch(newTestJob(func() {
		processed <- true
	}))

	// Verify the job was processed
	select {
	case <-processed:
		// Success, worker still alive
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Worker is not processing jobs after panic")
	}
}

// A job that was accepted has been promised a run. Stop used to cancel the
// workers' context and return, so anything sitting in the queue was discarded
// with no signal to the caller, who believed the work was handed over.
func TestStopFinishesQueuedJobs(t *testing.T) {
	const queued = 16
	pool, err := NewThreadPool(2, queued)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	var ran atomic.Int64
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	// occupy both workers so the rest of the jobs have to queue
	for i := 0; i < 2; i++ {
		pool.Dispatch(FuncRunner(func(context.Context) {
			started <- struct{}{}
			<-release
			ran.Add(1)
		}))
	}
	<-started
	<-started
	for i := 0; i < queued; i++ {
		pool.Dispatch(FuncRunner(func(context.Context) { ran.Add(1) }))
	}

	stopped := make(chan error, 1)
	go func() { close(release); stopped <- pool.Stop() }()
	select {
	case err = <-stopped:
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Stop never returned")
	}

	if got := ran.Load(); got != queued+2 {
		t.Errorf("ran %d jobs, want %d: the queue was discarded", got, queued+2)
	}
}

// The drain cannot be open-ended: a shutdown budget has to bound it, and what
// is still queued when it expires is abandoned rather than holding the process.
func TestStopWithContextGivesUpOnItsDeadline(t *testing.T) {
	pool, err := NewThreadPool(1, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	defer close(release)
	pool.Dispatch(FuncRunner(func(context.Context) {
		close(started)
		<-release
	}))
	<-started
	pool.Dispatch(FuncRunner(func(context.Context) {}))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err = pool.StopWithContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("StopWithContext = %v, want DeadlineExceeded", err)
	}
}

// The negative: with nothing in flight, the bounded stop drains and reports no
// error, so the deadline is a last resort rather than the normal path.
func TestStopWithContextDrainsWhenItCan(t *testing.T) {
	pool, err := NewThreadPool(2, 8)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	var ran atomic.Int64
	for i := 0; i < 8; i++ {
		pool.Dispatch(FuncRunner(func(context.Context) { ran.Add(1) }))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = pool.StopWithContext(ctx); err != nil {
		t.Fatalf("StopWithContext: %v", err)
	}
	if got := ran.Load(); got != 8 {
		t.Errorf("ran %d jobs, want 8", got)
	}
}

// A dispatch racing a shutdown must be reportable rather than fatal: Dispatch
// panics by design, so the shutdown-safe path is DispatchWithContext.
func TestDispatchAfterStopIsReportable(t *testing.T) {
	pool, err := NewThreadPool(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = pool.Stop(); err != nil {
		t.Fatal(err)
	}
	if err = pool.DispatchWithContext(context.Background(), FuncRunner(func(context.Context) {})); !errors.Is(err, ErrPoolNotStarted) {
		t.Errorf("DispatchWithContext after Stop = %v, want ErrPoolNotStarted", err)
	}
	if pool.TryDispatch(FuncRunner(func(context.Context) {})) {
		t.Error("TryDispatch after Stop reported success")
	}
}
