package debug

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	identifiableFunction := func() []string {
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

func TestGetStackTrace_Filtering(t *testing.T) {
	tests := []struct {
		name     string
		stack    []string
		contains string
		excluded []string
	}{
		{
			name:     "direct call",
			stack:    GetStackTrace(0),
			contains: "TestGetStackTrace_Filtering",
			excluded: []string{" runtime.", "debug.GetStackTrace"},
		},
		{
			name:     "user file under a runtime/ directory",
			stack:    stackFromRuntimeDir(),
			contains: "/project/runtime/helper.go",
			excluded: []string{" runtime.", "debug.GetStackTrace"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotEmpty(t, tt.stack)
			assert.Contains(t, tt.stack[0], tt.contains, "first frame must be the caller, got: %v", tt.stack)
			for _, frame := range tt.stack {
				for _, ex := range tt.excluded {
					assert.NotContains(t, frame, ex)
				}
			}
		})
	}
}

// stackFromRuntimeDir is reported as living in a user directory named "runtime"
//
//line /tmp/project/runtime/helper.go:1
func stackFromRuntimeDir() []string {
	return GetStackTrace(0)
}
