# Utilities

Blueprint ships a set of small helper packages used throughout the framework and available to applications:

| Package | Import path | Purpose |
|---------|-------------|---------|
| `utils` | `github.com/oddbit-project/blueprint/utils` | Constant error type, panic helpers, secure random bytes |
| `env` | `github.com/oddbit-project/blueprint/utils/env` | Cached environment variable access |
| `fs` | `github.com/oddbit-project/blueprint/utils/fs` | File/directory existence checks, reading trimmed strings from files |
| `debug` | `github.com/oddbit-project/blueprint/utils/debug` | Call stack capture as strings |
| `str` | `github.com/oddbit-project/blueprint/utils/str` | JSON dumping and character filtering |
| `parallel` | `github.com/oddbit-project/blueprint/utils/parallel` | Run an indexed function concurrently, returning the first error |
| `generator` | `github.com/oddbit-project/blueprint/generator` | Random alphabetic strings |

## utils

### Error Constants

`utils.Error` is a string type implementing `error`. Because it is a string, errors can be declared as
constants, which is the convention used by Blueprint packages (e.g. `db.ErrInvalidIdentifier`):

```go
package main

import (
	"errors"
	"fmt"

	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrNotFound     = utils.Error("record not found")
	ErrInvalidInput = utils.Error("invalid input")
)

func find(id int) error {
	if id < 0 {
		return ErrInvalidInput
	}
	return fmt.Errorf("find %d: %w", id, ErrNotFound)
}

func main() {
	err := find(1)
	fmt.Println(errors.Is(err, ErrNotFound)) // true
	fmt.Println(err == ErrInvalidInput)      // false
}
```

### Panic Helpers and Random Bytes

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/utils"
)

const ErrNilConfig = utils.Error("config is nil")

func main() {
	// panic if an error is returned
	utils.PanicOnError(log.Configure(log.NewDefaultConfig()))

	// panic with ErrNilConfig if the value is nil
	var cfg any = map[string]string{}
	utils.NotNil(cfg, ErrNilConfig)

	// cryptographically secure random bytes (crypto/rand)
	salt, err := utils.GenerateRandomBytes(16)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(salt)) // 16
}
```

### API Reference

```go
type Error string

func (e Error) Error() string
```

String-based error type; `Error()` returns the string itself.

```go
func NotNil(v any, e Error)
```

Panics with `e` if `v` is `nil`, including a typed nil stored in the interface (a nil pointer, map, slice,
function or channel).

```go
func PanicOnError(err error)
```

Panics with `err` if it is not nil.

```go
func GenerateRandomBytes(n uint32) ([]byte, error)
```

Returns `n` bytes read from `crypto/rand`, or the read error. Used by `crypt/hashing` to generate salts.

### Notes

- `NotNil` detects typed nils with reflection: a `(*Config)(nil)` passed as `any` triggers the panic. Empty but
  non-nil maps and slices, and zero values of non-nillable types (`0`, `""`, structs), do not.
- Two `utils.Error` values with the same text compare equal (`==` and `errors.Is`), even if declared in
  different packages.

## env

Reads environment variables through an in-process cache.

On the first call to `GetEnvVar`, the whole environment (`os.Environ()`) is loaded into a package-level map.
Subsequent lookups are served from that map; names not found in it fall back to `os.Getenv` and the result
(including an empty string) is cached. `SetEnvVar` updates both the process environment and the cache.

### Usage

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/utils/env"
)

func main() {
	home := env.GetEnvVar("HOME")
	fmt.Println(home)

	if err := env.SetEnvVar("APP_MODE", "production"); err != nil {
		panic(err)
	}
	fmt.Println(env.GetEnvVar("APP_MODE")) // production

	// read a secret (crypt/secure and provider/tls read *EnvVar credentials this way)
	secret := env.GetEnvVar("APP_SECRET")
	_ = secret
}
```

### API Reference

```go
func GetEnvVar(name string) string
```

Returns the value of the environment variable, or an empty string if it is not set. Safe for concurrent use.

```go
func SetEnvVar(name, value string) error
```

Calls `os.Setenv` and, on success, updates the cache. Returns the `os.Setenv` error otherwise.

### Notes

- Values are cached for the lifetime of the process. Changes made with `os.Setenv`/`os.Unsetenv` (or by any code
  not using `SetEnvVar`) after the first `GetEnvVar` call are **not** seen by `GetEnvVar`. This also applies to
  the environment-variable credentials of `crypt/secure` and `provider/tls`, which are read through this cache:
  change them with `SetEnvVar`, not `os.Setenv`.
- `crypt/secure` and `provider/tls` do not clear the variable after reading it, so fetching the same credential
  again returns the same value.
- An unset variable and a variable set to `""` are indistinguishable; there is no "lookup" variant.
- There is no unset function. `SetEnvVar(name, "")` sets the variable to an empty string; it does not remove it.
- The cache holds a copy of every environment variable, including secrets, in process memory.

## fs

Small filesystem helpers.

### Usage

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/utils/fs"
)

func main() {
	fmt.Println(fs.FileExists("/etc/hostname")) // true if it exists and is not a directory
	fmt.Println(fs.DirExists("/etc"))           // true if it exists and is a directory

	// read a secret file, stripping a UTF-8 BOM and surrounding whitespace and line breaks
	if fs.FileExists("/run/secrets/db_password") {
		password, err := fs.ReadString("/run/secrets/db_password")
		if err != nil {
			panic(err)
		}
		_ = password
	}
}
```

### API Reference

```go
func FileExists(filename string) bool
```

Returns `true` if `os.Stat` succeeds and the path is not a directory. Any stat error (not found, permission
denied, ...) yields `false`.

```go
func DirExists(dirname string) bool
```

Returns `true` if `os.Stat` succeeds and the path is a directory. Any stat error yields `false`.

```go
func ReadString(filename string) (string, error)
```

Reads the whole file, removes a leading UTF-8 byte order mark (BOM), and returns it with leading and trailing
spaces, tabs, `\r` and `\n` removed. Returns the `os.ReadFile` error on failure (including when `filename` is a
directory).

### Notes

- `FileExists` follows symlinks and returns `true` for any non-directory entry (sockets, devices, etc.).
- Files with Windows line endings are handled: `"secret\r\n"` becomes `"secret"`. Only a BOM at the very start
  of the file is removed, and characters inside the content are never altered.
- `ReadString` loads the whole file into memory.

## debug

Captures the current goroutine's call stack as human-readable strings. Used by the Blueprint logger
(`log.Logger.Error` and friends) to attach a `stack` field to error log entries.

### Usage

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/utils/debug"
)

func handler() {
	// skip=1 omits GetStackTrace itself; the first frame is handler()
	for _, frame := range debug.GetStackTrace(1) {
		fmt.Println(frame)
	}
}

func main() {
	handler()
}
```

Output (paths shortened):

```
/app/main.go:11 main.handler
/app/main.go:17 main.main
```

### API Reference

```go
func GetStackTrace(skip int) []string
```

Returns up to 32 frames, each formatted as `"<file>:<line> <function>"`. `skip` is the number of frames to omit,
counting from `GetStackTrace` itself. The `GetStackTrace` frame is never included: `0` and `1` both start at its
caller, `2` starts at the caller's caller, and so on.

### Notes

- At most 32 frames are captured (before filtering); deeper stacks are truncated.
- Only Go runtime frames are dropped (functions in package `runtime`, or files in the Go installation's
  `src/runtime` directory), such as `runtime.main` and `runtime.goexit`. Frames from your own files located under
  a directory named `runtime` are kept, as are other standard library frames.

## str

String helpers.

### Usage

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/utils/str"
)

func main() {
	fmt.Println(str.DumpJSON(map[string]any{"name": "blueprint", "tags": []string{"go"}}))

	allowed := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	fmt.Println(str.Filter("My-Id 42", allowed))      // yd42
	fmt.Println(str.Filter("My-Id 42", allowed, '_')) // _y__d_42
}
```

### API Reference

```go
func DumpJSON(src any) string
```

Returns `src` marshalled with `json.MarshalIndent(src, "", " ")` (no prefix, a single space per indentation
level). Intended for debugging output.

```go
func Filter(s string, set []rune, replacement ...rune) string
```

Returns `s` keeping only the runes present in `set`. If a `replacement` rune is given, every rune not in `set`
is replaced by it instead of being dropped; only the first replacement rune is used.

### Notes

- `DumpJSON` ignores marshalling errors and returns an empty string for values that cannot be encoded
  (channels, functions, cyclic structures).
- `Filter` checks membership with a linear scan of `set` for each rune (O(len(s) * len(set))).

## parallel

Runs a function once per index in `[0, to)`, each call in its own goroutine.

### Usage

```go
package main

import (
	"fmt"
	"sync"

	"github.com/oddbit-project/blueprint/utils/parallel"
)

func main() {
	urls := []string{"a", "b", "c"}
	results := make([]string, len(urls))
	var mu sync.Mutex

	err := parallel.ForInt(len(urls), func(i int) error {
		mu.Lock()
		defer mu.Unlock()
		results[i] = "processed " + urls[i]
		return nil
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(results)
}
```

### API Reference

```go
type ForIntFn func(i int) error

func ForInt(to int, fn ForIntFn) error
```

Starts `to` goroutines, calling `fn(0)` ... `fn(to-1)` concurrently, and waits for all of them to finish.
Returns the first non-nil error received, or `nil` if every call succeeds. With `to <= 0`, `fn` is never called
and `nil` is returned.

### Notes

- There is no concurrency limit: one goroutine is started per index. Use the `threadpool` package for bounded
  concurrency.
- "First error" means first to be received, not lowest index.
- An error does not cancel the other calls: `ForInt` always waits for every `fn` to return, so when it returns
  (with or without an error) no call is still running. There is no way to stop the remaining calls early.
- A panic inside `fn` is not recovered and crashes the program.

## generator

Generates random alphabetic strings. Used by the MQTT provider to create a default client ID.

### Usage

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/generator"
)

func main() {
	id := generator.RandomString(12) // e.g. "kQxbTzLpAeRw"
	fmt.Println(id)
}
```

### API Reference

```go
func RandomString(n int) string
```

Returns a string of `n` characters drawn from `a-z` and `A-Z`.

### Notes

- `RandomString` uses `math/rand` and is **not** cryptographically secure. Do not use it for tokens,
  passwords or other secrets; use `utils.GenerateRandomBytes` or the `crypt` packages instead.
- A negative `n` panics.
