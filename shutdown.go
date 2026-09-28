package blueprint

import (
	"github.com/oddbit-project/blueprint/types/callstack"
	"github.com/rs/zerolog/log"
	"sync"
)

var appDestructors *callstack.CallStack = nil
var shutdownMx = &sync.Mutex{}

// shutdownDone is closed when the destructors of the current Shutdown have run
var shutdownDone chan struct{}

// GetDestructorManager Retrieve callback manager; returns nil after Shutdown
func GetDestructorManager() *callstack.CallStack {
	shutdownMx.Lock()
	defer shutdownMx.Unlock()
	return appDestructors
}

// RegisterDestructor Register a function to perform shutdown procedures
// Calling RegisterDestructor after (or during) Shutdown is a no-op
func RegisterDestructor(fn callstack.CallableFn) {
	shutdownMx.Lock()
	defer shutdownMx.Unlock()
	if appDestructors == nil {
		return
	}
	appDestructors.Add(fn)
}

// Shutdown Shuts down the whole application
// Only the first call runs the destructors; a call made while they are running returns immediately
// (or, with a non-nil arg, logs the fatal error and exits without waiting for them)
func Shutdown(arg error) {
	shutdownMx.Lock()
	destructors := appDestructors
	appDestructors = nil
	var done chan struct{}
	if destructors != nil {
		done = make(chan struct{})
		shutdownDone = done
	}
	shutdownMx.Unlock()

	if destructors != nil {
		if err := destructors.Run(false); err != nil {
			log.Error().Err(err).Msg("Error while shutting down")
		}
		close(done)
	}
	if arg != nil {
		log.Fatal().Err(arg).Msg("Fatal error")
	}
}

// waitShutdown blocks until the destructors of an in-progress Shutdown have run
func waitShutdown() {
	shutdownMx.Lock()
	done := shutdownDone
	shutdownMx.Unlock()
	if done != nil {
		<-done
	}
}

func init() {
	appDestructors = callstack.NewCallStack()
}
