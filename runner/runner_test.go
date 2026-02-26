package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/log"
)

func testLogger(t *testing.T) *log.Logger {
	t.Helper()
	if err := log.Configure(log.NewDefaultConfig()); err != nil {
		t.Fatalf("failed to configure logger: %v", err)
	}
	return log.New("test-runner")
}

func TestNewUpdater(t *testing.T) {
	logger := testLogger(t)
	interval := 100 * time.Millisecond
	fn := func(ctx context.Context) error { return nil }

	runner := NewUpdater(interval, fn, logger)

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

func TestStart_Success(t *testing.T) {
	logger := testLogger(t)
	fn := func(ctx context.Context) error { return nil }
	runner := NewUpdater(100*time.Millisecond, fn, logger)

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
	runner.Stop(stopCtx)
}

func TestStart_AlreadyRunning(t *testing.T) {
	logger := testLogger(t)
	fn := func(ctx context.Context) error { return nil }
	runner := NewUpdater(100*time.Millisecond, fn, logger)

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
	runner.Stop(stopCtx)
}

func TestStop_Success(t *testing.T) {
	logger := testLogger(t)
	fn := func(ctx context.Context) error { return nil }
	runner := NewUpdater(100*time.Millisecond, fn, logger)

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
	runner := NewUpdater(100*time.Millisecond, fn, logger)

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

	runner := NewUpdater(50*time.Millisecond, fn, logger)

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

	runner := NewUpdater(50*time.Millisecond, fn, logger)

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

	runner := NewUpdater(50*time.Millisecond, fn, logger)

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

	runner := NewUpdater(10*time.Millisecond, fn, logger)

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

	runner := NewUpdater(50*time.Millisecond, fn, logger)

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
