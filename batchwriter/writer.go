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

// WithDisableClearBuffers disables clearing buffer slots after flush
func WithDisableClearBuffers() BatchWriterOption {
	return func(b *BatchWriter) {
		b.clearBuffers = false
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
		flushInterval: flushInterval,
		inputChan:     make(chan any, capacity),
		stopChan:      make(chan struct{}),
		flushFn:       flushFn,
		clearBuffers:  true, // enabled by default
	}

	// Apply the functional options
	for _, opt := range opts {
		opt(b)
	}

	b.wg.Add(1)
	go b.run(ctx)
	return b, nil
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
		b.recordsDropped.Add(1)
		return false
	}
}

// AddWithContext attempts to add a record, respecting context cancellation
// Returns error if context is done or nil on success
func (b *BatchWriter) AddWithContext(ctx context.Context, record any) error {
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
	defer b.wmx.Unlock()

	b.writeBuffer[b.idx] = record
	b.idx++
	if b.idx >= b.capacity {
		b.flushLocked()
	}
}

// flush safely flushes the current buffer
func (b *BatchWriter) flush() {
	b.wmx.Lock()
	defer b.wmx.Unlock()
	b.flushLocked()
}

// flushLocked flushes the buffer (assumes wmx already held)
func (b *BatchWriter) flushLocked() {
	if b.idx == 0 {
		// Nothing to flush
		return
	}

	// Acquire flush mutex to protect flushBuffer during flush operation
	b.fmx.Lock()

	// Swap buffers
	tmp := b.writeBuffer
	b.writeBuffer = b.flushBuffer
	b.flushBuffer = tmp
	flushLen := b.idx
	b.idx = 0

	// Release the write mutex before flushing to allow more items to be added
	b.wmx.Unlock()

	// Record start time for metrics
	startTime := time.Now()

	// Safely flush the data with recovery for panics
	func() {
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

			b.fmx.Unlock() // Always unlock the flush mutex
		}()

		if b.logger != nil {
			b.logger.Infof("Flushing %d records", flushLen)
		}

		// Execute the flush function with the correct slice length
		b.flushFn(b.flushBuffer[:flushLen]...)
	}()

	// Re-acquire write mutex as the deferred unlock in the calling function expects it
	b.wmx.Lock()
}
