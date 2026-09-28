package debug

import (
	"fmt"
	"path"
	"runtime"
	"strings"
)

// goRuntimeDir is the source directory of the Go runtime package (GOROOT/src/runtime/)
var goRuntimeDir = func() string {
	var pcs [1]uintptr
	runtime.Callers(0, pcs[:])
	frame, _ := runtime.CallersFrames(pcs[:]).Next()
	return path.Dir(frame.File) + "/"
}()

// isGoRuntimeFrame returns true if the frame belongs to the Go runtime
func isGoRuntimeFrame(frame runtime.Frame) bool {
	return strings.HasPrefix(frame.Function, "runtime.") || strings.HasPrefix(frame.File, goRuntimeDir)
}

// GetStackTrace returns a slice of strings representing the call stack,
// skipping the first 'skip' frames (counting GetStackTrace itself); GetStackTrace is never included
func GetStackTrace(skip int) []string {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(max(skip, 1)+1, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])

	stackFrames := make([]string, 0, n)
	for {
		frame, more := frames.Next()
		// Skip Go runtime functions
		if !isGoRuntimeFrame(frame) {
			stackFrames = append(stackFrames, fmt.Sprintf("%s:%d %s", frame.File, frame.Line, frame.Function))
		}
		if !more {
			break
		}
	}

	return stackFrames
}
