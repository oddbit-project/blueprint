package dbx

import (
	"context"
	"reflect"

	"github.com/oddbit-project/gohan"
)

// Query builds st against q.Dialect() and scans every row into a new D, for
// results that are not a repository's record type: joins, aggregates,
// projections into a DTO. D's fields map to result columns as with
// Repository (the `db` tag through sqlx; on provider/clickhouse's Querier,
// the `ch` tag and D must be a struct). When q implements RecordChecker,
// D is checked with CheckRecord before any query, and its error returned.
// It never returns a nil slice.
func Query[D any](ctx context.Context, q Querier, st *gohan.SelectBuilder) ([]*D, error) {
	if err := checkDTO[D](q); err != nil {
		return nil, err
	}
	sqlStr, args, err := st.Build(q.Dialect())
	if err != nil {
		return nil, err
	}
	out := make([]*D, 0)
	if err := q.Select(ctx, &out, sqlStr, args...); err != nil {
		return nil, err
	}
	return out, nil
}

// QueryOne is Query for a single row: it scans the first row of st into a
// new D, or returns (nil, ErrNotFound) when st matches no row. Unlike
// Repository.Get it does not add a LIMIT; add Limit(1) to st when it can
// match several rows. D is checked as by Query.
func QueryOne[D any](ctx context.Context, q Querier, st *gohan.SelectBuilder) (*D, error) {
	if err := checkDTO[D](q); err != nil {
		return nil, err
	}
	sqlStr, args, err := st.Build(q.Dialect())
	if err != nil {
		return nil, err
	}
	var out D
	if err := q.Get(ctx, &out, sqlStr, args...); err != nil {
		return nil, err
	}
	return &out, nil
}

// checkDTO runs q's RecordChecker, if it has one, on D.
func checkDTO[D any](q Querier) error {
	if rc, ok := q.(RecordChecker); ok {
		return rc.CheckRecord(reflect.TypeFor[D]())
	}
	return nil
}
