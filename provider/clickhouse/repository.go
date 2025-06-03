package clickhouse

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
	"github.com/oddbit-project/blueprint/db"
	"github.com/oddbit-project/blueprint/utils"
	"strings"
)

const (
	ErrNotSupported      = utils.Error("method not supported")
	ErrInvalidParameters = utils.Error("invalid parameters")

	ErrInsertReturningNotSupported = utils.Error("INSERT...RETURNING is not supported by ClickHouse")
	ErrDeleteNotSupported          = utils.Error("DELETE is not supported by ClickHouse")
)

type Repository interface {
	db.Identifier
	db.Builder
	db.Reader
	db.Executor
	db.Writer
	db.Deleter
	db.Counter
	db.GridOps
	Conn() clickhouse.Conn
}

type repository struct {
	conn      clickhouse.Conn
	ctx       context.Context
	tableName string
	mapper    *structMap
	dialect   goqu.DialectWrapper
	spec      *db.FieldSpec
}

func NewRepository(ctx context.Context, conn clickhouse.Conn, tableName string) Repository {
	return &repository{
		conn:      conn,
		ctx:       ctx,
		tableName: tableName,
		mapper:    &structMap{},
		dialect:   goqu.Dialect("clickhouse"),
		spec:      nil,
	}
}

// Db not supported
func (r *repository) Db() *sqlx.DB {
	return nil
}

// Conn clickhouse conn
func (r *repository) Conn() clickhouse.Conn {
	return r.conn
}

// Name table name
func (r *repository) Name() string {
	return r.tableName
}

func (r *repository) Sql() goqu.DialectWrapper {
	return r.dialect
}

// SqlSelect returns a Select query builder
func (r *repository) SqlSelect() *goqu.SelectDataset {
	return r.dialect.From(r.tableName)
}

// SqlInsert returns an Insert query builder
func (r *repository) SqlInsert() *goqu.InsertDataset {
	return r.dialect.Insert(r.tableName)
}

// SqlUpdate returns an Update query builder
func (r *repository) SqlUpdate() *goqu.UpdateDataset {
	return r.dialect.Update(r.tableName)
}

// SqlDelete returns a Delete query builder
func (r *repository) SqlDelete() *goqu.DeleteDataset {
	return r.dialect.Delete(r.tableName)
}

// FetchOne fetch a record; target must be a struct
// returns sql.ErrNoRows if nothing read
// Example:
//
//		wallets[i] = record
//	row:= &MyRecord{}
//	err := repo.FetchOne(repo.SqlSelect(), row)
func (r *repository) FetchOne(qry *goqu.SelectDataset, target any) error {
	if target == nil || qry == nil {
		return ErrInvalidParameters
	}
	qry.Limit(1)
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return r.conn.QueryRow(r.ctx, sqlQry, args...).ScanStruct(target)
}

// Fetch records; target must be a slice
// Example:
//
//	rows:= make([]*MyRecord,0)
//	err := repo.Fetch(repo.SqlSelect(), rows)
func (r *repository) Fetch(qry *goqu.SelectDataset, target any) error {
	if target == nil || qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return r.conn.Select(r.ctx, target, sqlQry, args...)
}

// FetchRecord fetch a single record with WHERE clause; all clauses are AND
// Example:
//
//	row:= &MyRecord{}
//	err:= repo.FetchRecord(map[string]any{"name":"foo","email":"foo@bar"}, row)
func (r *repository) FetchRecord(fieldValues map[string]any, target any) error {
	if fieldValues == nil {
		return ErrInvalidParameters
	}
	qry := r.SqlSelect()
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return r.FetchOne(qry, target)
}

// FetchByKey fetch a single record with WHERE keyField=value
func (r *repository) FetchByKey(keyField string, value any, target any) error {
	if target == nil {
		return ErrInvalidParameters
	}
	qry := r.SqlSelect().Where(goqu.C(keyField).Eq(value))
	return r.FetchOne(qry, target)
}

// FetchWhere fetch multiple records with WHERE clause
func (r *repository) FetchWhere(fieldValues map[string]any, target any) error {
	if fieldValues == nil {
		return ErrInvalidParameters
	}
	qry := r.SqlSelect()
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return r.Fetch(qry, target)
}

// Exec execute a query
func (r *repository) Exec(qry *goqu.SelectDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return r.conn.Exec(r.ctx, sqlQry, args...)
}

// RawExec executes a raw sql query
func (r *repository) RawExec(sql string, args ...any) error {
	return r.conn.Exec(r.ctx, sql, args...)
}

// Exists returns true if one or more records exist WHERE fieldName=fieldValue
// the optional skip parameter can be used to specify an extra clause WHERE skip[0]<>skip[1]
// Example:
//
//	// check if a record with label == "record 4"
//	exists, err := repo.Exists("label", "record 4")
//	// check if a record with label == "record 4" and id_sample_table<>4 exists
//	exists, err := repo.Exists("label", "record 4", "id_sample_table", 4)
func (r *repository) Exists(fieldName string, fieldValue any, skip ...any) (bool, error) {
	var result int
	qry := r.SqlSelect()
	qry = qry.Select(goqu.L("COUNT(*)")).Where(goqu.C(fieldName).Eq(fieldValue))
	if len(skip) > 0 {
		if len(skip) != 2 {
			return false, ErrInvalidParameters
		}
		qry = qry.Where(goqu.C(skip[0].(string)).Neq(skip[1]))
	}
	qrySql, args, err := qry.ToSQL()
	if err != nil {
		return false, err
	}

	row := r.conn.QueryRow(r.ctx, qrySql, args...)
	if err = row.Scan(&result); err != nil {
		return false, err
	}
	return result > 0, err
}

// Delete performs a delete operation
// Note:check limitations in https://clickhouse.com/docs/sql-reference/statements/delete
func (r *repository) Delete(qry *goqu.DeleteDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return r.conn.Exec(r.ctx, sqlQry, args...)
}

// DeleteWhere performs a conditional delete operation
// Note:check limitations in https://clickhouse.com/docs/sql-reference/statements/delete
func (r *repository) DeleteWhere(fieldNameValue map[string]any) error {
	if fieldNameValue == nil {
		return ErrInvalidParameters
	}
	qry := r.SqlDelete()
	for field, value := range fieldNameValue {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return r.conn.Exec(r.ctx, sqlQry, args...)
}

// DeleteByKey performs a delete operation
// Note:check limitations in https://clickhouse.com/docs/sql-reference/statements/delete
func (r *repository) DeleteByKey(keyField string, value any) error {
	qry := r.SqlDelete().Where(goqu.C(keyField).Eq(value))
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}

	return r.conn.Exec(r.ctx, sqlQry, args...)
}

func (r *repository) Select(sql string, target any, args ...any) error {
	return r.conn.Select(r.ctx, target, sql, args...)
}

// Insert inserts a collection of rows
// Note: inserts are always batched
func (r *repository) Insert(rows ...any) error {

	batch, err := r.conn.PrepareBatch(r.ctx, fmt.Sprintf("INSERT INTO %s", r.tableName))
	if err != nil {
		return err
	}
	for _, r := range rows {
		if err := batch.AppendStruct(r); err != nil {
			batch.Abort()
			return err
		}
	}
	return batch.Send()
}

// InsertAsync inserts a single record async
func (r *repository) InsertAsync(record any) error {
	cols, values, err := r.mapper.Map("InsertAsync", record, false)
	if err != nil {
		return err
	}
	// create string
	var qry strings.Builder
	qry.WriteString("INSERT INTO ")
	qry.WriteString(r.tableName)
	qry.WriteString(" VALUES(")
	qry.WriteString(strings.Join(cols, ","))
	qry.WriteString(")")

	return r.conn.AsyncInsert(r.ctx, qry.String(), false, values...)
}

// Count returns the total number of rows in the database table
func (r *repository) Count() (int64, error) {
	qry := r.SqlSelect().Select(goqu.L("COUNT(*)"))
	sqlQry, values, err := qry.ToSQL()
	if err != nil {
		return 0, err
	}
	var count int64
	if err = r.conn.QueryRow(r.ctx, sqlQry, values...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// CountWhere returns the number of rows matching the fieldValues map
func (r *repository) CountWhere(fieldValues map[string]any) (int64, error) {
	qry := r.SqlSelect().Select(goqu.L("COUNT(*)"))
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	sqlQry, values, err := qry.ToSQL()
	if err != nil {
		return 0, err
	}
	var count int64
	if err = r.conn.QueryRow(r.ctx, sqlQry, values...).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// InsertReturning is not supported
func (r *repository) InsertReturning(record any, returnFields []interface{}, target ...any) error {
	return ErrNotSupported
}

// Grid creates a new Grid object for the record, and caches the field spec for more efficient usage
func (r *repository) Grid(record any) (*db.Grid, error) {
	if r.spec == nil {
		var err error
		if r.spec, err = db.NewFieldSpec(record); err != nil {
			return nil, err
		}
	}
	return db.NewGridWithSpec(r.tableName, r.spec), nil
}

// QueryGrid creates a new Grid object and performs a query using GridQuery
func (r *repository) QueryGrid(record any, args db.GridQuery, dest any) error {
	var (
		err    error
		qry    *goqu.SelectDataset
		grid   *db.Grid
		sql    string
		values []interface{}
	)

	grid, err = r.Grid(record)
	if err != nil {
		return err
	}

	qry, err = grid.Build(r.SqlSelect(), args)
	if err != nil {
		return err
	}
	sql, values, err = qry.ToSQL()
	if err != nil {
		return err
	}
	return r.conn.Select(r.ctx, dest, sql, values...)
}
