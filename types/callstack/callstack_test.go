package callstack

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
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

func TestGet(t *testing.T) {
	// Get the callstack with 0 skip frames
	stack := Get(0)
	
	// Since Get itself filters runtime functions and adds frames,
	// we only test that we get a non-empty stack
	assert.Greater(t, len(stack), 0)
	
	// Define a function that we can identify in the stack
	var identifiableFunction func() []string
	identifiableFunction = func() []string {
		return Get(0)
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
