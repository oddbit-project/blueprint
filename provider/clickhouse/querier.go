package clickhouse

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"math"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/gohan"
)

// Querier adapts a ClickHouse connection to dbx.Querier and
// dbx.BatchInserter. ClickHouse does not use database/sql, binds
// placeholders client-side, and cannot fill a slice of pointers through
// its own Select, so this adapter exists instead of dbx's generic
// database/sql-backed Querier.
//
// Querier is not transactional: it does not implement dbx.TxBeginner or
// dbx.TxQuerier.
//
// Statements built through Dialect() use named "@pN" placeholders
// (gohan.ClickHouseNamed); time.Time arguments (bare, *time.Time, or a
// driver.Valuer producing one) keep full precision and compare by exact
// instant, rather than clickhouse-go's native whole-second binding.
// Callers passing their own SQL with positional "?" placeholders keep
// clickhouse-go's whole-second time binding; use "@name" with
// clickhouse.DateNamed instead if full precision is needed there.
type Querier struct {
	conn clickhouse.Conn
}

var (
	_ dbx.Querier       = (*Querier)(nil)
	_ dbx.BatchInserter = (*Querier)(nil)
)

// NewQuerier returns a Querier over conn.
func NewQuerier(conn clickhouse.Conn) *Querier {
	return &Querier{conn: conn}
}

// Querier returns a dbx Querier/BatchInserter bound to c's connection.
func (c *Client) Querier() *Querier {
	return NewQuerier(c.Conn)
}

// Dialect returns gohan.ClickHouseNamed(): statements are built with
// "@pN" placeholders bound by name, so namedArgs can convert time.Time
// arguments to clickhouse.DateNamed and keep full precision.
func (q *Querier) Dialect() gohan.Dialect {
	return gohan.ClickHouseNamed()
}

// dateNamed binds t as a DateTime64 literal at the smallest scale that holds
// t exactly, but never below milliseconds: a DateTime64(9) literal overflows
// after 2262 and a DateTime one after 2106, while DateTime64(3) covers
// 1900-2299. t keeps its location (so Date comparisons and date functions see
// the same calendar day as before), except where the driver's rendering fails,
// in which case the same instant is bound in UTC: time.Local (clickhouse-go
// renders it as a numeric string that ClickHouse misreads or rejects for
// almost every date) and a location ClickHouse cannot load by name (such as a
// time.FixedZone).
func dateNamed(name string, t time.Time) any {
	scale := clickhouse.MilliSeconds
	switch ns := t.Nanosecond(); {
	case ns%int(time.Microsecond) != 0:
		scale = clickhouse.NanoSeconds
	case ns%int(time.Millisecond) != 0:
		scale = clickhouse.MicroSeconds
	}
	if t.Location() == time.Local || !namedLocation(t.Location()) {
		t = t.UTC()
	}
	return clickhouse.DateNamed(name, t, scale)
}

// knownZones caches namedLocation's time.LoadLocation lookups by zone name.
var knownZones sync.Map // map[string]bool

// namedLocation reports whether loc is UTC or an IANA zone that can be
// loaded by its name (which clickhouse-go sends to the server as is).
func namedLocation(loc *time.Location) bool {
	name := loc.String()
	if name == "UTC" {
		return true
	}
	if ok, found := knownZones.Load(name); found {
		return ok.(bool)
	}
	_, err := time.LoadLocation(name)
	knownZones.Store(name, err == nil)
	return err == nil
}

// namedArgs converts each gohan-built sql.NamedArg in args to the native
// clickhouse-go binding: a time.Time value (bare, via *time.Time, or via a
// non-nil-pointer driver.Valuer producing one) becomes dateNamed(name, t),
// which the driver renders as a toDateTime64 literal that keeps the instant
// exactly; a nil
// *time.Time becomes clickhouse.Named(name, nil); anything else becomes
// clickhouse.Named(name, value). Arguments that are not sql.NamedArg
// (hand-written SQL with positional "?") pass through unchanged. args is
// never mutated; a new slice is returned.
func namedArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		na, ok := a.(sql.NamedArg)
		if !ok {
			out[i] = a
			continue
		}
		switch v := na.Value.(type) {
		case time.Time:
			out[i] = dateNamed(na.Name, v)
		case *time.Time:
			if v == nil {
				out[i] = clickhouse.Named(na.Name, nil)
			} else {
				out[i] = dateNamed(na.Name, *v)
			}
		default:
			if t, ok := asValuerTime(na.Value); ok {
				out[i] = dateNamed(na.Name, t)
			} else {
				out[i] = clickhouse.Named(na.Name, na.Value)
			}
		}
	}
	return out
}

// asValuerTime reports whether value is a non-nil pointer implementing
// driver.Valuer whose Value() returns a time.Time with no error (for
// example a valid sql.NullTime).
func asValuerTime(value any) (time.Time, bool) {
	rv := reflect.ValueOf(value)
	if !rv.IsValid() || (rv.Kind() == reflect.Pointer && rv.IsNil()) {
		return time.Time{}, false
	}
	valuer, ok := value.(driver.Valuer)
	if !ok {
		return time.Time{}, false
	}
	v, err := valuer.Value()
	if err != nil {
		return time.Time{}, false
	}
	t, ok := v.(time.Time)
	return t, ok
}

// Exec runs query and always returns 0: ClickHouse does not report the
// number of rows affected by a statement.
func (q *Querier) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	if err := q.conn.Exec(ctx, query, namedArgs(args)...); err != nil {
		return 0, err
	}
	return 0, nil
}

// Get scans the first row of query into dest (a *T). sql.ErrNoRows passes
// through unwrapped when the query matches no row (== dbx.ErrNotFound).
//
// Scanning goes through clickhouse-go's ScanStruct, which maps result
// columns to dest's fields by the `ch` struct tag (falling back to the Go
// field name), not the `db` tag gohan/dbx build column lists from. A record
// type used with this Querier needs `ch` tags equal to its `db` tags (or
// only `ch` tags), or the mapping silently diverges; dbx.NewRepository
// rejects such a type through CheckRecord.
func (q *Querier) Get(ctx context.Context, dest any, query string, args ...any) error {
	row := q.conn.QueryRow(ctx, query, namedArgs(args)...)
	return row.ScanStruct(dest)
}

// Select scans every row of query into dest, a *[]T or *[]*T. dest is
// validated before any query is sent, and is reset to length 0 first.
// See Get's doc comment for the `ch`/`db` tag requirement ScanStruct
// imposes.
func (q *Querier) Select(ctx context.Context, dest any, query string, args ...any) error {
	dv := reflect.ValueOf(dest)
	if dv.Kind() != reflect.Pointer || dv.IsNil() {
		return fmt.Errorf("clickhouse: Select requires a non-nil pointer to a slice, got %T", dest)
	}
	sv := dv.Elem()
	if sv.Kind() != reflect.Slice {
		return fmt.Errorf("clickhouse: Select requires a pointer to a slice, got %T", dest)
	}
	sv.Set(reflect.MakeSlice(sv.Type(), 0, 0))

	elemType := sv.Type().Elem()
	if elemType.Kind() != reflect.Pointer {
		// Element T (struct): delegate to conn.Select.
		return q.conn.Select(ctx, dest, query, namedArgs(args)...)
	}
	if elemType.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("clickhouse: Select requires []T or []*T with T a struct, got %T", dest)
	}

	rows, err := q.conn.Query(ctx, query, namedArgs(args)...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		p := reflect.New(elemType.Elem())
		if err := rows.ScanStruct(p.Interface()); err != nil {
			return err
		}
		sv.Set(reflect.Append(sv, p))
	}
	return rows.Err()
}

// QueryInt64 scans the first column of the first row of query. ClickHouse
// COUNT() scans only into a uint64; a value exceeding math.MaxInt64 is
// reported as dbx.ErrCountOverflow.
func (q *Querier) QueryInt64(ctx context.Context, query string, args ...any) (int64, error) {
	var n uint64
	row := q.conn.QueryRow(ctx, query, namedArgs(args)...)
	if err := row.Scan(&n); err != nil {
		if err == sql.ErrNoRows {
			return 0, dbx.ErrNotFound
		}
		return 0, err
	}
	if n > math.MaxInt64 {
		return 0, dbx.ErrCountOverflow
	}
	return int64(n), nil
}

// InsertBatch inserts rows (each a *T) into table in one batch, using the
// column list from rows[0]'s InsertColumns rather than an unqualified
// INSERT: an unqualified "INSERT INTO t" batch expects every
// non-MATERIALIZED/ALIAS column of t, so a DEFAULT column absent from T
// would fail with "missing destination name". Every row's InsertColumns
// must match row 0's exactly (same columns, same order); a row whose
// omitnil/omitempty-tagged fields are omitted or present differently from
// row 0 fails with gohan.ErrInconsistentOmit, since the batch's column list
// is fixed by row 0 and cannot vary per row.
//
// The driver's batch column-list parser strips quotes with a regex
// without un-escaping, so a column name containing `"`, `\`, `(`, `)`, a
// comma or whitespace cannot be used here; InsertBatch rejects such
// columns before preparing the batch. A comma cannot actually reach this
// check: struct tags are split on commas (runtime/tags.go), so a `ch:"a,b"`
// tag yields the column name "a", never "a,b".
//
// The column list itself comes from each row's `db`/field tags via
// gohan.InsertColumns, but AppendStruct (clickhouse-go) writes each column
// by matching the row's `ch` tag (falling back to the Go field name). A
// record type used here needs `ch` tags equal to its `db` tags (or only
// `ch` tags), or a column in the list has no matching struct field to read;
// dbx.NewRepository rejects such a type through CheckRecord.
func (q *Querier) InsertBatch(ctx context.Context, table string, rows []any) error {
	if len(rows) == 0 {
		return nil
	}
	tbl, err := gohan.ClickHouse().QuoteIdent(table)
	if err != nil {
		return err
	}
	cols, err := gohan.InsertColumns(rows[0])
	if err != nil {
		return err
	}
	for i := 1; i < len(rows); i++ {
		rowCols, err := gohan.InsertColumns(rows[i])
		if err != nil {
			return err
		}
		if !equalColumns(cols, rowCols) {
			return fmt.Errorf("%w: row %d", gohan.ErrInconsistentOmit, i)
		}
	}
	quoted := make([]string, len(cols))
	for i, col := range cols {
		if strings.ContainsAny(col, "\"\\(),") || strings.ContainsFunc(col, unicode.IsSpace) {
			return fmt.Errorf("clickhouse: column %q cannot be used in a batch column list", col)
		}
		qc, err := gohan.ClickHouse().QuoteIdent(col)
		if err != nil {
			return err
		}
		quoted[i] = qc
	}

	batch, err := q.conn.PrepareBatch(ctx, "INSERT INTO "+tbl+" ("+strings.Join(quoted, ", ")+")")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := batch.AppendStruct(row); err != nil {
			_ = batch.Abort()
			return err
		}
	}
	return batch.Send()
}

// equalColumns reports whether a and b have the same column names in the
// same order.
func equalColumns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
