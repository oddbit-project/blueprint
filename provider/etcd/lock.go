package etcd

import (
	"context"
	"go.etcd.io/etcd/client/v3/concurrency"
	"time"
)

// Lock represents a distributed lock backed by etcd.
// It maintains session state to enable proper lock/unlock operations.
// Each Lock instance should be closed when no longer needed to release resources.
type Lock struct {
	session *concurrency.Session
	mutex   *concurrency.Mutex
	name    string
	locked  bool
}

type LockOptions struct {
	TTL time.Duration
}

type LockOption func(*LockOptions)

func WithTTL(ttl time.Duration) LockOption {
	return func(o *LockOptions) {
		o.TTL = ttl
	}
}

// NewLock creates a new distributed lock for the given name.
// The lock uses etcd's session mechanism to ensure proper cleanup on disconnection.
// Each lock should be closed when no longer needed to prevent resource leaks.
func (c *Client) NewLock(name string) (*Lock, error) {
	session, err := concurrency.NewSession(c.client)
	if err != nil {
		return nil, err
	}

	mutex := concurrency.NewMutex(session, name)

	return &Lock{
		session: session,
		mutex:   mutex,
		name:    name,
		locked:  false,
	}, nil
}

// Lock acquires the distributed lock, blocking until available.
// If the context is cancelled or times out, the lock acquisition fails.
// The lock will be automatically released if the session expires.
func (l *Lock) Lock(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	err := l.mutex.Lock(ctx)
	if err != nil {
		return err
	}

	l.locked = true
	return nil
}

// Unlock releases the distributed lock.
// It's safe to call this multiple times - subsequent calls are no-ops.
// The lock is automatically released when the session is closed.
func (l *Lock) Unlock(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if !l.locked {
		return nil // Already unlocked
	}

	err := l.mutex.Unlock(ctx)
	if err != nil {
		return err
	}

	l.locked = false
	return nil
}

// IsLocked returns whether this Lock instance believes it holds the lock.
// Note: This is a local state check and doesn't verify with etcd.
func (l *Lock) IsLocked() bool {
	return l.locked
}

// Name returns the name/path of the distributed lock.
func (l *Lock) Name() string {
	return l.name
}

// Close closes the underlying etcd session and releases all resources.
// This automatically releases the lock if it's currently held.
// After closing, the Lock instance should not be used.
func (l *Lock) Close() error {
	if l.session != nil {
		return l.session.Close()
	}
	return nil
}

// TryLock attempts to acquire the lock without blocking.
// Returns true if the lock was successfully acquired, false if it's held by another process.
// This is useful for implementing non-blocking lock acquisition patterns.
func (l *Lock) TryLock(ctx context.Context, lockOptions ...LockOption) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	opts := &LockOptions{TTL: 1 * time.Millisecond}
	for _, fn := range lockOptions {
		fn(opts)
	}

	// Create a context with a very short timeout to make this non-blocking
	tryCtx, cancel := context.WithTimeout(ctx, opts.TTL)
	defer cancel()

	err := l.mutex.Lock(tryCtx)
	if err != nil {
		// If it's a timeout or cancellation, the lock is held by another process
		if err == context.DeadlineExceeded || err == context.Canceled {
			return false, nil
		}
		return false, err // Actual error
	}

	l.locked = true
	return true, nil
}
