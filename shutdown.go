package blueprint

import (
	"github.com/oddbit-project/blueprint/types/callstack"
	"github.com/rs/zerolog/log"
	"sync"
)

var appDestructors *callstack.CallStack = nil
var shutdownMx = &sync.Mutex{}

// GetDestructorManager Retrieve callback manager
func GetDestructorManager() *callstack.CallStack {
	return appDestructors
}

// RegisterDestructor Register a function to perform shutdown procedures
func RegisterDestructor(fn callstack.CallableFn) {
	appDestructors.Add(fn)
}

// Shutdown Shuts down the whole application
func Shutdown(arg error) {
	shutdownMx.Lock()
	defer shutdownMx.Unlock()

	if appDestructors == nil {
		return
	}
	if err := appDestructors.Run(false); err != nil {
		log.Error().Err(err).Msg("Error while shutting down")
	}
	appDestructors = nil
	if arg != nil {
		log.Fatal().Err(arg).Msg("Fatal error")
	}
}

func init() {
	appDestructors = callstack.NewCallStack()
}
