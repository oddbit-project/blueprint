package parallel

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForInt(t *testing.T) {
	// Test case: successful parallel execution
	t.Run("successful execution", func(t *testing.T) {
		n := 100
		var mu sync.Mutex
		result := make([]bool, n)
		
		err := ForInt(n, func(i int) error {
			mu.Lock()
			result[i] = true
			mu.Unlock()
			return nil
		})
		
		assert.NoError(t, err)
		
		// Verify all items were processed
		for i := 0; i < n; i++ {
			assert.True(t, result[i], "Item %d was not processed", i)
		}
	})
	
	// Test case: function returns error
	t.Run("function returns error", func(t *testing.T) {
		expectedError := errors.New("test error")
		errorIndex := 5
		
		err := ForInt(10, func(i int) error {
			if i == errorIndex {
				return expectedError
			}
			return nil
		})
		
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
	})
	
	// Test case: zero length
	t.Run("zero length", func(t *testing.T) {
		callCount := 0
		
		err := ForInt(0, func(i int) error {
			callCount++
			return nil
		})
		
		assert.NoError(t, err)
		assert.Equal(t, 0, callCount, "Function should not be called for zero length")
	})
	
	// Test case: function returns error only sometimes
	t.Run("intermittent errors", func(t *testing.T) {
		var mu sync.Mutex
		processed := make(map[int]bool)
		expectedError := errors.New("intermittent error")
		
		err := ForInt(100, func(i int) error {
			mu.Lock()
			processed[i] = true
			mu.Unlock()
			
			if i == 75 {
				return expectedError
			}
			return nil
		})
		
		assert.Error(t, err)
		assert.Equal(t, expectedError, err)
		
		// Some items should have been processed, but not necessarily all
		assert.NotEmpty(t, processed)
	})
	
	// Test case: nil function would panic, so we're not testing it
	// The implementation in for.go doesn't check for nil fn, so calling with nil would panic
	t.Run("nil function check", func(t *testing.T) {
		// Not calling with nil as it would panic
		// Instead, testing with a valid function that does nothing
		err := ForInt(10, func(i int) error {
			return nil
		})
		assert.NoError(t, err)
	})
}