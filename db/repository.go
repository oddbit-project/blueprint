package db

import (
	"context"
	"database/sql"
	"errors"
	"github.com/doug-martin/goqu/v9"
	"github.com/jackc/pgx/v5"
	"github.com/jmoiron/sqlx"
)

type Builder interface {
	Sql() goqu.DialectWrapper
	SqlSelect() *goqu.SelectDataset
	SqlInsert() *goqu.InsertDataset
	SqlUpdate() *goqu.UpdateDataset
	SqlDelete() *goqu.DeleteDataset
}

type Reader interface {
	FetchOne(qry *goqu.SelectDataset, target any) error
	FetchRecord(fieldValues map[string]any, target any) error
	Fetch(qry *goqu.SelectDataset, target any) error
	FetchWhere(fieldValues map[string]any, target any) error
	FetchByKey(keyField string, value any, target any) error
	Exists(fieldName string, fieldValue any, skip ...any) (bool, error)
}

type Counter interface {
	Count() (int64, error)
	CountWhere(fieldValues map[string]any) (int64, error)
}

type Executor interface {
	Exec(qry *goqu.SelectDataset) error
	RawExec(sql string, args ...any) error
	Select(sql string, target any, args ...any) error
}

type Writer interface {
	Insert(records ...any) error
	InsertReturning(record any, returnFields []interface{}, target ...any) error
}
type Updater interface {
	Update(qry *goqu.UpdateDataset) error
	UpdateRecord(record any, whereFieldsValues map[string]any) error
	UpdateByKey(record any, keyField string, keyValue any) error
}

type Deleter interface {
	Delete(qry *goqu.DeleteDataset) error
	DeleteWhere(fieldNameValue map[string]any) error
	DeleteByKey(keyField string, value any) error
}

type Identifier interface {
	Db() *sqlx.DB
	Name() string
}

type SqlxReaderCtx interface {
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
}

type GridOps interface {
	Grid(record any) (*Grid, error)
	QueryGrid(record any, args *GridQuery, dest any) error
}

type Repository interface {
	Identifier
	Builder
	Reader
	Executor
	Writer
	Deleter
	Updater
	Counter
	GridOps
	NewTransaction(opts *sql.TxOptions) (Transaction, error)
}

type Transaction interface {
	Builder
	Reader
	Executor
	Writer
	Deleter
	Updater
	Counter

	Db() *sqlx.Tx
	Name() string

	Commit() error
	Rollback() error
}

type FV map[string]any // alias for fieldValues maps

type repository struct {
	conn      *sqlx.DB
	ctx       context.Context
	tableName string
	dialect   goqu.DialectWrapper
	spec      *FieldSpec
}

type tx struct {
	conn      *sqlx.Tx
	ctx       context.Context
	tableName string
	dialect   goqu.DialectWrapper
}

func (r *repository) NewTransaction(opts *sql.TxOptions) (Transaction, error) {
	t, err := r.Db().BeginTxx(r.ctx, opts)
	if err != nil {
		return nil, err
	}
	return &tx{
		conn:      t,
		ctx:       r.ctx,
		tableName: r.tableName,
		dialect:   r.dialect,
	}, nil
}

func (r *repository) Db() *sqlx.DB {
	return r.conn
}

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
// Example:
//
//		wallets[i] = record
//	row:= &MyRecord{}
//	err := repo.FetchOne(repo.SqlSelect(), row)
func (r *repository) FetchOne(qry *goqu.SelectDataset, target any) error {
	return FetchOne(r.ctx, r.conn, qry, target)
}

// Fetch records; target must be a slice
// Example:
//
//	rows:= make([]*MyRecord,0)
//	err := repo.Fetch(repo.SqlSelect(), rows)
func (r *repository) Fetch(qry *goqu.SelectDataset, target any) error {
	return Fetch(r.ctx, r.conn, qry, target)
}

// FetchRecord fetch a single record with WHERE clause; all clauses are AND
// Example:
//
//	row:= &MyRecord{}
//	err:= repo.FetchRecord(map[string]any{"name":"foo","email":"foo@bar"}, row)
func (r *repository) FetchRecord(fieldValues map[string]any, target any) error {
	return FetchRecord(r.ctx, r.conn, r.SqlSelect(), fieldValues, target)
}

// FetchByKey fetch a single record with WHERE keyField=value
func (r *repository) FetchByKey(keyField string, value any, target any) error {
	return FetchByKey(r.ctx, r.conn, r.SqlSelect(), keyField, value, target)
}

// FetchWhere fetch multiple records with WHERE clause
func (r *repository) FetchWhere(fieldValues map[string]any, target any) error {
	return FetchWhere(r.ctx, r.conn, r.SqlSelect(), fieldValues, target)
}

// Exec execute a query
func (r *repository) Exec(qry *goqu.SelectDataset) error {
	return Exec(r.ctx, r.conn, qry)
}

// RawExec executes a raw sql query
func (r *repository) RawExec(sql string, args ...any) error {
	return RawExec(r.ctx, r.conn, sql, args...)
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
	return Exists(r.ctx, r.conn, r.SqlSelect(), fieldName, fieldValue, skip...)
}

func (r *repository) Delete(qry *goqu.DeleteDataset) error {
	return Del(r.ctx, r.conn, qry)
}

func (r *repository) DeleteWhere(fieldNameValue map[string]any) error {
	return DeleteWhere(r.ctx, r.conn, r.SqlDelete(), fieldNameValue)
}

func (r *repository) DeleteByKey(keyField string, value any) error {
	return DeleteByKey(r.ctx, r.conn, r.SqlDelete(), keyField, value)
}

func (r *repository) Select(sql string, target any, args ...any) error {
	return r.conn.SelectContext(r.ctx, target, sql, args...)
}

func (r *repository) Insert(rows ...any) error {
	return Insert(r.ctx, r.conn, r.SqlInsert(), rows...)
}

// Count returns the total number of rows in the database table
func (r *repository) Count() (int64, error) {
	return Count(r.ctx, r.conn, r.SqlSelect().Select(goqu.L("COUNT(*)")))
}

// CountWhere returns the number of rows matching the fieldValues map
func (r *repository) CountWhere(fieldValues map[string]any) (int64, error) {
	qry := r.SqlSelect().Select(goqu.L("COUNT(*)"))
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return Count(r.ctx, r.conn, qry)
}

// Grid creates a new Grid object for the record, and caches the field spec for more efficient usage
func (r *repository) Grid(record any) (*Grid, error) {
	if r.spec == nil {
		var err error
		if r.spec, err = NewFieldSpec(record); err != nil {
			return nil, err
		}
	}
	return NewGridWithSpec(r.tableName, r.spec), nil
}

// QueryGrid creates a new Grid object and performs a query using GridQuery
func (r *repository) QueryGrid(record any, args *GridQuery, dest any) error {
	var (
		err  error
		qry  *goqu.SelectDataset
		grid *Grid
	)

	grid, err = r.Grid(record)
	if err != nil {
		return err
	}

	qry, err = grid.Build(r.SqlSelect(), args)
	if err != nil {
		return err
	}

	return Fetch(r.ctx, r.conn, qry, dest)
}

// InsertReturning inserts a record, and returns the specified return fields into target
//
// Example:
//
//	record := &SomeRecord{}
//	err := InsertReturning(record, []any{"id_table"}, &record.Id)
func (r *repository) InsertReturning(record any, returnFields []interface{}, target ...any) error {
	return InsertReturning(r.ctx, r.conn, r.SqlInsert(), record, returnFields, target...)
}

// Update execute an update query
func (r *repository) Update(qry *goqu.UpdateDataset) error {
	return Update(r.ctx, r.conn, qry)
}

// UpdateRecord updates a record using a WHERE condition with AND
// record can be a map[string]any specifying fields & values
func (r *repository) UpdateRecord(record any, whereFieldsValues map[string]any) error {
	return UpdateRecord(r.ctx, r.conn, r.SqlUpdate(), record, whereFieldsValues)
}

// UpdateByKey updates a record using WHERE keyField=value
func (r *repository) UpdateByKey(record any, keyField string, value any) error {
	return UpdateByKey(r.ctx, r.conn, r.SqlUpdate(), record, keyField, value)
}

func (t *tx) Commit() error {
	return t.conn.Commit()
}

func (t *tx) Rollback() error {
	return t.conn.Rollback()
}

func (t *tx) Db() *sqlx.Tx {
	return t.conn
}

func (t *tx) Name() string {
	return t.tableName
}

func (t *tx) Sql() goqu.DialectWrapper {
	return t.dialect
}

func (t *tx) SqlSelect() *goqu.SelectDataset {
	return t.dialect.From(t.tableName)
}

func (t *tx) SqlInsert() *goqu.InsertDataset {
	return t.dialect.Insert(t.tableName)
}

func (t *tx) SqlUpdate() *goqu.UpdateDataset {
	return t.dialect.Update(t.tableName)
}

func (t *tx) SqlDelete() *goqu.DeleteDataset {
	return t.dialect.Delete(t.tableName)
}

// FetchOne fetch a record; target must be a struct
func (t *tx) FetchOne(qry *goqu.SelectDataset, target any) error {
	return FetchOne(t.ctx, t.conn, qry, target)
}

func (t *tx) Fetch(qry *goqu.SelectDataset, target any) error {
	return Fetch(t.ctx, t.conn, qry, target)
}

// FetchRecord fetch a single record with WHERE clause
func (t *tx) FetchRecord(fieldValues map[string]any, target any) error {
	return FetchRecord(t.ctx, t.conn, t.SqlSelect(), fieldValues, target)
}

// FetchByKey fetch a single record with WHERE keyField=value
func (t *tx) FetchByKey(keyField string, value any, target any) error {
	return FetchByKey(t.ctx, t.conn, t.SqlSelect(), keyField, value, target)
}

// FetchWhere fetch multiple records with WHERE clause
func (t *tx) FetchWhere(fieldValues map[string]any, target any) error {
	return FetchWhere(t.ctx, t.conn, t.SqlSelect(), fieldValues, target)
}

// Exists returns true if one or more records exist WHERE fieldName=fieldValue
// the optional skip parameter can be used to specify an extra clause WHERE skip[0]<>skip[1]
func (t *tx) Exists(fieldName string, fieldValue any, skip ...any) (bool, error) {
	return Exists(t.ctx, t.conn, t.SqlSelect(), fieldName, fieldValue, skip...)
}

func (t *tx) Exec(qry *goqu.SelectDataset) error {
	return Exec(t.ctx, t.conn, qry)
}

func (t *tx) RawExec(sql string, args ...any) error {
	return RawExec(t.ctx, t.conn, sql, args...)
}

func (t *tx) Delete(qry *goqu.DeleteDataset) error {
	return Del(t.ctx, t.conn, qry)
}

func (t *tx) DeleteWhere(fieldNameValue map[string]any) error {
	return DeleteWhere(t.ctx, t.conn, t.SqlDelete(), fieldNameValue)
}

func (t *tx) DeleteByKey(keyField string, value any) error {
	return DeleteByKey(t.ctx, t.conn, t.SqlDelete(), keyField, value)
}

func (t *tx) Select(sql string, target any, args ...any) error {
	return t.conn.SelectContext(t.ctx, target, sql, args...)
}

func (t *tx) Insert(rows ...any) error {
	return Insert(t.ctx, t.conn, t.SqlInsert(), rows...)
}

// InsertReturning inserts a record, and returns the specified return fields into target
func (t *tx) InsertReturning(record any, returnFields []interface{}, target ...any) error {
	return InsertReturning(t.ctx, t.conn, t.SqlInsert(), record, returnFields, target...)
}

// Update execute an update query
func (t *tx) Update(qry *goqu.UpdateDataset) error {
	return Update(t.ctx, t.conn, qry)
}

// UpdateRecord updates a record using a WHERE condition
// record can be a map[string]any specifying fields & values
func (t *tx) UpdateRecord(record any, whereFieldsValues map[string]any) error {
	return UpdateRecord(t.ctx, t.conn, t.SqlUpdate(), record, whereFieldsValues)
}

// UpdateByKey updates a record using WHERE keyField=value
func (t *tx) UpdateByKey(record any, keyField string, value any) error {
	return UpdateByKey(t.ctx, t.conn, t.SqlUpdate(), record, keyField, value)
}

// Count returns the total number of rows in the database table
func (t *tx) Count() (int64, error) {
	return Count(t.ctx, t.conn, t.SqlSelect().Select(goqu.L("COUNT(*)")))
}

// CountWhere returns the number of rows matching the fieldValues map
func (t *tx) CountWhere(fieldValues map[string]any) (int64, error) {
	qry := t.SqlSelect().Select(goqu.L("COUNT(*)"))
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return Count(t.ctx, t.conn, qry)
}

// EmptyResult returns true if error is empty result
func EmptyResult(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}
