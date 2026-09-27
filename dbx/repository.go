package dbx

import (
	"context"
	"reflect"

	"github.com/oddbit-project/blueprint/sqlb"
)

// Repository is a typed, generic repository over T, backed by a Querier and
// built entirely through sqlb (values always bound, identifiers always
// escaped). See package doc for the ErrNotFound and no-unfiltered-write
// semantics, and for caller obligations WithTx cannot enforce.
type Repository[T any] struct {
	q      Querier
	table  string
	cols   []string
	colSet map[string]bool
	sel    *sqlb.SelectBuilder
}

// NewRepository builds a Repository[T] bound to q, for table. T must be a
// struct type (else ErrNotStruct); its columns are resolved via
// sqlb.RecordColumns, which rejects field shapes db/field and sqlx would
// disagree on (returned unchanged: sqlb.ErrRecordShape,
// sqlb.ErrDuplicateColumn). A record type with zero mapped columns fails
// with ErrNoColumns. The base "SELECT <columns> FROM <table>" statement is
// built once against q.Dialect(), so an invalid table name or unknown
// dialect surfaces here, not at the first query.
func NewRepository[T any](q Querier, table string) (*Repository[T], error) {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		return nil, ErrNotStruct
	}
	cols, err := sqlb.RecordColumns(t)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, ErrNoColumns
	}

	colSet := make(map[string]bool, len(cols))
	selCols := make([]any, len(cols))
	for i, c := range cols {
		colSet[c] = true
		selCols[i] = c
	}

	sel := sqlb.Select(selCols...).From(table)
	if _, _, err := sel.Build(q.Dialect()); err != nil {
		return nil, err
	}

	return &Repository[T]{
		q:      q,
		table:  table,
		cols:   cols,
		colSet: colSet,
		sel:    sel,
	}, nil
}

// Table returns the repository's table name.
func (r *Repository[T]) Table() string { return r.table }

// Querier returns the repository's bound Querier.
func (r *Repository[T]) Querier() Querier { return r.q }

// With returns a copy of the repository bound to q instead (e.g. a
// transaction). See the package doc for why this must be used inside
// WithTx instead of the outer repository.
func (r *Repository[T]) With(q Querier) *Repository[T] {
	nr := *r
	nr.q = q
	return &nr
}

// Select returns "SELECT <T's columns> FROM <table>", the starting point
// for custom queries passed to Get/List.
func (r *Repository[T]) Select() *sqlb.SelectBuilder { return r.sel }

// checkColumns fails with ErrUnknownColumn for any name not in the
// repository's column list.
func (r *Repository[T]) checkColumns(names []string) error {
	for _, n := range names {
		if !r.colSet[n] {
			return ErrUnknownColumn
		}
	}
	return nil
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Get runs q (r.Select() when nil) with an added Limit(1) and returns the
// first matching row, or (nil, ErrNotFound) when none matches.
func (r *Repository[T]) Get(ctx context.Context, q *sqlb.SelectBuilder) (*T, error) {
	if q == nil {
		q = r.sel
	}
	q = q.Limit(1)
	sqlStr, args, err := q.Build(r.q.Dialect())
	if err != nil {
		return nil, err
	}
	var out T
	if err := r.q.Get(ctx, &out, sqlStr, args...); err != nil {
		return nil, err
	}
	return &out, nil
}

// List runs q (r.Select() when nil) and returns every matching row. It
// never returns a nil slice.
func (r *Repository[T]) List(ctx context.Context, q *sqlb.SelectBuilder) ([]*T, error) {
	if q == nil {
		q = r.sel
	}
	sqlStr, args, err := q.Build(r.q.Dialect())
	if err != nil {
		return nil, err
	}
	out := make([]*T, 0)
	if err := r.q.Select(ctx, &out, sqlStr, args...); err != nil {
		return nil, err
	}
	return out, nil
}

// QueryGrid builds r.Select() through g.Build(_, q) (which validates q
// against g's spec) and runs the result through List.
func (r *Repository[T]) QueryGrid(ctx context.Context, g *Grid[T], q *GridQuery) ([]*T, error) {
	sel, err := g.Build(r.sel, q)
	if err != nil {
		return nil, err
	}
	return r.List(ctx, sel)
}

// GetBy is Get filtered by an equality match on fields, whose keys must all
// be repository columns (else ErrUnknownColumn, checked before any query).
func (r *Repository[T]) GetBy(ctx context.Context, fields map[string]any) (*T, error) {
	if err := r.checkColumns(mapKeys(fields)); err != nil {
		return nil, err
	}
	return r.Get(ctx, r.sel.Where(sqlb.Match(fields)))
}

// ListBy is List filtered by an equality match on fields, whose keys must
// all be repository columns (else ErrUnknownColumn, checked before any
// query).
func (r *Repository[T]) ListBy(ctx context.Context, fields map[string]any) ([]*T, error) {
	if err := r.checkColumns(mapKeys(fields)); err != nil {
		return nil, err
	}
	return r.List(ctx, r.sel.Where(sqlb.Match(fields)))
}

// Count returns "SELECT COUNT(*) FROM <table>[ WHERE ...]"; where == nil
// counts every row.
func (r *Repository[T]) Count(ctx context.Context, where sqlb.Expr) (int64, error) {
	q := sqlb.Select(sqlb.CountAll()).From(r.table)
	if where != nil {
		q = q.Where(where)
	}
	sqlStr, args, err := q.Build(r.q.Dialect())
	if err != nil {
		return 0, err
	}
	return r.q.QueryInt64(ctx, sqlStr, args...)
}

// Exists reports whether any row matches where (nil matches every row). It
// is built as
// "SELECT COUNT(*) FROM (SELECT 1 FROM <table>[ WHERE ...] LIMIT 1) AS "e""
// so QueryInt64 always reads a COUNT column: on ClickHouse, COUNT(*) returns
// UInt64 but a bare SELECT 1 returns UInt8, and QueryInt64's scan target
// only accepts the former.
func (r *Repository[T]) Exists(ctx context.Context, where sqlb.Expr) (bool, error) {
	inner := sqlb.Select(sqlb.Int(1)).From(r.table)
	if where != nil {
		inner = inner.Where(where)
	}
	inner = inner.Limit(1)
	q := sqlb.Select(sqlb.CountAll()).From(inner.As("e"))
	sqlStr, args, err := q.Build(r.q.Dialect())
	if err != nil {
		return false, err
	}
	n, err := r.q.QueryInt64(ctx, sqlStr, args...)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// chunkRecords splits rows into slices of at most perChunk elements.
func chunkRecords(rows []any, perChunk int) [][]any {
	if len(rows) == 0 {
		return nil
	}
	chunks := make([][]any, 0, (len(rows)+perChunk-1)/perChunk)
	for i := 0; i < len(rows); i += perChunk {
		end := i + perChunk
		if end > len(rows) {
			end = len(rows)
		}
		chunks = append(chunks, rows[i:end])
	}
	return chunks
}

// execInsertChunk builds and runs a single "INSERT INTO <table> ..." for
// chunk against q.
func (r *Repository[T]) execInsertChunk(ctx context.Context, q Querier, chunk []any) error {
	st := sqlb.Insert(r.table).Rows(chunk...)
	sqlStr, args, err := st.Build(q.Dialect())
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, sqlStr, args...)
	return err
}

// Insert writes records. Zero records is a no-op. If the bound Querier
// implements BatchInserter, InsertBatch is used instead of building INSERT
// statements. Otherwise records are chunked at
// perChunk = Dialect().MaxArgs() / <T's full column count, auto columns
// included> (all records in one statement when MaxArgs() == 0); more than
// one chunk runs atomically through WithTx, which joins an existing
// transaction, begins a new one, or fails with ErrTxUnsupported — Insert
// never issues a silent non-atomic multi-statement write. Each chunk's
// column set is decided independently by its first record (sqlb's
// ErrInconsistentOmit still applies within a chunk).
func (r *Repository[T]) Insert(ctx context.Context, records ...*T) error {
	if len(records) == 0 {
		return nil
	}
	rows := make([]any, len(records))
	for i, rec := range records {
		rows[i] = rec
	}

	if bi, ok := r.q.(BatchInserter); ok {
		return bi.InsertBatch(ctx, r.table, rows)
	}

	maxArgs := r.q.Dialect().MaxArgs()
	perChunk := len(rows)
	if maxArgs > 0 {
		perChunk = maxArgs / len(r.cols)
		if perChunk < 1 {
			perChunk = 1
		}
	}

	chunks := chunkRecords(rows, perChunk)
	if len(chunks) == 1 {
		return r.execInsertChunk(ctx, r.q, chunks[0])
	}
	return WithTx(ctx, r.q, nil, func(tx Querier) error {
		for _, chunk := range chunks {
			if err := r.execInsertChunk(ctx, tx, chunk); err != nil {
				return err
			}
		}
		return nil
	})
}

// InsertReturning inserts rec and scans every repository column back from
// RETURNING into a new T. Dialects without sqlb's FeatureReturning fail
// with sqlb.ErrUnsupported.
func (r *Repository[T]) InsertReturning(ctx context.Context, rec *T) (*T, error) {
	returning := make([]any, len(r.cols))
	for i, c := range r.cols {
		returning[i] = c
	}
	st := sqlb.Insert(r.table).Rows(rec).Returning(returning...)
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return nil, err
	}
	var out T
	if err := r.q.Get(ctx, &out, sqlStr, args...); err != nil {
		return nil, err
	}
	return &out, nil
}

// subtractStrings returns the elements of a not present in b, preserving
// a's order.
func subtractStrings(a, b []string) []string {
	excl := make(map[string]bool, len(b))
	for _, s := range b {
		excl[s] = true
	}
	out := make([]string, 0, len(a))
	for _, s := range a {
		if !excl[s] {
			out = append(out, s)
		}
	}
	return out
}

// Upsert inserts rec with "ON CONFLICT (conflict...) DO UPDATE SET ...",
// updating the columns named by update, or (if update is empty) every
// column rec's INSERT would actually write minus conflict — never a column
// the INSERT omitted, which would overwrite existing data with its
// default. If that leaves nothing to update, the statement is
// "DO NOTHING". conflict and update names must all be repository columns
// (else ErrUnknownColumn, checked before any query).
func (r *Repository[T]) Upsert(ctx context.Context, rec *T, conflict []string, update ...string) error {
	if err := r.checkColumns(conflict); err != nil {
		return err
	}
	if err := r.checkColumns(update); err != nil {
		return err
	}

	updateCols := update
	if len(updateCols) == 0 {
		insCols, err := sqlb.InsertColumns(rec)
		if err != nil {
			return err
		}
		updateCols = subtractStrings(insCols, conflict)
	}

	cb := sqlb.Insert(r.table).Rows(rec).OnConflict(conflict...)
	var st *sqlb.InsertBuilder
	if len(updateCols) == 0 {
		st = cb.DoNothing()
	} else {
		st = cb.DoUpdateExcluded(updateCols...)
	}

	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return err
	}
	_, err = r.q.Exec(ctx, sqlStr, args...)
	return err
}

// Update sets every non-auto field of rec (per opts) and requires a
// non-nil where (else sqlb.ErrNoWhere, without touching the database).
func (r *Repository[T]) Update(ctx context.Context, rec *T, where sqlb.Expr, opts ...sqlb.RecordOption) (int64, error) {
	if where == nil {
		return 0, sqlb.ErrNoWhere
	}
	st := sqlb.Update(r.table).SetRecord(rec, opts...).Where(where)
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return 0, err
	}
	return r.q.Exec(ctx, sqlStr, args...)
}

// UpdateFields sets fields (whose keys must all be repository columns,
// else ErrUnknownColumn) and requires a non-nil where (else
// sqlb.ErrNoWhere). Both guards are checked before any query.
func (r *Repository[T]) UpdateFields(ctx context.Context, fields map[string]any, where sqlb.Expr) (int64, error) {
	if where == nil {
		return 0, sqlb.ErrNoWhere
	}
	if err := r.checkColumns(mapKeys(fields)); err != nil {
		return 0, err
	}
	st := sqlb.Update(r.table).SetMap(fields).Where(where)
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return 0, err
	}
	return r.q.Exec(ctx, sqlStr, args...)
}

// Delete requires a non-nil where (else sqlb.ErrNoWhere, without touching
// the database). A trivially-true where (e.g. sqlb.And() or
// Col(x).NotIn()) is rejected downstream by sqlb's own WHERE-clause check,
// not by this guard: there is deliberately no "delete all" method, so a
// caller who means it uses Exec(ctx, sqlb.Delete(table).All()).
func (r *Repository[T]) Delete(ctx context.Context, where sqlb.Expr) (int64, error) {
	if where == nil {
		return 0, sqlb.ErrNoWhere
	}
	st := sqlb.Delete(r.table).Where(where)
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return 0, err
	}
	return r.q.Exec(ctx, sqlStr, args...)
}

// Exec builds st against the repository's dialect and runs it, returning
// the number of rows affected.
func (r *Repository[T]) Exec(ctx context.Context, st sqlb.Statement) (int64, error) {
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return 0, err
	}
	return r.q.Exec(ctx, sqlStr, args...)
}
