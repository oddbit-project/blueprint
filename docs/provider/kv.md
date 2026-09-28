# KV Provider

The KV provider defines Blueprint's minimal key-value storage interface, `kv.KV`, and ships an in-memory
implementation. It is the storage abstraction used by the HTTP server session store and by the HMAC provider's generic
nonce store, and it is implemented by the [Redis](redis.md) client, so any of these backends can be swapped without
changing the consuming code.

## Features

- **Minimal Interface**: Five methods (`Set`, `SetTTL`, `Get`, `Delete`, `Prune`) operating on `string` keys and
  `[]byte` values
- **In-Memory Backend**: `NewMemoryKV()` returns a map-based store guarded by a `sync.RWMutex`
- **Per-Key TTL**: `SetTTL` stores a value that expires after a given duration; a TTL `<= 0` means no expiry
- **Lazy and Explicit Expiry**: Expired entries are dropped when read, and `Prune()` removes them in bulk
- **Atomic Set-If-Not-Exists**: The optional `kv.AtomicSetter` interface (`SetNX`) is implemented by the in-memory
  backend and by `*redis.Client`
- **Pluggable**: Any type implementing the five methods can be used as a backend (e.g. `*redis.Client`)

## Installation

`provider/kv` is not a separate Go module; it is a package of the core Blueprint module:

```bash
go get github.com/oddbit-project/blueprint
```

```go
import "github.com/oddbit-project/blueprint/provider/kv"
```

## Configuration

The package has no configuration struct. `NewMemoryKV()` takes no arguments, and TTLs are passed per call to
`SetTTL`.

## Usage Examples

### Basic Operations

```go
package main

import (
	"fmt"
	"time"

	"github.com/oddbit-project/blueprint/provider/kv"
)

func main() {
	store := kv.NewMemoryKV()

	// Store a value that expires after 10 minutes
	if err := store.SetTTL("user:42", []byte("alice"), 10*time.Minute); err != nil {
		panic(err)
	}

	// Read it back
	value, err := store.Get("user:42")
	if err != nil {
		panic(err)
	}
	if value == nil {
		fmt.Println("not found")
	} else {
		fmt.Printf("user:42 = %s\n", value)
	}

	// Remove it
	if err := store.Delete("user:42"); err != nil {
		panic(err)
	}
}
```

`Get` returns `nil, nil` when a key does not exist or has expired; a missing key is not reported as an error.
Callers must check the returned slice for `nil`.

### Periodic Pruning

Expired entries of the in-memory backend are only removed when they are read with `Get` or when `Prune()` is called.
Keys that are written and never read again stay in memory until pruned, so long-running processes should call
`Prune()` periodically:

```go
package main

import (
	"context"
	"time"

	"github.com/oddbit-project/blueprint/provider/kv"
)

func startPruner(ctx context.Context, store kv.KV, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = store.Prune()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store := kv.NewMemoryKV()
	startPruner(ctx, store, time.Minute)

	_ = store.SetTTL("token:abc", []byte("1"), 30*time.Second)
}
```

When the store is used as an HTTP session backend this is not needed: the session store already calls `Prune()` on
its own schedule (see below).

### Using Redis as a KV Backend

`*redis.Client` from the [Redis provider](redis.md) (a separate module, `github.com/oddbit-project/blueprint/provider/redis`)
implements all five methods and can be used wherever a `kv.KV` is expected:

```go
package main

import (
	"github.com/oddbit-project/blueprint/provider/kv"
	"github.com/oddbit-project/blueprint/provider/redis"
)

// compile-time checks
var _ kv.KV = (*redis.Client)(nil)
var _ kv.AtomicSetter = (*redis.Client)(nil)

func main() {
	cfg := redis.NewConfig()
	cfg.Address = "localhost:6379"

	client, err := redis.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	var store kv.KV = client
	_ = store.Set("greeting", []byte("hello")) // uses the client's default TTL
}
```

`redis.Client.Prune()` is a no-op: Redis expires keys itself, so it is safe to hand a Redis client to a component that
calls `Prune()` periodically, such as the HTTP session store.

### Atomic Set-If-Not-Exists

Backends that can store a key only when it is absent implement the optional `kv.AtomicSetter` interface. The in-memory
backend and `*redis.Client` both implement it. Because `NewMemoryKV()` returns a `kv.KV`, use a type assertion to
reach `SetNX`:

```go
package main

import (
	"fmt"
	"time"

	"github.com/oddbit-project/blueprint/provider/kv"
)

func main() {
	store := kv.NewMemoryKV()

	setter, ok := store.(kv.AtomicSetter)
	if !ok {
		panic("backend does not support SetNX")
	}

	added, err := setter.SetNX("lock:job-1", []byte("worker-a"), 30*time.Second)
	if err != nil {
		panic(err)
	}
	fmt.Println(added) // true

	added, _ = setter.SetNX("lock:job-1", []byte("worker-b"), 30*time.Second)
	fmt.Println(added) // false, key already exists
}
```

`SetNX` returns `true` if the key was stored and `false` if a (non-expired) value already exists. On the in-memory
backend an expired entry counts as absent and is replaced.

### Implementing a Custom Backend

Any type with the five methods satisfies `kv.KV`:

```go
package main

import (
	"sync"
	"time"

	"github.com/oddbit-project/blueprint/provider/kv"
)

// noExpiryKV is a trivial backend that ignores TTLs
type noExpiryKV struct {
	mu   sync.RWMutex
	data map[string][]byte
}

var _ kv.KV = (*noExpiryKV)(nil)

func (s *noExpiryKV) Set(k string, v []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[k] = v
	return nil
}

func (s *noExpiryKV) SetTTL(k string, v []byte, _ time.Duration) error {
	return s.Set(k, v)
}

func (s *noExpiryKV) Get(k string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[k], nil // nil, nil when not found
}

func (s *noExpiryKV) Delete(k string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, k)
	return nil
}

func (s *noExpiryKV) Prune() error {
	return nil
}

func main() {
	var store kv.KV = &noExpiryKV{data: make(map[string][]byte)}
	_ = store.Set("k", []byte("v"))
}
```

Consumers in Blueprint rely on the `nil, nil` "not found" convention of `Get`, so custom backends should follow it.

## Integration with Other Blueprint Components

### HTTP Server Sessions

The session store in `provider/httpserver/session` persists sessions in a `kv.KV`:

- `session.NewStore(config *session.Config, backend kv.KV, logger *log.Logger)` uses the given backend, or
  `kv.NewMemoryKV()` when `backend` is `nil`.
- `session.NewManager(...)` creates a store with `kv.NewMemoryKV()` when no store is supplied via
  `session.ManagerWithStore`.
- `(*httpserver.Server).UseSession(config *session.Config, backend kv.KV, logger *log.Logger)` builds the store and
  manager and registers the session middleware.

The store writes each session with `SetTTL`, using the smaller of `ExpirationSeconds` and `IdleTimeoutSeconds` as the
TTL, and treats a `nil` result from `Get` as `ErrSessionNotFound`. When the manager is created it starts the store's
cleanup goroutine, which calls `backend.Prune()` every `CleanupIntervalSeconds`.

```go
package main

import (
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/httpserver"
	"github.com/oddbit-project/blueprint/provider/httpserver/session"
	"github.com/oddbit-project/blueprint/provider/kv"
)

func main() {
	logger := log.New("app")

	server, err := httpserver.NewServer(httpserver.NewServerConfig(), logger)
	if err != nil {
		panic(err)
	}

	backend := kv.NewMemoryKV()
	manager, err := server.UseSession(session.NewConfig(), backend, logger)
	if err != nil {
		panic(err)
	}
	defer manager.Shutdown()
}
```

See [Session Management](httpserver/session.md) for details.

### HMAC Nonce Store

`provider/hmacprovider/store` provides `NewKvStore(kv kv.KV, ttl time.Duration) NonceStore`. When the backend
implements `kv.AtomicSetter` (the in-memory backend and `*redis.Client` do), each nonce is recorded atomically with
`SetNX(nonce, []byte("1"), ttl)`. Otherwise the store falls back to checking with `Get` that the nonce is not present
and then writing it with `SetTTL`, which is not atomic: two concurrent requests with the same nonce may both be
accepted.

```go
package main

import (
	"time"

	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/provider/hmacprovider"
	"github.com/oddbit-project/blueprint/provider/hmacprovider/store"
	"github.com/oddbit-project/blueprint/provider/kv"
)

func main() {
	key, err := secure.GenerateKey()
	if err != nil {
		panic(err)
	}
	secret, err := secure.NewCredential([]byte("my-hmac-secret"), key, false)
	if err != nil {
		panic(err)
	}

	nonceStore := store.NewKvStore(kv.NewMemoryKV(), time.Hour)

	provider := hmacprovider.NewHmacProvider(
		hmacprovider.NewSingleKeyProvider("api-key", secret),
		hmacprovider.WithNonceStore(nonceStore),
	)
	_ = provider
}
```

See [HMAC Provider](hmacprovider.md) for details.

## API Reference

### `KV` interface

```go
type KV interface {
	SetTTL(k string, v []byte, ttl time.Duration) error
	Set(k string, v []byte) error
	Get(k string) ([]byte, error)
	Delete(k string) error
	Prune() error
}
```

| Method   | Description                                                                         |
|----------|-------------------------------------------------------------------------------------|
| `SetTTL` | Stores `v` under `k`, expiring after `ttl` (`ttl <= 0`: no expiry)                  |
| `Set`    | Stores `v` under `k` without a TTL (backend-specific; see notes below)              |
| `Get`    | Returns the value for `k`, or `nil, nil` if it is missing or expired                |
| `Delete` | Removes `k`; deleting a missing key is not an error                                 |
| `Prune`  | Removes expired entries (backend-specific; a no-op on `*redis.Client`)              |

### `AtomicSetter` interface

```go
type AtomicSetter interface {
	SetNX(k string, v []byte, ttl time.Duration) (bool, error)
}
```

Optional interface for backends that support an atomic set-if-not-exists. `SetNX` stores `v` under `k` with `ttl`
only if `k` does not exist, and returns `true` if the value was stored. Implemented by the in-memory backend and by
`*redis.Client` (Redis `SET ... NX`).

### `NewMemoryKV`

```go
func NewMemoryKV() KV
```

Returns an in-memory `KV` backed by a `map[string]*record` protected by a `sync.RWMutex`. The returned value also
implements `AtomicSetter`. Every method of the in-memory implementation always returns a `nil` error.

## Notes

- **`Set` never expires**: `Set` stores the value with a zero TTL, and the in-memory backend treats any TTL `<= 0`
  (from `Set`, or `SetTTL` with a zero or negative duration) as "no expiry" in both `Get` and `Prune`. On
  `*redis.Client`, `Set` instead applies the client's default TTL.
- **Expiry is lazy**: expired in-memory entries are only removed by a `Get` of that key or by `Prune()`. `Prune()`
  scans and deletes under a single write lock, so a key re-written concurrently is never removed by mistake.
- **No copies**: the in-memory backend stores and returns the caller's slice as-is. Mutating a slice after `Set`/`SetTTL`,
  or mutating the slice returned by `Get`, changes the stored value.
- **Empty values**: because "not found" is `nil, nil`, storing a `nil` slice is indistinguishable from a missing key.
  An empty non-nil slice (`[]byte{}`) is returned as-is.
- **Concurrency**: all in-memory methods are safe for concurrent use. When `Get` finds an expired key it takes the write
  lock to delete it, and only deletes it if it has not been replaced in the meantime.
- **No `Close`**: the interface has no lifecycle methods; closing a backend such as `*redis.Client` is the caller's
  responsibility.

## See Also

- [Redis Provider](redis.md) - Redis-backed `kv.KV` implementation
- [Session Management](httpserver/session.md) - Session storage built on `kv.KV`
- [HMAC Provider](hmacprovider.md) - KV-backed nonce store
