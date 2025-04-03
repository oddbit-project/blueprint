package debug

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	// Get the callstack with 0 skip frames
	stack := GetStackTrace(0)

	// Since Get itself filters runtime functions and adds frames,
	// we only test that we get a non-empty stack
	assert.Greater(t, len(stack), 0)

	// Define a function that we can identify in the stack
	var identifiableFunction func() []string
	identifiableFunction = func() []string {
		return GetStackTrace(0)
	}

	// Call the function to get its stack
	stack = identifiableFunction()

	// Check that the stack includes our function name somewhere
	foundFunction := false
	for _, frame := range stack {
		if frame != "" && (strings.Contains(frame, "identifiableFunction") ||
			strings.Contains(frame, "TestGet")) {
			foundFunction = true
			break
		}
	}

	assert.True(t, foundFunction, "Stack should contain our function")
}
