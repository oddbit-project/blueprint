package main

import (
	"fmt"
	"github.com/oddbit-project/blueprint/console"
)

func main() {
	// use shortcuts
	fmt.Println(console.Error("something went %s", "wrong"))
	fmt.Println(console.Info("something went %s", "info"))
	fmt.Println(console.Warn("something went %s", "warn"))
	fmt.Println(console.Success("something went %s", "success"))

	// explicit color block
	fmt.Println(console.Color(console.FgGreen, console.BgBlue, console.Bold, console.Underline), "Name", console.Reset())

	// implicit color block
	fmt.Println(console.Colorize(console.BgBlue, console.FgWhite)("the quick brown %s jumps over the lazy dog", "fox"))
}
