package callstack

import (
	"errors"
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
