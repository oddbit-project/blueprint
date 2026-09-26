package dbx

import (
	"context"
	"database/sql"
	"errors"
)

// WithTx runs fn inside a transaction over q:
//   - if q is already a TxQuerier, fn runs directly in it (composable: no
//     nested BEGIN, no commit/rollback here — the outer WithTx owns them);
//   - if q is a TxBeginner, WithTx begins a transaction, runs fn, and
//     commits on success or rolls back otherwise;
//   - otherwise WithTx fails with ErrTxUnsupported.
//
// A panic, or a runtime.Goexit (as t.FailNow triggers in a goroutine), rolls
// back the transaction it began before propagating; the deferred rollback
// does not use recover(), so the panic/Goexit continues unimpeded and the
// transaction is still released. When fn returns an error and the resulting
// Rollback also fails, WithTx returns errors.Join(fnErr, rollbackErr), so
// errors.Is(err, fnErr) still holds.
func WithTx(ctx context.Context, q Querier, opts *sql.TxOptions, fn func(tx Querier) error) (err error) {
	if tx, ok := q.(TxQuerier); ok {
		return fn(tx)
	}
	beginner, ok := q.(TxBeginner)
	if !ok {
		return ErrTxUnsupported
	}
	tx, err := beginner.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if committed {
			return
		}
		if rbErr := tx.Rollback(); rbErr != nil {
			if err != nil {
				err = errors.Join(err, rbErr)
			} else {
				err = rbErr
			}
		}
	}()

	err = fn(tx)
	if err != nil {
		return err
	}

	if cErr := tx.Commit(); cErr != nil {
		committed = true // Commit was attempted; do not also roll back.
		return cErr
	}
	committed = true
	return nil
}
