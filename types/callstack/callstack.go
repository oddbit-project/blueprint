package callstack

import (
	"sync"
	"sync/atomic"
)

// CallStack callable
type CallableFn func() error

type CallStack struct {
	calling  int32
	handlers []CallableFn
	sync.Mutex
}

// Create new CallStack
func NewCallStack() *CallStack {
	return &CallStack{
		calling:  0,
		handlers: make([]CallableFn, 0),
	}
}

// Add a callback
func (c *CallStack) Add(fn CallableFn) {
	c.Lock()
	defer c.Unlock()
	c.handlers = append(c.handlers, fn)
}

// Run all callbacks in reverse order
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

// run all callbacks in sequential order
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

// returns true if in call loop
func (c *CallStack) IsCalling() bool {
	return atomic.LoadInt32(&c.calling) == 1
}
