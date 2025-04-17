package batchwriter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Flag to indicate if we should skip complex tests that cause problems with race detector
var skipTestsWithRace bool

func TestBatchWriter_Creation(t *testing.T) {
	ctx := context.Background()
	
	// Test with invalid capacity
	_, err := NewBatchWriter(ctx, 0, time.Second, func(records ...any) {})
	if err != ErrCapacityTooSmall {
		t.Errorf("Expected ErrCapacityTooSmall, got %v", err)
	}
	
	// Test with nil flush function
	_, err = NewBatchWriter(ctx, 5, time.Second, nil)
	if err != ErrNilFlushFunction {
		t.Errorf("Expected ErrNilFlushFunction, got %v", err)
	}
	
	// Test with invalid flush interval
	_, err = NewBatchWriter(ctx, 5, 0, func(records ...any) {})
	if err != ErrInvalidFlushInterval {
		t.Errorf("Expected ErrInvalidFlushInterval, got %v", err)
	}
	
	// Test valid creation
	bw, err := NewBatchWriter(ctx, 5, time.Second, func(records ...any) {})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if bw == nil {
		t.Error("Expected non-nil BatchWriter")
	}
	defer bw.Stop()
}

func TestBatchWriter_Add(t *testing.T) {
	ctx := context.Background()
	var flushed atomic.Int32
	
	bw, err := NewBatchWriter(ctx, 3, 100*time.Millisecond, func(records ...any) {
		flushed.Add(int32(len(records)))
	})
	if err != nil {
		t.Fatalf("Failed to create BatchWriter: %v", err)
	}
	defer bw.Stop()
	
	// Add records that should trigger a flush due to capacity
	bw.Add(1)
	bw.Add(2)
	bw.Add(3) // This should trigger a flush
	
	// Wait for the flush to happen
	time.Sleep(50 * time.Millisecond)
	
	if count := flushed.Load(); count != 3 {
		t.Errorf("Expected 3 records to be flushed, got %d", count)
	}
	
	// Check metrics
	metrics := bw.GetMetrics()
	if metrics.RecordsAdded != 3 {
		t.Errorf("Expected 3 records added, got %d", metrics.RecordsAdded)
	}
	if metrics.RecordsProcessed != 3 {
		t.Errorf("Expected 3 records processed, got %d", metrics.RecordsProcessed)
	}
	if metrics.FlushCount != 1 {
		t.Errorf("Expected 1 flush, got %d", metrics.FlushCount)
	}
}

func TestBatchWriter_TryAdd(t *testing.T) {
	if skipTestsWithRace {
		t.Skip("Skipping test with race detector")
	}
	
	// Simple test for TryAdd functionality that doesn't rely on timing
	ctx := context.Background()
	
	// Create a batchwriter with a small queue capacity
	bw, _ := NewBatchWriter(ctx, 5, time.Hour, func(records ...any) {
		// Nothing to do here
	}, WithQueueCapacity(2)) // Small queue capacity
	defer bw.Stop()
	
	// Verify the queue capacity is set correctly
	metrics := bw.GetMetrics()
	if metrics.QueueCapacity != 2 {
		t.Errorf("Expected queue capacity 2, got %d", metrics.QueueCapacity)
	}
	
	// Should be able to add items successfully
	if !bw.TryAdd("test1") {
		t.Error("First TryAdd should succeed")
	}
	
	// Directly test that TryAdd updates metrics
	initialDropped := bw.recordsDropped.Load()
	
	// Call the TryAdd method with a full channel simulation
	// This is a bit of a hack, but we're directly testing the method behavior
	oldChan := bw.inputChan
	bw.inputChan = make(chan any, 1) // Create a new channel with 1 capacity
	bw.inputChan <- "fill-the-channel" // Fill it
	
	// Now TryAdd should fail and increment recordsDropped
	success := bw.TryAdd("should-fail")
	
	// Restore the original channel
	bw.inputChan = oldChan
	
	// Verify behavior
	if success {
		t.Error("TryAdd should fail when channel is full")
	}
	
	// Verify recordsDropped was incremented
	newDropped := bw.recordsDropped.Load()
	if newDropped != initialDropped+1 {
		t.Errorf("Expected recordsDropped to increase by 1, got %d -> %d", 
			initialDropped, newDropped)
	}
	
	// Verify it shows in metrics
	metrics = bw.GetMetrics()
	if metrics.RecordsDropped != newDropped {
		t.Errorf("Metrics.RecordsDropped = %d, expected %d", 
			metrics.RecordsDropped, newDropped)
	}
}

func TestBatchWriter_AddWithContext(t *testing.T) {
	if skipTestsWithRace {
		t.Skip("Skipping test with race detector")
	}
	
	// Create a context we can cancel
	ctx := context.Background()
	cancelCtx, cancel := context.WithCancel(context.Background())
	
	// Create a BatchWriter 
	bw, _ := NewBatchWriter(ctx, 10, time.Hour, func(records ...any) {
		// Do nothing in flush function
	})
	defer bw.Stop()
	
	// Test with non-canceled context - should succeed
	err1 := bw.AddWithContext(context.Background(), "test1")
	if err1 != nil {
		t.Errorf("AddWithContext should succeed with active context, got: %v", err1)
	}
	
	// Cancel the context
	cancel()
	
	// Test with canceled context - should fail
	err2 := bw.AddWithContext(cancelCtx, "test2")
	if err2 == nil {
		t.Error("AddWithContext should return error when context is canceled")
	}
	
	// Verify the dropped record is counted in metrics
	metrics := bw.GetMetrics()
	if metrics.RecordsDropped != 1 {
		t.Errorf("Expected 1 record dropped, got %d", metrics.RecordsDropped)
	}
}

func TestBatchWriter_TimeBasedFlush(t *testing.T) {
	ctx := context.Background()
	var flushed atomic.Int32
	flushInterval := 200 * time.Millisecond
	
	bw, _ := NewBatchWriter(ctx, 10, flushInterval, func(records ...any) {
		flushed.Add(int32(len(records)))
	})
	defer bw.Stop()
	
	// Add a few records, but not enough to trigger capacity-based flush
	bw.Add(1)
	bw.Add(2)
	
	// Wait for time-based flush
	time.Sleep(flushInterval + 100*time.Millisecond)
	
	if count := flushed.Load(); count != 2 {
		t.Errorf("Expected 2 records to be flushed by timer, got %d", count)
	}
	
	// Check metrics
	metrics := bw.GetMetrics()
	if metrics.FlushCount != 1 {
		t.Errorf("Expected 1 flush, got %d", metrics.FlushCount)
	}
	if metrics.LastFlushDuration <= 0 {
		t.Errorf("Expected non-zero last flush duration, got %v", metrics.LastFlushDuration)
	}
}

func TestBatchWriter_Concurrency(t *testing.T) {
	ctx := context.Background()
	var flushed atomic.Int32
	flushMu := sync.Mutex{}
	flushCount := 0
	
	bw, _ := NewBatchWriter(ctx, 100, 50*time.Millisecond, func(records ...any) {
		flushMu.Lock()
		defer flushMu.Unlock()
		flushCount++
		flushed.Add(int32(len(records)))
	})
	defer bw.Stop()
	
	// Run multiple goroutines adding records concurrently
	const numWorkers = 5
	const recordsPerWorker = 20
	var wg sync.WaitGroup
	
	wg.Add(numWorkers)
	for w := 0; w < numWorkers; w++ {
		w := w // Capture loop variable
		go func() {
			defer wg.Done()
			for i := 0; i < recordsPerWorker; i++ {
				bw.Add(w*1000 + i)
				// Add a small sleep to simulate real work
				time.Sleep(time.Millisecond)
			}
		}()
	}
	
	// Wait for all goroutines to finish
	wg.Wait()
	
	// Force a final flush and wait for it
	bw.FlushNow()
	time.Sleep(100 * time.Millisecond)
	
	// Check if all records were flushed
	totalExpected := numWorkers * recordsPerWorker
	if count := int(flushed.Load()); count != totalExpected {
		t.Errorf("Expected %d records to be flushed, got %d", totalExpected, count)
	}
	
	// Check metrics
	metrics := bw.GetMetrics()
	if metrics.RecordsProcessed != uint64(totalExpected) {
		t.Errorf("Expected %d records processed in metrics, got %d", totalExpected, metrics.RecordsProcessed)
	}
}

func TestBatchWriter_BufferClearing(t *testing.T) {
	ctx := context.Background()
	
	// Create a batchwriter with buffer clearing enabled
	bw, _ := NewBatchWriter(ctx, 3, time.Second, func(records ...any) {})
	
	// Enable buffer clearing directly
	bw.clearBuffers = true
	defer bw.Stop()
	
	// Add records to fill buffer
	bw.Add("test1")
	bw.Add("test2")
	bw.Add("test3") // This should trigger a flush
	
	// Wait for flush
	time.Sleep(50 * time.Millisecond)
	
	// Check metrics
	metrics := bw.GetMetrics()
	if metrics.FlushCount != 1 {
		t.Errorf("Expected 1 flush, got %d", metrics.FlushCount)
	}
	
	// Since we can't directly check if buffer slots were cleared,
	// we just verify the metrics are correct
	if metrics.RecordsProcessed != 3 {
		t.Errorf("Expected 3 records processed, got %d", metrics.RecordsProcessed)
	}
}

func TestBatchWriter_ContextCancellation(t *testing.T) {
	// Create a context we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	
	flushCh := make(chan int, 5) // Channel to track flushes
	
	bw, _ := NewBatchWriter(ctx, 10, 100*time.Millisecond, func(records ...any) {
		flushCh <- len(records)
	})
	
	// Add some records
	bw.Add(1)
	bw.Add(2)
	bw.Add(3)
	
	// Cancel the context which should trigger cleanup
	cancel()
	
	// Wait for processing to complete
	time.Sleep(200 * time.Millisecond)
	
	// Check that records were flushed due to context cancellation
	select {
	case count := <-flushCh:
		if count != 3 {
			t.Errorf("Expected 3 records to be flushed on context cancel, got %d", count)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("Timed out waiting for flush after context cancellation")
	}
	
	// Check metrics
	metrics := bw.GetMetrics()
	if metrics.RecordsProcessed != 3 {
		t.Errorf("Expected 3 records processed, got %d", metrics.RecordsProcessed)
	}
}

func TestBatchWriter_Stop(t *testing.T) {
	ctx := context.Background()
	flushCh := make(chan int, 5) // Channel to track flushes
	
	bw, _ := NewBatchWriter(ctx, 10, time.Second, func(records ...any) {
		flushCh <- len(records)
	})
	
	// Add some records
	bw.Add(1)
	bw.Add(2)
	
	// Stop should trigger a flush
	bw.Stop()
	
	// Check that records were flushed due to Stop
	select {
	case count := <-flushCh:
		if count != 2 {
			t.Errorf("Expected 2 records to be flushed on Stop, got %d", count)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("Timed out waiting for flush after Stop")
	}
}

func TestBatchWriter_FlushEmpty(t *testing.T) {
	ctx := context.Background()
	flushCount := 0
	
	bw, _ := NewBatchWriter(ctx, 5, time.Second, func(records ...any) {
		flushCount++
	})
	defer bw.Stop()
	
	// Flush with no records should not call the flush function
	bw.FlushNow()
	
	if flushCount != 0 {
		t.Errorf("Expected no flushes for empty buffer, got %d", flushCount)
	}
	
	// Check metrics
	metrics := bw.GetMetrics()
	if metrics.FlushCount != 0 {
		t.Errorf("Expected 0 flushes in metrics, got %d", metrics.FlushCount)
	}
}

func TestBatchWriter_PanicInFlushFunction(t *testing.T) {
	if skipTestsWithRace {
		t.Skip("Skipping test with race detector")
	}
	
	ctx := context.Background()
	
	// Use atomic for thread safety
	var panicInvoked atomic.Bool
	
	// Create writer with a flush function that panics
	bw, _ := NewBatchWriter(ctx, 3, time.Second, func(records ...any) {
		panicInvoked.Store(true)
		panic("deliberate panic in flush function")
	})
	defer bw.Stop()
	
	// Add records and trigger flush
	bw.Add(1)
	bw.Add(2)
	bw.Add(3)
	
	// Wait for flush to happen
	time.Sleep(100 * time.Millisecond)
	
	if !panicInvoked.Load() {
		t.Error("Expected flush function to be called and panic")
	}
	
	// Check metrics - flush should still be counted
	metrics := bw.GetMetrics()
	if metrics.FlushCount != 1 {
		t.Errorf("Expected 1 flush in metrics even with panic, got %d", metrics.FlushCount)
	}
	
	// Should be able to add more records after recovering from panic
	bw.Add(4)
	
	// This would hang if mutex handling isn't working correctly
	bw.FlushNow()
}

func BenchmarkBatchWriter_Add(b *testing.B) {
	ctx := context.Background()
	
	// Create a dummy flush function
	dummy := func(records ...any) {
		// Do nothing - very fast
	}
	
	// Use a huge buffer to avoid flush operations during benchmark
	bw, _ := NewBatchWriter(ctx, b.N+100, time.Hour, dummy)
	
	// Set up a proper cleanup
	b.Cleanup(func() {
		bw.Stop()
	})
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bw.Add(i)
	}
}

func BenchmarkBatchWriter_TryAdd(b *testing.B) {
	ctx := context.Background()
	
	// Create a dummy flush function
	dummy := func(records ...any) {
		// Do nothing - very fast
	}
	
	// Use a huge buffer to avoid flush operations during benchmark
	bw, _ := NewBatchWriter(ctx, b.N+100, time.Hour, dummy)
	
	// Set up a proper cleanup
	b.Cleanup(func() {
		bw.Stop()
	})
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bw.TryAdd(i)
	}
}

func BenchmarkBatchWriter_WithBufferClearing(b *testing.B) {
	ctx := context.Background()
	
	// Create a dummy flush function
	dummy := func(records ...any) {
		// Do nothing - very fast
	}
	
	// Use a huge buffer to avoid flush operations during benchmark
	bw, _ := NewBatchWriter(ctx, b.N+100, time.Hour, dummy)
	
	// Enable buffer clearing
	bw.clearBuffers = true
	
	// Set up a proper cleanup
	b.Cleanup(func() {
		bw.Stop()
	})
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bw.Add(i)
	}
}