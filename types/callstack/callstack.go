package callstack

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// CallableFn CallStack callable function
type CallableFn func() error

type CallStack struct {
	calling  int32
	handlers []CallableFn
	sync.Mutex
}

// NewCallStack creates a new CallStack
func NewCallStack() *CallStack {
	return &CallStack{
		calling:  0,
		handlers: make([]CallableFn, 0),
	}
}

// Add Adds a callback
func (c *CallStack) Add(fn CallableFn) {
	c.Lock()
	defer c.Unlock()
	c.handlers = append(c.handlers, fn)
}

// Run executes the callback functions in the CallStack in reverse order.
// If abortOnError is true and any of the callback functions return an error, the execution stops and returns that error.
// If abortOnError is false, all callback functions are executed, regardless of errors.
// The CallStack is locked while executing the callbacks to ensure thread safety.
// The calling flag is set to 1 during the execution and reset to 0 after execution.
// If the CallStack is empty, Run returns nil.
// Returns an error if abortOnError is true and any callback function returns an error; otherwise, returns nil.
func (c *CallStack) Run(abortOnError bool) error {
	c.Lock()
	atomic.StoreInt32(&c.calling, 1)
	defer func() { atomic.StoreInt32(&c.calling, 0) }()
	defer c.Unlock()
	if len(c.handlers) == 0 {
		return nil
	}
	for i := len(c.handlers) - 1; i >= 0; i-- {
		if err := c.handlers[i](); err != nil && abortOnError {
			return err
		}
	}
	return nil
}

// RunLinear executes each callback function in the call stack linearly.
func (c *CallStack) RunLinear(abortOnError bool) error {
	c.Lock()
	atomic.StoreInt32(&c.calling, 1)
	defer func() { atomic.StoreInt32(&c.calling, 0) }()
	defer c.Unlock()
	for _, fn := range c.handlers {
		if err := fn(); err != nil && abortOnError {
			return err
		}
	}
	return nil
}

// IsCalling returns true if in call loop
func (c *CallStack) IsCalling() bool {
	return atomic.LoadInt32(&c.calling) == 1
}

// Get returns a slice of strings representing the call stack,
// skipping the first 'skip' frames
func Get(skip int) []string {
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
