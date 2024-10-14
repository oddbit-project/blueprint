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

func (r *repository) SqlSelect() *goqu.SelectDataset {
	return r.dialect.From(r.tableName)
}

func (r *repository) SqlInsert() *goqu.InsertDataset {
	return r.dialect.Insert(r.tableName)
}

func (r *repository) SqlUpdate() *goqu.UpdateDataset {
	return r.dialect.Update(r.tableName)
}

func (r *repository) SqlDelete() *goqu.DeleteDataset {
	return r.dialect.Delete(r.tableName)
}

// FetchOne fetch a record; target must be a struct
func (r *repository) FetchOne(qry *goqu.SelectDataset, target any) error {
	if target == nil || qry == nil {
		return ErrInvalidParameters
	}
	return fetchOne(r.ctx, r.conn, qry, target)
}

func fetchOne(ctx context.Context, conn *sqlx.DB, qry *goqu.SelectDataset, target any) error {
	qry.Limit(1)
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return conn.QueryRowxContext(ctx, sqlQry, args...).StructScan(target)
}

func (r *repository) Fetch(qry *goqu.SelectDataset, target any) error {
	if target == nil {
		return ErrInvalidParameters
	}
	return fetch(r.ctx, r.conn, qry, target)
}

// FetchRecord fetch a single record with WHERE clause
func (r *repository) FetchRecord(fieldValues map[string]any, target any) error {
	if fieldValues == nil {
		return ErrInvalidParameters
	}
	qry := r.SqlSelect()
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return fetchOne(r.ctx, r.conn, qry, target)
}

// FetchByKey fetch a single record with WHERE keyField=value
func (r *repository) FetchByKey(keyField string, value any, target any) error {
	if target == nil {
		return ErrInvalidParameters
	}
	qry := r.SqlSelect().Where(goqu.C(keyField).Eq(value))
	return fetchOne(r.ctx, r.conn, qry, target)
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
	return fetch(r.ctx, r.conn, qry, target)
}

func fetch(ctx context.Context, conn *sqlx.DB, qry *goqu.SelectDataset, target any) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return conn.SelectContext(ctx, target, sqlQry, args...)
}

func (r *repository) Exec(qry *goqu.SelectDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(r.ctx, sqlQry, args...)
	return err
}

func (r *repository) RawExec(sql string, args ...any) error {
	_, err := r.conn.ExecContext(r.ctx, sql, args...)
	return err
}

func (r *repository) Delete(qry *goqu.DeleteDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(r.ctx, sqlQry, args...)
	return err
}

func (r *repository) DeleteWhere(fieldNameValue map[string]any) error {
	if fieldNameValue == nil {
		return ErrInvalidParameters
	}
	ds := r.SqlDelete()
	for field, value := range fieldNameValue {
		ds = ds.Where(goqu.C(field).Eq(value))
	}
	sqlQry, args, err := ds.ToSQL()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(r.ctx, sqlQry, args...)
	return err
}

func (r *repository) DeleteByKey(keyField string, value any) error {
	ds := r.SqlDelete().Where(goqu.C(keyField).Eq(value))
	sqlQry, args, err := ds.ToSQL()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(r.ctx, sqlQry, args...)
	return err
}

func (r *repository) Select(sql string, target any, args ...any) error {
	return r.conn.SelectContext(r.ctx, target, sql, args...)
}

func (r *repository) Insert(rows ...any) error {
	if len(rows) == 0 {
		return ErrInvalidParameters
	}
	sqlQry, args, err := r.SqlInsert().Rows(rows...).Prepared(true).ToSQL()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(r.ctx, sqlQry, args...)
	return err
}

// InsertReturning inserts a record, and returns the specified return fields into target
//
// Example:
//
//	record := &SomeRecord{}
//	err := InsertReturning(record, []any{"id_table"}, &record.Id)
func (r *repository) InsertReturning(record any, returnFields []interface{}, target ...any) error {
	if record == nil || returnFields == nil || len(target) == 0 {
		return ErrInvalidParameters
	}
	qry, values, err := r.SqlInsert().Rows(record).Prepared(true).Returning(returnFields...).ToSQL()
	if err != nil {
		return err
	}
	return r.conn.QueryRowxContext(r.ctx, qry, values...).Scan(target...)
}

// Update execute an update query
func (r *repository) Update(qry *goqu.UpdateDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	qrySql, values, err := qry.Prepared(true).ToSQL()
	if err != nil {
		return err
	}
	_, err = r.conn.ExecContext(r.ctx, qrySql, values...)
	return err
}

// UpdateRecord updates a record using a WHERE condition
// record can be a map[string]any specifying fields & values
func (r *repository) UpdateRecord(record any, whereFieldsValues map[string]any) error {
	if record == nil {
		return ErrInvalidParameters
	}
	qry := r.SqlUpdate().Set(record)
	if whereFieldsValues != nil {
		for field, value := range whereFieldsValues {
			qry = qry.Where(goqu.C(field).Eq(value))
		}
	}
	return r.Update(qry)
}

// UpdateByKey updates a record using WHERE keyField=value
func (r *repository) UpdateByKey(record any, keyField string, value any) error {
	if record == nil {
		return ErrInvalidParameters
	}
	qry := r.SqlUpdate().Set(record).Where(goqu.C(keyField).Eq(value))
	return r.Update(qry)
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
	if target == nil || qry == nil {
		return ErrInvalidParameters
	}
	return txFetchOne(t.ctx, t.conn, qry, target)
}

func txFetchOne(ctx context.Context, conn *sqlx.Tx, qry *goqu.SelectDataset, target any) error {
	qry.Limit(1)
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return conn.QueryRowxContext(ctx, sqlQry, args...).StructScan(target)
}

func txFetch(ctx context.Context, conn *sqlx.Tx, qry *goqu.SelectDataset, target any) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	return conn.SelectContext(ctx, target, sqlQry, args...)
}

func (t *tx) Fetch(qry *goqu.SelectDataset, target any) error {
	if target == nil {
		return ErrInvalidParameters
	}
	return txFetch(t.ctx, t.conn, qry, target)
}

// FetchRecord fetch a single record with WHERE clause
func (t *tx) FetchRecord(fieldValues map[string]any, target any) error {
	if fieldValues == nil {
		return ErrInvalidParameters
	}
	qry := t.SqlSelect()
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return txFetchOne(t.ctx, t.conn, qry, target)
}

// FetchByKey fetch a single record with WHERE keyField=value
func (t *tx) FetchByKey(keyField string, value any, target any) error {
	if target == nil {
		return ErrInvalidParameters
	}
	qry := t.SqlSelect().Where(goqu.C(keyField).Eq(value))
	return txFetchOne(t.ctx, t.conn, qry, target)
}

// FetchWhere fetch multiple records with WHERE clause
func (t *tx) FetchWhere(fieldValues map[string]any, target any) error {
	if fieldValues == nil {
		return ErrInvalidParameters
	}
	qry := t.SqlSelect()
	for field, value := range fieldValues {
		qry = qry.Where(goqu.C(field).Eq(value))
	}
	return txFetch(t.ctx, t.conn, qry, target)
}

func (t *tx) Exec(qry *goqu.SelectDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	_, err = t.conn.ExecContext(t.ctx, sqlQry, args...)
	return err
}

func (t *tx) RawExec(sql string, args ...any) error {
	_, err := t.conn.ExecContext(t.ctx, sql, args...)
	return err
}

func (t *tx) Delete(qry *goqu.DeleteDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	sqlQry, args, err := qry.ToSQL()
	if err != nil {
		return err
	}
	_, err = t.conn.ExecContext(t.ctx, sqlQry, args...)
	return err
}

func (t *tx) DeleteWhere(fieldNameValue map[string]any) error {
	if fieldNameValue == nil {
		return ErrInvalidParameters
	}
	ds := t.SqlDelete()
	for field, value := range fieldNameValue {
		ds = ds.Where(goqu.C(field).Eq(value))
	}
	sqlQry, args, err := ds.ToSQL()
	if err != nil {
		return err
	}
	_, err = t.conn.ExecContext(t.ctx, sqlQry, args...)
	return err
}

func (t *tx) DeleteByKey(keyField string, value any) error {
	ds := t.SqlDelete().Where(goqu.C(keyField).Eq(value))
	sqlQry, args, err := ds.ToSQL()
	if err != nil {
		return err
	}
	_, err = t.conn.ExecContext(t.ctx, sqlQry, args...)
	return err
}

func (t *tx) Select(sql string, target any, args ...any) error {
	return t.conn.SelectContext(t.ctx, target, sql, args...)
}

func (t *tx) Insert(rows ...any) error {
	if len(rows) == 0 {
		return ErrInvalidParameters
	}
	sqlQry, args, err := t.SqlInsert().Rows(rows...).Prepared(true).ToSQL()
	if err != nil {
		return err
	}
	_, err = t.conn.ExecContext(t.ctx, sqlQry, args...)
	return err
}

// InsertReturning inserts a record, and returns the specified return fields into target
func (t *tx) InsertReturning(record any, returnFields []interface{}, target ...any) error {
	if record == nil || returnFields == nil || len(target) == 0 {
		return ErrInvalidParameters
	}
	qry, values, err := t.SqlInsert().Rows(record).Prepared(true).Returning(returnFields...).ToSQL()
	if err != nil {
		return err
	}
	return t.conn.QueryRowxContext(t.ctx, qry, values...).Scan(target...)
}

// Update execute an update query
func (t *tx) Update(qry *goqu.UpdateDataset) error {
	if qry == nil {
		return ErrInvalidParameters
	}
	qrySql, values, err := qry.Prepared(true).ToSQL()
	if err != nil {
		return err
	}
	_, err = t.conn.ExecContext(t.ctx, qrySql, values...)
	return err
}

// UpdateRecord updates a record using a WHERE condition
// record can be a map[string]any specifying fields & values
func (t *tx) UpdateRecord(record any, whereFieldsValues map[string]any) error {
	if record == nil {
		return ErrInvalidParameters
	}
	qry := t.SqlUpdate().Set(record)
	if whereFieldsValues != nil {
		for field, value := range whereFieldsValues {
			qry = qry.Where(goqu.C(field).Eq(value))
		}
	}
	return t.Update(qry)
}

// UpdateByKey updates a record using WHERE keyField=value
func (t *tx) UpdateByKey(record any, keyField string, value any) error {
	if record == nil {
		return ErrInvalidParameters
	}
	qry := t.SqlUpdate().Set(record).Where(goqu.C(keyField).Eq(value))
	return t.Update(qry)
}

// EmptyResult returns true if error is empty result
func EmptyResult(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}
