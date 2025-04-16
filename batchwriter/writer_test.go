package batchwriter

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

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
	ctx := context.Background()

	flushReady := make(chan struct{})
	flushDone := make(chan struct{})

	// Create a BatchWriter with controlled flush timing
	bw, _ := NewBatchWriter(ctx, 2, time.Hour, func(records ...any) {
		// Signal that we're ready to flush
		flushReady <- struct{}{}
		// Wait until test says we can proceed
		<-flushDone
	})
	defer bw.Stop()

	// Add two items to fill buffer and trigger a flush
	bw.Add(1)
	bw.Add(2) // This should trigger a flush

	// Wait for flush to begin
	<-flushReady

	// Now input channel should be empty, add two more items
	success1 := bw.TryAdd(3)
	success2 := bw.TryAdd(4)

	// This attempt should fail as channel capacity is 2
	success3 := bw.TryAdd(5)

	// Allow the flush to complete
	flushDone <- struct{}{}

	if !success1 {
		t.Error("First TryAdd should succeed")
	}
	if !success2 {
		t.Error("Second TryAdd should succeed")
	}
	if success3 {
		t.Error("Third TryAdd should fail when channel is full")
	}

	// Check metrics for dropped records
	metrics := bw.GetMetrics()
	if metrics.RecordsDropped != 1 {
		t.Errorf("Expected 1 record dropped, got %d", metrics.RecordsDropped)
	}
}

func TestBatchWriter_AddWithContext(t *testing.T) {
	ctx := context.Background()
	cancelCtx, cancel := context.WithCancel(context.Background())

	flushReady := make(chan struct{})
	flushDone := make(chan struct{})

	// Create a BatchWriter with controlled flush
	bw, _ := NewBatchWriter(ctx, 2, time.Hour, func(records ...any) {
		flushReady <- struct{}{}
		<-flushDone
	})
	defer bw.Stop()

	// Fill the buffer and trigger flush
	bw.Add(1)
	bw.Add(2)

	// Wait for flush to begin
	<-flushReady

	// Add two more items to fill channel
	err1 := bw.AddWithContext(context.Background(), 3)
	err2 := bw.AddWithContext(context.Background(), 4)

	// Cancel the context and try to add one more
	cancel()
	err3 := bw.AddWithContext(cancelCtx, 5)

	// Complete the flush
	flushDone <- struct{}{}

	if err1 != nil {
		t.Errorf("First AddWithContext should succeed, got: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second AddWithContext should succeed, got: %v", err2)
	}
	if err3 == nil {
		t.Error("AddWithContext should return error when context is canceled")
	}

	// Check metrics
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

	// Create BatchWriter with buffer clearing enabled
	clearBuffers := true
	bw, _ := NewBatchWriter(ctx, 3, time.Second, func(records ...any) {})
	// Set buffer clearing
	bw.clearBuffers = clearBuffers
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
	ctx := context.Background()
	panicInvoked := false

	// Create writer with a flush function that panics
	bw, _ := NewBatchWriter(ctx, 3, time.Second, func(records ...any) {
		panicInvoked = true
		panic("deliberate panic in flush function")
	})
	defer bw.Stop()

	// Add records and trigger flush
	bw.Add(1)
	bw.Add(2)
	bw.Add(3)

	// Wait for flush to happen
	time.Sleep(100 * time.Millisecond)

	if !panicInvoked {
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
	bw, _ := NewBatchWriter(ctx, 1000, time.Second, func(records ...any) {
		// Do nothing
	})
	defer bw.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bw.Add(i)
	}
}

func BenchmarkBatchWriter_TryAdd(b *testing.B) {
	ctx := context.Background()
	bw, _ := NewBatchWriter(ctx, 1000, time.Second, func(records ...any) {
		// Do nothing
	})
	defer bw.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bw.TryAdd(i)
	}
}

func BenchmarkBatchWriter_WithBufferClearing(b *testing.B) {
	ctx := context.Background()
	bw, _ := NewBatchWriter(ctx, 1000, time.Second, func(records ...any) {
		// Do nothing
	})
	// Enable buffer clearing
	bw.clearBuffers = true
	defer bw.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bw.Add(i)
	}
}
