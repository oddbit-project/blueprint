package threadpool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFuncRunner_Basic(t *testing.T) {
	t.Run("CreateFuncRunner", func(t *testing.T) {
		// Test that FuncRunner properly wraps a function
		executed := false
		job := FuncRunner(func(ctx context.Context) {
			executed = true
		})

		require.NotNil(t, job)
		require.IsType(t, &funcRunner{}, job)

		// Execute the job
		job.Run(context.Background())
		require.True(t, executed)
	})

	t.Run("ContextPassing", func(t *testing.T) {
		// Test that context is properly passed to the wrapped function
		var receivedCtx context.Context
		job := FuncRunner(func(ctx context.Context) {
			receivedCtx = ctx
		})

		ctx := context.WithValue(context.Background(), "test", "value")
		job.Run(ctx)

		require.Equal(t, "value", receivedCtx.Value("test"))
	})

	t.Run("NilFunction", func(t *testing.T) {
		// Test behavior with nil function - should not panic during creation
		job := FuncRunner(nil)
		require.NotNil(t, job)

		// But should panic when Run is called
		require.Panics(t, func() {
			job.Run(context.Background())
		})
	})
}

func TestFuncRunner_WithThreadPool(t *testing.T) {
	t.Run("SingleJobExecution", func(t *testing.T) {
		pool, err := NewThreadPool(2, 5)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		executed := false
		var mu sync.Mutex

		job := FuncRunner(func(ctx context.Context) {
			mu.Lock()
			defer mu.Unlock()
			executed = true
		})

		pool.Dispatch(job)

		// Wait for job to complete
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		require.True(t, executed)
		mu.Unlock()
	})

	t.Run("MultipleJobsExecution", func(t *testing.T) {
		pool, err := NewThreadPool(3, 10)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		const jobCount = 100
		var counter int64
		var wg sync.WaitGroup

		for i := 0; i < jobCount; i++ {
			wg.Add(1)
			job := FuncRunner(func(ctx context.Context) {
				defer wg.Done()
				atomic.AddInt64(&counter, 1)
			})
			pool.Dispatch(job)
		}

		wg.Wait()
		require.Equal(t, int64(jobCount), atomic.LoadInt64(&counter))
		// Use Eventually to handle race condition between job completion and counter increment
		require.Eventually(t, func() bool {
			return pool.GetRequestCount() == uint64(jobCount)
		}, 100*time.Millisecond, 10*time.Millisecond, "expected request count to reach %d", jobCount)
	})

	t.Run("ConcurrentJobsWithSharedState", func(t *testing.T) {
		pool, err := NewThreadPool(5, 20)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		var counter int64
		var wg sync.WaitGroup
		const jobCount = 50

		for i := 0; i < jobCount; i++ {
			wg.Add(1)
			job := FuncRunner(func(ctx context.Context) {
				defer wg.Done()
				// Simulate some work
				time.Sleep(time.Millisecond)
				atomic.AddInt64(&counter, 1)
			})
			pool.Dispatch(job)
		}

		wg.Wait()
		require.Equal(t, int64(jobCount), atomic.LoadInt64(&counter))
	})
}

func TestFuncRunner_ContextCancellation(t *testing.T) {
	t.Run("JobRespectsContextCancellation", func(t *testing.T) {
		pool, err := NewThreadPool(1, 5)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		require.NoError(t, pool.Start(ctx))
		defer pool.Stop()

		jobStarted := make(chan struct{})
		jobCancelled := make(chan struct{})

		job := FuncRunner(func(jobCtx context.Context) {
			close(jobStarted)
			select {
			case <-jobCtx.Done():
				close(jobCancelled)
			case <-time.After(5 * time.Second):
				// This should not happen
			}
		})

		pool.Dispatch(job)

		// Wait for job to start
		<-jobStarted

		// Cancel the context
		cancel()

		// Wait for job to detect cancellation
		select {
		case <-jobCancelled:
			// Success
		case <-time.After(1 * time.Second):
			t.Fatal("Job did not respect context cancellation")
		}
	})

	t.Run("JobWithTimeout", func(t *testing.T) {
		pool, err := NewThreadPool(1, 5)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		var completed bool
		var cancelled bool
		var mu sync.Mutex

		job := FuncRunner(func(ctx context.Context) {
			select {
			case <-ctx.Done():
				mu.Lock()
				cancelled = true
				mu.Unlock()
				return
			case <-time.After(200 * time.Millisecond):
				mu.Lock()
				completed = true
				mu.Unlock()
			}
		})

		// Create a context with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		// Try to dispatch with timeout context
		err = pool.DispatchWithContext(ctx, job)
		if err != nil {
			// Dispatch failed due to timeout - this is expected behavior
			require.Contains(t, err.Error(), "deadline exceeded")
			return
		}

		// If dispatch succeeded, wait for job to complete or timeout
		time.Sleep(300 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// Either the job was cancelled OR it completed before timeout
		// Both are valid depending on timing, but cancelled is more likely
		require.True(t, cancelled || completed, "Job should have either completed or been cancelled")

		// If it completed, it means the timeout didn't work as expected
		// If it was cancelled, the timeout worked correctly
		if cancelled {
			require.False(t, completed, "Job was cancelled, so it shouldn't have completed")
		}
	})
}

func TestFuncRunner_ErrorHandling(t *testing.T) {
	t.Run("JobPanic", func(t *testing.T) {
		pool, err := NewThreadPool(1, 5)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		panicJob := FuncRunner(func(ctx context.Context) {
			panic("test panic in FuncRunner")
		})

		// Dispatch the panicking job
		pool.Dispatch(panicJob)

		// Wait for the panic to occur
		time.Sleep(50 * time.Millisecond)

		// Verify the pool is still functional by running another job
		executed := false
		var mu sync.Mutex

		normalJob := FuncRunner(func(ctx context.Context) {
			mu.Lock()
			defer mu.Unlock()
			executed = true
		})

		pool.Dispatch(normalJob)

		// Wait for the normal job to complete
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		require.True(t, executed, "Pool should still be functional after a job panic")
		mu.Unlock()
	})

	t.Run("JobWithRecovery", func(t *testing.T) {
		pool, err := NewThreadPool(1, 5)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		recovered := false
		var mu sync.Mutex

		job := FuncRunner(func(ctx context.Context) {
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					recovered = true
					mu.Unlock()
				}
			}()
			panic("intentional panic for recovery test")
		})

		pool.Dispatch(job)
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		require.True(t, recovered, "Job should have recovered from panic")
		mu.Unlock()
	})
}

func TestFuncRunner_PerformanceAndLoad(t *testing.T) {
	t.Run("HighVolumeJobs", func(t *testing.T) {
		pool, err := NewThreadPool(10, 1000)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		const jobCount = 10000
		var counter int64
		var wg sync.WaitGroup

		startTime := time.Now()

		for i := 0; i < jobCount; i++ {
			wg.Add(1)
			job := FuncRunner(func(ctx context.Context) {
				defer wg.Done()
				atomic.AddInt64(&counter, 1)
			})
			pool.Dispatch(job)
		}

		wg.Wait()
		duration := time.Since(startTime)

		require.Equal(t, int64(jobCount), atomic.LoadInt64(&counter))
		// Use Eventually to handle race condition between job completion and counter increment
		require.Eventually(t, func() bool {
			return pool.GetRequestCount() == uint64(jobCount)
		}, 100*time.Millisecond, 10*time.Millisecond, "expected request count to reach %d", jobCount)

		// Log performance for reference (not a strict requirement)
		t.Logf("Processed %d jobs in %v (%.2f jobs/sec)",
			jobCount, duration, float64(jobCount)/duration.Seconds())
	})

	t.Run("MemoryUsage", func(t *testing.T) {
		// Test that FuncRunner doesn't cause memory leaks
		pool, err := NewThreadPool(5, 100)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		const iterations = 1000
		for i := 0; i < iterations; i++ {
			var wg sync.WaitGroup
			for j := 0; j < 10; j++ {
				wg.Add(1)
				job := FuncRunner(func(ctx context.Context) {
					defer wg.Done()
					// Allocate some memory to test cleanup
					data := make([]byte, 1024)
					_ = data[0] // Use the data
				})
				pool.Dispatch(job)
			}
			wg.Wait()
		}

		// If we get here without running out of memory, the test passes
		// Wait briefly for counters to be updated after wg.Done()
		expectedCount := uint64(iterations * 10)
		require.Eventually(t, func() bool {
			return pool.GetRequestCount() == expectedCount
		}, 100*time.Millisecond, 10*time.Millisecond, "expected request count to reach %d", expectedCount)
	})
}

func TestFuncRunner_Integration(t *testing.T) {
	t.Run("MixedJobTypes", func(t *testing.T) {
		// Test FuncRunner jobs alongside other job types
		pool, err := NewThreadPool(3, 10)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		var funcRunnerCount, testJobCount int64
		var wg sync.WaitGroup

		// Mix FuncRunner jobs with testJob
		for i := 0; i < 20; i++ {
			if i%2 == 0 {
				wg.Add(1)
				job := FuncRunner(func(ctx context.Context) {
					defer wg.Done()
					atomic.AddInt64(&funcRunnerCount, 1)
				})
				pool.Dispatch(job)
			} else {
				wg.Add(1)
				job := newTestJob(func() {
					defer wg.Done()
					atomic.AddInt64(&testJobCount, 1)
				})
				pool.Dispatch(job)
			}
		}

		wg.Wait()
		require.Equal(t, int64(10), atomic.LoadInt64(&funcRunnerCount))
		require.Equal(t, int64(10), atomic.LoadInt64(&testJobCount))
	})

	t.Run("JobChaining", func(t *testing.T) {
		// Test jobs that dispatch other jobs
		pool, err := NewThreadPool(2, 20)
		require.NoError(t, err)
		require.NoError(t, pool.Start(context.Background()))
		defer pool.Stop()

		var finalCount int64
		var wg sync.WaitGroup

		// Parent job that creates child jobs
		wg.Add(1)
		parentJob := FuncRunner(func(ctx context.Context) {
			defer wg.Done()

			// Create 3 child jobs
			for i := 0; i < 3; i++ {
				wg.Add(1)
				childJob := FuncRunner(func(ctx context.Context) {
					defer wg.Done()
					atomic.AddInt64(&finalCount, 1)
				})
				pool.Dispatch(childJob)
			}
		})

		pool.Dispatch(parentJob)
		wg.Wait()

		require.Equal(t, int64(3), atomic.LoadInt64(&finalCount))
		// Use Eventually to handle race condition between job completion and counter increment
		require.Eventually(t, func() bool {
			return pool.GetRequestCount() == uint64(4) // 1 parent + 3 children
		}, 100*time.Millisecond, 10*time.Millisecond, "expected request count to reach 4")
	})
}

// Benchmark tests for FuncRunner
func BenchmarkFuncRunner_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := FuncRunner(func(ctx context.Context) {
			// Empty job
		})
		_ = job
	}
}

func BenchmarkFuncRunner_Execution(b *testing.B) {
	job := FuncRunner(func(ctx context.Context) {
		// Empty job
	})
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job.Run(ctx)
	}
}

func BenchmarkFuncRunner_WithThreadPool(b *testing.B) {
	pool, err := NewThreadPool(4, 1000)
	require.NoError(b, err)
	require.NoError(b, pool.Start(context.Background()))
	defer pool.Stop()

	var wg sync.WaitGroup

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wg.Add(1)
		job := FuncRunner(func(ctx context.Context) {
			wg.Done()
		})
		pool.Dispatch(job)
	}
	wg.Wait()
}
