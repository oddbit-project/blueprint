package blueprint

import (
	"context"
	"github.com/oddbit-project/blueprint/types/callstack"
	"github.com/rs/zerolog/log"
	"sync"
	"time"
)

// DefaultShutdownTimeout is the default deadline of ShutdownContext, counted from the start of Shutdown
const DefaultShutdownTimeout = 5 * time.Second

var appDestructors *callstack.CallStack = nil
var shutdownMx = &sync.Mutex{}

// shutdownDone is closed when the destructors of the current Shutdown have run
var shutdownDone chan struct{}

var shutdownTimeout = DefaultShutdownTimeout

// shutdownCtx is the deadline context of the current (or last) Shutdown; nil before the first one
var shutdownCtx context.Context

// SetShutdownTimeout sets the deadline of ShutdownContext, counted from the start of Shutdown; a d of
// zero or less gives destructors an already expired context. Keep it below the grace period of whatever
// stops the process (docker stop: 10s), or later destructors are killed before they run.
// Shutdown does not enforce it: a destructor that does not use ShutdownContext is not interrupted
func SetShutdownTimeout(d time.Duration) {
	shutdownMx.Lock()
	defer shutdownMx.Unlock()
	shutdownTimeout = d
}

// ShutdownContext returns the context destructors pass to Shutdown/Stop calls, e.g.
// server.Shutdown(blueprint.ShutdownContext()); the application context cannot be used for this,
// as on a signal it is already cancelled when destructors run. Call it from a destructor: elsewhere
// it may be called before Shutdown has started.
// During Shutdown it has one deadline, the shutdown timeout counted from the start of Shutdown,
// shared by all destructors. Before Shutdown it returns context.Background(); after Shutdown,
// a cancelled context
func ShutdownContext() context.Context {
	shutdownMx.Lock()
	defer shutdownMx.Unlock()
	if shutdownCtx == nil {
		return context.Background()
	}
	return shutdownCtx
}

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
	cancel := context.CancelFunc(func() {})
	if destructors != nil {
		done = make(chan struct{})
		shutdownDone = done
		shutdownCtx, cancel = context.WithTimeout(context.Background(), shutdownTimeout)
	}
	shutdownMx.Unlock()

	if destructors != nil {
		if err := destructors.Run(false); err != nil {
			log.Error().Err(err).Msg("Error while shutting down")
		}
		close(done)
	}
	cancel()
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
