package runner

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain configures logging once; log.Configure() writes process-wide zerolog
// settings, which must not happen while runner goroutines are still logging
func TestMain(m *testing.M) {
	if err := log.Configure(log.NewDefaultConfig()); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func testLogger(t *testing.T) *log.Logger {
	t.Helper()
	return log.New("test-runner")
}

func TestNewUpdater(t *testing.T) {
	logger := testLogger(t)
	interval := 100 * time.Millisecond
	fn := func(ctx context.Context) error { return nil }

	runner, err := NewUpdater(interval, fn, logger)
	if err != nil {
		t.Fatalf("NewUpdater returned error: %v", err)
	}

	if runner == nil {
		t.Fatal("NewUpdater returned nil")
	}
	if runner.runInterval != interval {
		t.Errorf("expected interval %v, got %v", interval, runner.runInterval)
	}
	if runner.runFn == nil {
		t.Error("runFn is nil")
	}
	if runner.logger != logger {
		t.Error("logger not set correctly")
	}
	if runner.status.Load() != 0 {
		t.Errorf("expected initial status 0, got %d", runner.status.Load())
	}
}

func TestNewUpdater_InvalidInputs(t *testing.T) {
	logger := testLogger(t)
	fn := func(ctx context.Context) error { return nil }

	t.Run("zero interval", func(t *testing.T) {
		_, err := NewUpdater(0, fn, logger)
		if !errors.Is(err, ErrInvalidInterval) {
			t.Errorf("expected ErrInvalidInterval, got: %v", err)
		}
	})

	t.Run("negative interval", func(t *testing.T) {
		_, err := NewUpdater(-time.Second, fn, logger)
		if !errors.Is(err, ErrInvalidInterval) {
			t.Errorf("expected ErrInvalidInterval, got: %v", err)
		}
	})

	t.Run("nil function", func(t *testing.T) {
		_, err := NewUpdater(100*time.Millisecond, nil, logger)
		if !errors.Is(err, ErrNilRunnerFn) {
			t.Errorf("expected ErrNilRunnerFn, got: %v", err)
		}
	})

	t.Run("nil logger", func(t *testing.T) {
		_, err := NewUpdater(100*time.Millisecond, fn, nil)
		if !errors.Is(err, ErrNilLogger) {
			t.Errorf("expected ErrNilLogger, got: %v", err)
		}
	})
}

func TestStart_Success(t *testing.T) {
	logger := testLogger(t)
	fn := func(ctx context.Context) error { return nil }
	runner, _ := NewUpdater(100*time.Millisecond, fn, logger)

	ctx := context.Background()
	err := runner.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if runner.status.Load() != 1 {
		t.Errorf("expected status 1 after start, got %d", runner.status.Load())
	}

	// Cleanup
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestStart_AlreadyRunning(t *testing.T) {
	logger := testLogger(t)
	fn := func(ctx context.Context) error { return nil }
	runner, _ := NewUpdater(100*time.Millisecond, fn, logger)

	ctx := context.Background()
	err := runner.Start(ctx)
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	// Try to start again
	err = runner.Start(ctx)
	if err == nil {
		t.Error("expected error when starting already running runner")
	}
	if err.Error() != "already running" {
		t.Errorf("expected 'already running' error, got: %v", err)
	}

	// Cleanup
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runner.Stop(stopCtx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestStop_Success(t *testing.T) {
	logger := testLogger(t)
	fn := func(ctx context.Context) error { return nil }
	runner, _ := NewUpdater(100*time.Millisecond, fn, logger)

	ctx := context.Background()
	err := runner.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = runner.Stop(stopCtx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if runner.status.Load() != 0 {
		t.Errorf("expected status 0 after stop, got %d", runner.status.Load())
	}
}

func TestStop_NotRunning(t *testing.T) {
	logger := testLogger(t)
	fn := func(ctx context.Context) error { return nil }
	runner, _ := NewUpdater(100*time.Millisecond, fn, logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := runner.Stop(ctx)
	if err == nil {
		t.Error("expected error when stopping non-running runner")
	}
	if err.Error() != "not running" {
		t.Errorf("expected 'not running' error, got: %v", err)
	}
}

func TestPeriodicExecution(t *testing.T) {
	logger := testLogger(t)
	var count atomic.Int32

	fn := func(ctx context.Context) error {
		count.Add(1)
		return nil
	}

	runner, _ := NewUpdater(50*time.Millisecond, fn, logger)

	ctx := context.Background()
	err := runner.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for at least 3 executions
	time.Sleep(175 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = runner.Stop(stopCtx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	execCount := count.Load()
	if execCount < 3 {
		t.Errorf("expected at least 3 executions, got %d", execCount)
	}
}

func TestContextCancellation(t *testing.T) {
	logger := testLogger(t)
	var count atomic.Int32

	fn := func(ctx context.Context) error {
		count.Add(1)
		return nil
	}

	runner, _ := NewUpdater(50*time.Millisecond, fn, logger)

	ctx, cancel := context.WithCancel(context.Background())
	err := runner.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Let it run a bit
	time.Sleep(75 * time.Millisecond)

	// Cancel the parent context
	cancel()

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	// The runner should have stopped due to context cancellation
	countBefore := count.Load()
	time.Sleep(100 * time.Millisecond)
	countAfter := count.Load()

	if countAfter > countBefore {
		t.Error("runner continued executing after context cancellation")
	}
}

func TestRunFnError(t *testing.T) {
	logger := testLogger(t)
	var count atomic.Int32
	expectedErr := errors.New("test error")

	fn := func(ctx context.Context) error {
		count.Add(1)
		return expectedErr
	}

	runner, _ := NewUpdater(50*time.Millisecond, fn, logger)

	ctx := context.Background()
	err := runner.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for a few executions
	time.Sleep(125 * time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err = runner.Stop(stopCtx)
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Verify that the runner continued despite errors
	if count.Load() < 2 {
		t.Errorf("expected at least 2 executions despite errors, got %d", count.Load())
	}
}

func TestStopTimeout(t *testing.T) {
	logger := testLogger(t)

	started := make(chan struct{})
	// Create a function that blocks and ignores context cancellation
	fn := func(ctx context.Context) error {
		close(started)
		// Block for a long time, ignoring context
		time.Sleep(5 * time.Second)
		return nil
	}

	runner, _ := NewUpdater(10*time.Millisecond, fn, logger)

	ctx := context.Background()
	err := runner.Start(ctx)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Wait for the function to start executing
	<-started

	// Try to stop with a short timeout
	stopCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err = runner.Stop(stopCtx)

	// The stop should timeout because the function is blocking
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded error, got: %v", err)
	}
}

func TestStartStopRestart(t *testing.T) {
	logger := testLogger(t)
	var count atomic.Int32

	fn := func(ctx context.Context) error {
		count.Add(1)
		return nil
	}

	runner, _ := NewUpdater(50*time.Millisecond, fn, logger)

	// First start
	ctx := context.Background()
	err := runner.Start(ctx)
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}

	time.Sleep(75 * time.Millisecond)

	// Stop
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	err = runner.Stop(stopCtx)
	cancel()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	countAfterFirstRun := count.Load()

	// Restart
	err = runner.Start(ctx)
	if err != nil {
		t.Fatalf("Restart failed: %v", err)
	}

	time.Sleep(75 * time.Millisecond)

	// Stop again
	stopCtx, cancel = context.WithTimeout(context.Background(), time.Second)
	err = runner.Stop(stopCtx)
	cancel()
	if err != nil {
		t.Fatalf("Second Stop failed: %v", err)
	}

	if count.Load() <= countAfterFirstRun {
		t.Error("runner did not execute after restart")
	}
}

// syncBuffer is a goroutine-safe log sink
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestErrorValues(t *testing.T) {
	assert.Equal(t, "already running", ErrAlreadyRunning.Error())
	assert.Equal(t, "not running", ErrNotRunning.Error())

	runner, err := NewUpdater(50*time.Millisecond, func(ctx context.Context) error { return nil }, testLogger(t))
	require.NoError(t, err)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	assert.ErrorIs(t, runner.Stop(stopCtx), ErrNotRunning)
	require.NoError(t, runner.Start(context.Background()))
	assert.ErrorIs(t, runner.Start(context.Background()), ErrAlreadyRunning)
	require.NoError(t, runner.Stop(stopCtx))
	assert.ErrorIs(t, runner.Stop(stopCtx), ErrNotRunning)
}

func TestStop_NoErrorLogOnCancel(t *testing.T) {
	out := &syncBuffer{}
	logger := testLogger(t).WithOutput(out)
	runner, err := NewUpdater(10*time.Millisecond, func(ctx context.Context) error { return nil }, logger)
	require.NoError(t, err)

	require.NoError(t, runner.Start(context.Background()))
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, runner.Stop(stopCtx))

	assert.NotContains(t, out.String(), "runner error")
	assert.NotContains(t, out.String(), "context canceled")
}

func TestParentContextCancel_StopAndRestart(t *testing.T) {
	out := &syncBuffer{}
	logger := testLogger(t).WithOutput(out)
	runner, err := NewUpdater(10*time.Millisecond, func(ctx context.Context) error { return nil }, logger)
	require.NoError(t, err)

	parentCtx, cancelParent := context.WithCancel(context.Background())
	require.NoError(t, runner.Start(parentCtx))
	cancelParent()
	require.Eventually(t, func() bool { return runner.status.Load() == 0 }, time.Second, 5*time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.NoError(t, runner.Stop(stopCtx))
	assert.NotContains(t, out.String(), "runner error")
	assert.NotContains(t, out.String(), "context canceled")

	require.NoError(t, runner.Start(context.Background()))
	require.NoError(t, runner.Stop(stopCtx))
}

func TestConcurrentStartStop(t *testing.T) {
	logger := testLogger(t).WithOutput(io.Discard)
	fn := func(ctx context.Context) error { return nil }

	for i := 0; i < 200; i++ {
		runner, err := NewUpdater(time.Millisecond, fn, logger)
		require.NoError(t, err)

		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		var wg sync.WaitGroup
		var startErr, stopErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			startErr = runner.Start(context.Background())
		}()
		go func() {
			defer wg.Done()
			stopErr = runner.Stop(stopCtx)
		}()
		wg.Wait()

		require.NoError(t, startErr)
		require.NotErrorIs(t, stopErr, context.DeadlineExceeded, "Stop blocked on a started runner")
		if stopErr == nil {
			require.Equal(t, int32(0), runner.status.Load(), "Stop returned nil but runner is still running")
		} else {
			// Stop ran before Start
			require.NoError(t, runner.Stop(stopCtx))
		}
		cancel()
	}
}

func TestStopTimeout_NoGoroutineLeak(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	fn := func(ctx context.Context) error {
		once.Do(func() { close(started) })
		<-release
		return nil
	}
	runner, err := NewUpdater(time.Millisecond, fn, testLogger(t).WithOutput(io.Discard))
	require.NoError(t, err)
	require.NoError(t, runner.Start(context.Background()))
	<-started

	before := runtime.NumGoroutine()
	for i := 0; i < 50; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		assert.ErrorIs(t, runner.Stop(ctx), context.DeadlineExceeded)
		cancel()
	}
	assert.Less(t, runtime.NumGoroutine()-before, 10)

	close(release)
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, runner.Stop(stopCtx))
}
