package callstack

import (
	"errors"
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

// snapshot returns a copy of the registered handlers
func (c *CallStack) snapshot() []CallableFn {
	c.Lock()
	defer c.Unlock()
	handlers := make([]CallableFn, len(c.handlers))
	copy(handlers, c.handlers)
	return handlers
}

// Run executes the callback functions in the CallStack in reverse order.
// If abortOnError is true and any of the callback functions return an error, the execution stops and returns that error.
// If abortOnError is false, all callback functions are executed, regardless of errors, and the errors returned
// by the callbacks are combined with errors.Join.
// The callbacks registered at the time of the call are executed without holding the CallStack lock, so a callback
// may safely call Add; callbacks added during execution are not executed by the current call.
// The calling flag is set to 1 during the execution and reset to 0 after execution.
// If the CallStack is empty, Run returns nil.
func (c *CallStack) Run(abortOnError bool) error {
	handlers := c.snapshot()
	atomic.StoreInt32(&c.calling, 1)
	defer func() { atomic.StoreInt32(&c.calling, 0) }()
	var errs []error
	for i := len(handlers) - 1; i >= 0; i-- {
		if err := handlers[i](); err != nil {
			if abortOnError {
				return err
			}
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// RunLinear executes each callback function in the call stack linearly.
// Error handling and locking behave as in Run.
func (c *CallStack) RunLinear(abortOnError bool) error {
	handlers := c.snapshot()
	atomic.StoreInt32(&c.calling, 1)
	defer func() { atomic.StoreInt32(&c.calling, 0) }()
	var errs []error
	for _, fn := range handlers {
		if err := fn(); err != nil {
			if abortOnError {
				return err
			}
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// IsCalling returns true if in call loop
func (c *CallStack) IsCalling() bool {
	return atomic.LoadInt32(&c.calling) == 1
}
