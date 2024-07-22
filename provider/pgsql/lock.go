// AdvisoryLock implements primitives to use PostgreSQL's advisory locks.
//
// These locks can be used to pipeline concurrent access between multiple database sessions. Keep in mind, both [Lock]
// and [TryLock] are stackable, can be called multiple times on the same session, requiring the same amount of calls to
// [Unlock] to free up the lock:
//
//	   var l := NewAdvisoryLock(conn, 32)
//	   l.Lock(context.Background()) // lock does not exist previously in this session, so locks successfully
//	   l.Lock(context.Background()) // lock already exists, but this increments the lock
//
//	   l.Unlock() // lock is not freed yet, as it was locked twice
//	   l.Unlock() // only here lock is released
//
//	See https://www.postgresql.org/docs/current/functions-admin.html#FUNCTIONS-ADVISORY-LOCKS for more details on the specifics
//
// of PostgreSQL advisory locks
//
// Note: since pgxpool is used, the lock will keep a connection from the connection pool for himself; this connection
// is freed when the number of locked unlocks is achieved
package pgsql

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"sync"
)

type AdvisoryLock interface {
	Lock(ctx context.Context) error
	TryLock(ctx context.Context) (bool, error)
	Unlock(ctx context.Context) error
}

type advisoryLock struct {
	pool    *pgxpool.Pool
	id      int64
	counter int
	db      *pgxpool.Conn // we must use the same connection for locking/unlocking
	l       sync.Mutex
}

func NewAdvisoryLock(pool *pgxpool.Pool, id int64) AdvisoryLock {
	return &advisoryLock{
		pool:    pool,
		id:      id,
		db:      nil,
		counter: 0,
		l:       sync.Mutex{},
	}
}

// Lock attempts to perform a lock, and waits until it is available
func (l *advisoryLock) Lock(ctx context.Context) error {
	l.l.Lock()
	defer l.l.Unlock()

	var err error
	if l.db == nil {
		l.db, err = l.pool.Acquire(ctx)
		if err != nil {
			return err
		}
	}
	qry := "SELECT pg_advisory_lock($1)"
	_, err = l.db.Exec(ctx, qry, l.id)
	if err == nil {
		// lock acquired
		l.counter++
	}
	return err
}

// TryLock attempts to perform a lock, and returns true if operation was successful
func (l *advisoryLock) TryLock(ctx context.Context) (bool, error) {
	l.l.Lock()
	defer l.l.Unlock()

	var err error
	if l.db == nil {
		l.db, err = l.pool.Acquire(ctx)
		if err != nil {
			return false, err
		}
	}
	result := false
	qry := "SELECT pg_try_advisory_lock($1)"
	err = l.db.QueryRow(ctx, qry, l.id).Scan(&result)
	if err == nil && result {
		// lock acquired
		l.counter++
	}
	return result, err
}

// Unlock unlocks a given lock
// Unlock of a given lock needs to be done with the same connection
func (l *advisoryLock) Unlock(ctx context.Context) error {
	l.l.Lock()
	defer l.l.Unlock()

	var err error
	if l.db == nil {
		l.db, err = l.pool.Acquire(ctx)
		if err != nil {
			return err
		}
	}
	qry := "SELECT pg_advisory_unlock($1)"
	_, err = l.db.Exec(ctx, qry, l.id)
	if l.counter > 0 && err == nil {
		l.counter--
		if l.counter == 0 {
			l.db = nil
		}
	}
	return err
}
