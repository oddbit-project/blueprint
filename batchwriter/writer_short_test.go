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
	
	// Create BatchWriter with a capacity of 3 but a channel of capacity 2
	bw, _ := NewBatchWriter(ctx, 3, time.Hour, func(records ...any) {
		// Do nothing
	})
	defer bw.Stop()
	
	// These should succeed (channel capacity is 3)
	success1 := bw.TryAdd(1)
	success2 := bw.TryAdd(2)
	success3 := bw.TryAdd(3)
	
	// This might fail depending on timing
	success4 := bw.TryAdd(4)
	
	if !success1 || !success2 || !success3 {
		t.Error("First three TryAdd calls should succeed")
	}
	
	// Check metrics for drops
	metrics := bw.GetMetrics()
	if !success4 && metrics.RecordsDropped != 1 {
		t.Errorf("Expected 1 record dropped, got %d", metrics.RecordsDropped)
	}
}