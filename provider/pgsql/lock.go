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
package pgsql

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type LockConn interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

type AdvisoryLock struct {
	conn LockConn
	id   int
}

func NewAdvisoryLock(conn LockConn, id int) *AdvisoryLock {
	return &AdvisoryLock{
		conn: conn,
		id:   id,
	}
}

// Lock attempts to perform a lock, and waits until it is available
func (l *AdvisoryLock) Lock(ctx context.Context) error {
	qry := "SELECT pg_advisory_lock($1)"
	_, err := l.conn.Exec(ctx, qry, l.id)
	return err
}

// TryLock attempts to perform a lock, and returns true if operation was successful
func (l *AdvisoryLock) TryLock(ctx context.Context) (bool, error) {
	result := false
	qry := "SELECT pg_try_advisory_lock($1)"
	err := l.conn.QueryRow(ctx, qry, l.id).Scan(&result)
	return result, err
}

// Unlock unlocks a given lock
// Unlock of a given lock needs to be done with the same connection
func (l *AdvisoryLock) Unlock(ctx context.Context) error {
	qry := "SELECT pg_advisory_unlock($1)"
	_, err := l.conn.Exec(ctx, qry, l.id)
	return err
}
