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
	DeleteCascade(qry *goqu.DeleteDataset) error
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

type Repository interface {
	Identifier
	Builder
	Reader
	Executor
	Writer
	Deleter
	Updater
	NewTransaction(opts *sql.TxOptions) (Transaction, error)
}

type Transaction interface {
	Builder
	Reader
	Executor
	Writer
	Deleter
	Updater

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
}

type tx struct {
	conn      *sqlx.Tx
	ctx       context.Context
	tableName string
	dialect   goqu.DialectWrapper
}

func NewRepository(ctx context.Context, conn *SqlClient, tableName string) Repository {
	return &repository{
		conn:      conn.Db(),
		ctx:       ctx,
		tableName: tableName,
		dialect:   goqu.Dialect(conn.DriverName),
	}
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
//	row:= &MyRecord{}
//	err := repo.FetchOne(repo.SqlSelect(), row)
func (r *repository) FetchOne(qry *goqu.SelectDataset, target any) error {
	return fetchOne(r.ctx, r.conn, qry, target)
}

// Fetch records; target must be a slice
// Example:
//
//	rows:= make([]*MyRecord,0)
//	err := repo.Fetch(repo.SqlSelect(), rows)
func (r *repository) Fetch(qry *goqu.SelectDataset, target any) error {
	return fetch(r.ctx, r.conn, qry, target)
}

// FetchRecord fetch a single record with WHERE clause; all clauses are AND
// Example:
//
//	row:= &MyRecord{}
//	err:= repo.FetchRecord(map[string]any{"name":"foo","email":"foo@bar"}, row)
func (r *repository) FetchRecord(fieldValues map[string]any, target any) error {
	return fetchRecord(r.ctx, r.conn, r.SqlSelect(), fieldValues, target)
}

// FetchByKey fetch a single record with WHERE keyField=value
func (r *repository) FetchByKey(keyField string, value any, target any) error {
	return fetchByKey(r.ctx, r.conn, r.SqlSelect(), keyField, value, target)
}

// FetchWhere fetch multiple records with WHERE clause
func (r *repository) FetchWhere(fieldValues map[string]any, target any) error {
	return fetchWhere(r.ctx, r.conn, r.SqlSelect(), fieldValues, target)
}

// Exec execute a query
func (r *repository) Exec(qry *goqu.SelectDataset) error {
	return exec(r.ctx, r.conn, qry)
}

// RawExec executes a raw sql query
func (r *repository) RawExec(sql string, args ...any) error {
	return rawExec(r.ctx, r.conn, sql, args...)
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
	return exists(r.ctx, r.conn, r.SqlSelect(), fieldName, fieldValue, skip...)
}

func (r *repository) Delete(qry *goqu.DeleteDataset) error {
	return del(r.ctx, r.conn, qry)
}

func (r *repository) DeleteCascade(qry *goqu.DeleteDataset) error {
	return delCascade(r.ctx, r.conn, qry)
}

func (r *repository) DeleteWhere(fieldNameValue map[string]any) error {
	return deleteWhere(r.ctx, r.conn, r.SqlDelete(), fieldNameValue)
}

func (r *repository) DeleteByKey(keyField string, value any) error {
	return deleteByKey(r.ctx, r.conn, r.SqlDelete(), keyField, value)
}

func (r *repository) Select(sql string, target any, args ...any) error {
	return r.conn.SelectContext(r.ctx, target, sql, args...)
}

func (r *repository) Insert(rows ...any) error {
	return insert(r.ctx, r.conn, r.SqlInsert(), rows...)
}

// InsertReturning inserts a record, and returns the specified return fields into target
//
// Example:
//
//	record := &SomeRecord{}
//	err := InsertReturning(record, []any{"id_table"}, &record.Id)
func (r *repository) InsertReturning(record any, returnFields []interface{}, target ...any) error {
	return insertReturning(r.ctx, r.conn, r.SqlInsert(), record, returnFields, target...)
}

// Update execute an update query
func (r *repository) Update(qry *goqu.UpdateDataset) error {
	return update(r.ctx, r.conn, qry)
}

// UpdateRecord updates a record using a WHERE condition with AND
// record can be a map[string]any specifying fields & values
func (r *repository) UpdateRecord(record any, whereFieldsValues map[string]any) error {
	return updateRecord(r.ctx, r.conn, r.SqlUpdate(), record, whereFieldsValues)
}

// UpdateByKey updates a record using WHERE keyField=value
func (r *repository) UpdateByKey(record any, keyField string, value any) error {
	return updateByKey(r.ctx, r.conn, r.SqlUpdate(), record, keyField, value)
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
	return fetchOne(t.ctx, t.conn, qry, target)
}

func (t *tx) Fetch(qry *goqu.SelectDataset, target any) error {
	return fetch(t.ctx, t.conn, qry, target)
}

// FetchRecord fetch a single record with WHERE clause
func (t *tx) FetchRecord(fieldValues map[string]any, target any) error {
	return fetchRecord(t.ctx, t.conn, t.SqlSelect(), fieldValues, target)
}

// FetchByKey fetch a single record with WHERE keyField=value
func (t *tx) FetchByKey(keyField string, value any, target any) error {
	return fetchByKey(t.ctx, t.conn, t.SqlSelect(), keyField, value, target)
}

// FetchWhere fetch multiple records with WHERE clause
func (t *tx) FetchWhere(fieldValues map[string]any, target any) error {
	return fetchWhere(t.ctx, t.conn, t.SqlSelect(), fieldValues, target)
}

// Exists returns true if one or more records exist WHERE fieldName=fieldValue
// the optional skip parameter can be used to specify an extra clause WHERE skip[0]<>skip[1]
func (t *tx) Exists(fieldName string, fieldValue any, skip ...any) (bool, error) {
	return exists(t.ctx, t.conn, t.SqlSelect(), fieldName, fieldValue, skip...)
}

func (t *tx) Exec(qry *goqu.SelectDataset) error {
	return exec(t.ctx, t.conn, qry)
}

func (t *tx) RawExec(sql string, args ...any) error {
	return rawExec(t.ctx, t.conn, sql, args...)
}

func (t *tx) Delete(qry *goqu.DeleteDataset) error {
	return del(t.ctx, t.conn, qry)
}

func (t *tx) DeleteCascade(qry *goqu.DeleteDataset) error {
	return delCascade(t.ctx, t.conn, qry)
}

func (t *tx) DeleteWhere(fieldNameValue map[string]any) error {
	return deleteWhere(t.ctx, t.conn, t.SqlDelete(), fieldNameValue)
}

func (t *tx) DeleteByKey(keyField string, value any) error {
	return deleteByKey(t.ctx, t.conn, t.SqlDelete(), keyField, value)
}

func (t *tx) Select(sql string, target any, args ...any) error {
	return t.conn.SelectContext(t.ctx, target, sql, args...)
}

func (t *tx) Insert(rows ...any) error {
	return insert(t.ctx, t.conn, t.SqlInsert(), rows...)
}

// InsertReturning inserts a record, and returns the specified return fields into target
func (t *tx) InsertReturning(record any, returnFields []interface{}, target ...any) error {
	return insertReturning(t.ctx, t.conn, t.SqlInsert(), record, returnFields, target...)
}

// Update execute an update query
func (t *tx) Update(qry *goqu.UpdateDataset) error {
	return update(t.ctx, t.conn, qry)
}

// UpdateRecord updates a record using a WHERE condition
// record can be a map[string]any specifying fields & values
func (t *tx) UpdateRecord(record any, whereFieldsValues map[string]any) error {
	return updateRecord(t.ctx, t.conn, t.SqlUpdate(), record, whereFieldsValues)
}

// UpdateByKey updates a record using WHERE keyField=value
func (t *tx) UpdateByKey(record any, keyField string, value any) error {
	return updateByKey(t.ctx, t.conn, t.SqlUpdate(), record, keyField, value)
}

// EmptyResult returns true if error is empty result
func EmptyResult(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

// Exists returns true if one or more records exist WHERE fieldName=fieldValue
// the optional skip parameter can be used to specify an extra clause WHERE skip[0]<>skip[1]
func (t *tx) exists(ctx context.Context, conn sqlx.QueryerContext, fieldName string, fieldValue any, skip ...any) (bool, error) {
	result := 0
	qry := t.SqlSelect().Select(goqu.L("COUNT(*)")).Where(goqu.C(fieldName).Eq(fieldValue))
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

	if err = conn.QueryRowxContext(ctx, qrySql, args...).Scan(&result); err != nil {
		return false, err
	}
	return result > 0, err
}

func rawExec(ctx context.Context, conn sqlx.ExecerContext, sql string, args ...any) error {
	_, err := conn.ExecContext(ctx, sql, args...)
	return err
}

func exec(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.SelectDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return rawExec(ctx, conn, sqlQry, args...)
}

func fetchOne(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, target any) error {
	if target == nil || qry == nil {
		return ErrInvalidParameters
	}
	qry.Limit(1)
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return conn.QueryRowxContext(ctx, sqlQry, args...).StructScan(target)
}

func fetch(ctx context.Context, conn SqlxReaderCtx, qry *goqu.SelectDataset, target any) error {
	if target == nil || qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return conn.SelectContext(ctx, target, sqlQry, args...)
}

func fetchRecord(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, fieldValues map[string]any, target any) error {
	if fieldValues == nil {
		return ErrInvalidParameters
	}
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return fetchOne(ctx, conn, qry, target)
}

func fetchByKey(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, keyField string, value any, target any) error {
	if target == nil {
		return ErrInvalidParameters
	}
	return fetchOne(ctx, conn, qry.Where(goqu.C(keyField).Eq(value)), target)
}

func fetchWhere(ctx context.Context, conn SqlxReaderCtx, qry *goqu.SelectDataset, fieldValues map[string]any, target any) error {
	if fieldValues == nil {
		return ErrInvalidParameters
	}
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return fetch(ctx, conn, qry, target)
}

func exists(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.SelectDataset, fieldName string, fieldValue any, skip ...any) (bool, error) {
	result := 0
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

	if err = conn.QueryRowxContext(ctx, qrySql, args...).Scan(&result); err != nil {
		return false, err
	}
	return result > 0, err
}

func del(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, sqlQry, args...)
	return err
}

func delCascade(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	sqlQry = sqlQry + " CASCADE"
	_, err = conn.ExecContext(ctx, sqlQry, args...)
	return err
}

func deleteWhere(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset, fieldNameValue map[string]any) error {
	if fieldNameValue == nil {
		return ErrInvalidParameters
	}
	for field, value := range fieldNameValue {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return del(ctx, conn, qry)
}

func deleteByKey(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.DeleteDataset, keyField string, value any) error {
	qry = qry.Where(goqu.C(keyField).Eq(value))
	return del(ctx, conn, qry)
}

func insert(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.InsertDataset, rows ...any) error {
	if len(rows) == 0 {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.Rows(rows...).Prepared(true).ToSQL()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, sqlQry, args...)
	return err
}

func insertReturning(ctx context.Context, conn sqlx.QueryerContext, qry *goqu.InsertDataset, record any, returnFields []interface{}, target ...any) error {
	if record == nil || returnFields == nil || len(target) == 0 {
		return ErrInvalidParameters
	}
	sqlQry, values, err := qry.Rows(record).Prepared(true).Returning(returnFields...).ToSQL()
	if err != nil {
		return err
	}
	return conn.QueryRowxContext(ctx, sqlQry, values...).Scan(target...)
}

func update(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.UpdateDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	qrySql, values, err := qry.Prepared(true).ToSQL()
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, qrySql, values...)
	return err
}

func updateRecord(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.UpdateDataset, record any, whereFieldsValues map[string]any) error {
	if record == nil {
		return ErrInvalidParameters
	}
	qry = qry.Set(record)
	if whereFieldsValues != nil {
		for field, value := range whereFieldsValues {
			qry = qry.Where(goqu.C(field).Eq(value))
		}
	}
	return update(ctx, conn, qry)
}

func updateByKey(ctx context.Context, conn sqlx.ExecerContext, qry *goqu.UpdateDataset, record any, keyField string, value any) error {
	if record == nil {
		return ErrInvalidParameters
	}
	return update(ctx, conn, qry.Set(record).Where(goqu.C(keyField).Eq(value)))
}
