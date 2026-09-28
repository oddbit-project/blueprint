package dbx

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/oddbit-project/gohan"
)

// Repository is a typed, generic repository over T, backed by a Querier and
// built entirely through gohan (values always bound, identifiers always
// escaped). See package doc for the ErrNotFound and no-unfiltered-write
// semantics, and for caller obligations WithTx cannot enforce.
type Repository[T any] struct {
	q      Querier
	table  string
	cols   []string
	colSet map[string]bool
	sel    *gohan.SelectBuilder

	// grouped enables grouped inserts (see WithGroupedInserts).
	grouped bool
}

// NewRepository builds a Repository[T] bound to q, for table. T must be a
// struct type (else ErrNotStruct); its columns are resolved via
// gohan.RecordColumns, which rejects field shapes db/field and sqlx would
// disagree on (returned unchanged: gohan.ErrRecordShape,
// gohan.ErrDuplicateColumn). A record type with zero mapped columns fails
// with ErrNoColumns. The base "SELECT <columns> FROM <table>" statement is
// built once against q.Dialect(), so an invalid table name or unknown
// dialect surfaces here, not at the first query. When q implements
// RecordChecker, its CheckRecord error for T is returned unchanged.
func NewRepository[T any](q Querier, table string) (*Repository[T], error) {
	t := reflect.TypeFor[T]()
	if t.Kind() != reflect.Struct {
		return nil, ErrNotStruct
	}
	cols, err := gohan.RecordColumns(t)
	if err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, ErrNoColumns
	}
	if rc, ok := q.(RecordChecker); ok {
		if err := rc.CheckRecord(t); err != nil {
			return nil, err
		}
	}

	colSet := make(map[string]bool, len(cols))
	selCols := make([]any, len(cols))
	for i, c := range cols {
		colSet[c] = true
		selCols[i] = c
	}

	sel := gohan.Select(selCols...).From(table)
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

// WithGroupedInserts returns a copy of the repository whose Insert accepts
// records that omit different columns (omitnil/omitempty fields): records
// are grouped by the set of columns their INSERT writes, and each group is
// chunked and inserted as Insert would; a record that writes no column at
// all is inserted on its own as "INSERT INTO <table> DEFAULT VALUES". When
// this takes more than one statement, they all run atomically through
// WithTx (ErrTxUnsupported if the Querier cannot begin a transaction).
// Groups are inserted in order of their first record, so rows are not
// written in argument order across groups (auto-generated ids follow the
// groups, not the arguments). Without it, Insert fails such a call with
// gohan.ErrInconsistentOmit (or gohan.ErrNoColumns for a record that writes
// no column). The option does not apply to a BatchInserter Querier (the
// ClickHouse adapter), whose InsertBatch still rejects mixed records:
// several ClickHouse batches cannot be made atomic. The receiver is not
// modified; With keeps the option.
func (r *Repository[T]) WithGroupedInserts() *Repository[T] {
	nr := *r
	nr.grouped = true
	return &nr
}

// Select returns "SELECT <T's columns> FROM <table>", the starting point
// for custom queries passed to Get/List.
func (r *Repository[T]) Select() *gohan.SelectBuilder { return r.sel }

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
func (r *Repository[T]) Get(ctx context.Context, q *gohan.SelectBuilder) (*T, error) {
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
func (r *Repository[T]) List(ctx context.Context, q *gohan.SelectBuilder) ([]*T, error) {
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

// QueryGridWithCount returns QueryGrid's rows and the total number of rows
// q's filters and search match, ignoring its sort and paging: the total is
// counted with Count over g.Conds(q), so the COUNT carries no ORDER BY,
// LIMIT or OFFSET. q is validated before any query runs. Rows and total are
// read by two separate statements, not in one transaction or snapshot, so
// the total can disagree with the rows under concurrent writes. If that
// matters, run it through a repository bound (With) to a transaction whose
// statements share one snapshot: on PostgreSQL that needs REPEATABLE READ
// or SERIALIZABLE isolation (WithTx(ctx, q, &sql.TxOptions{Isolation:
// sql.LevelRepeatableRead}, fn)), since READ COMMITTED takes a snapshot per
// statement; on SQLite any transaction will do. ClickHouse has no
// transactions, so there the total is always a separate read.
func (r *Repository[T]) QueryGridWithCount(ctx context.Context, g *Grid[T], q *GridQuery) ([]*T, int64, error) {
	conds, err := g.Conds(q)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.QueryGrid(ctx, g, q)
	if err != nil {
		return nil, 0, err
	}
	var where gohan.Expr
	if len(conds) > 0 {
		where = gohan.And(conds...)
	}
	total, err := r.Count(ctx, where)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetBy is Get filtered by an equality match on fields, whose keys must all
// be repository columns (else ErrUnknownColumn, checked before any query).
func (r *Repository[T]) GetBy(ctx context.Context, fields map[string]any) (*T, error) {
	if err := r.checkColumns(mapKeys(fields)); err != nil {
		return nil, err
	}
	return r.Get(ctx, r.sel.Where(gohan.Match(fields)))
}

// ListBy is List filtered by an equality match on fields, whose keys must
// all be repository columns (else ErrUnknownColumn, checked before any
// query).
func (r *Repository[T]) ListBy(ctx context.Context, fields map[string]any) ([]*T, error) {
	if err := r.checkColumns(mapKeys(fields)); err != nil {
		return nil, err
	}
	return r.List(ctx, r.sel.Where(gohan.Match(fields)))
}

// Count returns "SELECT COUNT(*) FROM <table>[ WHERE ...]"; where == nil
// counts every row.
func (r *Repository[T]) Count(ctx context.Context, where gohan.Expr) (int64, error) {
	q := gohan.Select(gohan.CountAll()).From(r.table)
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
func (r *Repository[T]) Exists(ctx context.Context, where gohan.Expr) (bool, error) {
	inner := gohan.Select(gohan.Int(1)).From(r.table)
	if where != nil {
		inner = inner.Where(where)
	}
	inner = inner.Limit(1)
	q := gohan.Select(gohan.CountAll()).From(inner.As("e"))
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
	st := gohan.Insert(r.table).Rows(chunk...)
	sqlStr, args, err := st.Build(q.Dialect())
	if err != nil {
		return err
	}
	_, err = q.Exec(ctx, sqlStr, args...)
	return err
}

// Insert writes records. Zero records is a no-op. If the bound Querier
// implements BatchInserter, InsertBatch is used instead of building INSERT
// statements (also with WithGroupedInserts). Otherwise records are chunked at
// perChunk = Dialect().MaxArgs() / <T's full column count, auto columns
// included> (all records in one statement when MaxArgs() == 0); more than
// one chunk runs atomically through WithTx, which joins an existing
// transaction, begins a new one, or fails with ErrTxUnsupported — Insert
// never issues a silent non-atomic multi-statement write. Each chunk's
// column set is decided independently by its first record (gohan's
// ErrInconsistentOmit still applies within a chunk, unless the repository
// was built with WithGroupedInserts).
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

	if r.grouped {
		return r.insertGrouped(ctx, rows)
	}

	chunks := chunkRecords(rows, r.perChunk(len(rows)))
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

// perChunk returns how many of n records one INSERT statement may carry:
// Dialect().MaxArgs() / <T's full column count>, at least 1, or n when the
// dialect has no argument limit.
func (r *Repository[T]) perChunk(n int) int {
	maxArgs := r.q.Dialect().MaxArgs()
	if maxArgs == 0 {
		return n
	}
	return max(maxArgs/len(r.cols), 1)
}

// insertGrouped is Insert under WithGroupedInserts: rows are grouped by
// their INSERT column set, in order of each group's first row; each group
// is chunked, except rows that write no column, which get one DEFAULT
// VALUES statement each. Every statement is built before any runs, and
// more than one runs inside WithTx.
func (r *Repository[T]) insertGrouped(ctx context.Context, rows []any) error {
	// noColumns keys the rows that write no column; a real key is never
	// empty.
	const noColumns = ""
	var order []string
	groups := make(map[string][]any)
	for _, row := range rows {
		key := noColumns
		cols, err := gohan.InsertColumns(row)
		if err == nil {
			key = strings.Join(cols, "\x00")
		} else if !errors.Is(err, gohan.ErrNoColumns) {
			return err
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], row)
	}

	var stmts []*gohan.InsertBuilder
	for _, key := range order {
		group := groups[key]
		if key == noColumns {
			for range group {
				stmts = append(stmts, gohan.Insert(r.table).DefaultValues())
			}
			continue
		}
		for _, chunk := range chunkRecords(group, r.perChunk(len(group))) {
			stmts = append(stmts, gohan.Insert(r.table).Rows(chunk...))
		}
	}

	type built struct {
		sql  string
		args []any
	}
	queries := make([]built, len(stmts))
	for i, st := range stmts {
		sqlStr, args, err := st.Build(r.q.Dialect())
		if err != nil {
			return err
		}
		queries[i] = built{sqlStr, args}
	}

	run := func(q Querier) error {
		for _, b := range queries {
			if _, err := q.Exec(ctx, b.sql, b.args...); err != nil {
				return err
			}
		}
		return nil
	}
	if len(queries) == 1 {
		return run(r.q)
	}
	return WithTx(ctx, r.q, nil, run)
}

// InsertReturning inserts rec and scans every repository column back from
// RETURNING into a new T. Dialects without gohan's FeatureReturning fail
// with gohan.ErrUnsupported.
func (r *Repository[T]) InsertReturning(ctx context.Context, rec *T) (*T, error) {
	st := gohan.Insert(r.table).Rows(rec).Returning(r.returningCols()...)
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

// returningCols returns every repository column, for a RETURNING list.
func (r *Repository[T]) returningCols() []any {
	cols := make([]any, len(r.cols))
	for i, c := range r.cols {
		cols[i] = c
	}
	return cols
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
	st, err := r.upsertStmt(rec, conflict, update)
	if err != nil {
		return err
	}
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return err
	}
	_, err = r.q.Exec(ctx, sqlStr, args...)
	return err
}

// upsertStmt builds Upsert's statement (see Upsert for the rules).
func (r *Repository[T]) upsertStmt(rec *T, conflict, update []string) (*gohan.InsertBuilder, error) {
	if err := r.checkColumns(conflict); err != nil {
		return nil, err
	}
	if err := r.checkColumns(update); err != nil {
		return nil, err
	}

	updateCols := update
	if len(updateCols) == 0 {
		insCols, err := gohan.InsertColumns(rec)
		if err != nil {
			return nil, err
		}
		updateCols = subtractStrings(insCols, conflict)
	}

	cb := gohan.Insert(r.table).Rows(rec).OnConflict(conflict...)
	if len(updateCols) == 0 {
		return cb.DoNothing(), nil
	}
	return cb.DoUpdateExcluded(updateCols...), nil
}

// UpsertReturning is Upsert with "RETURNING <T's columns>", scanned into a
// new T: the row as inserted or as updated, including values the database
// filled in (defaults, triggers, ...). rec itself is never written to, so a
// failed scan cannot leave it half-updated. When the statement resolves to
// "DO NOTHING" (every written column is a conflict column) and the row
// already exists, the database returns no row and UpsertReturning fails
// with ErrNotFound. Dialects without gohan's FeatureUpsert and
// FeatureReturning (ClickHouse) fail with gohan.ErrUnsupported.
func (r *Repository[T]) UpsertReturning(ctx context.Context, rec *T, conflict []string, update ...string) (*T, error) {
	st, err := r.upsertStmt(rec, conflict, update)
	if err != nil {
		return nil, err
	}
	sqlStr, args, err := st.Returning(r.returningCols()...).Build(r.q.Dialect())
	if err != nil {
		return nil, err
	}
	var out T
	if err := r.q.Get(ctx, &out, sqlStr, args...); err != nil {
		return nil, err
	}
	return &out, nil
}

// InsertIgnore inserts rec with "ON CONFLICT [(conflict...)] DO NOTHING" and
// reports whether a row was inserted (rows affected > 0), so a conflicting
// row is skipped rather than failing. With no conflict columns the clause is
// untargeted and any unique or exclusion constraint violation is ignored.
// conflict names must all be repository columns (else ErrUnknownColumn,
// checked before any query). Dialects without gohan's FeatureUpsert
// (ClickHouse) fail with gohan.ErrUnsupported.
func (r *Repository[T]) InsertIgnore(ctx context.Context, rec *T, conflict ...string) (bool, error) {
	if err := r.checkColumns(conflict); err != nil {
		return false, err
	}
	st := gohan.Insert(r.table).Rows(rec).OnConflict(conflict...).DoNothing()
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return false, err
	}
	n, err := r.q.Exec(ctx, sqlStr, args...)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// Update sets every non-auto field of rec (per opts) and requires a
// non-nil where (else gohan.ErrNoWhere, without touching the database).
func (r *Repository[T]) Update(ctx context.Context, rec *T, where gohan.Expr, opts ...gohan.RecordOption) (int64, error) {
	if where == nil {
		return 0, gohan.ErrNoWhere
	}
	st := gohan.Update(r.table).SetRecord(rec, opts...).Where(where)
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return 0, err
	}
	return r.q.Exec(ctx, sqlStr, args...)
}

// UpdateFields sets fields (whose keys must all be repository columns,
// else ErrUnknownColumn) and requires a non-nil where (else
// gohan.ErrNoWhere). Both guards are checked before any query.
func (r *Repository[T]) UpdateFields(ctx context.Context, fields map[string]any, where gohan.Expr) (int64, error) {
	if where == nil {
		return 0, gohan.ErrNoWhere
	}
	if err := r.checkColumns(mapKeys(fields)); err != nil {
		return 0, err
	}
	st := gohan.Update(r.table).SetMap(fields).Where(where)
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return 0, err
	}
	return r.q.Exec(ctx, sqlStr, args...)
}

// UpdateReturning is UpdateFields with "RETURNING <T's columns>": it
// returns every updated row as a new T, as the database left it (including
// values set by triggers), and an empty, non-nil slice when nothing
// matched. The same guards as UpdateFields apply before any query
// (ErrUnknownColumn, gohan.ErrNoWhere). Dialects without gohan's
// FeatureUpdate and FeatureReturning (ClickHouse) fail with
// gohan.ErrUnsupported.
func (r *Repository[T]) UpdateReturning(ctx context.Context, fields map[string]any, where gohan.Expr) ([]*T, error) {
	if where == nil {
		return nil, gohan.ErrNoWhere
	}
	if err := r.checkColumns(mapKeys(fields)); err != nil {
		return nil, err
	}
	st := gohan.Update(r.table).SetMap(fields).Where(where).Returning(r.returningCols()...)
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return nil, err
	}
	out := make([]*T, 0)
	if err := r.q.Select(ctx, &out, sqlStr, args...); err != nil {
		return nil, err
	}
	return out, nil
}

// Delete requires a non-nil where (else gohan.ErrNoWhere, without touching
// the database). A trivially-true where (e.g. gohan.And() or
// Col(x).NotIn()) is rejected downstream by gohan's own WHERE-clause check,
// not by this guard: there is deliberately no "delete all" method, so a
// caller who means it uses Exec(ctx, gohan.Delete(table).All()).
func (r *Repository[T]) Delete(ctx context.Context, where gohan.Expr) (int64, error) {
	if where == nil {
		return 0, gohan.ErrNoWhere
	}
	st := gohan.Delete(r.table).Where(where)
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return 0, err
	}
	return r.q.Exec(ctx, sqlStr, args...)
}

// Exec builds st against the repository's dialect and runs it, returning
// the number of rows affected.
func (r *Repository[T]) Exec(ctx context.Context, st gohan.Statement) (int64, error) {
	sqlStr, args, err := st.Build(r.q.Dialect())
	if err != nil {
		return 0, err
	}
	return r.q.Exec(ctx, sqlStr, args...)
}
