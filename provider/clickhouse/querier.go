package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/oddbit-project/blueprint/dbx"
	"github.com/oddbit-project/blueprint/sqlb"
)

// Querier adapts a ClickHouse connection to dbx.Querier and
// dbx.BatchInserter. ClickHouse does not use database/sql, binds
// placeholders client-side, and cannot fill a slice of pointers through
// its own Select, so this adapter exists instead of dbx's generic
// database/sql-backed Querier.
//
// Querier is not transactional: it does not implement dbx.TxBeginner or
// dbx.TxQuerier.
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

// Dialect returns sqlb.ClickHouse().
func (q *Querier) Dialect() sqlb.Dialect {
	return sqlb.ClickHouse()
}

// Exec runs query and always returns 0: ClickHouse does not report the
// number of rows affected by a statement.
func (q *Querier) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	if err := q.conn.Exec(ctx, query, args...); err != nil {
		return 0, err
	}
	return 0, nil
}

// Get scans the first row of query into dest (a *T). sql.ErrNoRows passes
// through unwrapped when the query matches no row (== dbx.ErrNotFound).
func (q *Querier) Get(ctx context.Context, dest any, query string, args ...any) error {
	row := q.conn.QueryRow(ctx, query, args...)
	return row.ScanStruct(dest)
}

// Select scans every row of query into dest, a *[]T or *[]*T. dest is
// validated before any query is sent, and is reset to length 0 first.
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
		return q.conn.Select(ctx, dest, query, args...)
	}
	if elemType.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("clickhouse: Select requires []T or []*T with T a struct, got %T", dest)
	}

	rows, err := q.conn.Query(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

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
	row := q.conn.QueryRow(ctx, query, args...)
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
// would fail with "missing destination name". Using the first row's
// columns also means rows whose SkipZeroValues-omitted fields differ from
// the first row still send those columns, with their zero value.
//
// The driver's batch column-list parser strips quotes with a regex
// without un-escaping, so a column name containing `"`, `\`, `,` or a
// space cannot be used here; InsertBatch rejects such columns before
// preparing the batch.
func (q *Querier) InsertBatch(ctx context.Context, table string, rows []any) error {
	if len(rows) == 0 {
		return nil
	}
	tbl, err := sqlb.ClickHouse().QuoteIdent(table)
	if err != nil {
		return err
	}
	cols, err := sqlb.InsertColumns(rows[0])
	if err != nil {
		return err
	}
	quoted := make([]string, len(cols))
	for i, col := range cols {
		if strings.ContainsAny(col, "\"\\, ") {
			return fmt.Errorf("clickhouse: column %q cannot be used in a batch column list", col)
		}
		qc, err := sqlb.ClickHouse().QuoteIdent(col)
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
			batch.Abort()
			return err
		}
	}
	return batch.Send()
}
