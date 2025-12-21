# ThreadPool

The ThreadPool package provides a robust, flexible worker pool implementation for Go applications. 
It efficiently manages a pool of goroutines (workers) that execute jobs from a shared queue, providing graceful 
resource management and concurrent execution control.

## Overview

ThreadPool implements the classic worker pool pattern with several important features:

- Thread-safe job dispatch with multiple dispatch options
- Controlled concurrency with fixed worker count
- Automatic worker recovery from panics
- Queue-based job management
- Context cancellation support
- Performance metrics

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "github.com/oddbit-project/blueprint/log"
    "github.com/oddbit-project/blueprint/threadpool"
    "time"
)

// Define a job that implements the threadpool.Job interface
type MyJob struct {
    ID int
}

// Run is required by the threadpool.Job interface
func (j *MyJob) Run(ctx context.Context) {
    fmt.Printf("Job %d is running\n", j.ID)
    // Simulate work
    time.Sleep(100 * time.Millisecond)
    fmt.Printf("Job %d completed\n", j.ID)
}

func main() {
    // Create a logger
    logger := log.New("threadpool")

    // Create a thread pool with 5 workers and a queue capacity of 10
    // The pool will process at most 5 jobs concurrently
    pool, err := threadpool.NewThreadPool(5, 10, threadpool.WithLogger(logger))
    if err != nil {
        panic(err)
    }

    // Start the pool with a context
    ctx := context.Background()
    if err := pool.Start(ctx); err != nil {
        panic(err)
    }

    // Use defer to ensure the pool is gracefully stopped
    defer pool.Stop()

    // Submit 20 jobs to the pool
    for i := 0; i < 20; i++ {
        job := &MyJob{ID: i}
        
        // Option 1: Blocking dispatch (blocks if queue is full)
        pool.Dispatch(job)
        
        // Option 2: Non-blocking dispatch (returns false if queue is full)
        // if !pool.TryDispatch(job) {
        //     fmt.Printf("Queue full, job %d rejected\n", i)
        // }
        
        // Option 3: Dispatch with timeout
        // if !pool.DispatchWithTimeout(job, 100*time.Millisecond) {
        //     fmt.Printf("Timeout while dispatching job %d\n", i)
        // }
        
        // Option 4: Dispatch with context cancellation support
        // err := pool.DispatchWithContext(ctx, job)
        // if err != nil {
        //     fmt.Printf("Context cancelled while dispatching job %d: %v\n", i, err)
        // }
    }

    // Display pool metrics
    fmt.Printf("Total jobs processed: %d\n", pool.GetRequestCount())
    fmt.Printf("Jobs in queue: %d\n", pool.GetQueueLen())
    fmt.Printf("Worker count: %d\n", pool.GetWorkerCount())

    // Wait for all jobs to complete (in a real application, you might want to use a WaitGroup or similar)
    time.Sleep(1 * time.Second)
}
```

### Handling Job Panics

The ThreadPool automatically recovers from panics in job execution, ensuring that worker goroutines continue to operate even if a job panics:

```go
// Even if this job panics, the worker will recover and continue processing
pool.Dispatch(&PanicJob{})

// Subsequent jobs will still be processed
pool.Dispatch(&NormalJob{})
```

### Context Cancellation

You can stop all workers gracefully by canceling the context:

```go
// Create a context that can be canceled
ctx, cancel := context.WithCancel(context.Background())

// Start the pool with this context
pool.Start(ctx)

// Later, when you want to stop processing:
cancel()
// All workers will finish their current job and then exit
```

## API Reference

### Types

#### ThreadPool

```go
type ThreadPool struct {
    // Contains unexported fields
}
```

The main type providing thread pool functionality.

#### Job

```go
type Job interface {
    Run(ctx context.Context)
}
```

Interface that must be implemented by all jobs submitted to the thread pool.

#### Pool

```go
type Pool interface {
    Start(ctx context.Context) error
    Stop() error
    Dispatch(j Job)
    TryDispatch(j Job) bool
    DispatchWithTimeout(j Job, timeout time.Duration) bool
    DispatchWithContext(ctx context.Context, j Job) error
    GetRequestCount() uint64
    GetQueueLen() int
    GetQueueCapacity() int
    GetWorkerCount() int
}
```

Interface defining all pool operations. Useful for mocking in tests.

#### FuncRunner

```go
func FuncRunner(job func(ctx context.Context)) Job
```

Helper function that wraps a simple function as a Job. This allows using anonymous functions directly without creating a struct.

**Example:**
```go
pool, _ := threadpool.NewThreadPool(5, 10)
pool.Start(context.Background())

// Using FuncRunner instead of creating a Job struct
pool.Dispatch(threadpool.FuncRunner(func(ctx context.Context) {
    // Your job logic here
    fmt.Println("Job executed!")
}))

// With closure over variables
userID := 123
pool.Dispatch(threadpool.FuncRunner(func(ctx context.Context) {
    processUser(ctx, userID)
}))
```

### Functions

#### NewThreadPool

```go
func NewThreadPool(
    workerCount int, 
    queueSize int, 
    opts ...OptionsFn,
) (*ThreadPool, error)
```

Creates a new thread pool with the specified parameters:
- `workerCount` - Number of worker goroutines to create
- `queueSize` - Capacity of the job queue
- `opts` - Optional functional options

Returns error if:
- `workerCount` < 1 (`ErrInvalidWorkerCount`)
- `queueSize` < 1 (`ErrInvalidQueueSize`)

#### Available Options

```go
func WithLogger(logger *log.Logger) OptionsFn
```

Attaches a logger to the thread pool for operation logging and panic recovery.

### Methods

#### Start

```go
func (t *ThreadPool) Start(ctx context.Context) error
```

Starts the thread pool workers. The context allows for graceful cancellation. 
Returns `ErrPoolAlreadyStarted` if the pool has already been started.

#### Stop

```go
func (t *ThreadPool) Stop() error
```

Gracefully stops all workers after they complete their current jobs.
Returns `ErrPoolNotStarted` if the pool has not been started.

#### Dispatch

```go
func (t *ThreadPool) Dispatch(j Job)
```

Adds a job to the queue. Blocks if the queue is full.

#### TryDispatch

```go
func (t *ThreadPool) TryDispatch(j Job) bool
```

Attempts to add a job to the queue without blocking. Returns true if successful,
false if the queue is full.

#### DispatchWithTimeout

```go
func (t *ThreadPool) DispatchWithTimeout(j Job, timeout time.Duration) bool
```

Attempts to add a job to the queue with a timeout. Returns true if successful,
false if the timeout was reached.

#### DispatchWithContext

```go
func (t *ThreadPool) DispatchWithContext(ctx context.Context, j Job) error
```

Attempts to add a job to the queue, respecting context cancellation. Returns nil if successful,
the context error if the context was canceled.

#### Metrics Methods

```go
func (t *ThreadPool) GetRequestCount() uint64
func (t *ThreadPool) GetQueueLen() int
func (t *ThreadPool) GetQueueCapacity() int
func (t *ThreadPool) GetWorkerCount() int
```

Methods for retrieving performance metrics and status information.

## Best Practices

1. **Choose Appropriate Worker Count**: The ideal number of workers depends on the nature of the tasks:
   - CPU-bound tasks: roughly equal to the number of CPU cores
   - I/O-bound tasks: higher than CPU cores (experiment to find optimal)

2. **Queue Size Management**: Set queue size based on expected job arrival rate and acceptable latency:
   - Smaller queues: Less memory usage, potentially more dropped jobs
   - Larger queues: Higher memory usage, lower job rejection rate

3. **Error Handling**: Even though the pool recovers from panics, implement proper error handling in jobs:
   ```go
   func (j *MyJob) Run(ctx context.Context) {
       defer func() {
           if r := recover(); r != nil {
               // Log the error but don't re-panic
               log.Printf("Job recovered from panic: %v", r)
           }
       }()
       
       // Actual job logic
   }
   ```

4. **Context Awareness**: Make jobs respect the context for cancellation:
   ```go
   func (j *MyJob) Run(ctx context.Context) {
       select {
       case <-ctx.Done():
           // Clean up and exit
           return
       default:
           // Continue processing
       }
       
       // For longer operations, check context periodically
       for i := 0; i < steps; i++ {
           if ctx.Err() != nil {
               return
           }
           // Do work step
       }
   }
   ```

5. **Clean Shutdown**: Always call `Stop()` to ensure all resources are cleaned up properly.

## Thread Safety

All ThreadPool operations are thread-safe and can be called from multiple goroutines concurrently.

## Performance Considerations

- **Job Design**: Keep jobs appropriately sized - not too small (overhead dominates) or too large (blocks workers)
- **Dispatch Method**: Use the appropriate dispatch method based on your needs:
  - `Dispatch`: When jobs must be processed and you can afford to wait
  - `TryDispatch`: When jobs can be dropped if system is under load
  - `DispatchWithTimeout`: When you need bounded wait times
  - `DispatchWithContext`: When you need cancellation support
- **Worker Count**: Monitor and adjust worker count based on CPU usage and throughput requirements