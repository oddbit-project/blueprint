package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError(t *testing.T) {
	// Test Error creation and string representation
	errMsg := "test error message"
	err := Error(errMsg)
	assert.Equal(t, errMsg, err.Error())
}

func TestNotNil(t *testing.T) {
	// Test with non-nil value (should not panic)
	assert.NotPanics(t, func() {
		NotNil("not nil", Error("this error won't be used"))
	})

	// Test with nil value (should panic with correct error)
	expectedError := Error("expected panic error")
	assert.PanicsWithValue(t, expectedError, func() {
		NotNil(nil, expectedError)
	})

	// Test with nil interface (should panic)
	var nilInterface interface{}
	assert.PanicsWithValue(t, expectedError, func() {
		NotNil(nilInterface, expectedError)
	})
}

func TestNotNil_TypedNil(t *testing.T) {
	expectedError := Error("typed nil")
	var nilPtr *int
	var nilMap map[string]int
	var nilSlice []int
	var nilFunc func()
	var nilChan chan int
	var nilErr error
	var nilPtrErr *Error

	tests := []struct {
		name        string
		value       any
		shouldPanic bool
	}{
		{"nil pointer", nilPtr, true},
		{"nil map", nilMap, true},
		{"nil slice", nilSlice, true},
		{"nil func", nilFunc, true},
		{"nil chan", nilChan, true},
		{"nil interface pointer", &nilErr, false},
		{"typed nil in error interface", error(nilPtrErr), true},
		{"non-nil pointer", new(int), false},
		{"empty map", map[string]int{}, false},
		{"empty slice", []int{}, false},
		{"zero int", 0, false},
		{"empty string", "", false},
		{"zero struct", struct{}{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPanic {
				assert.PanicsWithValue(t, expectedError, func() { NotNil(tt.value, expectedError) })
			} else {
				assert.NotPanics(t, func() { NotNil(tt.value, expectedError) })
			}
		})
	}
}
