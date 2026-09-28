# Application Container

The root `blueprint` package provides a small application container (`Container`) and a process-wide
shutdown manager. Together they give an application a shared configuration provider, a cancellable
application context, a simple named service registry, OS signal handling and an ordered list of
cleanup functions ("destructors") that run when the application shuts down.

```go
import "github.com/oddbit-project/blueprint"
```

## Overview

- `Container` holds a `config.ConfigProvider`, an application `context.Context` and its `CancelFunc`
- A thread-safe, name-keyed service registry (`Register`, `Get`, `Exists`)
- `Run` executes a list of startup functions and then blocks, waiting for `SIGINT`, `SIGTERM` or `SIGHUP`
- Destructors are registered globally with `RegisterDestructor` and executed in reverse registration order by `Shutdown`
- `AbortFatal` runs the destructors, logs the error and terminates the process
- Destructors run at most once per process

The shutdown manager is package-level state, not part of `Container`: every container in the process
shares the same destructor list. Blueprint's own `log` package registers a destructor from its `init()`
function to close log files opened by file logging.

## Usage

### Basic Application

```go
package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/oddbit-project/blueprint"
	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/utils"
)

func main() {
	utils.PanicOnError(log.Configure(log.NewDefaultConfig()))
	logger := log.New("my-app")

	cfg, err := provider.NewJsonProvider("config.json")
	if err != nil {
		logger.Error(err, "failed to load configuration")
		return
	}

	app := blueprint.NewContainer(cfg)

	srv := &http.Server{Addr: ":8080"}

	// executed by Shutdown(), in reverse registration order
	blueprint.RegisterDestructor(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	})

	// Run() never returns; the process exits when a signal is received
	// or when the application context is cancelled
	app.Run(func(a interface{}) error {
		c := a.(*blueprint.Container)
		go func() {
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				c.AbortFatal(err)
			}
		}()
		return nil
	})
}
```

The functions passed to `Run` are expected to be non-blocking: they are executed sequentially on the
calling goroutine, and the signal loop only starts after all of them return. Long-running work
(servers, consumers) should be started in its own goroutine, as above.

### Embedding the Container

Applications usually embed `*blueprint.Container` in their own application type (see
`samples/httpserver-session` and `samples/application`):

```go
package main

import (
	"github.com/oddbit-project/blueprint"
	"github.com/oddbit-project/blueprint/config/provider"
	"github.com/oddbit-project/blueprint/log"
)

type AppConfig struct {
	Name string `json:"name"`
}

type Application struct {
	*blueprint.Container
	logger *log.Logger
}

func NewApplication(configFile string) (*Application, error) {
	cfg, err := provider.NewJsonProvider(configFile)
	if err != nil {
		return nil, err
	}
	return &Application{
		Container: blueprint.NewContainer(cfg),
		logger:    log.New("application"),
	}, nil
}

func (a *Application) Build() {
	cfg := &AppConfig{}
	// AbortFatal is a no-op for nil errors
	a.AbortFatal(a.Config.Get(cfg))
	a.logger.Infof("building %s", cfg.Name)
}

func main() {
	app, err := NewApplication("config.json")
	if err != nil {
		panic(err)
	}
	app.Build()
	app.Run()
}
```

### Service Registry

The container keeps a map of named services. Values are stored as `interface{}` and must be
type-asserted when retrieved:

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint"
	"github.com/oddbit-project/blueprint/config/provider"
)

type Mailer struct {
	From string
}

func main() {
	app := blueprint.NewContainer(provider.NewEnvProvider("APP_", false))

	app.Register("mailer", &Mailer{From: "noreply@example.com"})

	if app.Exists("mailer") {
		svc, _ := app.Get("mailer")
		mailer := svc.(*Mailer)
		fmt.Println(mailer.From)
	}

	if _, ok := app.Get("cache"); !ok {
		fmt.Println("cache not registered")
	}
}
```

`Register` overwrites any service previously registered under the same name.

### Shutdown and Destructors

Destructors are `func() error` callbacks (`callstack.CallableFn`) registered process-wide. `Shutdown`
executes them in reverse order of registration (last registered, first executed). The destructor list
is discarded before the destructors run, so subsequent `Shutdown` calls do not run them again:

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint"
)

func main() {
	blueprint.RegisterDestructor(func() error {
		fmt.Println("close database") // runs second
		return nil
	})
	blueprint.RegisterDestructor(func() error {
		fmt.Println("stop http server") // runs first
		return nil
	})

	blueprint.Shutdown(nil)
	blueprint.Shutdown(nil) // destructors already executed, nothing to run
}
```

All destructors are executed even if some of them fail; their errors are combined with `errors.Join`
and logged once with the message `"Error while shutting down"`.

When using `Container.Run`, `Shutdown` is invoked automatically on `SIGINT`, `SIGTERM` and `SIGHUP`,
when the application context is cancelled, and by `AbortFatal`.

### Aborting on Fatal Errors

`AbortFatal` is designed for inline error checks during application setup. It does nothing when the
error is `nil`; otherwise it runs the destructors, logs the error at fatal level and exits:

```go
a.AbortFatal(a.Config.Get(cfg))
a.AbortFatal(cfg.Validate())
```

If a function passed to `Run` returns an error, `Run` calls `AbortFatal` with it.

## Lifecycle

`Run(mainFn ...RuntimeFn)` performs the following steps:

1. Registers a signal channel (buffer size 1) for `SIGINT`, `SIGTERM` and `SIGHUP`
2. Calls each `mainFn` in order, passing the container itself; the first error triggers `AbortFatal(err)`
3. Loops forever:
    - on a signal: logs `"Shutting down application..."`, cancels the application context and then calls
      `Shutdown(nil)` (destructors run); the cancelled context is picked up on the next iteration
    - on application context cancellation: stops signal notification, calls `Shutdown(nil)` (destructors
      run, unless they already did) and calls `Terminate(nil)`, which exits the process

`Run` never returns.

How the process exits depends on the path taken:

| Trigger | Destructors run | Exit code |
|---------|-----------------|-----------|
| `SIGINT` / `SIGTERM` / `SIGHUP` received by `Run` | yes | `0` |
| `CancelCtx()` called while `Run` is looping | yes | `0` |
| `AbortFatal(err)` / `mainFn` returns an error | yes | `1` (from the fatal log call) |
| `AbortFatal(err)` after `Shutdown` has already run | no (already executed) | `1` (from the fatal log call) |
| `Terminate(nil)` | **no** | `0` |
| `Terminate(err)` with `err != nil` | **no** | `-1` (255 on Unix) |

## API Reference

### Types

#### RuntimeFn

```go
type RuntimeFn func(app interface{}) error
```

Startup function executed by `Container.Run`. The `app` argument is the `*Container` on which `Run`
was called.

#### Container

```go
type Container struct {
	Config    config.ConfigProvider
	Context   context.Context
	CancelCtx context.CancelFunc
	// contains unexported fields
}
```

- `Config` - configuration provider passed to `NewContainer`
- `Context` - application context, derived from `context.Background()`
- `CancelCtx` - cancels `Context`; when `Run` is active, cancelling it runs the destructors and terminates the process

### Functions

#### NewContainer

```go
func NewContainer(config config.ConfigProvider) *Container
```

Creates a container with the given configuration provider, a new cancellable application context
and an empty service registry. The provider is not validated and may be `nil`.

#### RegisterDestructor

```go
func RegisterDestructor(fn callstack.CallableFn)
```

Adds `fn` (`func() error`) to the global destructor list. Destructors are executed by `Shutdown` in
reverse registration order. Calling `RegisterDestructor` while `Shutdown` is running, or after it has
run, is a no-op: `fn` is silently discarded.

#### GetDestructorManager

```go
func GetDestructorManager() *callstack.CallStack
```

Returns the global destructor `*callstack.CallStack` (package `github.com/oddbit-project/blueprint/types/callstack`).
Returns `nil` after `Shutdown` has run.

#### Shutdown

```go
func Shutdown(arg error)
```

Clears the destructor list and runs the destructors it held (in reverse order), so the destructors are
executed only once. Calls are serialized: a concurrent caller waits until the running `Shutdown` has
finished its destructors, and later calls find no destructors to run. Destructor errors are combined
with `errors.Join` and logged at error level. If `arg` is not nil, it is then logged with the global
zerolog logger at fatal level, which exits the process with code 1; this happens on every call with a
non-nil `arg`, including when the destructors have already run.

### Methods

#### Register

```go
func (c *Container) Register(name string, service interface{})
```

Stores `service` under `name`, replacing any existing entry. Thread-safe.

#### Get

```go
func (c *Container) Get(name string) (interface{}, bool)
```

Returns the service registered under `name` and whether it exists. Thread-safe.

#### Exists

```go
func (c *Container) Exists(name string) bool
```

Reports whether a service is registered under `name`. Thread-safe.

#### GetContext

```go
func (c *Container) GetContext() context.Context
```

Returns the application context (`c.Context`).

#### Run

```go
func (c *Container) Run(mainFn ...RuntimeFn)
```

Executes the startup functions and blocks, handling OS signals and context cancellation as described
in [Lifecycle](#lifecycle). Never returns.

#### AbortFatal

```go
func (c *Container) AbortFatal(err error)
```

No-op if `err` is nil. Otherwise calls `Shutdown(err)` followed by `Terminate(err)`.

#### Terminate

```go
func (c *Container) Terminate(err error)
```

Cancels the application context (if not already cancelled) and exits the process with `os.Exit`: code
`0` if `err` is nil, `-1` otherwise. Only the first call proceeds; concurrent or later calls return
immediately. `Terminate` does **not** run destructors - call `Shutdown` first if needed.

## Notes and Gotchas

- **Destructor errors are logged, not returned.** `Shutdown` runs every destructor, joins their errors
  with `errors.Join` and logs the result; the caller does not receive them.
- **Graceful shutdown from application code.** Calling `CancelCtx()` while `Run` is active runs the
  destructors and exits with code `0`. On a signal, the context is cancelled before the destructors run,
  so goroutines watching `Context` can start winding down while resources are being released.
- **`os.Exit` semantics apply.** `Terminate` and the fatal log in `Shutdown` call `os.Exit`: deferred
  functions do not run, and goroutines watching the application context are not waited for.
- **`RegisterDestructor` during or after `Shutdown` is a no-op**, and `GetDestructorManager` returns `nil`
  after `Shutdown`.
- **Limited re-entrancy.** A destructor may call `RegisterDestructor` (it is ignored), but must not call
  `Shutdown` or `AbortFatal`: `Shutdown` is serialized by a mutex held while destructors run, so these
  calls deadlock.
- **Destructors are global.** They are shared across all containers in the process, and run in reverse
  order, so destructors registered in `init()` functions (e.g. the `log` package's file closer) run last.
- **`SIGHUP` terminates the application**; it is not treated as a reload signal.
- **Startup functions must not block**, otherwise `Run` never reaches its signal loop. A signal received
  during startup is buffered and handled once all startup functions return.
- **Exit code for errors.** `AbortFatal` and `Shutdown(err)` always log the error and exit with code 1
  via zerolog's fatal handler; the `-1` code in `Terminate` is only reached when the fatal log does not
  exit (a custom `zerolog.FatalExitFunc` that returns).
