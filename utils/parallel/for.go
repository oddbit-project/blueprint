package parallel

import (
	"sync"
)

type ForIntFn func(i int) error

// ForInt iterate a function in parallel using goroutines
// Adapted from https://github.com/tsenart/nap scatter() function
// It waits for all goroutines to finish and returns the first error, if any; to <= 0 returns nil
func ForInt(to int, fn ForIntFn) error {
	if to <= 0 {
		return nil
	}

	errChan := make(chan error, to)
	var wg sync.WaitGroup

	for i := 0; i < to; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errChan <- fn(i)
		}(i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	var firstErr error
	for err := range errChan {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
