//go:build !race
// +build !race

package batchwriter

// Don't skip tests when not using race detector
func init() {
	skipTestsWithRace = false
}