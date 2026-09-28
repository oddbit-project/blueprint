# Types

The `types` directory groups small, general-purpose data types used across Blueprint. Each subdirectory is an
independent package:

| Package | Import path | Purpose |
|---------|-------------|---------|
| `threadsafe` | `github.com/oddbit-project/blueprint/types/threadsafe` | Generic mutex-protected `Map` and `Slice` |
| `collections` | `github.com/oddbit-project/blueprint/types/collections` | Generic mutex-protected `Map` with error-returning lookups and convenience aliases |
| `duration` | `github.com/oddbit-project/blueprint/types/duration` | Duration in whole seconds that serializes to JSON as an integer |
| `callstack` | `github.com/oddbit-project/blueprint/types/callstack` | Ordered list of `func() error` callbacks, run in reverse (LIFO) or insertion order |

## threadsafe

Generic thread-safe wrappers around a Go map and slice. Each value holds its data behind a `sync.RWMutex`; read
operations take a read lock and mutations take a write lock. The package is imported from
[hayageek/threadsafe](https://github.com/hayageek/threadsafe) (MIT license, see `types/threadsafe/LICENSE`).

### Usage

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/types/threadsafe"
)

func main() {
	// Map
	m := threadsafe.NewMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	if v, ok := m.Get("a"); ok {
		fmt.Println("a =", v)
	}
	fmt.Println(m.Contains("c"), m.Length()) // false 2

	snapshot := m.Copy() // independent copy
	m.Clear()
	fmt.Println(m.Length(), snapshot.Length()) // 0 2

	// Slice
	s := threadsafe.NewSlice[int]()
	s.Append(10)
	s.Append(30)
	s.Insert(1, 20) // [10 20 30]

	if v, ok := s.Get(1); ok {
		fmt.Println("index 1 =", v)
	}
	s.Set(0, 5)                 // [5 20 30]
	s.Remove(2)                 // [5 20]
	fmt.Println(s.Contains(20)) // true
	fmt.Println(s.Values())     // [5 20] (a copy)
}
```

### API Reference

#### Map

```go
type Map[K comparable, V any] struct {
    // Contains unexported fields
}

func NewMap[K comparable, V any]() *Map[K, V]
```

| Method | Description |
|--------|-------------|
| `Get(key K) (V, bool)` | Returns the value and whether the key exists |
| `Set(key K, value V)` | Stores a value |
| `Delete(key K)` | Removes a key (no-op if absent) |
| `Length() int` | Number of entries |
| `Keys() []K` | New slice with all keys (unordered) |
| `Values() []V` | New slice with all values (unordered) |
| `Contains(key K) bool` | Whether the key exists |
| `Clear()` | Removes all entries |
| `Copy() *Map[K, V]` | New `Map` with a shallow copy of the entries |

#### Slice

```go
type Slice[T any] struct {
    // Contains unexported fields
}

func NewSlice[T any]() *Slice[T]
```

| Method | Description |
|--------|-------------|
| `Append(value T)` | Appends a value |
| `Get(index int) (T, bool)` | Value at `index`; `false` (and zero value) if out of range |
| `Set(index int, value T) bool` | Replaces the value at `index`; `false` if out of range |
| `Length() int` | Number of elements |
| `Values() []T` | Copy of the underlying data |
| `Remove(index int) bool` | Removes the element at `index`, shifting the rest; `false` if out of range |
| `Contains(value T) bool` | Linear search using `reflect.DeepEqual` |
| `Clear()` | Removes all elements |
| `Insert(index int, value T) bool` | Inserts at `index` (`0 <= index <= Length()`); `false` otherwise |
| `Copy() *Slice[T]` | New `Slice` with a shallow copy of the data |

### Notes

- The zero value of `Map` is usable: the internal map is created on the first `Set`. `NewMap` is still the
  idiomatic constructor. A zero-value `Slice` is also usable.
- Each method is individually atomic; sequences such as "check `Contains`, then `Set`" are not.
- `Copy()`, `Keys()`, `Values()` copy only the container: pointer, slice or map values still share their
  underlying data.
- `Slice.Contains` uses reflection and is O(n).

## collections

A generic thread-safe map, similar in spirit to `threadsafe.Map`, but with an error-returning `Get`, a
zero-value-returning `MustGet`, and a few ready-made aliases.

### Usage

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/types/collections"
)

func main() {
	m := collections.NewMap[string, int]()
	m.Add("one", 1)
	m.Add("two", 2)

	v, err := m.Get("two")
	if err != nil {
		panic(err)
	}
	fmt.Println(v) // 2

	if _, err := m.Get("missing"); err != nil {
		fmt.Println(err) // cannot find item with key missing in map
	}
	fmt.Println(m.MustGet("missing")) // 0 (zero value, no panic)

	fmt.Println(m.Contains("one"), m.Len(), m.GetKeys())
	m.Delete("one")
	m.Purge()

	// convenience aliases
	tags := collections.NewStringListMap()
	tags.Add("fruits", []string{"apple", "banana"})

	names := collections.NewStringMap()
	names.Add("name", "John")

	mixed := collections.NewIntMap()
	mixed.Add(1, "first")
	mixed.Add(2, 42)
}
```

### API Reference

#### Map

```go
type Map[K comparable, V any] struct {
    // Contains unexported fields
    sync.RWMutex
}

func NewMap[K comparable, V any]() *Map[K, V]
```

| Method | Description |
|--------|-------------|
| `Contains(key K) bool` | Whether the key exists |
| `Add(key K, value V)` | Stores a value (overwrites an existing key) |
| `Get(key K) (V, error)` | Returns the value, or the zero value and an error `cannot find item with key <key> in map` |
| `MustGet(key K) V` | Returns the value, or the zero value if the key does not exist |
| `GetKeys() []K` | New slice with all keys (unordered) |
| `Delete(key K)` | Removes a key |
| `Purge()` | Removes all entries |
| `Len() int` | Number of entries |

Because `sync.RWMutex` is embedded, `Lock`, `Unlock`, `RLock`, `RUnlock`, `TryLock`, `TryRLock` and `RLocker`
are also part of the exported method set.

#### Aliases and Constructors

```go
type StringListMap = Map[string, []string]
type StringMap = Map[string, string]
type IntMap = Map[int, any]

func NewStringListMap() *StringListMap
func NewStringMap() *StringMap
func NewIntMap() *IntMap
```

### Notes

- Despite its name, `MustGet` never panics; it returns the zero value for missing keys.
- The error returned by `Get` is created with `fmt.Errorf` on each call; there is no sentinel error to compare
  against with `errors.Is`.
- As with `threadsafe.Map`, the zero value is usable: the internal map is created on the first `Add`. `NewMap`
  (or one of the typed constructors) is still the idiomatic way to create one.
- Do not call map methods while holding the embedded lock yourself: the methods acquire the same (non-reentrant)
  mutex and will deadlock.

## duration

`duration.Seconds` is an `int64` number of whole seconds. Because it is a plain defined integer type, the standard
`encoding/json` codec reads and writes it as a JSON number (e.g. `900` for fifteen minutes) with no custom
marshaler. It is used, for example, by the SMTP provider (`provider/smtp`) for its `timeout` setting.

### Usage

```go
package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/oddbit-project/blueprint/types/duration"
)

type Config struct {
	AccessTokenTTL  duration.Seconds `json:"accessTokenTtl"`
	RefreshTokenTTL duration.Seconds `json:"refreshTokenTtl"`
	Timeout         duration.Seconds `json:"timeout"`
}

func main() {
	cfg := Config{
		AccessTokenTTL:  duration.Minutes(15),         // 900
		RefreshTokenTTL: duration.Days(7),             // 604800
		Timeout:         duration.FromStd(time.Minute), // 60
	}

	b, _ := json.Marshal(cfg)
	fmt.Println(string(b)) // {"accessTokenTtl":900,"refreshTokenTtl":604800,"timeout":60}

	fmt.Println(cfg.AccessTokenTTL)              // 15m0s
	fmt.Println(cfg.AccessTokenTTL.IsPositive()) // true

	// interop with the standard library
	timer := time.NewTimer(cfg.Timeout.Std())
	defer timer.Stop()
	deadline := time.Now().Add(cfg.AccessTokenTTL.Std())
	fmt.Println(deadline.After(time.Now()))

	// literal seconds are a plain conversion
	fmt.Println(duration.Seconds(30)) // 30s
}
```

### API Reference

```go
type Seconds int64
```

A duration measured in whole seconds.

| Function | Description |
|----------|-------------|
| `Minutes(n int64) Seconds` | `n * 60` seconds |
| `Hours(n int64) Seconds` | `n * 3600` seconds |
| `Days(n int64) Seconds` | `n * 86400` seconds (fixed 24-hour days) |
| `FromStd(d time.Duration) Seconds` | Converts a `time.Duration`, truncating toward zero |

| Method | Description |
|--------|-------------|
| `Std() time.Duration` | Converts to `time.Duration` |
| `IsPositive() bool` | `true` if strictly greater than zero |
| `String() string` | Formats via `time.Duration.String()`, e.g. `15m0s`, `168h0m0s` |

There is no `Seconds(n)` constructor function; `duration.Seconds(n)` is a type conversion.

### Notes

- Sub-second precision is discarded: `FromStd(1500 * time.Millisecond)` is `1`.
- JSON input must be an integer number of seconds; strings such as `"15m"` are rejected by `encoding/json`.
- Negative values are allowed by the type; use `IsPositive()` to validate TTL-style settings.
- The constructors do not check for `int64` overflow.

## callstack

`CallStack` stores a list of `CallableFn` callbacks and runs them either in reverse order of registration
(`Run`, like `defer`) or in registration order (`RunLinear`). Blueprint uses it for application shutdown: the
root package's `RegisterDestructor()` adds callbacks to a global `CallStack`, and `Shutdown()` runs them with
`Run(false)`.

### Usage

```go
package main

import (
	"errors"
	"fmt"

	"github.com/oddbit-project/blueprint/types/callstack"
)

func main() {
	cs := callstack.NewCallStack()

	cs.Add(func() error {
		fmt.Println("close database")
		return nil
	})
	cs.Add(func() error {
		fmt.Println("stop http server")
		return errors.New("server already stopped")
	})
	cs.Add(func() error {
		fmt.Println("flush logs")
		return nil
	})

	// reverse order: flush logs, stop http server, close database
	// with abortOnError=false every callback runs and the errors are joined
	if err := cs.Run(false); err != nil {
		fmt.Println("errors:", err) // errors: server already stopped
	}

	// insertion order; stops at the first error and returns it
	if err := cs.RunLinear(true); err != nil {
		fmt.Println("aborted:", err)
	}

	fmt.Println(cs.IsCalling()) // false
}
```

### API Reference

```go
type CallableFn func() error

type CallStack struct {
    // Contains unexported fields
    sync.Mutex
}

func NewCallStack() *CallStack
```

| Method | Description |
|--------|-------------|
| `Add(fn CallableFn)` | Appends a callback |
| `Run(abortOnError bool) error` | Runs callbacks from last to first |
| `RunLinear(abortOnError bool) error` | Runs callbacks from first to last |
| `IsCalling() bool` | `true` while `Run` or `RunLinear` is executing |

With `abortOnError` set to `true`, execution stops at the first callback returning a non-nil error and that
error is returned. With `false`, all callbacks run and the errors they return are combined with `errors.Join`
(`nil` if every callback succeeded); use `errors.Is`/`errors.As` to inspect individual errors.

Because `sync.Mutex` is embedded, `Lock`, `Unlock` and `TryLock` are also exported.

### Notes

- The stack is not cleared after running; calling `Run`/`RunLinear` again executes the same callbacks again.
- `Run` and `RunLinear` take a snapshot of the registered callbacks under the mutex and execute it without
  holding the lock. A callback may therefore call `Add` on the same `CallStack` safely; the new callback is not
  executed by the current run, only by later ones.
- Concurrent calls to `Run`/`RunLinear` are not serialized: each runs its own snapshot in parallel.
  `IsCalling()` is a single flag, so with overlapping runs it becomes `false` as soon as the first one finishes.
- `IsCalling()` does not take the lock, so it is safe to call from inside a callback or from another goroutine
  to detect an in-progress run.
- There is no way to remove a registered callback.
