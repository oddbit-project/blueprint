# BatchWriter

The BatchWriter is a high-performance, thread-safe component for buffering, batching, and asynchronously processing data
records.

## Overview

BatchWriter solves the common problem of efficiently collecting individual records and processing them in batches. It
offers:

- Thread-safe batch processing with configurable batch sizes
- Time-based automatic flushing
- Multiple record addition strategies (blocking, non-blocking, context-aware)
- Comprehensive performance metrics
- Memory optimization through buffer reuse
- Graceful shutdown with proper cleanup
- Panic recovery for flush operations

## Usage

### Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"github.com/oddbit-project/blueprint/batchwriter"
	"github.com/oddbit-project/blueprint/log"
	"time"
)

func main() {
	ctx := context.Background()
	logger := log.NewLogger()

	// Process records in batches of 100 or every 5 seconds, whichever comes first
	writer, err := batchwriter.NewBatchWriter(
		ctx,
		100,           // capacity
		5*time.Second, // flush interval 
		func(records ...any) {
			// Process the batch of records
			fmt.Printf("Processing %d records\n", len(records))
			for _, record := range records {
				// Process each record
				fmt.Printf("Record: %v\n", record)
			}
		},
	)
	if err != nil {
		panic(err)
	}

	// Set a logger for operations
	writer.SetLogger(logger)

	// Add records (blocking if buffer is full)
	writer.Add("record1")
	writer.Add("record2")

	// Try to add without blocking
	if !writer.TryAdd("record3") {
		fmt.Println("Buffer full, record dropped")
	}

	// Add with context awareness
	timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	if err := writer.AddWithContext(timeoutCtx, "record4"); err != nil {
		fmt.Printf("Failed to add record: %v\n", err)
	}

	// Get current metrics
	metrics := writer.GetMetrics()
	fmt.Printf("Records processed: %d\n", metrics.RecordsProcessed)

	// Force an immediate flush
	writer.FlushNow()

	// Gracefully stop the writer
	writer.Stop()
}
```

### Advanced Configuration

The BatchWriter supports functional options for advanced configuration:

```go
// Enable buffer clearing to help with garbage collection
writer, err := batchwriter.NewBatchWriter(
ctx,
1000,
time.Second,
processBatch,
batchwriter.WithClearBuffers(true),
batchwriter.WithLogger(logger),
)
```

## Performance Considerations

- **Add** - Blocking operation, use for critical data that must be processed
- **TryAdd** - Non-blocking, ~3.7x faster than Add, use for high-throughput scenarios where occasional drops are
  acceptable
- **Buffer Clearing** - Minimal performance impact (~6% slower) but helps with memory usage for long-running services

## API Reference

### Types

#### BatchWriter

```go
type BatchWriter struct {
// Contains unexported fields
}
```

The main type providing batch writing functionality.

#### Metrics

```go
type Metrics struct {
RecordsAdded      uint64 // Total number of records added
RecordsProcessed  uint64 // Total number of records processed by flush
RecordsDropped    uint64 // Total number of records dropped (TryAdd failures)
RecordsInBuffer   uint64        // Current number of records in write buffer
FlushCount        uint64        // Number of flushes performed
LastFlushDuration time.Duration // Duration of the last flush operation
AvgFlushDuration  time.Duration // Average flush duration
TotalFlushTime    time.Duration // Total time spent in flush operations
}
```

Provides comprehensive performance metrics for the BatchWriter.

#### FlushFn

```go
type FlushFn func (records ...any)
```

Function type for processing batches of records.

### Functions

#### NewBatchWriter

```go
func NewBatchWriter(
ctx context.Context,
capacity int,
flushInterval time.Duration,
flushFn FlushFn,
opts ...BatchWriterOption,
) (*BatchWriter, error)
```

Creates a new BatchWriter with the specified parameters:

- `ctx` - Context for lifecycle management
- `capacity` - Maximum batch size
- `flushInterval` - Maximum time between flushes
- `flushFn` - Function to process batches
- `opts` - Optional configuration options

Returns error if:

- `capacity` < 1 (`ErrCapacityTooSmall`)
- `flushFn` is nil (`ErrNilFlushFunction`)
- `flushInterval` < 1ms (`ErrInvalidFlushInterval`)

#### Option Functions

```go
func WithLogger(logger *log.Logger) BatchWriterOption
func WithClearBuffers(clear bool) BatchWriterOption
```

### Methods

#### Add

```go
func (b *BatchWriter) Add(record any)
```

Adds a record to the batch. Blocks if the input channel is full.

#### TryAdd

```go
func (b *BatchWriter) TryAdd(record any) bool
```

Attempts to add a record without blocking. Returns true if successful, false if the buffer is full.

#### AddWithContext

```go
func (b *BatchWriter) AddWithContext(ctx context.Context, record any) error
```

Attempts to add a record with context cancellation support. Returns error if context is done.

#### FlushNow

```go
func (b *BatchWriter) FlushNow()
```

Forces an immediate flush of the current buffer.

#### Stop

```go
func (b *BatchWriter) Stop()
```

Gracefully stops the BatchWriter and flushes any remaining records. Blocks until processing is complete.

#### GetMetrics

```go
func (b *BatchWriter) GetMetrics() Metrics
```

Returns the current performance metrics.

#### SetLogger

```go
func (b *BatchWriter) SetLogger(logger *log.Logger)
```

Sets a logger for operations.

## Error Handling

- If the flush function panics, the panic is recovered and logged if a logger is configured
- All BatchWriter operations continue to function normally after a panic in the flush function
- Errors from the flush function are not returned directly; use the logger to capture them

## Thread Safety

All BatchWriter operations are thread-safe. It can be safely used from multiple goroutines without additional
synchronization.

## Benchmarks

| Operation            | Operations/sec | ns/op | B/op | allocs/op |
|----------------------|----------------|-------|------|-----------|
| Add                  | 4,954,447      | 213.1 | 8    | 0         |
| TryAdd               | 29,089,545     | 57.7  | 8    | 0         |
| With Buffer Clearing | 6,011,731      | 225.5 | 8    | 0         |