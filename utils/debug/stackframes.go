package debug

import (
	"fmt"
	"runtime"
	"strings"
)

// GetStackTrace returns a slice of strings representing the call stack,
// skipping the first 'skip' frames
func GetStackTrace(skip int) []string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip+1, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	stackFrames := make([]string, 0, n)
	for {
		frame, more := frames.Next()
		// Skip runtime and standard library functions
		if !strings.Contains(frame.File, "runtime/") {
			stackFrames = append(stackFrames, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		}
		if !more {
			break
		}
	}

	return stackFrames
}
