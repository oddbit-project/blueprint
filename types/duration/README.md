# Duration

A JSON-friendly duration type that stores values as whole seconds (`int64`). Serializes as a plain integer, matching the OAuth/OIDC `expires_in` convention.

## Overview

`duration.Seconds` wraps an `int64` representing seconds. Because the underlying type is `int64`, the standard `encoding/json` codec handles marshal/unmarshal without a custom marshaler — config files contain readable integers like `900` instead of Go-style strings like `"15m0s"`.

## Quick Start

```go
import "github.com/oddbit-project/blueprint/types/duration"

type Config struct {
    AccessTokenTTL  duration.Seconds `json:"accessTokenTtl"`
    RefreshTokenTTL duration.Seconds `json:"refreshTokenTtl"`
}

cfg := Config{
    AccessTokenTTL:  duration.Minutes(15),   // JSON: 900
    RefreshTokenTTL: duration.Days(7),       // JSON: 604800
}
```

## Constructors

| Function | Description | Example |
|---|---|---|
| `Minutes(n)` | Duration of n minutes | `Minutes(15)` → 900 |
| `Hours(n)` | Duration of n hours | `Hours(2)` → 7200 |
| `Days(n)` | Duration of n days (24h, no DST) | `Days(7)` → 604800 |
| `FromStd(d)` | Convert `time.Duration`, truncating sub-seconds | `FromStd(time.Hour)` → 3600 |

## Methods

| Method | Description |
|---|---|
| `Std()` | Returns `time.Duration` for standard library interop |
| `IsPositive()` | Reports whether the duration is strictly greater than zero |
| `String()` | Formats using `time.Duration.String()` (e.g. `"15m0s"`) |

## Standard Library Interop

```go
// Convert to time.Duration for timers, deadlines, etc.
timer := time.NewTimer(cfg.AccessTokenTTL.Std())
deadline := time.Now().Add(cfg.AccessTokenTTL.Std())

// Convert from time.Duration (sub-second precision is lost)
d := duration.FromStd(1500 * time.Millisecond) // 1s, not 1.5s
```

## JSON Encoding

Duration values serialize as plain integers (seconds):

```json
{
    "accessTokenTtl": 900,
    "refreshTokenTtl": 604800
}
```

No custom marshaler is needed — the defined `int64` type handles this natively.
