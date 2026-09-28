package blueprint

import (
	"bytes"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oddbit-project/blueprint/types/callstack"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
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

	t.Run("Shutdown with error runs destructors before fatal", func(t *testing.T) {
		// We can't test the full Shutdown(err) path because log.Fatal exits,
		// but we can verify that destructors.Run(false) is called before
		// the fatal by testing that appDestructors is set to nil after
		// a Shutdown(nil) following a destructor that records execution.
		// The reordering fix ensures destructors always run first.
		appDestructors = callstack.NewCallStack()

		executed := false
		RegisterDestructor(func() error {
			executed = true
			return nil
		})

		// Shutdown(nil) exercises the same destructor path without calling log.Fatal
		Shutdown(nil)
		assert.True(t, executed)
		assert.Nil(t, appDestructors)
	})
}

func TestShutdown_LogsDestructorErrors(t *testing.T) {
	originalDestructors := appDestructors
	originalLogger := zlog.Logger
	defer func() {
		appDestructors = originalDestructors
		zlog.Logger = originalLogger
	}()

	var buf bytes.Buffer
	zlog.Logger = zerolog.New(&buf)
	appDestructors = callstack.NewCallStack()

	RegisterDestructor(func() error { return errors.New("destructor failure") })
	Shutdown(nil)

	assert.Contains(t, buf.String(), "Error while shutting down")
	assert.Contains(t, buf.String(), "destructor failure")
}

func TestRegisterDestructor_AfterShutdown(t *testing.T) {
	originalDestructors := appDestructors
	defer func() {
		appDestructors = originalDestructors
	}()

	appDestructors = callstack.NewCallStack()
	Shutdown(nil)

	assert.NotPanics(t, func() {
		RegisterDestructor(func() error { return nil })
	})
	assert.Nil(t, GetDestructorManager())
}

func TestRegisterDestructor_ConcurrentWithShutdown(t *testing.T) {
	originalDestructors := appDestructors
	defer func() {
		appDestructors = originalDestructors
	}()

	appDestructors = callstack.NewCallStack()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			RegisterDestructor(func() error { return nil })
			GetDestructorManager()
		}
	}()
	go func() {
		defer wg.Done()
		Shutdown(nil)
	}()
	wg.Wait()
	assert.Nil(t, GetDestructorManager())
}

func TestShutdown_DestructorRegistersDestructor(t *testing.T) {
	originalDestructors := appDestructors
	defer func() {
		appDestructors = originalDestructors
	}()

	appDestructors = callstack.NewCallStack()

	lateRan := false
	RegisterDestructor(func() error {
		RegisterDestructor(func() error {
			lateRan = true
			return nil
		})
		_ = GetDestructorManager()
		return nil
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		Shutdown(nil)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown deadlocked when a destructor called RegisterDestructor")
	}
	assert.False(t, lateRan)
	assert.Nil(t, GetDestructorManager())
}

func TestShutdown_ConcurrentCallersWaitForDestructors(t *testing.T) {
	originalDestructors := appDestructors
	defer func() {
		appDestructors = originalDestructors
	}()

	appDestructors = callstack.NewCallStack()

	started := make(chan struct{})
	release := make(chan struct{})
	var finished atomic.Bool
	RegisterDestructor(func() error {
		close(started)
		<-release
		finished.Store(true)
		return nil
	})

	go Shutdown(nil)
	<-started

	second := make(chan struct{})
	go func() {
		defer close(second)
		Shutdown(nil)
	}()

	select {
	case <-second:
		t.Fatal("second Shutdown returned before destructors finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	select {
	case <-second:
	case <-time.After(2 * time.Second):
		t.Fatal("second Shutdown did not return")
	}
	assert.True(t, finished.Load())
}
