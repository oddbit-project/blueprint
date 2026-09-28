package dbx

import (
	"context"
	"database/sql"
	"errors"
	"math/rand/v2"
	"time"
)

const (
	// retryBaseDelay is the backoff window before the first retry; it
	// doubles on each further retry up to retryMaxDelay.
	retryBaseDelay = 2 * time.Millisecond
	retryMaxDelay  = 100 * time.Millisecond
)

// retryBackoff returns the delay before retry number retry (0 for the
// first retry). It is a variable so tests can make it deterministic.
var retryBackoff = fullJitterBackoff

// fullJitterBackoff draws a uniformly random delay in
// [0, min(retryBaseDelay<<retry, retryMaxDelay)).
func fullJitterBackoff(retry int) time.Duration {
	window := retryMaxDelay
	// retryBaseDelay<<16 is already far beyond retryMaxDelay; the bound
	// also keeps the shift from overflowing.
	if retry < 16 && retryBaseDelay<<retry < window {
		window = retryBaseDelay << retry
	}
	return rand.N(window)
}

// WithTxRetry runs fn in a transaction like WithTx, retrying the whole
// transaction when it fails with an error q classifies as transient
// (e.g. a PostgreSQL serialization failure under sql.LevelSerializable).
//
// Each attempt is a fresh WithTx over q: a failed attempt is rolled back
// before the next one begins. Between attempts WithTxRetry sleeps a
// full-jitter backoff (a random delay below min(2ms<<n, 100ms) before
// retry n); if ctx is done before the next attempt it stops and returns
// errors.Join(ctx.Err(), lastErr).
//
// attempts is the maximum number of times fn runs; a value below 1 is
// treated as 1. When every attempt fails, the last attempt's error is
// returned.
//
// Only errors q classifies as retryable are retried: q must implement
// RetryClassifier (SQLQuerier does, for PostgreSQL and SQLite). With any
// other Querier, or any other error, WithTxRetry returns after the first
// attempt, exactly like WithTx.
//
// When q is already a TxQuerier, WithTx joins that outer transaction, so
// WithTxRetry runs fn once and never retries: after a serialization
// failure the outer transaction is aborted, and only the code that owns it
// can retry it.
//
// fn may run more than once, so it must be idempotent apart from its
// database writes, which are rolled back with each failed attempt. Values
// fn stores outside the transaction persist across attempts: a record
// reused between attempts keeps what a failed attempt assigned to it (for
// example an id copied from InsertReturning's result, whose row was rolled
// back), so build such values inside fn.
func WithTxRetry(ctx context.Context, q Querier, opts *sql.TxOptions, attempts int, fn func(tx Querier) error) error {
	classifier, ok := q.(RetryClassifier)
	if _, joined := q.(TxQuerier); joined || !ok {
		return WithTx(ctx, q, opts, fn)
	}
	if attempts < 1 {
		attempts = 1
	}

	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			// sleepCtx may return nil for a timer that fired together with
			// ctx.Done(), so check ctx again before starting the attempt.
			cErr := sleepCtx(ctx, retryBackoff(attempt-1))
			if cErr == nil {
				cErr = ctx.Err()
			}
			if cErr != nil {
				return errors.Join(cErr, err)
			}
		}
		err = WithTx(ctx, q, opts, fn)
		if err == nil || !classifier.IsRetryable(err) {
			return err
		}
	}
	return err
}

// sleepCtx waits for d, returning ctx.Err() early if ctx is done first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
