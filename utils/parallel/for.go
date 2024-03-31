package parallel

type ForIntFn func(i int) error

// Iterate a function in parallel using goroutines
// Adapted from https://github.com/tsenart/nap scatter() function
func ForInt(to int, fn ForIntFn) error {
	errChan := make(chan error, to)
	for i := 0; i < to; i++ {
		go func(i int) {
			errChan <- fn(i)
		}(i)
	}
	for i := 0; i < cap(errChan); i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}
	return nil
}
