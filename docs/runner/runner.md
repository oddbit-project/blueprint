# Runner

The runner package provides `PeriodicRunner`, a minimal helper that executes a function at a fixed
interval in a background goroutine until it is stopped or its context is cancelled.

```go
import "github.com/oddbit-project/blueprint/runner"
```

## Overview

- Executes a `RunnerFn` on every tick of a `time.Ticker`
- Runs in a single background goroutine; executions never overlap
- Errors returned by the function are logged and do not stop the runner
- Panics in the function are recovered and logged; the runner keeps going
- Stops on `Stop()` or when the context passed to `Start()` is cancelled
- Can be restarted after it has stopped
- `Start` and `Stop` are safe to call concurrently

## Usage

### Basic Usage

```go
package main

import (
	"context"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/runner"
)

func main() {
	logger := log.New("cache-refresh")

	refresh := func(ctx context.Context) error {
		// ctx is cancelled when the runner is stopped
		logger.Info("refreshing cache")
		return nil
	}

	r, err := runner.NewUpdater(30*time.Second, refresh, logger)
	if err != nil {
		logger.Error(err, "invalid runner configuration")
		return
	}

	if err := r.Start(context.Background()); err != nil {
		logger.Error(err, "failed to start runner")
		return
	}

	// ... application runs ...
	time.Sleep(2 * time.Minute)

	// wait at most 5 seconds for an in-flight execution to finish
	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := r.Stop(stopCtx); err != nil {
		logger.Error(err, "runner did not stop in time")
	}
}
```

The first execution happens one interval after `Start()`, not immediately.

### With the Application Container

Starting the runner with the container's application context and stopping it from a destructor
ties it to the application lifecycle (see [Application Container](../container.md)):

```go
package main

import (
	"context"
	"time"

	"github.com/oddbit-project/blueprint"
	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/runner"
)

func main() {
	logger := log.New("app")
	app := blueprint.NewContainer(provider.NewEnvProvider("APP_", false))

	r, err := runner.NewUpdater(time.Minute, func(ctx context.Context) error {
		logger.Info("periodic task")
		return nil
	}, logger)
	app.AbortFatal(err)

	blueprint.RegisterDestructor(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return r.Stop(ctx)
	})

	app.Run(func(a interface{}) error {
		return r.Start(app.GetContext())
	})
}
```

## API Reference

### Constants

```go
const (
	ErrInvalidInterval = utils.Error("interval must be positive")
	ErrNilRunnerFn     = utils.Error("runner function must not be nil")
	ErrNilLogger       = utils.Error("logger must not be nil")
	ErrAlreadyRunning  = utils.Error("already running")
	ErrNotRunning      = utils.Error("not running")
)
```

`ErrInvalidInterval`, `ErrNilRunnerFn` and `ErrNilLogger` are validation errors returned by `NewUpdater`.
`ErrAlreadyRunning` is returned by `Start` and `ErrNotRunning` by `Stop`; match them with `errors.Is`.

### Types

#### RunnerFn

```go
type RunnerFn func(ctx context.Context) error
```

Function executed on each tick. `ctx` is derived from the context passed to `Start` and is cancelled
by `Stop`.

#### PeriodicRunner

```go
type PeriodicRunner struct {
	// contains unexported fields
}
```

Periodic executor created by `NewUpdater`.

### Functions

#### NewUpdater

```go
func NewUpdater(updateInterval time.Duration, updateFn RunnerFn, logger *log.Logger) (*PeriodicRunner, error)
```

Creates a stopped `PeriodicRunner`. `logger` is a `*log.Logger` from `github.com/oddbit-project/blueprint/log`.

Returns an error if:

- `updateInterval <= 0` (`ErrInvalidInterval`)
- `updateFn` is nil (`ErrNilRunnerFn`)
- `logger` is nil (`ErrNilLogger`)

### Methods

#### Start

```go
func (u *PeriodicRunner) Start(ctx context.Context) error
```

Starts the background goroutine and returns immediately. Returns `ErrAlreadyRunning` if the runner is
already running. Cancelling `ctx` stops the runner.

#### Stop

```go
func (u *PeriodicRunner) Stop(ctx context.Context) error
```

Cancels the runner context and waits for the background goroutine to exit. Returns:

- `nil` once the goroutine has exited
- `ErrNotRunning` if the runner was never started or has already been stopped by `Stop`
- `ctx.Err()` if `ctx` is done before the goroutine exits (e.g. `context.DeadlineExceeded`); no
  background goroutine is left waiting

## Error Handling

- Errors returned by `RunnerFn` are logged at error level (`"runner function error"`); execution
  continues on the next tick
- Panics in `RunnerFn` are recovered and logged at warning level; execution continues on the next tick
- `Start` and `Stop` return the exported `ErrAlreadyRunning` and `ErrNotRunning` values, which can be
  matched with `errors.Is`

## Notes and Gotchas

- **Stop logging.** A normal stop (via `Stop` or cancellation of the context passed to `Start`) logs
  nothing at error level. If the parent context ends because its deadline expired, the loop returns
  `context.DeadlineExceeded`, which is logged at error level as `"runner error"`.
- **Stop does not interrupt the function.** `Stop` only cancels the context; a `RunnerFn` that ignores
  its context keeps running. If `Stop` times out, the runner remains in the running state, and `Stop`
  can be called again to keep waiting.
- **Parent context cancellation.** If the context passed to `Start` is cancelled, the runner stops on
  its own; the first subsequent `Stop` returns `nil`, and later calls return `ErrNotRunning`.
- **Slow functions skip ticks.** Executions are sequential on the ticker goroutine; if `RunnerFn`
  takes longer than the interval, missed ticks are dropped (standard `time.Ticker` behaviour) rather
  than queued.
- **Restart.** After the runner has stopped (via `Stop` or parent context cancellation), `Start` can be
  called again.
- **Concurrent Start/Stop.** The runner state is protected by a mutex, so `Start` and `Stop` can be
  called concurrently from different goroutines.
