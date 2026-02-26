package runner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oddbit-project/blueprint/log"
)

type RunnerFn func(ctx context.Context) error

type PeriodicRunner struct {
	runInterval time.Duration
	runFn       RunnerFn
	logger      *log.Logger
	status      atomic.Int32
	cancelFn    context.CancelFunc
	wg          sync.WaitGroup
}

func NewUpdater(updateInterval time.Duration, updateFn RunnerFn, logger *log.Logger) *PeriodicRunner {
	return &PeriodicRunner{
		runInterval: updateInterval,
		runFn:       updateFn,
		logger:      logger,
	}
}

func (u *PeriodicRunner) Start(ctx context.Context) error {
	if !u.status.CompareAndSwap(0, 1) {
		return errors.New("already running")
	}

	runCtx, cancelFn := context.WithCancel(ctx)
	u.cancelFn = cancelFn

	u.wg.Add(1)
	go func() {
		defer u.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				u.logger.Warnf("Recovered from panic in runner: %v", r)
				u.status.Store(0)
			}
		}()
		if err := u.run(runCtx); err != nil {
			u.logger.Error(err, "runner error")
		}
	}()

	u.logger.Infof("periodic runner started with interval %v", u.runInterval)
	return nil
}

func (u *PeriodicRunner) Stop(ctx context.Context) error {
	if !u.status.CompareAndSwap(1, 0) {
		return errors.New("not running")
	}

	if u.cancelFn != nil {
		u.cancelFn()
	}

	done := make(chan struct{})
	go func() {
		u.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
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
