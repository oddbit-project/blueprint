package console

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	FgBlack = iota + 30
	FgRed
	FgGreen
	FgYellow
	FgBlue
	FgMagenta
	FgCyan
	FgWhite
	FgDefault = 39
)
const (
	BgBlack = iota + 40
	BgRed
	BgGreen
	BgYellow
	BgBlue
	BgMagenta
	BgCyan
	BgWhite
	BgDefault = 39
)

const (
	FgLightBlack = iota + 90
	FgLightRed
	FgLightGreen
	FgLightYellow
	FgLightBlue
	FgLightMagenta
	FgLightCyan
	FgLightWhite
)

const (
	BgLightBlack = iota + 100
	BgLightRed
	BgLightGreen
	BgLightYellow
	BgLightBlue
	BgLightMagenta
	BgLightCyan
	BgLightWhite
)
const (
	Bold      = 1
	Dim       = 2
	Italic    = 3 // not always supported
	Underline = 4
	Blink     = 5
	Reversed  = 7
	Hide      = 8
)

// shortcuts
// Example:
//
//	fmt.Println(console.Error("something went %s", "wrong"))
var (
	Regular = Colorize(FgWhite, BgBlack)
	Info    = Colorize(FgCyan, BgBlack)
	Warn    = Colorize(FgYellow, BgBlack)
	Error   = Colorize(FgRed, BgBlack)
	Success = Colorize(FgGreen, BgBlack)
)

// Color returns the ANSI escaping for the specified foreground and background colors, as well as modifiers
// Example:
//
//	fmt.Println(console.Color(console.FgGreen, console.BgBlue, console.Bold, console.Underline), "Name", console.Reset())
func Color(colors ...int) string {
	var ansi string
	if len(colors) > 0 {
		tmp := make([]string, len(colors))
		for i, attr := range colors {
			tmp[i] = strconv.Itoa(attr)
		}
		ansi += "\033[" + strings.Join(tmp, ";") + "m"
	}
	return ansi
}

// Reset returns the ANSI escaping for color reset
func Reset() string {
	return "\u001B[0m"
}

// Colorize returns a function analogous to fmt.Sprintf() but with the specified coloring format applied
// Example:
//
//	fmt.Println(console.Colorize(console.BgBlue, console.FgWhite)("the quick brown %s jumps over the lazy dog", "fox"))
func Colorize(colors ...int) func(s string, args ...any) string {
	return func(s string, args ...any) string {
		return fmt.Sprintf(
			Color(colors...)+"%s"+Reset(),
			fmt.Sprintf(s, args...))
	}
}
