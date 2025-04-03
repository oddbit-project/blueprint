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