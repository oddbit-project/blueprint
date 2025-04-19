package batchwriter

import (
	"context"
	"errors"
	"github.com/oddbit-project/blueprint/log"
	"sync"
	"sync/atomic"
	"time"
)

// ErrCapacityTooSmall is returned when batch capacity is less than 1
var ErrCapacityTooSmall = errors.New("batch capacity must be at least 1")

// ErrNilFlushFunction is returned when flush function is nil
var ErrNilFlushFunction = errors.New("flush function cannot be nil")

// ErrInvalidFlushInterval is returned when flush interval is less than 1ms
var ErrInvalidFlushInterval = errors.New("flush interval must be at least 1ms")

// Metrics contains the performance metrics for the BatchWriter
type Metrics struct {
	RecordsAdded      uint64        // Total number of records added
	RecordsProcessed  uint64        // Total number of records processed by flush
	RecordsDropped    uint64        // Total number of records dropped (TryAdd failures)
	RecordsInBuffer   uint64        // Current number of records in write buffer
	QueueCapacity     int           // Capacity of the input channel
	FlushCount        uint64        // Number of flushes performed
	LastFlushDuration time.Duration // Duration of the last flush operation
	AvgFlushDuration  time.Duration // Average flush duration
	TotalFlushTime    time.Duration // Total time spent in flush operations
}

// FlushFn represents a function that processes a batch of records
type FlushFn func(records ...any)

// BatchWriter handles batch writes with controlled flushing behaviors
type BatchWriter struct {
	writeBuffer   []any
	flushBuffer   []any
	idx           int
	wmx           sync.Mutex // protects writeBuffer and idx
	fmx           sync.Mutex // protects flushBuffer
	capacity      int
	queueCapacity int        // Capacity of the input channel
	flushInterval time.Duration
	inputChan     chan any
	stopChan      chan struct{}
	wg            sync.WaitGroup
	flushFn       FlushFn
	logger        *log.Logger

	// Metrics
	recordsAdded     atomic.Uint64
	recordsProcessed atomic.Uint64
	recordsDropped   atomic.Uint64
	flushCount       atomic.Uint64
	lastFlushTime    atomic.Int64
	totalFlushTime   atomic.Int64
	clearBuffers     bool
}

// BatchWriterOption is a functional option for configuring BatchWriter
type BatchWriterOption func(*BatchWriter)

// WithLogger sets a logger for the BatchWriter
func WithLogger(logger *log.Logger) BatchWriterOption {
	return func(b *BatchWriter) {
		b.logger = logger
	}
}

// WithClearBuffers enables clearing buffer slots after flush
func WithClearBuffers(clear bool) BatchWriterOption {
	return func(b *BatchWriter) {
		b.clearBuffers = clear
	}
}

// WithQueueCapacity sets the capacity of the input channel
// This controls how many records can be queued before TryAdd starts failing
func WithQueueCapacity(capacity int) BatchWriterOption {
	return func(b *BatchWriter) {
		if capacity > 0 {
			b.queueCapacity = capacity
		}
	}
}

// NewBatchWriter creates a new BatchWriter
func NewBatchWriter(ctx context.Context, capacity int, flushInterval time.Duration, flushFn FlushFn, opts ...BatchWriterOption) (*BatchWriter, error) {
	if capacity < 1 {
		return nil, ErrCapacityTooSmall
	}
	if flushFn == nil {
		return nil, ErrNilFlushFunction
	}
	if flushInterval < time.Millisecond {
		return nil, ErrInvalidFlushInterval
	}

	b := &BatchWriter{
		writeBuffer:   make([]any, capacity),
		flushBuffer:   make([]any, capacity),
		capacity:      capacity,
		queueCapacity: 10,               // Default queue capacity
		flushInterval: flushInterval,
		stopChan:      make(chan struct{}),
		flushFn:       flushFn,
		clearBuffers:  false, // disabled by default
	}
	
	// Apply the functional options
	for _, opt := range opts {
		opt(b)
	}
	
	// Create the input channel with the configured capacity
	b.inputChan = make(chan any, b.queueCapacity)

	b.wg.Add(1)
	go b.run(ctx)
	return b, nil
}

// SetLogger set logger to be used
func (b *BatchWriter) SetLogger(logger *log.Logger) {
	b.logger = logger
}

// Add queues a record for batch writing
// This method blocks if the input channel is full
func (b *BatchWriter) Add(record any) {
	b.inputChan <- record
	b.recordsAdded.Add(1)
}

// TryAdd attempts to add a record without blocking
// Returns true if successful, false if the buffer is full
func (b *BatchWriter) TryAdd(record any) bool {
	select {
	case b.inputChan <- record:
		b.recordsAdded.Add(1)
		return true
	default:
		// Channel is full, increment drop counter
		b.recordsDropped.Add(1)
		return false
	}
}

// AddWithContext attempts to add a record, respecting context cancellation
// Returns error if context is done or nil on success
func (b *BatchWriter) AddWithContext(ctx context.Context, record any) error {
	// First check if context is already canceled
	select {
	case <-ctx.Done():
		b.recordsDropped.Add(1)
		return ctx.Err()
	default:
		// Context not canceled, continue normally
	}
	
	// Try to add with context awareness
	select {
	case b.inputChan <- record:
		b.recordsAdded.Add(1)
		return nil
	case <-ctx.Done():
		b.recordsDropped.Add(1)
		return ctx.Err()
	}
}

// Stop gracefully stops the BatchWriter and flushes remaining records
// This method blocks until all pending records are processed
func (b *BatchWriter) Stop() {
	close(b.stopChan)
	b.wg.Wait()
}

// FlushNow forces an immediate flush of the current buffer
func (b *BatchWriter) FlushNow() {
	b.flush()
}

// GetMetrics returns the current performance metrics
func (b *BatchWriter) GetMetrics() Metrics {
	b.wmx.Lock()
	currentInBuffer := uint64(b.idx)
	b.wmx.Unlock()
	
	flushCount := b.flushCount.Load()
	totalFlushTime := time.Duration(b.totalFlushTime.Load())
	var avgFlushDuration time.Duration
	if flushCount > 0 {
		avgFlushDuration = totalFlushTime / time.Duration(flushCount)
	}
	
	return Metrics{
		RecordsAdded:      b.recordsAdded.Load(),
		RecordsProcessed:  b.recordsProcessed.Load(),
		RecordsDropped:    b.recordsDropped.Load(),
		RecordsInBuffer:   currentInBuffer,
		QueueCapacity:     b.queueCapacity,
		FlushCount:        flushCount,
		LastFlushDuration: time.Duration(b.lastFlushTime.Load()),
		AvgFlushDuration:  avgFlushDuration,
		TotalFlushTime:    totalFlushTime,
	}
}

// run is the main processing loop
func (b *BatchWriter) run(ctx context.Context) {
	defer b.wg.Done()
	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case record, ok := <-b.inputChan:
			if !ok {
				// Channel closed, rare case
				b.flush()
				return
			}
			b.add(record)
		case <-ticker.C:
			b.flush()
		case <-b.stopChan:
			// Process any remaining items in the input channel
			b.drainAndStop()
			return
		case <-ctx.Done():
			// Context cancelled
			b.drainAndStop()
			return
		}
	}
}

// drainAndStop processes any remaining items and performs final flush
func (b *BatchWriter) drainAndStop() {
	// Process any remaining items in the channel (non-blocking)
	if b.logger != nil {
		b.logger.Infof("Shutting down, processing remaining items...")
	}
	for {
		select {
		case record, ok := <-b.inputChan:
			if !ok {
				break
			}
			b.add(record)
		default:
			// No more items in the channel
			b.flush()
			return
		}
	}
}

// add inserts a record into the buffer and flushes if buffer is full
func (b *BatchWriter) add(record any) {
	b.wmx.Lock()
	
	// Store the record
	b.writeBuffer[b.idx] = record
	b.idx++
	
	// Check if we need to flush
	needFlush := b.idx >= b.capacity
	
	// If we don't need to flush, just unlock and return
	if !needFlush {
		b.wmx.Unlock()
		return
	}
	
	// We need to flush, do it with the lock still held
	b.flushLocked()
	
	// flushLocked will re-acquire write mutex before returning
}

// flush safely flushes the current buffer
func (b *BatchWriter) flush() {
	b.wmx.Lock()
	// Don't use defer here as flushLocked already manages the lock
	if b.idx > 0 {
		b.flushLocked()
	} else {
		// Nothing to flush, just release the lock
		b.wmx.Unlock()
	}
}

// flushLocked flushes the buffer (assumes wmx already held)
func (b *BatchWriter) flushLocked() {
	if b.idx == 0 {
		// Nothing to flush
		return
	}

	// Swap buffers while holding both locks to ensure clean handoff
	b.fmx.Lock()
	tmp := b.writeBuffer
	b.writeBuffer = b.flushBuffer
	b.flushBuffer = tmp
	flushLen := b.idx
	b.idx = 0

	// Create a copy of the slice to be flushed to avoid potential
	// race conditions if flushBuffer is modified during async flush
	toFlush := make([]any, flushLen)
	copy(toFlush, b.flushBuffer[:flushLen])

	// Release the write mutex before flushing to allow more items to be added
	b.wmx.Unlock()

	// Record start time for metrics
	startTime := time.Now()
	
	// Use a separate goroutine to perform the actual flush
	// This prevents blocking the main loop but still maintains proper lock handling
	go func() {
		defer func() {
			// Recovery mechanism
			if r := recover(); r != nil {
				if b.logger != nil {
					b.logger.Warnf("Recovered from panic in flush function: %v", r)
				}
			}
			
			// Update metrics
			flushDuration := time.Since(startTime)
			b.lastFlushTime.Store(int64(flushDuration))
			b.totalFlushTime.Add(int64(flushDuration))
			b.flushCount.Add(1)
			b.recordsProcessed.Add(uint64(flushLen))
			
			// Clear buffer slots if enabled
			if b.clearBuffers {
				for i := 0; i < flushLen; i++ {
					b.flushBuffer[i] = nil
				}
			}
			
			// Always unlock the flush mutex at the end
			b.fmx.Unlock()
		}()
		
		if b.logger != nil {
			b.logger.Infof("Flushing %d records", flushLen)
		}
		
		// Execute the flush function with the copied slice
		b.flushFn(toFlush...)
	}()
}