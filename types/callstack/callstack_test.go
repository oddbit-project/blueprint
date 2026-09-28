package callstack

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const (
	testString        = "ABCDEF"
	reverseTestString = "FEDCBA"
)

func TestCallStack(t *testing.T) {

	cs := NewCallStack()
	if cs == nil {
		t.Fatal("NewCallStack(): failed")
	}

	returnValue := ""
	for _, c := range testString {
		// callable is wrapped in a generator function to copy "c" value
		cs.Add(func(v rune) CallableFn {
			return func() error {
				returnValue = returnValue + string(v)
				return nil
			}
		}(c))
	}

	// test reverse callback
	err := cs.Run(false)
	if err != nil {
		t.Error("Run(): failed")
	}
	if returnValue != reverseTestString {
		t.Error("Run(): result does not match reverse string")
	}

	// test forward callback
	returnValue = ""
	err = cs.RunLinear(false)
	if err != nil {
		t.Error("RunLinear(): failed")
	}
	if returnValue != testString {
		t.Error("RunLinear(): result does not match test string")
	}

	// test failure
	returnValue = ""
	myError := errors.New("expected error")
	cs.Add(func() error {
		return myError
	})
	if err = cs.Run(true); err == nil {
		t.Error("Run(): failed to return expected error")
	} else if !errors.Is(err, myError) {
		t.Error("Run(): unexpected error returned")
	}
	if len(returnValue) > 0 {
		t.Error("Run(): failed callback not executed in order")
	}

	if err = cs.RunLinear(true); err == nil {
		t.Error("RunLinear(): failed to return expected error")
	} else if !errors.Is(err, myError) {
		t.Error("RunLinear(): unexpected error returned")
	}
	if returnValue != testString {
		t.Error("RunLinear(): failed callback not executed in order")
	}
}

func TestCallStack_IsCalling(t *testing.T) {
	c := NewCallStack()
	assert.False(t, c.IsCalling())
}

func TestCallStack_RunNoAbortReturnsJoinedErrors(t *testing.T) {
	err1 := errors.New("first error")
	err2 := errors.New("second error")

	for name, run := range map[string]func(c *CallStack) error{
		"Run":       func(c *CallStack) error { return c.Run(false) },
		"RunLinear": func(c *CallStack) error { return c.RunLinear(false) },
	} {
		t.Run(name, func(t *testing.T) {
			c := NewCallStack()
			calls := 0
			c.Add(func() error { calls++; return err1 })
			c.Add(func() error { calls++; return nil })
			c.Add(func() error { calls++; return err2 })

			err := run(c)
			assert.Equal(t, 3, calls)
			assert.ErrorIs(t, err, err1)
			assert.ErrorIs(t, err, err2)
		})
	}
}

func TestCallStack_RunNoAbortNoErrors(t *testing.T) {
	c := NewCallStack()
	c.Add(func() error { return nil })
	assert.NoError(t, c.Run(false))
	assert.NoError(t, c.RunLinear(false))
}

func TestCallStack_CallbackCanUseCallStack(t *testing.T) {
	for name, run := range map[string]func(c *CallStack) error{
		"Run":       func(c *CallStack) error { return c.Run(false) },
		"RunLinear": func(c *CallStack) error { return c.RunLinear(false) },
	} {
		t.Run(name, func(t *testing.T) {
			c := NewCallStack()
			c.Add(func() error {
				c.Add(func() error { return nil })
				assert.True(t, c.IsCalling())
				return nil
			})

			done := make(chan error, 1)
			go func() { done <- run(c) }()

			select {
			case err := <-done:
				assert.NoError(t, err)
			case <-time.After(2 * time.Second):
				t.Fatal("callback calling Add deadlocked")
			}
			assert.False(t, c.IsCalling())
		})
	}
}
