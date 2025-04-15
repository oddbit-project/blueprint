package parallel

import (
	"context"
	"sync"
)

type ForIntFn func(i int) error

// ForInt iterate a function in parallel using goroutines
// Adapted from https://github.com/tsenart/nap scatter() function
func ForInt(to int, fn ForIntFn) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, to)
	var wg sync.WaitGroup

	for i := 0; i < to; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case errChan <- fn(i):
			case <-ctx.Done():
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errChan)
	}()

	for err := range errChan {
		if err != nil {
			cancel() // cancel other goroutines
			return err
		}
	}
	return nil
}
