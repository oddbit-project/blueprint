package blueprint

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/oddbit-project/blueprint/types/callstack"
	"github.com/stretchr/testify/assert"
)

func TestShutdownManager(t *testing.T) {
	// Save original state
	originalDestructors := appDestructors

	// Restore state after tests
	defer func() {
		appDestructors = originalDestructors
	}()

	t.Run("GetDestructorManager returns manager", func(t *testing.T) {
		// Setup a fresh callstack
		appDestructors = callstack.NewCallStack()
		
		// Get the manager
		manager := GetDestructorManager()

		// It should be the same instance
		assert.Equal(t, appDestructors, manager)
		assert.NotNil(t, manager)
	})

	t.Run("RegisterDestructor adds function to stack", func(t *testing.T) {
		// Setup a fresh callstack
		appDestructors = callstack.NewCallStack()
		
		executed := false
		RegisterDestructor(func() error {
			executed = true
			return nil
		})

		// Check length of handlers slice using reflection (not ideal but necessary for testing)
		handlersValue := reflect.ValueOf(appDestructors).Elem().FieldByName("handlers")
		assert.Equal(t, 1, handlersValue.Len())

		// Execute the shutdown to verify our function runs
		Shutdown(nil)
		assert.True(t, executed)
	})

	t.Run("Shutdown executes destructors in reverse order", func(t *testing.T) {
		// Setup a fresh callstack
		appDestructors = callstack.NewCallStack()
		
		executionOrder := []int{}
		
		RegisterDestructor(func() error {
			executionOrder = append(executionOrder, 1)
			return nil
		})
		
		RegisterDestructor(func() error {
			executionOrder = append(executionOrder, 2)
			return nil
		})
		
		RegisterDestructor(func() error {
			executionOrder = append(executionOrder, 3)
			return nil
		})

		Shutdown(nil)
		
		// Should execute in reverse order: 3, 2, 1
		assert.Equal(t, []int{3, 2, 1}, executionOrder)
	})

	t.Run("Shutdown is thread-safe", func(t *testing.T) {
		// Setup a fresh callstack
		appDestructors = callstack.NewCallStack()

		var counter int
		var mu sync.Mutex
		
		// Register a destructor that increments a counter
		RegisterDestructor(func() error {
			mu.Lock()
			defer mu.Unlock()
			counter++
			return nil
		})

		// Call Shutdown concurrently from multiple goroutines
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				Shutdown(nil)
			}()
		}
		
		// Wait for all goroutines to complete
		wg.Wait()
		
		// The destructor should only execute once since appDestructors gets set to nil
		assert.Equal(t, 1, counter)
	})

	t.Run("Shutdown handles nil destructor manager", func(t *testing.T) {
		// Set callstack to nil
		appDestructors = nil
		
		// Shutdown with nil appDestructors should not panic
		assert.NotPanics(t, func() {
			Shutdown(nil)
		})
	})

	// Skip the test with error because it calls log.Fatal which will exit the program
	// In a real test environment, we would use a logger interface that could be mocked
	t.Skip("Skipping test with error argument because it calls log.Fatal")
	t.Run("Shutdown with error argument", func(t *testing.T) {
		// Setup a fresh callstack
		appDestructors = callstack.NewCallStack()
		
		// This destructor won't be executed because log.Fatal will exit the program
		RegisterDestructor(func() error {
			return nil
		})

		// This would normally log a fatal error and exit
		err := errors.New("test error")
		Shutdown(err)
		// Cannot actually test what happens after this because log.Fatal will exit
	})
}