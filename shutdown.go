package blueprint

import (
	"github.com/oddbit-project/blueprint/types/callstack"
	"github.com/rs/zerolog/log"
	"sync"
)

var appDestructors *callstack.CallStack = nil
var shutdownMx = &sync.Mutex{}

// shutdownRunMx serializes Shutdown calls, so concurrent callers wait for the destructors to finish
var shutdownRunMx = &sync.Mutex{}

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
func Shutdown(arg error) {
	shutdownRunMx.Lock()
	defer shutdownRunMx.Unlock()

	shutdownMx.Lock()
	destructors := appDestructors
	appDestructors = nil
	shutdownMx.Unlock()

	if destructors != nil {
		if err := destructors.Run(false); err != nil {
			log.Error().Err(err).Msg("Error while shutting down")
		}
	}
	if arg != nil {
		log.Fatal().Err(arg).Msg("Fatal error")
	}
}

func init() {
	appDestructors = callstack.NewCallStack()
}
