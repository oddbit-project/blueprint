package runner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/utils"
)

const (
	ErrInvalidInterval = utils.Error("interval must be positive")
	ErrNilRunnerFn     = utils.Error("runner function must not be nil")
	ErrNilLogger       = utils.Error("logger must not be nil")
	ErrAlreadyRunning  = utils.Error("already running")
	ErrNotRunning      = utils.Error("not running")
)

type RunnerFn func(ctx context.Context) error

type PeriodicRunner struct {
	runInterval time.Duration
	runFn       RunnerFn
	logger      *log.Logger
	status      atomic.Int32
	mx          sync.Mutex
	cancelFn    context.CancelFunc
	done        chan struct{}
}

func NewUpdater(updateInterval time.Duration, updateFn RunnerFn, logger *log.Logger) (*PeriodicRunner, error) {
	if updateInterval <= 0 {
		return nil, ErrInvalidInterval
	}
	if updateFn == nil {
		return nil, ErrNilRunnerFn
	}
	if logger == nil {
		return nil, ErrNilLogger
	}
	return &PeriodicRunner{
		runInterval: updateInterval,
		runFn:       updateFn,
		logger:      logger,
	}, nil
}

func (u *PeriodicRunner) Start(ctx context.Context) error {
	u.mx.Lock()
	defer u.mx.Unlock()

	if !u.status.CompareAndSwap(0, 1) {
		return ErrAlreadyRunning
	}

	runCtx, cancelFn := context.WithCancel(ctx)
	done := make(chan struct{})
	u.cancelFn = cancelFn
	u.done = done

	go func() {
		defer close(done)
		defer u.status.Store(0)
		defer func() {
			if r := recover(); r != nil {
				u.logger.Warnf("Recovered from panic in runner: %v", r)
			}
		}()
		if err := u.run(runCtx); err != nil && !errors.Is(err, context.Canceled) {
			u.logger.Error(err, "runner error")
		}
	}()

	u.logger.Infof("periodic runner started with interval %v", u.runInterval)
	return nil
}

func (u *PeriodicRunner) Stop(ctx context.Context) error {
	u.mx.Lock()
	cancelFn, done := u.cancelFn, u.done
	u.mx.Unlock()

	if cancelFn == nil {
		return ErrNotRunning
	}
	cancelFn()

	select {
	case <-done:
		u.mx.Lock()
		if u.done == done {
			u.cancelFn = nil
			u.done = nil
		}
		u.mx.Unlock()
		u.logger.Infof("periodic runner stopped")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (u *PeriodicRunner) run(ctx context.Context) error {
	ticker := time.NewTicker(u.runInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						u.logger.Warnf("Recovered from panic in runner function: %v", r)
					}
				}()
				if err := u.runFn(ctx); err != nil {
					u.logger.Error(err, "runner function error")
				}
			}()
		}
	}
}
