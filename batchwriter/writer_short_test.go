package batchwriter

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// These tests are simplified versions of the main test file
// to avoid timeouts during initial testing

func TestSimple_Creation(t *testing.T) {
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

func TestSimple_Add(t *testing.T) {
	ctx := context.Background()
	var flushed atomic.Int32
	
	bw, _ := NewBatchWriter(ctx, 3, 100*time.Millisecond, func(records ...any) {
		flushed.Add(int32(len(records)))
	})
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
}

func TestSimple_TryAdd(t *testing.T) {
	ctx := context.Background()
	
	// Create an extremely slow flush function that sleeps
	bw, _ := NewBatchWriter(ctx, 2, time.Hour, func(records ...any) {
		// Simulate slow processing (this causes channel to fill up)
		time.Sleep(500 * time.Millisecond)
	})
	defer bw.Stop()
	
	// These should succeed
	if !bw.TryAdd(1) {
		t.Error("First TryAdd should succeed")
	}
	if !bw.TryAdd(2) {
		t.Error("Second TryAdd should succeed")
	}
	
	// Trigger a flush (will run in background)
	bw.FlushNow()
	
	// Add more items to the channel until we hit capacity
	// The channel capacity is 2, so we keep adding until we get a failure
	var failedCount int
	for i := 0; i < 20; i++ {
		if !bw.TryAdd(i) {
			failedCount++
			break
		}
		// Small delay between tries
		time.Sleep(time.Millisecond)
	}
	
	if failedCount == 0 {
		t.Error("Expected at least one TryAdd to fail due to full channel")
	}
	
	// Allow time for processing to complete
	time.Sleep(600 * time.Millisecond)
	
	// Verify metrics
	metrics := bw.GetMetrics()
	if metrics.RecordsDropped == 0 {
		t.Errorf("Expected at least one record to be dropped")
	}
}