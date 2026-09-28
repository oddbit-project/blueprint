# Rate Limiter Provider

The rate limiter provider offers a generic, in-memory, keyed rate limiter. It keeps one token-bucket limiter
(`golang.org/x/time/rate`) per key, so independent limits can be enforced per user, per client IP, per API key or any
other identifier. Limiters that have not been used for a configurable TTL are removed by a background cleanup
goroutine. It is suitable for rate-limiting operations such as login attempts.

## Features

- **Per-Key Limits**: Each key gets its own `*rate.Limiter`, created on first use
- **Token Bucket**: Sustained rate plus burst, as implemented by `golang.org/x/time/rate`
- **Automatic Expiry**: Limiters idle for longer than the TTL are evicted by a background goroutine
- **Idempotent Lifecycle**: `Start()` and `Shutdown()` are safe to call multiple times
- **Graceful Shutdown**: `ShutdownWithContext()` waits for the cleanup goroutine to exit
- **Concurrency-Safe**: All access to the limiter map is guarded by a mutex

## Installation

`provider/ratelimiter` is not a separate Go module; it is a package of the core Blueprint module:

```bash
go get github.com/oddbit-project/blueprint
```

```go
import "github.com/oddbit-project/blueprint/provider/ratelimiter"
```

## Configuration

### Basic Configuration

```go
package main

import (
	"github.com/oddbit-project/blueprint/provider/ratelimiter"
)

func main() {
	cfg := ratelimiter.NewConfig()
	cfg.RateLimit = 10       // 10 events per second
	cfg.Burst = 20           // up to 20 events at once
	cfg.TTL = 300            // forget a key after 5 minutes of inactivity
	cfg.CleanupInterval = 60 // scan for idle keys every minute

	limiter, err := ratelimiter.NewRateLimiter(cfg)
	if err != nil {
		panic(err)
	}
	limiter.Start()
	defer limiter.Shutdown()
}
```

### JSON Configuration

```json
{
  "rateLimiter": {
    "rateLimit": 10,
    "burst": 20,
    "ttl": 300,
    "cleanupInterval": 60
  }
}
```

`rateLimit` is a `rate.Limit` (a `float64`), so fractional rates such as `0.2` (one event every 5 seconds) are valid.

## Configuration Options

| Field             | JSON              | Type         | Default | Description                                                     |
|-------------------|-------------------|--------------|---------|-----------------------------------------------------------------|
| `RateLimit`       | `rateLimit`       | `rate.Limit` | `60`    | Sustained rate, in events per second                            |
| `Burst`           | `burst`           | `int`        | `4`     | Maximum number of events allowed at once (bucket size)          |
| `TTL`             | `ttl`             | `int`        | `60`    | Seconds a key may stay unused before its limiter is removed     |
| `CleanupInterval` | `cleanupInterval` | `int`        | `60`    | Seconds between cleanup runs                                    |

Defaults are the values set by `NewConfig()`; a zero-value `Config{}` fails validation.

### Validation

`NewRateLimiter` calls `Config.Validate()`, which checks the fields in order and returns the first error:

| Condition              | Error                       | Message                               |
|------------------------|-----------------------------|---------------------------------------|
| `RateLimit <= 0`       | `ErrInvalidRateLimit`       | `rate limit must be positive`         |
| `Burst <= 0`           | `ErrInvalidBurst`           | `burst must be positive`              |
| `TTL <= 0`             | `ErrInvalidTTL`             | `TTL must be positive`                |
| `CleanupInterval <= 0` | `ErrInvalidCleanupInterval` | `cleanup interval must be positive`   |

## Usage Examples

### Limiting Login Attempts

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/provider/ratelimiter"
	"golang.org/x/time/rate"
)

func main() {
	cfg := ratelimiter.NewConfig()
	cfg.RateLimit = rate.Limit(5.0 / 60.0) // 5 attempts per minute
	cfg.Burst = 5
	cfg.TTL = 900 // 15 minutes
	cfg.CleanupInterval = 60

	limiter, err := ratelimiter.NewRateLimiter(cfg)
	if err != nil {
		panic(err)
	}
	limiter.Start()
	defer limiter.Shutdown()

	for i := 1; i <= 7; i++ {
		if limiter.Allow("user@example.com") {
			fmt.Printf("attempt %d: allowed\n", i)
		} else {
			fmt.Printf("attempt %d: rate limited\n", i)
		}
	}
}
```

The first five attempts consume the burst; subsequent ones are rejected until tokens refill at the configured rate.
Each key is limited independently, so another username is not affected.

### Using the Underlying Limiter

`GetLimiter` returns the `*rate.Limiter` for a key (creating it if needed), which exposes the full
`golang.org/x/time/rate` API, e.g. blocking until a token is available or reserving tokens:

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/oddbit-project/blueprint/provider/ratelimiter"
)

func main() {
	limiter, err := ratelimiter.NewRateLimiter(ratelimiter.NewConfig())
	if err != nil {
		panic(err)
	}
	limiter.Start()
	defer limiter.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// block until the key "worker-1" may proceed, or the context expires
	if err := limiter.GetLimiter("worker-1").Wait(ctx); err != nil {
		fmt.Println("gave up:", err)
		return
	}

	// inspect how long the next event would have to wait
	r := limiter.GetLimiter("worker-1").Reserve()
	fmt.Println("next event delay:", r.Delay())
	r.Cancel()
}
```

Every call to `GetLimiter` (and therefore `Allow`) refreshes the key's last-seen time. Using a `*rate.Limiter`
reference that was obtained earlier does not.

### HTTP Middleware

The package does not ship an HTTP middleware; `provider/httpserver` has its own per-IP limiter
(`security.RateLimitMiddleware` / `Server.UseRateLimiting`, see [Security & Headers](httpserver/security.md)). A keyed
limit on specific routes can be built with `Allow`:

```go
package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oddbit-project/blueprint/provider/ratelimiter"
)

func rateLimit(limiter *ratelimiter.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.Allow(c.ClientIP()) {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	}
}

func main() {
	limiter, err := ratelimiter.NewRateLimiter(ratelimiter.NewConfig())
	if err != nil {
		panic(err)
	}
	limiter.Start()
	defer limiter.Shutdown()

	router := gin.New()
	router.POST("/login", rateLimit(limiter), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	_ = router.Run(":8080")
}
```

### Graceful Shutdown

```go
package main

import (
	"context"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/ratelimiter"
)

func main() {
	logger := log.New("app")

	limiter, err := ratelimiter.NewRateLimiter(ratelimiter.NewConfig())
	if err != nil {
		panic(err)
	}
	limiter.Start()

	// ... application runs ...

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := limiter.ShutdownWithContext(ctx); err != nil {
		logger.Error(err, "rate limiter cleanup did not stop in time")
	}
}
```

## API Reference

### Constants

```go
const (
	ErrInvalidRateLimit       = utils.Error("rate limit must be positive")
	ErrInvalidBurst           = utils.Error("burst must be positive")
	ErrInvalidTTL             = utils.Error("TTL must be positive")
	ErrInvalidCleanupInterval = utils.Error("cleanup interval must be positive")
)
```

### `Config`

```go
type Config struct {
	RateLimit       rate.Limit `json:"rateLimit"`
	Burst           int        `json:"burst"`
	TTL             int        `json:"ttl"`             // seconds
	CleanupInterval int        `json:"cleanupInterval"` // seconds
}

func NewConfig() *Config
func (c *Config) Validate() error
```

### `RateLimiter`

```go
func NewRateLimiter(cfg *Config) (*RateLimiter, error)
```

Validates `cfg` and returns a new limiter; a `nil` `cfg` uses the defaults from `NewConfig()`. The cleanup goroutine
is not started; call `Start()`.

| Method                                                   | Description                                                                                          |
|----------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| `Start()`                                                | Starts the background cleanup goroutine; subsequent calls are no-ops                                 |
| `GetLimiter(key string) *rate.Limiter`                   | Returns the limiter for `key`, creating one with the configured rate and burst; updates last-seen    |
| `Allow(key string) bool`                                 | Shorthand for `GetLimiter(key).Allow()`: consumes a token if one is available                        |
| `Shutdown()`                                             | Signals the cleanup goroutine to stop and returns immediately; safe to call multiple times           |
| `ShutdownWithContext(ctx context.Context) error`         | Signals the cleanup goroutine to stop and waits for it to exit, returning `ctx.Err()` on timeout; returns `nil` immediately if `Start()` was never called |

Every `CleanupInterval` seconds the cleanup goroutine removes keys whose last `GetLimiter`/`Allow` call is older than
`TTL`.

## Notes

- **Not used elsewhere in Blueprint**: no other package in the repository imports `provider/ratelimiter`. The HTTP
  server's rate limiting (`provider/httpserver/security.ClientRateLimiter`) is a separate implementation.
- **In-memory only**: limits are per process and are lost on restart; multiple instances do not share state.
- **Call `Start()`**: without it, idle limiters are never evicted and the map grows with every distinct key.
- **`ShutdownWithContext` without `Start`**: returns `nil` immediately, and any later `Start()` is a no-op (the cleanup
  goroutine is never started).
- **Restarting**: a stopped `RateLimiter` cannot be restarted. `Start()` is a no-op once it has been called or after
  `ShutdownWithContext`, and if it is first called after `Shutdown()` the cleanup goroutine exits immediately. Create a
  new instance instead.
- **TTL vs refill time**: an evicted key gets a fresh limiter with a full burst on its next use. If `TTL` is shorter
  than the time the bucket needs to refill (`Burst / RateLimit` seconds), a client that pauses for `TTL` seconds
  regains its full burst earlier than the rate alone would allow. For slow rates such as login throttling, set `TTL`
  to at least `Burst / RateLimit`.
- **Nil config**: `NewRateLimiter(nil)` is equivalent to `NewRateLimiter(NewConfig())`.

## See Also

- [Security & Headers](httpserver/security.md) - Per-IP rate limiting middleware for the HTTP server
- [`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate) - Underlying token-bucket implementation
