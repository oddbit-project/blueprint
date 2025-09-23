//go:build race
// +build race

package batchwriter

// Skip complex tests when running with race detector
func init() {
	// Mark tests that cause problems with race detector
	skipTestsWithRace = true
}
