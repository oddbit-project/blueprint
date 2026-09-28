# Console

The console package provides small helpers for producing ANSI-colored terminal output: SGR code constants,
a function to build escape sequences, and printf-style "colorizer" functions.

## Overview

The package is a thin layer over ANSI SGR (Select Graphic Rendition) escape sequences:

- Integer constants for foreground/background colors (normal and "light"/bright variants) and text attributes
- `Color()` builds an escape sequence from any combination of those codes
- `Reset()` returns the reset sequence
- `Colorize()` returns a `fmt.Sprintf`-like function that wraps its output in a color block
- Predefined colorizers: `Regular`, `Info`, `Warn`, `Error`, `Success`

The package does not detect terminal capabilities; escape sequences are always emitted.

## Usage

### Predefined Shortcuts

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/console"
)

func main() {
	fmt.Println(console.Error("something went %s", "wrong"))  // red on black
	fmt.Println(console.Info("processing %d items", 10))      // cyan on black
	fmt.Println(console.Warn("disk usage at %d%%", 91))       // yellow on black
	fmt.Println(console.Success("done"))                      // green on black
	fmt.Println(console.Regular("plain white on black"))      // white on black
}
```

### Explicit Color Blocks

`Color()` returns only the opening escape sequence; you are responsible for emitting `Reset()` afterwards.

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/console"
)

func main() {
	// green text on blue background, bold and underlined
	fmt.Println(console.Color(console.FgGreen, console.BgBlue, console.Bold, console.Underline), "Name", console.Reset())
}
```

### Custom Colorizers

`Colorize()` returns a reusable function with the same calling convention as `fmt.Sprintf`:

```go
package main

import (
	"fmt"

	"github.com/oddbit-project/blueprint/console"
)

func main() {
	highlight := console.Colorize(console.BgBlue, console.FgWhite)
	fmt.Println(highlight("the quick brown %s jumps over the lazy dog", "fox"))

	header := console.Colorize(console.FgLightYellow, console.Bold)
	fmt.Println(header("== %s ==", "Report"))
}
```

This is how the migration manager (`db/migrations`) formats its progress messages.

## API Reference

### Constants

#### Foreground Colors

| Constant | Value |
|----------|-------|
| `FgBlack` | 30 |
| `FgRed` | 31 |
| `FgGreen` | 32 |
| `FgYellow` | 33 |
| `FgBlue` | 34 |
| `FgMagenta` | 35 |
| `FgCyan` | 36 |
| `FgWhite` | 37 |
| `FgDefault` | 39 |

#### Background Colors

| Constant | Value |
|----------|-------|
| `BgBlack` | 40 |
| `BgRed` | 41 |
| `BgGreen` | 42 |
| `BgYellow` | 43 |
| `BgBlue` | 44 |
| `BgMagenta` | 45 |
| `BgCyan` | 46 |
| `BgWhite` | 47 |
| `BgDefault` | 49 |

#### Light (Bright) Foreground Colors

| Constant | Value |
|----------|-------|
| `FgLightBlack` | 90 |
| `FgLightRed` | 91 |
| `FgLightGreen` | 92 |
| `FgLightYellow` | 93 |
| `FgLightBlue` | 94 |
| `FgLightMagenta` | 95 |
| `FgLightCyan` | 96 |
| `FgLightWhite` | 97 |

#### Light (Bright) Background Colors

| Constant | Value |
|----------|-------|
| `BgLightBlack` | 100 |
| `BgLightRed` | 101 |
| `BgLightGreen` | 102 |
| `BgLightYellow` | 103 |
| `BgLightBlue` | 104 |
| `BgLightMagenta` | 105 |
| `BgLightCyan` | 106 |
| `BgLightWhite` | 107 |

#### Text Attributes

| Constant | Value | Notes |
|----------|-------|-------|
| `Bold` | 1 | |
| `Dim` | 2 | |
| `Italic` | 3 | Not supported by all terminals |
| `Underline` | 4 | |
| `Blink` | 5 | |
| `Reversed` | 7 | Swaps foreground and background |
| `Hide` | 8 | |

All constants are untyped integer constants and can be passed directly to `Color()` and `Colorize()`.

### Variables

```go
var (
    Regular = Colorize(FgWhite, BgBlack)
    Info    = Colorize(FgCyan, BgBlack)
    Warn    = Colorize(FgYellow, BgBlack)
    Error   = Colorize(FgRed, BgBlack)
    Success = Colorize(FgGreen, BgBlack)
)
```

Predefined colorizers, each of type `func(s string, args ...any) string`. All of them force a black background.

### Functions

#### Color

```go
func Color(colors ...int) string
```

Returns the ANSI escape sequence `ESC[<c1>;<c2>;...m` for the given codes, in the order given. Returns an empty
string when called with no arguments.

#### Reset

```go
func Reset() string
```

Returns the ANSI reset sequence (`ESC[0m`).

#### Colorize

```go
func Colorize(colors ...int) func(s string, args ...any) string
```

Returns a function that formats `s` with `args` using `fmt.Sprintf`, prefixes the result with `Color(colors...)`
and appends `Reset()`.

## Notes

- `Colorize` functions always pass their first argument through `fmt.Sprintf` as a format string. When printing
  arbitrary text (e.g. containing `%`), use `console.Info("%s", text)` rather than `console.Info(text)`.
- Output always contains escape codes, even when stdout is not a terminal (pipes, log files). Guard calls
  yourself if you need plain output in those cases.
