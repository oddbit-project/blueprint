package blueprint

import (
	"context"
	"errors"
	"github.com/oddbit-project/blueprint/config"
	"github.com/rs/zerolog/log"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

type RuntimeFn func(app interface{}) error

type Container struct {
	Config         config.ConfigProvider
	Context        context.Context
	CancelCtx      context.CancelFunc
	services       map[string]interface{}
	mu             sync.RWMutex
	isShuttingDown atomic.Bool
}

// NewContainer create new container runtime with the specified config provider and a new application context
func NewContainer(config config.ConfigProvider) *Container {
	ctx, cancelFn := context.WithCancel(context.Background())
	return &Container{
		Config:         config,
		Context:        ctx,
		CancelCtx:      cancelFn,
		services:       make(map[string]interface{}),
		mu:             sync.RWMutex{},
		isShuttingDown: atomic.Bool{},
	}
}

// Register a service by name
func (c *Container) Register(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = service
}

// Get retrieve a service by name
func (c *Container) Get(name string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	service, exists := c.services[name]
	return service, exists
}

// Exists check if a service is registered
func (c *Container) Exists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	_, exists := c.services[name]
	return exists
}

// GetContext helper function to retrieve context
func (c *Container) GetContext() context.Context {
	return c.Context
}

// Run runs application container
// mainFn is a collection of non-blocking functions; they will be executed in order.
// each one will receive the Container object as the parameter:
// Example:
//
//	object.Run(func(app interface{}) error{
//			app := a.(*Container)
//			app.AbortFatal(nil) // won't abort because arg is nil
//			return nil
//	})
//
// the main loop will wait for an os signal on the 'monitor' channel; when signal is
// received, the application is terminated in an orderly fashion by invoking Terminate()
func (c *Container) Run(mainFn ...RuntimeFn) {
	// capture os signals
	monitor := make(chan os.Signal, 1)
	signal.Notify(monitor, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	for _, fn := range mainFn {
		if err := fn(c); err != nil {
			c.Terminate(err)
		}
	}

	for {
		select {
		case <-monitor:
			log.Info().Msg("Shutting down application...")
			Shutdown(nil)
			c.CancelCtx()

		case <-c.Context.Done():
			signal.Stop(monitor)
			c.Terminate(nil)
		}
	}
}

// AbortFatal aborts execution in case of fatal error
func (c *Container) AbortFatal(err error) {
	if err != nil {
		Shutdown(err)
		c.Terminate(err)
	}
}

// Terminate application execution and exit to operating system
func (c *Container) Terminate(err error) {
	if c.isShuttingDown.Swap(true) {
		return // Already shutting down
	}

	retCode := 0
	if err != nil {
		retCode = -1
	}

	// cancel application context
	if c.Context != nil {
		// cancel context if not canceled yet
		if c.CancelCtx != nil && !errors.Is(c.Context.Err(), context.Canceled) {
			c.CancelCtx()
		}
	}

	// exit to os
	os.Exit(retCode)
}
